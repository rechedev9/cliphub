package httpapi

import (
	"errors"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/importedvideos"
)

// ImportedVideos is the store behind /api/imported-videos; *importedvideos.Store satisfies it.
type ImportedVideos interface {
	List() []importedvideos.Video
	Open(cloudJobID, name string) (*os.File, error)
	Delete(cloudJobID, name string) error
}

// WithImportedVideos enables /api/imported-videos. Without it the list is empty.
func WithImportedVideos(store ImportedVideos) Option {
	return func(h *Handlers) {
		h.importedVideos = store
	}
}

// ListImportedVideos handles GET /api/imported-videos.
func (h *Handlers) ListImportedVideos(w http.ResponseWriter, _ *http.Request) {
	videos := []importedvideos.Video{}
	if h.importedVideos != nil {
		videos = h.importedVideos.List()
	}
	writeJSON(w, http.StatusOK, map[string]any{"videos": videos})
}

// GetImportedVideo handles GET /api/imported-videos/{id}/{name}.
func (h *Handlers) GetImportedVideo(w http.ResponseWriter, r *http.Request) {
	if h.importedVideos == nil {
		writeError(w, http.StatusNotFound, "video not found")
		return
	}
	file, err := h.importedVideos.Open(chi.URLParam(r, "id"), chi.URLParam(r, "name"))
	if err != nil {
		writeError(w, http.StatusNotFound, "video not found")
		return
	}
	serveArtifact(w, r, "video/mp4", file)
}

// DeleteImportedVideo handles DELETE /api/imported-videos/{id}/{name}.
func (h *Handlers) DeleteImportedVideo(w http.ResponseWriter, r *http.Request) {
	if h.importedVideos == nil {
		writeError(w, http.StatusNotFound, "video not found")
		return
	}
	err := h.importedVideos.Delete(chi.URLParam(r, "id"), chi.URLParam(r, "name"))
	switch {
	case errors.Is(err, importedvideos.ErrNotFound):
		writeError(w, http.StatusNotFound, "video not found")
	case err != nil:
		internalError(w, "delete imported video", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
