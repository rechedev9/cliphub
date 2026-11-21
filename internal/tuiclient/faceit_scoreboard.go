package tuiclient

import "context"

// FaceitScoreboard is GET /api/jobs/{id}/faceit-scoreboard: the FACEIT room's numbers for a FACEIT demo.
type FaceitScoreboard struct {
	MatchID string                 `json:"match_id"`
	RoomURL string                 `json:"room_url"`
	Map     string                 `json:"map"`
	Rounds  int                    `json:"rounds"`
	Teams   []FaceitScoreboardTeam `json:"teams"`
}

// FaceitScoreboardTeam is one side of the room scoreboard.
type FaceitScoreboardTeam struct {
	Name       string                   `json:"name"`
	Score      int                      `json:"score"`
	Won        bool                     `json:"won"`
	AverageELO *int                     `json:"average_elo,omitempty"`
	Players    []FaceitScoreboardPlayer `json:"players"`
}

// FaceitScoreboardPlayer is one row of the room scoreboard.
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

// FaceitScoreboard reads the FACEIT room scoreboard of a job; ErrorCode is not_faceit_demo for any other demo.
func (c *Client) FaceitScoreboard(ctx context.Context, jobID string) (FaceitScoreboard, error) {
	var out struct {
		Scoreboard FaceitScoreboard `json:"scoreboard"`
	}
	err := c.getJSON(ctx, jobPath(jobID)+"/faceit-scoreboard", &out)
	return out.Scoreboard, err
}

// FaceitCountries lists the country codes the FACEIT ranking selector offers.
func (c *Client) FaceitCountries(ctx context.Context) ([]string, error) {
	var out struct {
		Countries []string `json:"countries"`
	}
	err := c.getJSON(ctx, "/api/faceit/countries", &out)
	return out.Countries, err
}
