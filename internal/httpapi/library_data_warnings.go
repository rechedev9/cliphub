package httpapi

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// dataWarningsResponse is the line on top of Preparar: what to distrust in the scope's sample.
type dataWarningsResponse struct {
	TeamID   uuid.UUID                  `json:"team_id"`
	TeamName string                     `json:"team_name"`
	Warnings []tacticalplan.DataWarning `json:"warnings"`
}

// GetLibraryTeamWarnings handles GET /api/library/teams/{id}/warnings with the
// scout filter; map and side are optional, as on Preparar's bar.
func (h *Handlers) GetLibraryTeamWarnings(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	filter, matchIDs, err := h.teamScope(r.Context(), scopeRequest{teamID: id, q: r.URL.Query()})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	warnings, err := h.teamDataWarnings(r.Context(), framedQuery{team: team, matchIDs: matchIDs, filter: filter})
	if err != nil {
		internalError(w, "team data warnings", err)
		return
	}
	writeJSON(w, http.StatusOK, dataWarningsResponse{TeamID: team.ID, TeamName: team.Name, Warnings: warnings})
}

// teamDataWarnings loads the scope's matches once without the lineup filter:
// they are what the lineup is compared against, and the lineup's own matches are the sample.
func (h *Handlers) teamDataWarnings(ctx context.Context, q framedQuery) ([]tacticalplan.DataWarning, error) {
	// The newest analysed match is read whatever the map, dates and lineup, so none goes to SQL.
	every := q.filter
	every.Maps, every.From, every.To, every.Lineup = nil, nil, nil, nil
	candidates, pending, _, err := h.teamCandidates(ctx, candidateQuery{team: q.team, matchIDs: q.matchIDs, scope: every})
	if err != nil {
		return nil, err
	}
	selected, _ := selectTeamMatches(q.filter, pending)
	all, _ := h.keepTeamMatches(keepQuery{teamID: q.team.ID, candidates: candidates, selected: selected})
	limit := q.filter.MaxMatches
	compared := make([]tacticalplan.ScoutMatch, 0, len(all))
	kept := make([]tacticalplan.ScoutMatch, 0, len(all))
	read := map[uuid.UUID]bool{}
	for _, k := range all {
		read[k.scout.MatchID] = true
		if limit <= 0 || len(compared) < limit {
			compared = append(compared, k.scout)
		}
		if (limit <= 0 || len(kept) < limit) && q.filter.Lineup.PlayedIn(k.scout.Document, k.scout.TeamKey) {
			kept = append(kept, k.scout)
		}
	}
	return tacticalplan.DataWarnings(tacticalplan.DataWarningInput{
		Newest: h.newestAnalysed(candidates, read), Compared: compared, Kept: kept, Filter: q.filter, Now: time.Now(),
	}), nil
}

// newestAnalysed is when the team's newest match with a readable analysis was played,
// whatever the filters; read marks the matches already known to have one.
func (h *Handlers) newestAnalysed(candidates map[uuid.UUID]scoutCandidate, read map[uuid.UUID]bool) time.Time {
	sorted := make([]library.Match, 0, len(candidates))
	for _, c := range candidates {
		sorted = append(sorted, c.match)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].PlayedAt.After(sorted[j].PlayedAt) })
	for _, m := range sorted {
		if read[m.ID] {
			return m.PlayedAt
		}
		if _, err := h.readLibraryDocument(m.DemoID); err == nil {
			return m.PlayedAt
		}
	}
	return time.Time{}
}
