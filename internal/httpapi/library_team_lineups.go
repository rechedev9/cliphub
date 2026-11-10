package httpapi

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// teamLineupMatch is the match a team's latest lineup is read from.
type teamLineupMatch struct {
	MatchID  uuid.UUID `json:"match_id"`
	Map      string    `json:"map"`
	PlayedAt time.Time `json:"played_at"`
}

// teamLineupsResponse is a team's roster as the lineup picker groups it: who
// played its latest match, then everyone else who played for it, with dates.
// LatestMatch is nil, and Latest empty, when that match names none of the team's players.
type teamLineupsResponse struct {
	TeamID      uuid.UUID             `json:"team_id"`
	TeamName    string                `json:"team_name"`
	LatestMatch *teamLineupMatch      `json:"latest_match"`
	Latest      []library.RosterEntry `json:"latest"`
	Others      []library.RosterEntry `json:"others"`
}

// lineupSplit is a team's roster and the side (teamKey) it played in its latest match.
type lineupSplit struct {
	roster  []library.RosterEntry
	latest  library.Match
	teamKey string
}

// splitLineups puts the roster players of the team's side of latest first, by
// nick, and keeps the others in roster order (most recent first).
func splitLineups(s lineupSplit) (inLatest, others []library.RosterEntry) {
	played := map[string]bool{}
	for _, p := range s.latest.Players {
		if p.TeamKey == s.teamKey {
			played[p.SteamID64] = true
		}
	}
	inLatest, others = []library.RosterEntry{}, []library.RosterEntry{}
	for _, e := range s.roster {
		if played[e.SteamID64] {
			inLatest = append(inLatest, e)
		} else {
			others = append(others, e)
		}
	}
	sort.SliceStable(inLatest, func(i, j int) bool {
		return strings.ToLower(inLatest[i].Nick) < strings.ToLower(inLatest[j].Nick)
	})
	return inLatest, others
}

// GetLibraryTeamLineups handles GET /api/library/teams/{id}/lineups: the
// players a team view's lineup picks from, grouped by the team's latest match.
func (h *Handlers) GetLibraryTeamLineups(w http.ResponseWriter, r *http.Request) {
	team, ok := h.loadLibraryTeam(w, r)
	if !ok {
		return
	}
	resp := teamLineupsResponse{TeamID: team.ID, TeamName: team.Name, Latest: []library.RosterEntry{}, Others: team.Roster}
	if resp.Others == nil {
		resp.Others = []library.RosterEntry{}
	}
	recent, err := h.libraryRepo.ListMatches(r.Context(), library.MatchFilter{TeamID: &team.ID, Limit: 1})
	if err != nil {
		internalError(w, "list team matches", err)
		return
	}
	if len(recent) == 0 {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	latest, err := h.libraryRepo.GetMatch(r.Context(), recent[0].ID)
	if err != nil {
		writeLibraryError(w, "get match", err)
		return
	}
	c, ok := candidateFor(latest, team.ID, nil)
	if !ok {
		// No side of the latest match is the team's, so there is no latest lineup to offer.
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.Latest, resp.Others = splitLineups(lineupSplit{roster: team.Roster, latest: latest, teamKey: c.scout.TeamKey})
	if len(resp.Latest) > 0 {
		resp.LatestMatch = &teamLineupMatch{MatchID: latest.ID, Map: latest.Map, PlayedAt: latest.PlayedAt}
	}
	writeJSON(w, http.StatusOK, resp)
}
