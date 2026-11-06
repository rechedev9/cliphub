package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

func (h *Handlers) GetStreamEditPlan(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	if len(j.EditPlan) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(j.EditPlan)
		return
	}
	key := streamclips.EditPlanKey(j.ID)
	rc, err := h.storage.Open(key)
	if err == nil {
		defer rc.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, rc)
		return
	}
	if !storageNotExist(err) {
		internalError(w, "open stream edit plan", err)
		return
	}
	writeJSON(w, http.StatusOK, streamclips.DefaultEditPlan())
}

// currentStreamEditPlan returns the job's edit plan the same way
// GetStreamEditPlan does (the job row, else the storage artifact, else the
// default plan), so StartStreamRender validates the same plan the user last
// saved without duplicating the fallback chain.
func (h *Handlers) currentStreamEditPlan(j streamclips.Job) (streamclips.EditPlan, error) {
	plan, saved, err := h.savedStreamEditPlan(j)
	if err != nil || saved {
		return plan, err
	}
	return streamclips.DefaultEditPlan(), nil
}

// savedStreamEditPlan reports false when no plan was ever saved for the job.
func (h *Handlers) savedStreamEditPlan(j streamclips.Job) (streamclips.EditPlan, bool, error) {
	var plan streamclips.EditPlan
	if len(j.EditPlan) > 0 {
		if err := json.Unmarshal(j.EditPlan, &plan); err != nil {
			return streamclips.EditPlan{}, false, err
		}
		return plan, true, nil
	}
	rc, err := h.storage.Open(streamclips.EditPlanKey(j.ID))
	if err == nil {
		defer rc.Close()
		if err := json.NewDecoder(rc).Decode(&plan); err != nil {
			return streamclips.EditPlan{}, false, err
		}
		return plan, true, nil
	}
	if !storageNotExist(err) {
		return streamclips.EditPlan{}, false, err
	}
	return streamclips.EditPlan{}, false, nil
}

// PutStreamEditPlan saves the plan. An expected_updated_at query value makes
// the save conditional: it fails with 409 when another save landed after it.
func (h *Handlers) PutStreamEditPlan(w http.ResponseWriter, r *http.Request) {
	var expected time.Time
	if raw := r.URL.Query().Get("expected_updated_at"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "expected_updated_at must be an RFC3339 timestamp")
			return
		}
		expected = parsed
	}
	// Serialize every edit-plan mutation with render admission so none can
	// validate plan A and overwrite a newer plan B.
	releaseJob := h.lockStreamJobRequest(r)
	defer releaseJob()
	h.streamPlanMu.Lock()
	defer h.streamPlanMu.Unlock()
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	if j.Status == streamclips.StatusRendering {
		writeError(w, http.StatusConflict, "stream edit plan cannot change while a render is running")
		return
	}
	if !expected.IsZero() {
		current, saved, err := h.savedStreamEditPlan(j)
		if err != nil {
			internalError(w, "load stream edit plan", err)
			return
		}
		// A never-saved plan has no revision to race with.
		if saved && !current.UpdatedAt.Equal(expected) {
			writeError(w, http.StatusConflict, "stream edit plan changed since it was read; read the latest plan and apply the change again")
			return
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, "edit plan JSON is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid edit plan JSON")
		return
	}
	plan, err := streamclips.DecodeEditPlan(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid edit plan JSON: "+err.Error())
		return
	}
	plan = streamclips.NormalizeEditPlan(plan)
	plan.UpdatedAt = time.Now().UTC()
	if err := plan.ValidateForSourceDuration(j.Probe.DurationSeconds); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.streamRepo.SetEditPlan(r.Context(), j.ID, plan); err != nil {
		if errors.Is(err, streamclips.ErrNotFound) {
			writeError(w, http.StatusNotFound, "stream job not found")
			return
		}
		internalError(w, "save stream edit plan", err)
		return
	}
	if err := h.writeStreamEditPlanArtifact(j.ID, plan); err != nil {
		internalError(w, "write stream edit plan artifact", err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handlers) writeStreamEditPlanArtifact(id uuid.UUID, plan streamclips.EditPlan) error {
	b, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return h.storage.Put(streamclips.EditPlanKey(id), bytes.NewReader(append(b, '\n')))
}
