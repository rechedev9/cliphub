package httpapi

import (
	"context"
	"net/http"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// teamMapCount is how many matches of a team a view would read on one map.
type teamMapCount struct {
	Map     string `json:"map"`
	Matches int    `json:"matches"`
}

// teamMapCountsResponse lists every map the team played, with the matches in
// scope on each; a map the scope leaves empty comes back with 0.
type teamMapCountsResponse struct {
	TeamID   uuid.UUID      `json:"team_id"`
	TeamName string         `json:"team_name"`
	Maps     []teamMapCount `json:"maps"`
}

type teamMapCountsQuery struct {
	team   library.Team
	filter tacticalplan.ScoutFilter
}

// teamMapCounts counts per map the matches keepTeamMatches would read; map,
// side and round criteria do not narrow it.
func (h *Handlers) teamMapCounts(ctx context.Context, q teamMapCountsQuery) (teamMapCountsResponse, error) {
	resp := teamMapCountsResponse{TeamID: q.team.ID, TeamName: q.team.Name, Maps: []teamMapCount{}}
	every := q.filter
	every.Maps = nil
	candidates, pending, _, err := h.teamCandidates(ctx, candidateQuery{team: q.team, scope: every})
	if err != nil {
		return resp, err
	}
	counts := map[string]int{}
	for _, m := range pending {
		counts[m.Document.Demo.Map] = 0
	}
	selected, _ := selectTeamMatches(q.filter, pending)
	reads := maxTeamDocReads
	byMap := map[string][]tacticalplan.ScoutMatch{}
	for _, m := range selected {
		byMap[m.Document.Demo.Map] = append(byMap[m.Document.Demo.Map], m)
	}
	for name, matches := range byMap {
		keep := keepQuery{teamID: q.team.ID, candidates: candidates, selected: matches, lineup: q.filter.Lineup, limit: q.filter.MaxMatches, reads: &reads}
		if len(q.filter.Lineup) == 0 {
			counts[name] = h.countAnalysed(keep)
			continue
		}
		kept, _ := h.keepTeamMatches(keep)
		counts[name] = len(kept)
	}
	for name, n := range counts {
		resp.Maps = append(resp.Maps, teamMapCount{Map: name, Matches: n})
	}
	sort.Slice(resp.Maps, func(i, j int) bool { return resp.Maps[i].Map < resp.Maps[j].Map })
	return resp, nil
}

// countAnalysed is keepTeamMatches for a scope without a lineup: every analysed
// match stays, so a stat of the document replaces decoding it.
func (h *Handlers) countAnalysed(q keepQuery) int {
	n := 0
	for _, sm := range q.selected {
		if (q.limit > 0 && n >= q.limit) || n >= library.MaxReportMatches {
			break
		}
		if ok, err := h.storage.Exists(artifacts.LibraryTacticalIndexKey(q.candidates[sm.MatchID].match.DemoID)); err == nil && ok {
			n++
		}
	}
	return n
}

// GetLibraryTeamMapCounts handles GET /api/library/teams/{id}/map-counts: the
// matches per map under the team scope the bar sends, for the map selector.
func (h *Handlers) GetLibraryTeamMapCounts(w http.ResponseWriter, r *http.Request) {
	team, ok := h.loadLibraryTeam(w, r)
	if !ok {
		return
	}
	filter, _, err := h.teamScope(r.Context(), r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter.Maps = nil
	resp, err := h.teamMapCounts(r.Context(), teamMapCountsQuery{team: team, filter: filter})
	if err != nil {
		internalError(w, "team map counts", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
