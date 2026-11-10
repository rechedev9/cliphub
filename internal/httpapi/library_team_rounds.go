package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// teamRoundOut is one round the team scope keeps, read from the team's side.
type teamRoundOut struct {
	Round       int                  `json:"round"`
	Side        tacticalplan.Side    `json:"side"`
	Winner      tacticalplan.Side    `json:"winner"`
	Buy         tacticalplan.BuyType `json:"buy"`
	OpponentBuy tacticalplan.BuyType `json:"opponent_buy"`
	Site        tacticalplan.Site    `json:"site"`
	// Pattern is the round shape of the team's side, such as execute or hold.
	Pattern string `json:"pattern,omitempty"`
}

// teamRoundsMatch is one match of the scope with the rounds it keeps, in round order.
type teamRoundsMatch struct {
	MatchID  uuid.UUID      `json:"match_id"`
	Map      string         `json:"map"`
	PlayedAt time.Time      `json:"played_at"`
	TeamKey  string         `json:"team_key"`
	Opponent string         `json:"opponent,omitempty"`
	Rounds   []teamRoundOut `json:"rounds"`
	Total    int            `json:"total"`
}

// teamRoundsResponse is GET /api/library/teams/{id}/rounds: the rounds the team
// scope keeps, newest match first, the same ones every team figure counts.
type teamRoundsResponse struct {
	TeamID   uuid.UUID         `json:"team_id"`
	TeamName string            `json:"team_name"`
	Matches  []teamRoundsMatch `json:"matches"`
	Rounds   int               `json:"rounds"`
	Skipped  []scoutSkipped    `json:"skipped"`
	// AWPEstimated counts the matches whose AWP filter read kills and hits (older analyses).
	AWPEstimated int `json:"awp_estimated,omitempty"`
}

// GetLibraryTeamRounds handles GET /api/library/teams/{id}/rounds.
func (h *Handlers) GetLibraryTeamRounds(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	filter, matchIDs, err := h.teamScope(r.Context(), r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	candidates, pending, skipped, err := h.teamCandidates(r.Context(), team, matchIDs)
	if err != nil {
		internalError(w, "read team matches", err)
		return
	}
	selected, unrated := selectTeamMatches(filter, pending)
	kept, more := h.keepTeamMatches(keepQuery{teamID: team.ID, candidates: candidates, selected: selected, lineup: filter.Lineup, limit: filter.MaxMatches})
	resp := teamRoundsResponse{TeamID: team.ID, TeamName: team.Name, Matches: []teamRoundsMatch{}, Skipped: append(append(skipped, unrated...), more...)}
	for _, k := range kept {
		resp.Matches = append(resp.Matches, teamRoundsOf(filter, k.scout))
		resp.Rounds += len(resp.Matches[len(resp.Matches)-1].Rounds)
		resp.AWPEstimated += awpEstimated(filter, k.scout)
	}
	writeJSON(w, http.StatusOK, resp)
}

func teamRoundsOf(f tacticalplan.ScoutFilter, sm tacticalplan.ScoutMatch) teamRoundsMatch {
	doc := sm.Document
	out := teamRoundsMatch{MatchID: sm.MatchID, Map: doc.Demo.Map, PlayedAt: sm.PlayedAt, TeamKey: sm.TeamKey, Opponent: sm.Opponent, Rounds: []teamRoundOut{}}
	for _, r := range doc.Rounds {
		if doc.TeamSide(sm.TeamKey, r) != "" {
			out.Total++
		}
	}
	for _, r := range f.RoundFilter(sm).Apply(doc) {
		side := doc.TeamSide(sm.TeamKey, r)
		pattern := string(r.Class.TSide)
		if side == tacticalplan.SideCT {
			pattern = string(r.Class.CTSide)
		}
		out.Rounds = append(out.Rounds, teamRoundOut{
			Round: r.Number, Side: side, Winner: r.Winner, Buy: r.Economy.Buy(side), OpponentBuy: r.Economy.Buy(side.Opponent()), Site: r.Class.Site, Pattern: pattern,
		})
	}
	return out
}
