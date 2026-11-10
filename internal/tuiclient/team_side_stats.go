package tuiclient

import "context"

// SideStat is one team figure: nil Value or GroupAvg when there is no data.
type SideStat struct {
	Value      *float64 `json:"value"`
	Count      float64  `json:"count"`
	Total      int      `json:"total"`
	GroupAvg   *float64 `json:"group_avg"`
	GroupTeams int      `json:"group_teams"`
}

// SideStats is a team's figures over one side, or both.
type SideStats struct {
	MapsWon       SideStat `json:"maps_won"`
	RoundsWon     SideStat `json:"rounds_won"`
	PistolsWon    SideStat `json:"pistols_won"`
	OpeningKills  SideStat `json:"opening_kills"`
	Won5v4        SideStat `json:"won_5v4"`
	Won4v5        SideStat `json:"won_4v5"`
	Trades        SideStat `json:"trades"`
	UtilityDamage SideStat `json:"utility_damage_per_round"`
	FlashAssists  SideStat `json:"flash_assists_per_round"`
	RoundsPlayed  SideStat `json:"rounds_played"`
}

// SideStatsPlayer is one player card of the side stats.
type SideStatsPlayer struct {
	SteamID64 string  `json:"steamid64"`
	Name      string  `json:"name"`
	Role      string  `json:"role"`
	Rounds    int     `json:"rounds"`
	Rating    float64 `json:"rating"`
	KD        float64 `json:"kd"`
	ADR       float64 `json:"adr"`
	KPR       float64 `json:"kpr"`
	DPR       float64 `json:"dpr"`
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
