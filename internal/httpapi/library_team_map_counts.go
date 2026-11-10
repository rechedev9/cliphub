package httpapi

import (
	"context"
	"net/http"
	"sort"

	"github.com/google/uuid"

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

// teamMapCounts counts, map by map, the matches the scope keeps with the
// pipeline the views use (period, rival, rival level, whose, lineup and the
// "last N" cap per map). Map and side do not narrow it, and round criteria
// count rounds, not matches.
func (h *Handlers) teamMapCounts(ctx context.Context, team library.Team, filter tacticalplan.ScoutFilter) (teamMapCountsResponse, error) {
	resp := teamMapCountsResponse{TeamID: team.ID, TeamName: team.Name, Maps: []teamMapCount{}}
	candidates, pending, _, err := h.teamCandidates(ctx, team, nil)
	if err != nil {
		return resp, err
	}
	counts := map[string]int{}
	for _, m := range pending {
		counts[m.Document.Demo.Map] = 0
	}
	selected, _ := selectTeamMatches(filter, pending)
	byMap := map[string][]tacticalplan.ScoutMatch{}
	for _, m := range selected {
		byMap[m.Document.Demo.Map] = append(byMap[m.Document.Demo.Map], m)
	}
	for name, matches := range byMap {
		kept, _ := h.keepTeamMatches(keepQuery{
			teamID: team.ID, candidates: candidates, selected: matches, lineup: filter.Lineup, limit: filter.MaxMatches,
		})
		counts[name] = len(kept)
	}
	for name, n := range counts {
		resp.Maps = append(resp.Maps, teamMapCount{Map: name, Matches: n})
	}
	sort.Slice(resp.Maps, func(i, j int) bool { return resp.Maps[i].Map < resp.Maps[j].Map })
	return resp, nil
}

// GetLibraryTeamMapCounts handles GET /api/library/teams/{id}/map-counts: the
// matches per map under the team scope the bar sends, for the map selector.
func (h *Handlers) GetLibraryTeamMapCounts(w http.ResponseWriter, r *http.Request) {
	team, ok := h.loadLibraryTeam(w, r)
	if !ok {
		return
	}
	filter, _, err := scoutQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter.Maps = nil
	resp, err := h.teamMapCounts(r.Context(), team, filter)
	if err != nil {
		internalError(w, "team map counts", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
