package httpapi

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/renderplan"
)

// batchStatusMaxItems caps one GET /api/jobs/batch-status call; Studio keeps
// well under it because only reels still moving are reconciled.
const batchStatusMaxItems = 100

// batchStatusItem is one reel's reconciliation input: the job's status view
// and its render-variant state, both nil-able so a client can tell "unknown
// job" (job nil) and "no render yet" (render nil) apart without a 404.
type batchStatusItem struct {
	JobID   uuid.UUID              `json:"job_id"`
	Variant string                 `json:"variant"`
	Job     *jobStatusResponse     `json:"job"`
	Render  *renderVariantResponse `json:"render"`
}

// BatchStatus handles GET /api/jobs/batch-status?items=<job>:<variant>,...
// It folds the per-reel status + render polls Studio used to issue into one
// request. The job half matches GET ?view=status; the render half is omitted
// until the job can have render state so leftover documents during recapture
// stay hidden the same way the client's canHaveRenderState guard hides them.
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
	items := make([]batchStatusItem, 0, len(pairs))
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
		item := batchStatusItem{JobID: id, Variant: variant}
		status, found, err := h.jobStatusView(r.Context(), id)
		if err != nil {
			internalError(w, "get job status", err)
			return
		}
		if found {
			item.Job = &status
			if status.Status.CanHaveRenderState() {
				state, exists, err := h.readOrMaterializeRenderVariantState(id, variant)
				if err != nil {
					internalError(w, "read render state", err)
					return
				}
				if exists {
					view, err := h.renderVariantView(state)
					if err != nil {
						internalError(w, "read render state", err)
						return
					}
					item.Render = &view
				}
			}
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
