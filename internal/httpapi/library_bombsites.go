package httpapi

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// bombsitesResponse is a team's record on each bombsite with the matches it skipped.
type bombsitesResponse struct {
	TeamID   uuid.UUID      `json:"team_id"`
	TeamName string         `json:"team_name"`
	Skipped  []scoutSkipped `json:"skipped"`
	tacticalplan.BombsiteReport
}

// GetLibraryTeamBombsites handles GET /api/library/teams/{id}/bombsites: per
// map and site, how the team defends and attacks it, with the scout filter.
func (h *Handlers) GetLibraryTeamBombsites(w http.ResponseWriter, r *http.Request) {
	team, ok := h.loadLibraryTeam(w, r)
	if !ok {
		return
	}
	filter, matchIDs, err := h.teamScope(r.Context(), r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	loaded, skipped, err := h.loadTeamMatches(r.Context(), team, matchIDs, filter)
	if err != nil {
		internalError(w, "team bombsites", err)
		return
	}
	writeJSON(w, http.StatusOK, bombsitesResponse{
		TeamID: team.ID, TeamName: team.Name, Skipped: skipped,
		BombsiteReport: tacticalplan.Bombsites(loaded, filter),
	})
}
