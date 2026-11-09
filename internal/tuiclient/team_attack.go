package tuiclient

import "context"

// AttackCell is the rounds of one site or tempo key of an attack prep.
type AttackCell struct {
	Key string `json:"key"`
	ReportGroup
}

// AttackMatchup is a buy band against the rival's on T, with its sites and tempo.
type AttackMatchup struct {
	Buy         string `json:"buy"`
	OpponentBuy string `json:"opponent_buy"`
	ReportGroup
	Sites []AttackCell `json:"sites"`
	Tempo []AttackCell `json:"tempo"`
}

// AttackDefault is a spread the team stood in at the read second, such as "2-1-2".
type AttackDefault struct {
	Formation string `json:"formation"`
	ReportGroup
	Zones []struct {
		Zone  string `json:"zone"`
		Label string `json:"label"`
		ReportGroup
	} `json:"zones"`
}

// AttackExecute is one way the team took a site on a buy matchup.
type AttackExecute struct {
	Buy         string `json:"buy"`
	OpponentBuy string `json:"opponent_buy"`
	Site        string `json:"site"`
	Route       string `json:"route"`
	RouteLabel  string `json:"route_label"`
	ReportGroup
	Rushes         Rate    `json:"rushes"`
	Plants         Rate    `json:"plants"`
	ContactSeconds float64 `json:"contact_seconds"`
}

// AttackPlayer is one T player of an attack prep and their spots per window.
type AttackPlayer struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Rounds    int    `json:"rounds"`
	Windows   []struct {
		FromSeconds int `json:"from_seconds"`
		ToSeconds   int `json:"to_seconds"`
		Rounds      int `json:"rounds"`
		Spots       []struct {
			Callout        string     `json:"callout"`
			Share          Rate       `json:"share"`
			Refs           []Evidence `json:"refs"`
			Floor          string     `json:"floor"`
			ArrivalSeconds float64    `json:"arrival_seconds"`
		} `json:"spots"`
		Pace struct {
			Static Rate `json:"static"`
			Slow   Rate `json:"slow"`
			Fast   Rate `json:"fast"`
		} `json:"pace"`
	} `json:"windows"`
}

// PrepWarning is what to distrust in a prep.
type PrepWarning struct {
	Kind   string   `json:"kind"`
	Days   int      `json:"days"`
	Rounds int      `json:"rounds"`
	Joined []string `json:"joined"`
	Left   []string `json:"left"`
}

// TeamAttack is GET /api/library/teams/{id}/attack.
type TeamAttack struct {
	TeamName     string            `json:"team_name"`
	Skipped      []AnalysisSkipped `json:"skipped"`
	Map          string            `json:"map"`
	Matches      int               `json:"matches"`
	RoundCount   int               `json:"round_count"`
	Framed       int               `json:"framed"`
	LastPlayedAt string            `json:"last_played_at"`
	DaysSince    int               `json:"days_since_last_match"`
	Sites        []AttackCell      `json:"sites"`
	Tempo        []AttackCell      `json:"tempo"`
	Matchups     []AttackMatchup   `json:"matchups"`
	Defaults     []AttackDefault   `json:"defaults"`
	Executes     []AttackExecute   `json:"executes"`
	Players      []AttackPlayer    `json:"players"`
	Warnings     []PrepWarning     `json:"warnings"`
}

// TeamAttackReport returns how a team attacks one map.
func (c *Client) TeamAttackReport(ctx context.Context, teamID string, q AnalysisQuery) (TeamAttack, error) {
	q.Limit, q.Side = 0, ""
	var out TeamAttack
	err := c.getJSON(ctx, teamPath(teamID, "attack")+q.encode(), &out)
	return out, err
}
