package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// batchStatusMaxItems caps one GET /api/jobs/batch-status call; Studio keeps
// well under it because only reels still moving are reconciled.
const batchStatusMaxItems = 100

// Stable per-item read-failure codes. They name the half that could not be
// read and nothing else: the storage or SQL detail behind the failure is logged
// at the boundary, exactly as internalError does, and never sent to a client.
const (
	batchStatusCodeJobUnreadable    = "job_status_unreadable"
	batchStatusCodeRenderUnreadable = "render_state_unreadable"
)

// batchStatusItemError is one row the server could not read. It is a field of
// its own because neither nil half can carry that meaning: `job: null` means
// the orchestrator no longer knows the job, which latches the reel as gone
// after repeated ticks, and `render: null` means "no render yet", which lets
// Studio drive the next step - for a recorded job, an unrequested capture. A
// read failure is neither, so it travels as `error` and both halves stay null
// beside it.
type batchStatusItemError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// batchStatusItem is one reel's reconciliation input: the job's status view and
// its render-variant state, both nil-able so a client can tell "unknown job"
// (job nil) and "no render yet" (render nil) apart without a 404, plus the
// explicit error that marks a row neither half describes.
type batchStatusItem struct {
	JobID   uuid.UUID              `json:"job_id"`
	Variant string                 `json:"variant"`
	Job     *jobStatusResponse     `json:"job"`
	Render  *renderVariantResponse `json:"render"`
	Error   *batchStatusItemError  `json:"error,omitempty"`
}

// statusBatchReader is the optional bulk projection a job repository can offer:
// one query for every id instead of one query per id. The SQLite and memory
// repositories implement it; a repository without it falls back to GetStatus
// per id, which is what this endpoint did before.
type statusBatchReader interface {
	GetStatuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]job.StatusRow, error)
}

// BatchStatus handles GET /api/jobs/batch-status?items=<job>:<variant>,...
// It folds the per-reel status + render polls Studio used to issue into one
// request and one status query. The job half matches GET ?view=status; the
// render half is omitted until the job can have render state so leftover
// documents during recapture stay hidden the same way the client's
// canHaveRenderState guard hides them.
//
// Only a malformed request fails the call. A row the server cannot read is
// reported in that row's `error` and the rest of the batch is still served,
// because collapsing the response sends the client back to the per-reel polls
// this endpoint exists to replace.
func (h *Handlers) BatchStatus(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("items"))
	if raw == "" {
		writeJSON(w, http.StatusOK, map[string]any{"items": []batchStatusItem{}})
		return
	}
	pairs := strings.Split(raw, ",")
	if len(pairs) > batchStatusMaxItems {
		writeError(w, http.StatusBadRequest, "too many items; at most 100 per request")
		return
	}
	// Validate the whole request before reading anything: a malformed pair is
	// the caller's bug and still fails the call, and one pass leaves every id
	// known up front so the status read is a single query.
	items := make([]batchStatusItem, 0, len(pairs))
	ids := make([]uuid.UUID, 0, len(pairs))
	for _, pair := range pairs {
		jobPart, variant, ok := strings.Cut(pair, ":")
		if !ok {
			writeError(w, http.StatusBadRequest, "each item must be <job_id>:<variant>")
			return
		}
		id, err := uuid.Parse(jobPart)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid job id")
			return
		}
		if _, err := renderplan.LoadoutForVariant(variant); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		items = append(items, batchStatusItem{JobID: id, Variant: variant})
		ids = append(ids, id)
	}

	ctx := r.Context()
	rows, failures := h.batchStatusRows(ctx, ids)
	for i := range items {
		// The client hung up or navigated away: the remaining render-state
		// reads would only fill a response nothing will read.
		if ctx.Err() != nil {
			return
		}
		item := &items[i]
		if failures[item.JobID] != nil {
			item.Error = &batchStatusItemError{
				Code:    batchStatusCodeJobUnreadable,
				Message: "job status could not be read",
			}
			continue
		}
		row, found := rows[item.JobID]
		if !found {
			// Unknown job: the job half stays null, the same answer a single
			// ?view=status gives with a 404.
			continue
		}
		status := h.jobStatusViewOf(item.JobID, row)
		if row.Status.CanHaveRenderState() {
			view, exists, err := h.batchRenderVariantView(item.JobID, item.Variant)
			if err != nil {
				logBoundaryError("batch status: read render state", err)
				item.Error = &batchStatusItemError{
					Code:    batchStatusCodeRenderUnreadable,
					Message: "render state could not be read",
				}
				// Both halves stay null: a job view beside a null render would
				// read as "no render yet" and re-drive the reel.
				continue
			}
			if exists {
				item.Render = &view
			}
		}
		item.Job = &status
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// batchStatusRows reads every id's lifecycle projection, in one query when the
// repository offers the bulk read and one query per id otherwise. An id whose
// job does not exist is absent from both maps, exactly the "not found" a single
// GetStatus reports; an id whose read failed is in failures, so the caller
// degrades that item instead of the whole response. Every failure is logged
// once here, at the same boundary internalError logs.
func (h *Handlers) batchStatusRows(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]job.StatusRow, map[uuid.UUID]error) {
	if bulk, ok := h.repo.(statusBatchReader); ok {
		rows, err := bulk.GetStatuses(ctx, ids)
		if err == nil {
			return rows, nil
		}
		// One failed query is one failure: logged once, charged to every id it
		// covered so each of those rows degrades on its own.
		logBoundaryError("batch status: get job statuses", err)
		failures := make(map[uuid.UUID]error, len(ids))
		for _, id := range ids {
			failures[id] = err
		}
		return nil, failures
	}
	rows := make(map[uuid.UUID]job.StatusRow, len(ids))
	var failures map[uuid.UUID]error
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		if _, done := rows[id]; done {
			continue
		}
		if _, done := failures[id]; done {
			continue
		}
		status, failureReason, segmentCount, err := h.repo.GetStatus(ctx, id)
		if errors.Is(err, job.ErrNotFound) {
			continue
		}
		if err != nil {
			logBoundaryError("batch status: get job status", err)
			if failures == nil {
				failures = make(map[uuid.UUID]error, 1)
			}
			failures[id] = err
			continue
		}
		rows[id] = job.StatusRow{Status: status, FailureReason: failureReason, SegmentCount: segmentCount}
	}
	return rows, failures
}

// jobStatusViewOf builds the ?view=status document from a row a bulk read
// already returned. It is jobStatusView without the repository read, and the
// two must stay the same document.
func (h *Handlers) jobStatusViewOf(id uuid.UUID, row job.StatusRow) jobStatusResponse {
	resp := jobStatusResponse{
		Status:        row.Status,
		FailureReason: row.FailureReason,
		FailureCode:   jobFailureCode(row.FailureReason, ""),
	}
	if row.Status == job.StatusRecording {
		if progress, ok := captureProgressWithTotal(h.storage, id, row.Status, row.SegmentCount); ok {
			resp.Progress = &progress
		}
	} else if progress, ok := renderProgressDocument(h.storage, id); ok {
		resp.Progress = &progress
	}
	return resp
}

// batchRenderVariantView is the render half of one row: the same document
// GET /renders/{variant} serves, with exists false when the variant has no
// durable state yet.
func (h *Handlers) batchRenderVariantView(id uuid.UUID, variant string) (renderVariantResponse, bool, error) {
	state, exists, err := h.readOrMaterializeRenderVariantState(id, variant)
	if err != nil {
		return renderVariantResponse{}, false, err
	}
	if !exists {
		return renderVariantResponse{}, false, nil
	}
	view, err := h.renderVariantView(state)
	if err != nil {
		return renderVariantResponse{}, false, err
	}
	return view, true, nil
}

// logBoundaryError records an internal failure the way internalError does, for
// a path that degrades one item instead of failing the whole response. Driver,
// SQL and storage internals stay in the log and out of the body.
func logBoundaryError(op string, err error) {
	log.Printf("httpapi: %s: %v", op, err)
}
