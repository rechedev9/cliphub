package tuiclient

import (
	"context"
	"net/http"
	"time"
)

// TeamRound is one round a team scope keeps, from the team's side.
type TeamRound struct {
	Round       int    `json:"round"`
	Side        string `json:"side"`
	Winner      string `json:"winner"`
	Buy         string `json:"buy"`
	OpponentBuy string `json:"opponent_buy"`
	Site        string `json:"site"`
}

// TeamRoundsMatch is one match of a team scope with the rounds it keeps.
type TeamRoundsMatch struct {
	MatchID  string      `json:"match_id"`
	Map      string      `json:"map"`
	PlayedAt time.Time   `json:"played_at"`
	Opponent string      `json:"opponent"`
	Rounds   []TeamRound `json:"rounds"`
	Total    int         `json:"total"`
}

// TeamRounds is GET /api/library/teams/{id}/rounds.
type TeamRounds struct {
	TeamName string            `json:"team_name"`
	Matches  []TeamRoundsMatch `json:"matches"`
	Rounds   int               `json:"rounds"`
	Skipped  []AnalysisSkipped `json:"skipped"`
}

// TeamRounds reads the rounds a team scope keeps over its matches, newest match first.
func (c *Client) TeamRounds(ctx context.Context, teamID string, q AnalysisQuery) (TeamRounds, error) {
	q.Limit = 0
	var out TeamRounds
	err := c.getJSON(ctx, teamPath(teamID, "rounds")+q.encode(), &out)
	return out, err
}

// TeamProfile says whether a team is the user's own and its HLTV rank (0 when unranked).
type TeamProfile struct {
	Own      bool `json:"own"`
	HLTVRank int  `json:"hltv_rank"`
	OwnTeams int  `json:"own_teams"`
}

// TeamProfilePatch changes a team profile; nil fields stay unchanged.
type TeamProfilePatch struct {
	Own      *bool `json:"own,omitempty"`
	HLTVRank *int  `json:"hltv_rank,omitempty"`
}

// SetTeamProfile applies a profile patch and returns the profile.
func (c *Client) SetTeamProfile(ctx context.Context, teamID string, patch TeamProfilePatch) (TeamProfile, error) {
	var out TeamProfile
	err := c.doJSON(ctx, http.MethodPut, teamPath(teamID, "profile"), patch, &out)
	return out, err
}
