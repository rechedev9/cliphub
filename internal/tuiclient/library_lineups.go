package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Lineup is a grenade of the Libreta: where to stand and aim, how to throw it
// and where it lands. Pitch and yaw are in setang's order and ranges.
type Lineup struct {
	ID                 string      `json:"id,omitempty"`
	Title              string      `json:"title"`
	Map                string      `json:"map"`
	Side               string      `json:"side"`
	Grenade            string      `json:"grenade"`
	Status             string      `json:"status,omitempty"`
	Tags               []string    `json:"tags,omitempty"`
	Notes              string      `json:"notes,omitempty"`
	Origin             [3]float64  `json:"origin"`
	Pitch              float64     `json:"pitch"`
	Yaw                float64     `json:"yaw"`
	Landing            *[3]float64 `json:"landing,omitempty"`
	ThrowCallout       string      `json:"throw_callout,omitempty"`
	LandingCallout     string      `json:"landing_callout,omitempty"`
	Movement           string      `json:"movement,omitempty"`
	Jump               bool        `json:"jump,omitempty"`
	Click              string      `json:"click,omitempty"`
	TechniqueEstimated bool        `json:"technique_estimated,omitempty"`
	Source             *Evidence   `json:"source,omitempty"`
	UpdatedAt          time.Time   `json:"updated_at,omitzero"`
}

// LineupQuery filters the Libreta's lineups; zero fields are unset.
type LineupQuery struct {
	Map             string
	Side            string
	Grenade         string
	Status          string
	Tag             string
	IncludeArchived bool
	Limit           int
	Offset          int
}

// LineupFromThrow names one throw of a library round.
type LineupFromThrow struct {
	MatchID   string `json:"match_id"`
	Round     int    `json:"round"`
	ThrowTick int    `json:"throw_tick"`
}

// NewLineup is what save_lineup sends: a throw to read, or the fields of a lineup written by hand.
type NewLineup struct {
	From   *LineupFromThrow `json:"from,omitempty"`
	Title  string           `json:"title"`
	Status string           `json:"status,omitempty"`
	Tags   []string         `json:"tags,omitempty"`
	Notes  string           `json:"notes,omitempty"`
	// Map, side, grenade and the angles come from the throw when From is set.
	Map     string      `json:"map,omitempty"`
	Side    string      `json:"side,omitempty"`
	Grenade string      `json:"grenade,omitempty"`
	Origin  *[3]float64 `json:"origin,omitempty"`
	Pitch   *float64    `json:"pitch,omitempty"`
	Yaw     *float64    `json:"yaw,omitempty"`
}

// LineupPatch edits a lineup; nil fields stay.
type LineupPatch struct {
	Title    *string   `json:"title,omitempty"`
	Status   *string   `json:"status,omitempty"`
	Tags     *[]string `json:"tags,omitempty"`
	Notes    *string   `json:"notes,omitempty"`
	Movement *string   `json:"movement,omitempty"`
	Jump     *bool     `json:"jump,omitempty"`
	Click    *string   `json:"click,omitempty"`
}

func (q LineupQuery) encode() string {
	params := map[string]string{"map": q.Map, "side": q.Side, "grenade": q.Grenade, "status": q.Status, "tag": q.Tag}
	if q.IncludeArchived {
		params["include_archived"] = "true"
	}
	if q.Limit > 0 {
		params["limit"] = strconv.Itoa(q.Limit)
	}
	if q.Offset > 0 {
		params["offset"] = strconv.Itoa(q.Offset)
	}
	return query(params)
}

func lineupPath(id string) string { return "/api/library/lineups/" + url.PathEscape(id) }

// ListLineups lists the Libreta's lineups.
func (c *Client) ListLineups(ctx context.Context, q LineupQuery) ([]Lineup, error) {
	var out struct {
		Lineups []Lineup `json:"lineups"`
	}
	err := c.getJSON(ctx, "/api/library/lineups"+q.encode(), &out)
	return out.Lineups, err
}

// GetLineup returns one lineup.
func (c *Client) GetLineup(ctx context.Context, id string) (Lineup, error) {
	var out Lineup
	err := c.getJSON(ctx, lineupPath(id), &out)
	return out, err
}

// CreateLineup stores a lineup read from a throw or written by hand.
func (c *Client) CreateLineup(ctx context.Context, in NewLineup) (Lineup, error) {
	var out Lineup
	err := c.doJSON(ctx, http.MethodPost, "/api/library/lineups", in, &out)
	return out, err
}

// UpdateLineup edits a lineup.
func (c *Client) UpdateLineup(ctx context.Context, id string, patch LineupPatch) (Lineup, error) {
	var out Lineup
	err := c.doJSON(ctx, http.MethodPatch, lineupPath(id), patch, &out)
	return out, err
}

// DeleteLineup removes a lineup from the Libreta.
func (c *Client) DeleteLineup(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, lineupPath(id), nil, nil)
}
