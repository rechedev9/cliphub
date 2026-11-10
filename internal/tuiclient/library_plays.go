package tuiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// PlayGrenade is one grenade of a play, as the orchestrator stores it.
type PlayGrenade struct {
	Kind     string     `json:"kind"`
	Seconds  float64    `json:"seconds"`
	Pos      [3]float64 `json:"pos"`
	Callout  string     `json:"callout,omitempty"`
	Player   string     `json:"player,omitempty"`
	Share    float64    `json:"share,omitempty"`
	LineupID string     `json:"lineup_id,omitempty"`
	Lineup   string     `json:"lineup,omitempty"`
}

// PlayPosition is where one player of a play stood at the play's second.
type PlayPosition struct {
	Player    string     `json:"player,omitempty"`
	SteamID64 string     `json:"steamid64,omitempty"`
	Pos       [3]float64 `json:"pos"`
	Callout   string     `json:"callout,omitempty"`
	Share     float64    `json:"share,omitempty"`
}

// Play is a strat of the stratbook. MissingMatchIDs is set on single reads.
type Play struct {
	ID               string         `json:"id,omitempty"`
	Title            string         `json:"title"`
	TeamID           *string        `json:"team_id"`
	TeamName         string         `json:"team_name,omitempty"`
	Map              string         `json:"map"`
	Side             string         `json:"side"`
	Kind             string         `json:"kind"`
	Site             string         `json:"site"`
	Origin           string         `json:"origin"`
	PositionsSeconds float64        `json:"positions_seconds"`
	Grenades         []PlayGrenade  `json:"grenades"`
	Positions        []PlayPosition `json:"positions"`
	Notes            string         `json:"notes"`
	Examples         []Evidence     `json:"examples"`
	Status           string         `json:"status,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	Roles            []RoleSlot     `json:"roles,omitempty"`
	UpdatedAt        time.Time      `json:"updated_at,omitzero"`
	MissingMatchIDs  []string       `json:"missing_match_ids,omitempty"`
}

// PlayQuery filters the stratbook; zero fields are unset.
type PlayQuery struct {
	TeamID          string
	Map             string
	Side            string
	Kind            string
	Status          string
	Tag             string
	IncludeArchived bool
	Limit           int
	Offset          int
}

func (q PlayQuery) encode() string {
	params := map[string]string{"team_id": q.TeamID, "map": q.Map, "side": q.Side, "kind": q.Kind, "status": q.Status, "tag": q.Tag}
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

func playPath(id string) string { return "/api/library/plays/" + url.PathEscape(id) }

// ListPlays lists the stratbook's plays.
func (c *Client) ListPlays(ctx context.Context, q PlayQuery) ([]Play, error) {
	var out struct {
		Plays []Play `json:"plays"`
	}
	err := c.getJSON(ctx, "/api/library/plays"+q.encode(), &out)
	return out.Plays, err
}

// GetPlay returns one play with the example matches no longer in the library.
func (c *Client) GetPlay(ctx context.Context, id string) (Play, error) {
	var out Play
	err := c.getJSON(ctx, playPath(id), &out)
	return out, err
}

// CreatePlay stores a play; the server validates every field again.
func (c *Client) CreatePlay(ctx context.Context, p Play) (Play, error) {
	p.ID, p.TeamName, p.UpdatedAt, p.MissingMatchIDs = "", "", time.Time{}, nil
	var out Play
	err := c.doJSON(ctx, http.MethodPost, "/api/library/plays", p, &out)
	return out, err
}

// PlayPatch edits a play; nil fields stay. ClearTeam sends team_id null.
type PlayPatch struct {
	Title     *string
	Kind      *string
	Site      *string
	Notes     *string
	Status    *string
	Tags      *[]string
	Roles     *[]RoleSlot
	TeamID    *string
	ClearTeam bool
}

// UpdatePlay edits a play's title, kind, site, notes, status, tags or team.
func (c *Client) UpdatePlay(ctx context.Context, id string, patch PlayPatch) (Play, error) {
	body := map[string]any{}
	for k, v := range map[string]*string{
		"title": patch.Title, "kind": patch.Kind, "site": patch.Site, "notes": patch.Notes, "status": patch.Status, "team_id": patch.TeamID,
	} {
		if v != nil {
			body[k] = *v
		}
	}
	if patch.Tags != nil {
		body["tags"] = *patch.Tags
	}
	if patch.Roles != nil {
		body["roles"] = *patch.Roles
	}
	if patch.ClearTeam {
		body["team_id"] = nil
	}
	var out Play
	err := c.doJSON(ctx, http.MethodPatch, playPath(id), body, &out)
	return out, err
}

// DeletePlay removes a play from the stratbook.
func (c *Client) DeletePlay(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, playPath(id), nil, nil)
}

// RoundPlayDraft returns the play one side's team ran in a round, with the
// positions read at tick (0 means the server's default second).
func (c *Client) RoundPlayDraft(ctx context.Context, matchID string, round int, teamKey string, tick int) (Play, error) {
	params := map[string]string{"team_key": teamKey}
	if tick > 0 {
		params["tick"] = strconv.Itoa(tick)
	}
	var out struct {
		Draft Play `json:"draft"`
	}
	err := c.getJSON(ctx, "/api/library/matches/"+url.PathEscape(matchID)+"/tactical/rounds/"+strconv.Itoa(round)+"/play"+query(params), &out)
	return out.Draft, err
}

// DetectedPlay is a group of a team's rounds that share utility and positions.
type DetectedPlay struct {
	Map       string         `json:"map"`
	Side      string         `json:"side"`
	Kind      string         `json:"kind"`
	Site      string         `json:"site"`
	Rounds    int            `json:"rounds"`
	Share     Rate           `json:"share"`
	WinRate   Rate           `json:"win_rate"`
	Grenades  []PlayGrenade  `json:"grenades"`
	Positions []PlayPosition `json:"positions"`
	Seed      Evidence       `json:"seed"`
	Examples  []Evidence     `json:"examples"`
	// Refs holds every round of the group, Examples only the first ones.
	Refs []Evidence `json:"refs"`
}

// PlayDetection is GET /api/library/teams/{id}/plays.
type PlayDetection struct {
	TeamName   string            `json:"team_name"`
	Skipped    []AnalysisSkipped `json:"skipped"`
	Matches    int               `json:"matches"`
	Rounds     int               `json:"rounds"`
	Compared   int               `json:"compared"`
	Thresholds struct {
		PositionSeconds float64 `json:"position_seconds"`
	} `json:"thresholds"`
	Plays []DetectedPlay `json:"plays"`
}

// DetectTeamPlays finds the plays a team repeats over the filtered matches.
func (c *Client) DetectTeamPlays(ctx context.Context, teamID string, q AnalysisQuery) (PlayDetection, error) {
	q.Limit = 0
	var out PlayDetection
	err := c.getJSON(ctx, teamPath(teamID, "plays")+q.encode(), &out)
	return out, err
}

// ExportPlays returns the stratbook file of the plays the query selects.
func (c *Client) ExportPlays(ctx context.Context, q PlayQuery) (json.RawMessage, error) {
	q.Limit, q.Offset = 0, 0
	var out json.RawMessage
	err := c.getJSON(ctx, "/api/library/plays/export"+q.encode(), &out)
	return out, err
}

// StratbookImport is the outcome of importing a stratbook file.
type StratbookImport struct {
	Imported   int `json:"imported"`
	Duplicates int `json:"duplicates"`
	Rejected   []struct {
		Index int    `json:"index"`
		Title string `json:"title"`
		Error string `json:"error"`
	} `json:"rejected"`
	Plays []Play `json:"plays"`
	// Sections counts the file's other sections (lineups, calls, callout_names, team_roles).
	Sections map[string]struct {
		Imported   int `json:"imported"`
		Duplicates int `json:"duplicates"`
		Rejected   int `json:"rejected"`
	} `json:"sections"`
}

// ImportPlays stores the plays of a stratbook file, each on its own.
func (c *Client) ImportPlays(ctx context.Context, file json.RawMessage) (StratbookImport, error) {
	var out StratbookImport
	err := c.doJSON(ctx, http.MethodPost, "/api/library/plays/import", file, &out)
	return out, err
}
