package tuiclient

import (
	"context"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

type SideStat = tacticalplan.SideStat

type SideStats = tacticalplan.SideStats

// SideStatsPlayer is one player card of the side stats.
type SideStatsPlayer struct {
	SteamID64 string                       `json:"steamid64"`
	Name      string                       `json:"name"`
	Role      string                       `json:"role"`
	MapRoles  *tacticalplan.PlayerMapRoles `json:"map_roles"`
	Rounds    int                          `json:"rounds"`
	Rating    float64                      `json:"rating"`
	KD        float64                      `json:"kd"`
	ADR       float64                      `json:"adr"`
	KPR       float64                      `json:"kpr"`
	DPR       float64                      `json:"dpr"`
	KAST      float64                      `json:"kast"`
	UDR       *float64                     `json:"udr"`
	FAR       *float64                     `json:"far"`
	HS        *float64                     `json:"hs"`
}

// TeamSideStatsReport is GET /api/library/teams/{id}/side-stats.
type TeamSideStatsReport struct {
	TeamName     string            `json:"team_name"`
	Skipped      []AnalysisSkipped `json:"skipped"`
	Compare      string            `json:"compare"`
	CompareTeams []struct {
		TeamID string `json:"team_id"`
		Name   string `json:"name"`
	} `json:"compare_teams"`
	CompareSkipped []string `json:"compare_skipped"`
	GroupCapped    bool     `json:"group_capped"`
	AWPEstimated   int      `json:"awp_estimated"`
	GroupMatches   int      `json:"group_matches"`
	GroupMinRounds int      `json:"group_min_rounds"`
	MatchList      []struct {
		MatchID  string `json:"match_id"`
		Map      string `json:"map"`
		PlayedAt string `json:"played_at"`
		Opponent string `json:"opponent"`
		Rounds   int    `json:"rounds"`
		Won      bool   `json:"won"`
	} `json:"match_list"`
	Matches       int               `json:"matches"`
	Rounds        int               `json:"rounds"`
	GroupTeams    int               `json:"group_teams"`
	Overall       SideStats         `json:"overall"`
	CT            SideStats         `json:"ct"`
	T             SideStats         `json:"t"`
	Players       []SideStatsPlayer `json:"players"`
	RatingFormula string            `json:"rating_formula"`
	// PlayersAverage pools the team's players; PlayersGroupAvg is the comparison group's mean of that line.
	PlayersAverage  tacticalplan.PlayerLine `json:"players_average"`
	PlayersGroupAvg tacticalplan.PlayerLine `json:"players_group_avg"`
}

// SideStatsQuery is a side stats read: the team scope, buys included, and the
// comparison teams (none compares with every other team of the library).
type SideStatsQuery struct {
	TeamID    string
	Scope     AnalysisQuery
	CompareTo []string
}

// TeamSideStats returns a team's figures per side against a comparison group.
func (c *Client) TeamSideStats(ctx context.Context, q SideStatsQuery) (TeamSideStatsReport, error) {
	q.Scope.Limit = 0
	values := q.Scope.values()
	for _, id := range q.CompareTo {
		values.Add("compare_team", id)
	}
	var out TeamSideStatsReport
	err := c.getJSON(ctx, teamPath(q.TeamID, "side-stats")+encodeValues(values), &out)
	return out, err
}
