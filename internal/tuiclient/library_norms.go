package tuiclient

import (
	"context"
	"net/url"
	"time"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// LibraryNorms is GET /api/library/norms: team rates pooled over every match of a map and dates.
type LibraryNorms struct {
	Map     string `json:"map"`
	Skipped int    `json:"skipped"`
	tacticalplan.Norms
}

// LibraryNorms reads the library reference the team screens compare a team with; zero times mean no bound.
func (c *Client) LibraryNorms(ctx context.Context, mapName string, from, to time.Time) (LibraryNorms, error) {
	q := url.Values{}
	if mapName != "" {
		q.Set("map", mapName)
	}
	if !from.IsZero() {
		q.Set("from", from.UTC().Format(time.RFC3339))
	}
	if !to.IsZero() {
		q.Set("to", to.UTC().Format(time.RFC3339))
	}
	path := "/api/library/norms"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out LibraryNorms
	err := c.getJSON(ctx, path, &out)
	return out, err
}
