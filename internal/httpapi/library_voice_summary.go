package httpapi

import "net/http"

// ListLibraryVoiceSummaries handles GET /api/library/voice/summary: each match with voice and its sync state, no tracks.
func (h *Handlers) ListLibraryVoiceSummaries(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	voices, err := h.libraryRepo.ListVoiceSummaries(r.Context())
	if err != nil {
		internalError(w, "list voice summaries", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"voices": voices})
}
