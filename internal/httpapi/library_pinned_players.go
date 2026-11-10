package httpapi

import (
	"net/http"

	"github.com/rechedev9/cliphub/internal/library"
)

type pinnedPlayersBody struct {
	Players []library.PinnedPlayer `json:"players"`
}

// GetLibraryPinnedPlayers handles GET /api/library/pinned-players: the players the Ranking keeps on top.
func (h *Handlers) GetLibraryPinnedPlayers(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	players, err := h.libraryRepo.ListPinnedPlayers(r.Context())
	if err != nil {
		internalError(w, "list pinned players", err)
		return
	}
	writeJSON(w, http.StatusOK, pinnedPlayersBody{Players: players})
}

// PutLibraryPinnedPlayers handles PUT /api/library/pinned-players: it replaces the whole list, in table order.
func (h *Handlers) PutLibraryPinnedPlayers(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var req pinnedPlayersBody
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid pinned players JSON")
		return
	}
	players, err := library.CleanPinnedPlayers(req.Players)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.libraryRepo.SetPinnedPlayers(r.Context(), players); err != nil {
		internalError(w, "set pinned players", err)
		return
	}
	writeJSON(w, http.StatusOK, pinnedPlayersBody{Players: players})
}
