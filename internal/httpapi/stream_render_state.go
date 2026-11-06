package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

// streamRenderStateBody is a rendering state with its live encode progress.
type streamRenderStateBody struct {
	streamclips.RenderState
	RenderProgress *streamclips.RenderProgress `json:"render_progress,omitempty"`
}

func (h *Handlers) GetStreamRender(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	key, err := streamclips.RenderStateKey(j.ID, variant)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rc, err := h.storage.Open(key)
	if err == nil {
		defer rc.Close()
		progress := h.liveStreamRenderProgress(j, variant)
		if progress == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, rc)
			return
		}
		var state streamclips.RenderState
		if err := json.NewDecoder(rc).Decode(&state); err != nil {
			internalError(w, "decode stream render state", err)
			return
		}
		if state.Status != streamclips.StatusRendering {
			progress = nil
		}
		writeJSON(w, http.StatusOK, streamRenderStateBody{RenderState: state, RenderProgress: progress})
		return
	}
	if !storageNotExist(err) {
		internalError(w, "open stream render state", err)
		return
	}
	state, err := streamclips.NewRenderState(j.ID, variant, j.Status, nil, j.FailureReason, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// liveStreamRenderProgress is the progress of the job's running render, or nil.
// An empty variant accepts the render of any variant.
func (h *Handlers) liveStreamRenderProgress(j streamclips.Job, variant string) *streamclips.RenderProgress {
	if j.Status != streamclips.StatusRendering {
		return nil
	}
	p, ok := h.streamRenderBoard.Get(j.ID)
	if !ok || (variant != "" && p.Variant != variant) {
		return nil
	}
	return &p
}
