package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/overlayassets"
)

func (h *Handlers) CreateFullDemoOverlayAsset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, overlayassets.MaxBytes+1<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "La captura supera los 10 MB o la carga no es válida")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Selecciona una captura PNG o JPG")
		return
	}
	defer file.Close()
	a, err := overlayassets.Store(r.Context(), h.storage, file, header.Filename)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (h *Handlers) GetFullDemoOverlayAsset(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "assetID")
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		writeError(w, http.StatusBadRequest, "invalid screenshot id")
		return
	}
	a, err := overlayassets.Load(h.storage, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Captura no disponible")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	h.streamStorageKey(w, r, a.ContentType, overlayassets.MediaKey(id))
}
