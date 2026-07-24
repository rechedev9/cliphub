package httpapi

import (
	"bytes"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/fragforge/internal/anticheat"
	"github.com/rechedev9/fragforge/internal/artifacts"
	"github.com/rechedev9/fragforge/internal/job"
	"github.com/rechedev9/fragforge/internal/storage"
	"github.com/rechedev9/fragforge/internal/tasks"
)

// StartAnticheat handles POST /api/jobs/{id}/anticheat: it queues the
// CheaterDetect screening pass over the job's demo.
//
// The screening is a side lane on an already uploaded demo, so it only needs
// the demo to exist — it does not wait for, or interfere with, the clip
// pipeline's own status. Re-posting while a screening is in flight is rejected
// so a demo is never parsed twice for the same answer.
func (h *Handlers) StartAnticheat(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	if j.Status == job.StatusQueued || j.Status == job.StatusScanning {
		writeError(w, http.StatusConflict, "demo is still being ingested; retry once the roster scan finishes")
		return
	}

	if doc, err := h.readAnticheatDocument(j.ID); err == nil && doc.Status == anticheat.StatusRunning {
		writeJSON(w, http.StatusAccepted, map[string]any{"id": j.ID, "status": doc.Status})
		return
	}

	// Claim the lane before enqueueing so a poll issued right after this
	// response already sees "running" instead of a stale ready document.
	doc := anticheat.NewRunningDocument(j.ID.String(), time.Now())
	if err := h.putAnticheatDocument(j.ID, doc); err != nil {
		internalError(w, "store anticheat document", err)
		return
	}

	task, err := tasks.NewAnalyzeAnticheatTask(j.ID)
	if err != nil {
		internalError(w, "build anticheat task", err)
		return
	}
	if _, err := h.queue.Enqueue(task); err != nil {
		// The lane was claimed above, so a rejected enqueue must release it or
		// the job would poll a screening that nothing owns.
		if putErr := h.putAnticheatDocument(j.ID, doc.Fail("no se pudo encolar el análisis: "+err.Error(), time.Now())); putErr != nil {
			internalError(w, "release anticheat document", putErr)
			return
		}
		internalError(w, "enqueue anticheat task", err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{"id": j.ID, "status": anticheat.StatusRunning})
}

// GetAnticheat handles GET /api/jobs/{id}/anticheat and returns the stored
// screening document, whatever state it is in. A job that was never screened
// answers 409 so the UI can tell "not started" from "running".
func (h *Handlers) GetAnticheat(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	doc, err := h.readAnticheatDocument(j.ID)
	if err != nil {
		if storage.IsNotExist(err) {
			writeError(w, http.StatusConflict, "anticheat analysis not started")
			return
		}
		internalError(w, "open anticheat document", err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// GetAnticheatDossier handles GET /api/jobs/{id}/anticheat/dossier/{steamid}.
// It renders the evidence pack for one screened player: the material a user
// needs to file their own report, never a submission of one.
func (h *Handlers) GetAnticheatDossier(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	doc, err := h.readAnticheatDocument(j.ID)
	if err != nil {
		if storage.IsNotExist(err) {
			writeError(w, http.StatusConflict, "anticheat analysis not started")
			return
		}
		internalError(w, "open anticheat document", err)
		return
	}
	if doc.Status != anticheat.StatusReady || doc.Report == nil {
		writeError(w, http.StatusConflict, "anticheat analysis is not ready")
		return
	}

	steamID := chi.URLParam(r, "steamid")
	player, found := doc.Report.Player(steamID)
	if !found {
		writeError(w, http.StatusNotFound, "player not present in the anticheat analysis")
		return
	}
	writeJSON(w, http.StatusOK, anticheat.BuildDossier(*doc.Report, player))
}

func (h *Handlers) readAnticheatDocument(id uuid.UUID) (anticheat.Document, error) {
	rc, err := h.storage.Open(artifacts.AnticheatKey(id))
	if err != nil {
		return anticheat.Document{}, err
	}
	defer rc.Close()
	return anticheat.DecodeDocument(rc)
}

func (h *Handlers) putAnticheatDocument(id uuid.UUID, doc anticheat.Document) error {
	var buf bytes.Buffer
	if err := doc.Encode(&buf); err != nil {
		return err
	}
	return h.storage.Put(artifacts.AnticheatKey(id), bytes.NewReader(buf.Bytes()))
}
