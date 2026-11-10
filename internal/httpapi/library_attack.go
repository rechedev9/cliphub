package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// attackResponse is how a team attacks one map, with the matches skipped.
type attackResponse struct {
	TeamID   uuid.UUID      `json:"team_id"`
	TeamName string         `json:"team_name"`
	Skipped  []scoutSkipped `json:"skipped"`
	tacticalplan.AttackPrep
}

// GetLibraryTeamAttack handles GET /api/library/teams/{id}/attack with the scout
// filter minus side, since it reads the T side only; map is required.
func (h *Handlers) GetLibraryTeamAttack(w http.ResponseWriter, r *http.Request) {
	q, ok := h.framedRequest(w, r)
	if !ok {
		return
	}
	if q.filter.Side != "" {
		writeError(w, http.StatusBadRequest, "the attack prep reads the T side; drop side")
		return
	}
	loaded, skipped, err := h.loadFramedMatches(r.Context(), q)
	if err != nil {
		internalError(w, "team attack", err)
		return
	}
	metrics := make([]tacticalplan.MetricsMatch, 0, len(loaded))
	for _, m := range loaded {
		metrics = append(metrics, tacticalplan.MetricsMatch{MatchID: m.MatchID, Document: m.Document, TeamKey: m.TeamKey})
	}
	attack := tacticalplan.BuildAttack(loaded, tacticalplan.AttackOptions{
		Filter: q.filter, Roles: inferRoles(teamSteamIDs(metrics), roleDocuments(metrics)), Now: time.Now(),
	})
	beyond := 0
	for _, s := range skipped {
		if s.Reason == scoutSkipBeyondLimits {
			beyond++
		}
	}
	if beyond > 0 {
		attack.Warnings = append(attack.Warnings, tacticalplan.PrepWarning{Kind: tacticalplan.WarningCapped, Matches: len(loaded), Available: len(loaded) + beyond})
	}
	writeJSON(w, http.StatusOK, attackResponse{TeamID: q.team.ID, TeamName: q.team.Name, Skipped: skipped, AttackPrep: attack})
}
