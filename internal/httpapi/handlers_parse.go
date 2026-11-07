package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// startParseRequest is the JSON body for POST /api/jobs/{id}/parse.
type startParseRequest struct {
	TargetSteamID string `json:"target_steamid"`
	// TargetSteamIDs picks several teammates for one multi-POV job.
	TargetSteamIDs []string     `json:"target_steamids,omitempty"`
	Rules          *rules.Rules `json:"rules,omitempty"`
}

// StartParse handles POST /api/jobs/{id}/parse. After a roster scan it records
// the picked target (and optional rules) and enqueues the full parse.
func (h *Handlers) StartParse(w http.ResponseWriter, r *http.Request) {
	// Metadata only: parse reads the status and the stored rules, never the
	// kill plan a previous parse left behind.
	j, ok := h.loadJobMeta(w, r)
	if !ok {
		return
	}
	// Friendly early-out with the current status. The race-safe guard is the
	// status-gated SetParseInputs below, which atomically claims the job, so a
	// second concurrent request that slips past this check still conflicts there.
	if j.Status != job.StatusScanned && j.Status != job.StatusParsed {
		writeError(w, http.StatusConflict, fmt.Sprintf("job is not ready to parse (status=%s)", j.Status))
		return
	}

	var req startParseRequest
	if err := decodeSingleJSONBody(w, r, &req, false); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, "parse request JSON is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid parse request JSON")
		return
	}
	inputs, err := h.parseTargets(j, req)
	if err != nil {
		writeMultiPOVError(w, err)
		return
	}

	effectiveRules := j.Rules
	if req.Rules != nil {
		effectiveRules = *req.Rules
		if err := effectiveRules.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid rules: "+err.Error())
			return
		}
	}

	inputs.Rules = effectiveRules
	if err := h.repo.SetParseInputs(r.Context(), j.ID, inputs); err != nil {
		switch {
		case errors.Is(err, job.ErrNotFound):
			writeError(w, http.StatusNotFound, "job not found")
		case errors.Is(err, job.ErrConflict):
			writeError(w, http.StatusConflict, "job is no longer ready to parse")
		default:
			internalError(w, "set parse inputs", err)
		}
		return
	}

	task, err := tasks.NewParseDemoTask(j.ID)
	if err != nil {
		internalError(w, "build parse task", err)
		return
	}
	if _, err := h.queue.EnqueueWithTransition(task, func(decision error) error {
		return h.persistJobQueueDecision(j.ID, "parse", decision)
	}); err != nil {
		internalError(w, "enqueue parse task", err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":     j.ID,
		"status": job.StatusParsing,
	})
}
