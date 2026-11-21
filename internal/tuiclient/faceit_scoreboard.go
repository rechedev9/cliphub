package tuiclient

import "context"

type FaceitScoreboard struct {
	MatchID string                 `json:"match_id"`
	RoomURL string                 `json:"room_url"`
	Map     string                 `json:"map"`
	Rounds  int                    `json:"rounds"`
	Teams   []FaceitScoreboardTeam `json:"teams"`
}

type FaceitScoreboardTeam struct {
	Name       string                   `json:"name"`
	Score      int                      `json:"score"`
	Won        bool                     `json:"won"`
	AverageELO *int                     `json:"average_elo,omitempty"`
	Players    []FaceitScoreboardPlayer `json:"players"`
}

type FaceitScoreboardPlayer struct {
	Nickname   string  `json:"nickname"`
	SteamID64  string  `json:"steam_id64,omitempty"`
	SkillLevel int     `json:"skill_level,omitempty"`
	ELO        *int    `json:"elo,omitempty"`
	Kills      int     `json:"kills"`
	Deaths     int     `json:"deaths"`
	Assists    int     `json:"assists"`
	HSPct      float64 `json:"hs_pct"`
	ADR        float64 `json:"adr"`
	MVPs       int     `json:"mvps"`
}

func (c *Client) FaceitScoreboard(ctx context.Context, jobID string) (FaceitScoreboard, error) {
	var out struct {
		Scoreboard FaceitScoreboard `json:"scoreboard"`
	}
	err := c.getJSON(ctx, jobPath(jobID)+"/faceit-scoreboard", &out)
	return out.Scoreboard, err
}

func (c *Client) FaceitCountries(ctx context.Context) ([]string, error) {
	var out struct {
		Countries []string `json:"countries"`
	}
	err := c.getJSON(ctx, "/api/faceit/countries", &out)
	return out.Countries, err
}
