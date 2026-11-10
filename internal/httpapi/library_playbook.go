package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// playbookResponse is a team's playbook on one map and side, with the matches skipped.
type playbookResponse struct {
	TeamID   uuid.UUID      `json:"team_id"`
	TeamName string         `json:"team_name"`
	Skipped  []scoutSkipped `json:"skipped"`
	tacticalplan.Playbook
}

// prepResponse is the match prep against a team on one map, with the matches skipped.
type prepResponse struct {
	TeamID   uuid.UUID      `json:"team_id"`
	TeamName string         `json:"team_name"`
	Skipped  []scoutSkipped `json:"skipped"`
	tacticalplan.Prep
}

// framedQuery names the team, the explicit matches and the scout filter of a
// view that reads positions.
type framedQuery struct {
	team     library.Team
	matchIDs []uuid.UUID
	filter   tacticalplan.ScoutFilter
}

// loadFramedMatches reads the team's matches the filter selects, with their
// documents; each position blob is read only when the builder reaches it.
func (h *Handlers) loadFramedMatches(ctx context.Context, q framedQuery) ([]tacticalplan.FramedMatch, []scoutSkipped, error) {
	kept, skipped, err := h.keepFramedMatches(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	return h.framedMatches(kept), skipped, nil
}

// keepFramedMatches reads the team's matches the filter selects, with their documents.
func (h *Handlers) keepFramedMatches(ctx context.Context, q framedQuery) ([]keptMatch, []scoutSkipped, error) {
	candidates, pending, skipped, err := h.teamCandidates(ctx, q.team, q.matchIDs)
	if err != nil {
		return nil, nil, err
	}
	selected, unrated := selectTeamMatches(q.filter, pending)
	kept, more := h.keepTeamMatches(keepQuery{teamID: q.team.ID, candidates: candidates, selected: selected, lineup: q.filter.Lineup, limit: q.filter.MaxMatches})
	return kept, append(append(skipped, unrated...), more...), nil
}

func (h *Handlers) framedMatches(kept []keptMatch) []tacticalplan.FramedMatch {
	loaded := make([]tacticalplan.FramedMatch, 0, len(kept))
	for _, k := range kept {
		loaded = append(loaded, tacticalplan.FramedMatch{ScoutMatch: k.scout, Blob: h.positionsReader(k.demoID)})
	}
	return loaded
}

// framedRequest reads the team and the scout filter of a playbook or a prep.
// Both read one map, since zones and callouts belong to a map.
func (h *Handlers) framedRequest(w http.ResponseWriter, r *http.Request) (framedQuery, bool) {
	if !h.libraryReady(w) {
		return framedQuery{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return framedQuery{}, false
	}
	filter, matchIDs, err := h.teamScope(r.Context(), r.URL.Query())
	if err == nil && len(filter.Maps) != 1 {
		err = errors.New("map is required, exactly once")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return framedQuery{}, false
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return framedQuery{}, false
	}
	return framedQuery{team: team, matchIDs: matchIDs, filter: filter}, true
}

// playbookCombo reads the with and without tactic keys of a playbook request.
func playbookCombo(r *http.Request) (tacticalplan.PlaybookCombo, error) {
	combo := tacticalplan.PlaybookCombo{With: r.URL.Query()["with"], Without: r.URL.Query()["without"]}
	if err := tacticalplan.CheckTacticKeys("with", combo.With); err != nil {
		return combo, err
	}
	return combo, tacticalplan.CheckTacticKeys("without", combo.Without)
}

// GetLibraryTeamPlaybook handles GET /api/library/teams/{id}/playbook with the
// scout filter and the tactic combination (with, without); map and side are required.
func (h *Handlers) GetLibraryTeamPlaybook(w http.ResponseWriter, r *http.Request) {
	q, ok := h.framedRequest(w, r)
	if !ok {
		return
	}
	if q.filter.Side == "" {
		writeError(w, http.StatusBadRequest, "side is required")
		return
	}
	combo, err := playbookCombo(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	loaded, skipped, err := h.loadFramedMatches(r.Context(), q)
	if err != nil {
		internalError(w, "team playbook", err)
		return
	}
	writeJSON(w, http.StatusOK, playbookResponse{
		TeamID: q.team.ID, TeamName: q.team.Name, Skipped: skipped, Playbook: tacticalplan.BuildPlaybook(loaded, tacticalplan.PlaybookOptions{Filter: q.filter, Combo: combo}),
	})
}

// GetLibraryTeamPrep handles GET /api/library/teams/{id}/prep with the scout
// filter minus side, since it reads both; map is required.
func (h *Handlers) GetLibraryTeamPrep(w http.ResponseWriter, r *http.Request) {
	q, ok := h.framedRequest(w, r)
	if !ok {
		return
	}
	if q.filter.Side != "" {
		writeError(w, http.StatusBadRequest, "the prep reads both sides; drop side")
		return
	}
	kept, skipped, err := h.keepFramedMatches(r.Context(), q)
	if err != nil {
		internalError(w, "team prep", err)
		return
	}
	loaded := h.framedMatches(kept)
	metrics := make([]tacticalplan.MetricsMatch, 0, len(loaded))
	for _, m := range loaded {
		metrics = append(metrics, tacticalplan.MetricsMatch{MatchID: m.MatchID, Document: m.Document, TeamKey: m.TeamKey})
	}
	prep := tacticalplan.BuildPrep(loaded, tacticalplan.PrepOptions{
		Filter: q.filter, Roles: inferRoles(teamSteamIDs(metrics), roleDocuments(metrics)), MapRoles: h.teamMapRoles(kept),
	})
	writeJSON(w, http.StatusOK, prepResponse{TeamID: q.team.ID, TeamName: q.team.Name, Skipped: skipped, Prep: prep})
}
