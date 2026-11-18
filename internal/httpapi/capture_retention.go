package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/captureretention"
)

// captureSettings: keepAll is ZV_KEEP_FULL_DEMO_CAPTURES, which wins over the
// retention set in Almacenamiento.
type captureSettings struct {
	keepAll bool
}

var errNoLocalDisk = errors.New("storage backend has no local disk")

// WithKeepFullDemoCaptures keeps every Full Demo capture, for `zv full-demo lab-bundle`.
func WithKeepFullDemoCaptures(keep bool) Option {
	return func(h *Handlers) { h.captures.keepAll = keep }
}

type captureRetention struct {
	Keep     int  `json:"keep"`
	Max      int  `json:"max"`
	KeepsAll bool `json:"keeps_all,omitempty"`
}

func (h *Handlers) captureRetention() (captureRetention, error) {
	keep, err := captureretention.Load(h.storage)
	return captureRetention{Keep: keep, Max: captureretention.MaxKeep, KeepsAll: h.captures.keepAll}, err
}

// PutCaptureRetention handles PUT /api/storage/capture-retention: how many
// rendered Full Demos keep their captures. It frees the ones past it now.
func (h *Handlers) PutCaptureRetention(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Keep *int `json:"keep"`
	}
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || req.Keep == nil || *req.Keep < 0 || *req.Keep > captureretention.MaxKeep {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("keep must be a whole number from 0 to %d", captureretention.MaxKeep))
		return
	}
	if err := captureretention.Save(h.storage, *req.Keep); err != nil {
		internalError(w, "save capture retention", err)
		return
	}
	var freed int64
	if !h.captures.keepAll {
		var err error
		busy := func(id uuid.UUID) bool { return h.capturesBusy(r.Context(), id) }
		freed, err = captureretention.Apply(captureretention.ApplyOptions{Store: h.storage, Keep: *req.Keep, Busy: busy})
		if err != nil {
			log.Printf("httpapi: apply capture retention: %v", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"keep": *req.Keep, "freed_bytes": freed})
}

// capturesBusy keeps the clips of a job whose capture, render or generate run may still read them.
func (h *Handlers) capturesBusy(ctx context.Context, id uuid.UUID) bool {
	j, err := h.repo.GetMeta(ctx, id)
	return err != nil || jobIsInFlight(j.Status) || h.requireGenerateRenderIdle(id) != nil
}
