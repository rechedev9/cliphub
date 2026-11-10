package httpapi

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// firstDeathResponse is where a team's players stand on one side until the round's first death.
type firstDeathResponse struct {
	TeamID   uuid.UUID      `json:"team_id"`
	TeamName string         `json:"team_name"`
	Skipped  []scoutSkipped `json:"skipped"`
	tacticalplan.FirstDeathPositions
}

// firstDeathWindow reads t_from and t_to in whole seconds after freeze end; t_to 0 runs to the round end.
func firstDeathWindow(q url.Values) (tacticalplan.PrepWindow, error) {
	from, err := secondsQuery(q, "t_from")
	if err != nil {
		return tacticalplan.PrepWindow{}, err
	}
	to, err := secondsQuery(q, "t_to")
	if err != nil {
		return tacticalplan.PrepWindow{}, err
	}
	if to > 0 && to <= from {
		return tacticalplan.PrepWindow{}, errors.New("t_to must be after t_from")
	}
	return tacticalplan.PrepWindow{FromSeconds: int(from), ToSeconds: int(to)}, nil
}

// GetLibraryTeamFirstDeath handles GET /api/library/teams/{id}/first-death with
// the scout filter, t_from, t_to and movement; map and side are required.
func (h *Handlers) GetLibraryTeamFirstDeath(w http.ResponseWriter, r *http.Request) {
	q, ok := h.framedRequest(w, r)
	if !ok {
		return
	}
	window, err := firstDeathWindow(r.URL.Query())
	var movement tacticalplan.Movement
	if err == nil {
		movement, err = tacticalplan.ParseMovement(r.URL.Query().Get("movement"))
	}
	if err == nil && q.filter.Side == "" {
		err = errors.New("side is required")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	kept, skipped, err := h.keepFramedMatches(r.Context(), q)
	if err != nil {
		internalError(w, "team first death positions", err)
		return
	}
	loaded := h.framedMatches(kept)
	metrics := make([]tacticalplan.MetricsMatch, 0, len(loaded))
	for _, m := range loaded {
		metrics = append(metrics, tacticalplan.MetricsMatch{MatchID: m.MatchID, Document: m.Document, TeamKey: m.TeamKey})
	}
	positions := tacticalplan.BuildFirstDeathPositions(loaded, tacticalplan.FirstDeathOptions{
		Filter: q.filter, Window: window, Movement: movement, Roles: inferRoles(teamSteamIDs(metrics), roleDocuments(metrics)),
		MapRoles: h.teamMapRoles(kept),
	})
	writeJSON(w, http.StatusOK, firstDeathResponse{TeamID: q.team.ID, TeamName: q.team.Name, Skipped: skipped, FirstDeathPositions: positions})
}
