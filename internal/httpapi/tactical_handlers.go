package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tactical"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tasks"
)

const tacticalUniqueTTL = 24 * time.Hour

// positionsHeaderBytes is the blob header a single-round read stitches in front
// of the round's own bytes, because the frame decoder refuses an offset that
// would start inside the header. The stored format is checked against
// tacticalplan.PositionsFormat first, so a future layout is rejected rather
// than misread.
const positionsHeaderBytes = tacticalplan.PositionsHeaderSize

// mapNamePattern bounds a caller-supplied map name to the vocabulary CS2 uses
// for its own map files, before it is looked up or used to build anything.
var mapNamePattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// startTacticalRequest is the optional JSON body for POST /api/jobs/{id}/tactical.
type startTacticalRequest struct {
	SampleHZ float64 `json:"sample_hz"`
}

// tacticalRoundResponse is one round of the index together with the position
// frames decoded for that round alone.
type tacticalRoundResponse struct {
	Round  tacticalplan.Round   `json:"round"`
	Frames []tacticalplan.Frame `json:"frames"`
}

// StartTacticalAnalysis handles POST /api/jobs/{id}/tactical. The scan is
// deterministic and idempotent, so unlike recording the task keeps the default
// retry policy; it also never touches cs2.exe, so it runs on the default queue
// lane instead of the serial capture lane.
func (h *Handlers) StartTacticalAnalysis(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	if j.DemoPath == "" {
		writeError(w, http.StatusConflict, fmt.Sprintf("job has no demo to analyze (status=%s)", j.Status))
		return
	}
	sampleHZ, ok := decodeTacticalSampleHZ(w, r)
	if !ok {
		return
	}
	task, err := tasks.NewAnalyzeTacticalTask(j.ID, sampleHZ)
	if err != nil {
		internalError(w, "build tactical task", err)
		return
	}
	status := artifacts.TacticalStatus{
		State:         artifacts.TacticalStateQueued,
		GeneratedAt:   time.Now().UTC(),
		SchemaVersion: tacticalplan.SchemaVersion,
		SampleHZ:      sampleHZ,
	}
	_, err = h.queue.EnqueueWithTransition(task, func(decision error) error {
		switch {
		case decision == nil:
			return h.writeTacticalStatus(j.ID, status)
		case errors.Is(decision, asynq.ErrDuplicateTask):
			existing, exists, readErr := h.readTacticalStatus(j.ID)
			if readErr != nil {
				return readErr
			}
			if exists {
				status = existing
			}
			return nil
		default:
			status.State = artifacts.TacticalStateFailed
			status.Error = "enqueue tactical analysis: " + decision.Error()
			status.GeneratedAt = time.Now().UTC()
			return h.writeTacticalStatus(j.ID, status)
		}
	}, asynq.Unique(tacticalUniqueTTL))
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
		internalError(w, "enqueue tactical analysis", err)
		return
	}
	writeJSON(w, http.StatusAccepted, status)
}

// GetTacticalDocument handles GET /api/jobs/{id}/tactical, streaming the stored
// document as written by the worker.
func (h *Handlers) GetTacticalDocument(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	rc, err := h.storage.Open(artifacts.TacticalIndexKey(j.ID))
	if err != nil {
		if storage.IsNotExist(err) {
			writeError(w, http.StatusConflict, "tactical analysis is not ready")
			return
		}
		internalError(w, "open tactical document", err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

// GetTacticalStatus handles GET /api/jobs/{id}/tactical/status. A job whose
// analysis was never requested reports the "none" state rather than 404, so a
// poller has one shape to read.
func (h *Handlers) GetTacticalStatus(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	status, exists, err := h.readTacticalStatus(j.ID)
	if err != nil {
		internalError(w, "read tactical status", err)
		return
	}
	if !exists {
		status = artifacts.TacticalStatus{
			State:         artifacts.TacticalStateNone,
			GeneratedAt:   time.Now().UTC(),
			SchemaVersion: tacticalplan.SchemaVersion,
			SampleHZ:      tactical.DefaultSampleHZ,
		}
	}
	writeJSON(w, http.StatusOK, status)
}

// GetTacticalRound handles GET /api/jobs/{id}/tactical/rounds/{round}. It
// decodes only the requested round's frames out of the position blob, using the
// byte range the document records for it.
func (h *Handlers) GetTacticalRound(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	raw := chi.URLParam(r, "round")
	number, err := strconv.Atoi(raw)
	if err != nil || number < 1 {
		writeError(w, http.StatusBadRequest, "round must be a positive integer")
		return
	}
	doc, exists, err := h.readTacticalDocument(j.ID)
	if err != nil {
		internalError(w, "read tactical document", err)
		return
	}
	if !exists {
		writeError(w, http.StatusConflict, "tactical analysis is not ready")
		return
	}
	round, ok := doc.RoundByNumber(number)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("round %d is not in the tactical document", number))
		return
	}
	frames, err := h.readTacticalRoundFrames(j.ID, doc.Positions, number)
	if err != nil {
		internalError(w, "read tactical round frames", err)
		return
	}
	writeJSON(w, http.StatusOK, tacticalRoundResponse{Round: round, Frames: frames})
}

// GetTacticalPositions handles GET /api/jobs/{id}/tactical/positions. The blob
// is served straight from the storage-resolved artifact: local storage hands
// out a seekable file, so serveArtifact answers Range requests through
// http.ServeContent and the orchestrator never buffers the whole stream.
func (h *Handlers) GetTacticalPositions(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	rc, err := h.storage.Open(artifacts.TacticalPositionsKey(j.ID))
	if err != nil {
		if storage.IsNotExist(err) {
			writeError(w, http.StatusConflict, "tactical analysis is not ready")
			return
		}
		internalError(w, "open tactical positions", err)
		return
	}
	serveArtifact(w, r, "application/octet-stream", rc)
}

// GetTacticalAggregate handles GET /api/jobs/{id}/tactical/aggregate, computing
// tendencies over the rounds the query filter selects.
func (h *Handlers) GetTacticalAggregate(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	filter, err := tacticalplan.FilterFromValues(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	doc, exists, err := h.readTacticalDocument(j.ID)
	if err != nil {
		internalError(w, "read tactical document", err)
		return
	}
	if !exists {
		writeError(w, http.StatusConflict, "tactical analysis is not ready")
		return
	}
	writeJSON(w, http.StatusOK, tacticalplan.Aggregate(doc, filter))
}

// GetMapRadar handles GET /api/maps/{map}/radar. An uncalibrated map is a 404
// rather than an identity transform, which would silently misplace every
// position a client draws.
func (h *Handlers) GetMapRadar(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "map")
	if !mapNamePattern.MatchString(name) {
		writeError(w, http.StatusBadRequest, "map must match ^[a-z0-9_]+$")
		return
	}
	calibration, ok := radarmap.Lookup(name)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("map %q has no radar calibration", name))
		return
	}
	writeJSON(w, http.StatusOK, calibration)
}

// decodeTacticalSampleHZ reads the optional sampling rate from the request.
// A job owns one canonical tactical artifact set, so the API accepts only the
// canonical sampling rate. Ad-hoc rates remain available through the CLI,
// where the caller also owns a distinct output path.
func decodeTacticalSampleHZ(w http.ResponseWriter, r *http.Request) (float64, bool) {
	if r.Body == nil {
		return tactical.DefaultSampleHZ, true
	}
	var req startTacticalRequest
	switch err := decodeSingleJSONBody(w, r, &req, false); {
	case err == nil, errors.Is(err, io.EOF):
	default:
		writeError(w, http.StatusBadRequest, "invalid tactical request JSON")
		return 0, false
	}
	if req.SampleHZ < 0 || req.SampleHZ > tactical.MaxSampleHZ {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("sample_hz must be between 0 and %d", tactical.MaxSampleHZ))
		return 0, false
	}
	if req.SampleHZ == 0 {
		return tactical.DefaultSampleHZ, true
	}
	if req.SampleHZ != tactical.DefaultSampleHZ {
		writeError(w, http.StatusConflict, fmt.Sprintf(
			"job tactical analysis uses the canonical sample_hz %.0f; use the CLI with a distinct output for custom rates",
			float64(tactical.DefaultSampleHZ),
		))
		return 0, false
	}
	return tactical.DefaultSampleHZ, true
}

func (h *Handlers) readTacticalDocument(id uuid.UUID) (tacticalplan.Document, bool, error) {
	rc, err := h.storage.Open(artifacts.TacticalIndexKey(id))
	if err != nil {
		if storage.IsNotExist(err) {
			return tacticalplan.Document{}, false, nil
		}
		return tacticalplan.Document{}, false, err
	}
	defer rc.Close()
	var doc tacticalplan.Document
	if err := json.NewDecoder(rc).Decode(&doc); err != nil {
		return tacticalplan.Document{}, false, err
	}
	return doc, true, nil
}

func (h *Handlers) readTacticalStatus(id uuid.UUID) (artifacts.TacticalStatus, bool, error) {
	rc, err := h.storage.Open(artifacts.TacticalStatusKey(id))
	if err != nil {
		if storage.IsNotExist(err) {
			return artifacts.TacticalStatus{}, false, nil
		}
		return artifacts.TacticalStatus{}, false, err
	}
	defer rc.Close()
	var status artifacts.TacticalStatus
	if err := json.NewDecoder(rc).Decode(&status); err != nil {
		return artifacts.TacticalStatus{}, false, err
	}
	// Status documents written before sample_hz was added decode the missing
	// field as zero. Zero is not a valid persisted rate under the canonical job
	// contract, so expose the default without rewriting a healthy artifact.
	if status.SampleHZ == 0 {
		status.SampleHZ = tactical.DefaultSampleHZ
	}
	return status, true, nil
}

func (h *Handlers) writeTacticalStatus(id uuid.UUID, status artifacts.TacticalStatus) error {
	b, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return h.storage.Put(artifacts.TacticalStatusKey(id), bytes.NewReader(append(b, '\n')))
}

// readTacticalRoundFrames decodes the frames of one round out of the position
// blob without reading the rounds before it. A seekable artifact (the local
// filesystem backend) is read as the blob header plus that round's byte range;
// a backend that cannot seek falls back to reading the blob and decoding at the
// recorded offset.
func (h *Handlers) readTacticalRoundFrames(id uuid.UUID, desc tacticalplan.Positions, round int) ([]tacticalplan.Frame, error) {
	offset, ok := desc.Offset(round)
	if !ok {
		// A round with no sampled positions is not an error: the index is still
		// complete, there is simply nothing to draw.
		return []tacticalplan.Frame{}, nil
	}
	rc, err := h.storage.Open(artifacts.TacticalPositionsKey(id))
	if err != nil {
		return nil, fmt.Errorf("open tactical positions: %w", err)
	}
	defer rc.Close()

	rs, ok := rc.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("read tactical positions: %w", err)
		}
		header, err := tacticalplan.DecodeHeader(data)
		if err != nil {
			return nil, err
		}
		return tacticalplan.DecodeFrames(data, offset.ByteOffset, offset.FrameCount, header)
	}
	if desc.Format != tacticalplan.PositionsFormat {
		return nil, fmt.Errorf("tactical positions format %q is not %q", desc.Format, tacticalplan.PositionsFormat)
	}

	// The decoder addresses frames by their offset inside a blob, so the buffer
	// is the real header followed by exactly this round's bytes.
	buf := make([]byte, positionsHeaderBytes+offset.ByteLength)
	if _, err := io.ReadFull(rs, buf[:positionsHeaderBytes]); err != nil {
		return nil, fmt.Errorf("read tactical positions header: %w", err)
	}
	header, err := tacticalplan.DecodeHeader(buf[:positionsHeaderBytes])
	if err != nil {
		return nil, err
	}
	if _, err := rs.Seek(offset.ByteOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek tactical round %d: %w", round, err)
	}
	if _, err := io.ReadFull(rs, buf[positionsHeaderBytes:]); err != nil {
		return nil, fmt.Errorf("read tactical round %d: %w", round, err)
	}
	return tacticalplan.DecodeFrames(buf, positionsHeaderBytes, offset.FrameCount, header)
}
