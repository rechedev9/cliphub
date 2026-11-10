package tuiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// PatternScope is a saved pattern's own scope: the most recent N matches and the buys.
type PatternScope struct {
	Matches     int    `json:"matches,omitempty"`
	Buy         string `json:"buy,omitempty"`
	OpponentBuy string `json:"opponent_buy,omitempty"`
}

// PatternStats are a saved pattern's figures over the scope the list was asked for.
type PatternStats struct {
	Rounds        int     `json:"rounds"`
	Matched       int     `json:"matched"`
	Won           int     `json:"won"`
	WinRate       float64 `json:"win_rate"`
	RecentMatched int     `json:"recent_matched"`
	Trend         string  `json:"trend"`
}

// SavedPattern is a pattern stored in the library; without a team it reads the whole library.
type SavedPattern struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Map       string               `json:"map"`
	TeamID    string               `json:"team_id,omitempty"`
	Pattern   tacticalplan.Pattern `json:"pattern"`
	Scope     PatternScope         `json:"scope"`
	UpdatedAt time.Time            `json:"updated_at,omitzero"`
	Stats     *PatternStats        `json:"stats,omitempty"`
}

// PatternRun is a pattern over a team or the library: the map read and the matched rounds by match.
type PatternRun struct {
	Map     string `json:"map"`
	Matches []struct {
		MatchID string `json:"match_id"`
	} `json:"matches"`
	Pattern tacticalplan.Pattern       `json:"pattern"`
	Result  tacticalplan.PatternResult `json:"result"`
}

// PatternListQuery lists a map's saved patterns; with Stats each runs over the team's scope, or the library without a team.
type PatternListQuery struct {
	Map    string
	TeamID string
	Stats  bool
	Scope  AnalysisQuery
	Extra  url.Values
}

// TeamPatternQuery is a pattern over a team's matches on one map: Scope is the
// team's scope, Extra the buys and matches (the most recent N).
type TeamPatternQuery struct {
	TeamID  string
	Scope   AnalysisQuery
	Extra   url.Values
	Pattern tacticalplan.Pattern
}

// TeamPattern runs a pattern over a team's matches on one map.
func (c *Client) TeamPattern(ctx context.Context, q TeamPatternQuery) (PatternRun, error) {
	q.Scope.Limit = 0
	values := q.Scope.values()
	for key, list := range q.Extra {
		values[key] = list
	}
	raw, err := json.Marshal(q.Pattern)
	if err != nil {
		return PatternRun{}, err
	}
	values.Set("pattern", string(raw))
	var out PatternRun
	err = c.getJSON(ctx, teamPath(q.TeamID, "pattern")+encodeValues(values), &out)
	return out, err
}

func patternPath(id string) string { return "/api/library/patterns/" + url.PathEscape(id) }

// ListPatterns lists a map's saved patterns, by name.
func (c *Client) ListPatterns(ctx context.Context, q PatternListQuery) ([]SavedPattern, error) {
	q.Scope.Limit = 0
	values := q.Scope.values()
	for key, list := range q.Extra {
		values[key] = list
	}
	values.Set("map", q.Map)
	if q.TeamID != "" {
		values.Set("team_id", q.TeamID)
	}
	if q.Stats {
		values.Set("stats", strconv.Itoa(1))
	}
	var out struct {
		Patterns []SavedPattern `json:"patterns"`
	}
	err := c.getJSON(ctx, "/api/library/patterns"+encodeValues(values), &out)
	return out.Patterns, err
}

// GetPattern returns one saved pattern.
func (c *Client) GetPattern(ctx context.Context, id string) (SavedPattern, error) {
	var out SavedPattern
	err := c.getJSON(ctx, patternPath(id), &out)
	return out, err
}

// CreatePattern saves a pattern; an empty TeamID saves it for the whole library.
func (c *Client) CreatePattern(ctx context.Context, p SavedPattern) (SavedPattern, error) {
	body := map[string]any{"name": p.Name, "map": p.Map, "pattern": p.Pattern, "scope": p.Scope}
	if p.TeamID != "" {
		body["team_id"] = p.TeamID
	}
	var out SavedPattern
	err := c.doJSON(ctx, http.MethodPost, "/api/library/patterns", body, &out)
	return out, err
}

// DeletePattern removes a saved pattern.
func (c *Client) DeletePattern(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, patternPath(id), nil, nil)
}

// PatternRounds runs a saved pattern over a team (its own when teamID is empty) or, without one, the whole library on its map.
func (c *Client) PatternRounds(ctx context.Context, id, teamID string) (PatternRun, error) {
	values := url.Values{}
	if teamID != "" {
		values.Set("team_id", teamID)
	}
	var out PatternRun
	err := c.getJSON(ctx, patternPath(id)+"/rounds"+encodeValues(values), &out)
	return out, err
}
