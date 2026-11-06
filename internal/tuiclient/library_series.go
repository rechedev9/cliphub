package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// VetoStep is one step of a known veto: team a or b (empty for the decider),
// ban, pick or decider, and the map.
type VetoStep struct {
	Team   string `json:"team"`
	Action string `json:"action"`
	Map    string `json:"map"`
}

// LibrarySeries groups the maps of one BO1, BO3 or BO5.
type LibrarySeries struct {
	ID     string `json:"id"`
	Format string `json:"format"`
	Event  string `json:"event"`
	Veto   string `json:"veto"`
	Teams  []struct {
		Slot   string `json:"slot"`
		TeamID string `json:"team_id"`
		Name   string `json:"name"`
	} `json:"teams"`
	Vetoes    []VetoStep `json:"vetoes"`
	MatchIDs  []string   `json:"match_ids"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// SeriesInput is the body of a series create or edit; nil fields are left out.
// TeamIDs holds no team or two, slot a first.
type SeriesInput struct {
	Format  *string     `json:"format,omitempty"`
	Event   *string     `json:"event,omitempty"`
	Veto    *string     `json:"veto,omitempty"`
	TeamIDs *[]string   `json:"team_ids,omitempty"`
	Vetoes  *[]VetoStep `json:"vetoes,omitempty"`
}

func seriesPath(id string) string { return "/api/library/series/" + url.PathEscape(id) }

// ListLibrarySeries lists the library's series, optionally those of one team.
func (c *Client) ListLibrarySeries(ctx context.Context, teamID string) ([]LibrarySeries, error) {
	var out struct {
		Series []LibrarySeries `json:"series"`
	}
	err := c.getJSON(ctx, "/api/library/series"+query(map[string]string{"team_id": teamID, "limit": "200"}), &out)
	return out.Series, err
}

// GetLibrarySeries returns one series.
func (c *Client) GetLibrarySeries(ctx context.Context, id string) (LibrarySeries, error) {
	var out LibrarySeries
	err := c.getJSON(ctx, seriesPath(id), &out)
	return out, err
}

// CreateLibrarySeries stores a new series; matches join it with SetMatchSeries.
func (c *Client) CreateLibrarySeries(ctx context.Context, in SeriesInput) (LibrarySeries, error) {
	var out LibrarySeries
	err := c.doJSON(ctx, http.MethodPost, "/api/library/series", in, &out)
	return out, err
}

// UpdateLibrarySeries edits a series' format, event, notes, teams or vetoes.
func (c *Client) UpdateLibrarySeries(ctx context.Context, id string, in SeriesInput) (LibrarySeries, error) {
	var out LibrarySeries
	err := c.doJSON(ctx, http.MethodPatch, seriesPath(id), in, &out)
	return out, err
}

// DeleteLibrarySeries removes a series; its matches stay without one.
func (c *Client) DeleteLibrarySeries(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, seriesPath(id), nil, nil)
}

// SetMatchSeries puts a match in a series, or takes it out when seriesID is nil.
func (c *Client) SetMatchSeries(ctx context.Context, matchID string, seriesID *string) (LibraryMatch, error) {
	var out LibraryMatch
	err := c.doJSON(ctx, http.MethodPatch, libraryMatchPath(matchID), map[string]*string{"series_id": seriesID}, &out)
	return out, err
}

// TeamMapRecord is a team's record on one map.
type TeamMapRecord struct {
	Map        string `json:"map"`
	Matches    int    `json:"matches"`
	Wins       int    `json:"wins"`
	Losses     int    `json:"losses"`
	Ties       int    `json:"ties"`
	RoundsWon  int    `json:"rounds_won"`
	RoundsLost int    `json:"rounds_lost"`
	Sides      []struct {
		Side   string `json:"side"`
		Rounds int    `json:"rounds"`
		Won    struct {
			Count int `json:"count"`
		} `json:"won"`
	} `json:"sides"`
	Veto map[string]struct {
		Count int `json:"count"`
	} `json:"veto"`
	Preference struct {
		Score int `json:"score"`
	} `json:"preference"`
}

// TeamMaps is GET /api/library/teams/{id}/maps.
type TeamMaps struct {
	TeamName string            `json:"team_name"`
	Matches  int               `json:"matches"`
	Series   int               `json:"series"`
	Skipped  []AnalysisSkipped `json:"skipped"`
	Maps     []TeamMapRecord   `json:"maps"`
}

// LibraryTeamMaps returns a team's results, side rounds and vetoes per map.
func (c *Client) LibraryTeamMaps(ctx context.Context, teamID string, q AnalysisQuery) (TeamMaps, error) {
	var out TeamMaps
	err := c.getJSON(ctx, teamPath(teamID, "maps")+AnalysisQuery{From: q.From, To: q.To}.encode(), &out)
	return out, err
}

// VetoQuery sets up a simulated veto; empty fields take the server defaults
// (bo3, team a first, the active duty pool).
type VetoQuery struct {
	OpponentID string
	Format     string
	First      string
	Pool       []string
	Done       []string
}

// VetoSimulation is GET /api/library/teams/{id}/veto.
type VetoSimulation struct {
	TeamName     string `json:"team_name"`
	OpponentName string `json:"opponent_name"`
	Plan         struct {
		Format string `json:"format"`
		First  string `json:"first"`
		Maps   []struct {
			Map          string  `json:"map"`
			OurMatches   int     `json:"our_matches"`
			OurRate      float64 `json:"our_rate"`
			TheirMatches int     `json:"their_matches"`
			TheirRate    float64 `json:"their_rate"`
			Edge         float64 `json:"edge"`
			TheirBans    int     `json:"their_bans"`
			TheirPicks   int     `json:"their_picks"`
		} `json:"maps"`
		Steps []struct {
			Team   string `json:"team"`
			Action string `json:"action"`
			Map    string `json:"map"`
			Basis  string `json:"basis"`
		} `json:"steps"`
		Played          []string `json:"played"`
		OpponentHistory int      `json:"opponent_history"`
	} `json:"plan"`
}

// SimulateVeto recommends a veto against an opponent from both map records.
func (c *Client) SimulateVeto(ctx context.Context, teamID string, q VetoQuery) (VetoSimulation, error) {
	values := url.Values{"opponent_id": {q.OpponentID}}
	for k, v := range map[string]string{"format": q.Format, "first": q.First} {
		if v != "" {
			values.Set(k, v)
		}
	}
	for _, m := range q.Pool {
		values.Add("pool", m)
	}
	for _, m := range q.Done {
		values.Add("done", m)
	}
	var out VetoSimulation
	err := c.getJSON(ctx, teamPath(teamID, "veto")+encodeValues(values), &out)
	return out, err
}
