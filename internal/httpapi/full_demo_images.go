package httpapi

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/overlayassets"
	"github.com/rechedev9/cliphub/internal/storage"
)

func (h *Handlers) CreateFullDemoImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, overlayassets.MaxBytes+(1<<20))
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	// #nosec G120 -- MaxBytesReader bounds both memory and temporary files.
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Selecciona una imagen PNG o JPG de hasta 10 MB")
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Falta la imagen")
		return
	}
	defer file.Close()
	asset, err := overlayassets.Store(r.Context(), h.storage, file, header.Filename)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

func (h *Handlers) GetFullDemoImage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil || id == uuid.Nil {
		writeError(w, http.StatusBadRequest, "Referencia de imagen inválida")
		return
	}
	asset, err := overlayassets.Load(h.storage, id)
	if err != nil {
		if storage.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "Imagen no encontrada")
		} else {
			writeError(w, http.StatusInternalServerError, "No se pudo leer la imagen")
		}
		return
	}
	file, err := h.storage.Open(overlayassets.MediaKey(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "Imagen no encontrada")
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = io.Copy(w, io.LimitReader(file, overlayassets.MaxBytes))
}
