package tuiclient

import (
	"context"
	"strconv"
)

// SearchResult is one record a library search found.
type SearchResult struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Map      string `json:"map,omitempty"`
	Side     string `json:"side,omitempty"`
	Status   string `json:"status,omitempty"`
	Detail   string `json:"detail,omitempty"`
	TeamName string `json:"team_name,omitempty"`
}

// SearchResults is a library search: the map and grenade it read from the query, and what it found.
type SearchResults struct {
	Map     string         `json:"map,omitempty"`
	Grenade string         `json:"grenade,omitempty"`
	Results []SearchResult `json:"results"`
}

// Search looks for plays, lineups, calls, teams, players and matches.
func (c *Client) Search(ctx context.Context, q string, perKind int) (SearchResults, error) {
	params := map[string]string{"q": q}
	if perKind > 0 {
		params["limit"] = strconv.Itoa(perKind)
	}
	var out SearchResults
	err := c.getJSON(ctx, "/api/library/search"+query(params), &out)
	return out, err
}
