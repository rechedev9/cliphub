package faceit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type apiMatchTimes struct {
	StartedAt int64 `json:"started_at"`
}

// MatchStartedAt returns when a FACEIT match started, or the zero time when FACEIT has no such
// match or no start time for it.
func (c *Client) MatchStartedAt(ctx context.Context, matchID string) (time.Time, error) {
	var times apiMatchTimes
	err := c.getJSON(ctx, "/matches/"+url.PathEscape(matchID), nil, &times)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("match %s: %w", matchID, err)
	}
	if times.StartedAt <= 0 {
		return time.Time{}, nil
	}
	return time.Unix(times.StartedAt, 0).UTC(), nil
}
