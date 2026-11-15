package tuiclient

import (
	"context"
	"net/url"
	"strconv"

	"github.com/rechedev9/cliphub/internal/moments"
)

// AutoPOVQuery asks for the Auto-POV pick of a multi-POV job; zero fields keep the server defaults.
type AutoPOVQuery struct {
	JobID         string
	TargetSeconds float64
	ExtraSeconds  float64
}

// GetAutoPOV reads GET /api/jobs/{id}/auto-pov: the plays and players the summary shows, in match order.
func (c *Client) GetAutoPOV(ctx context.Context, q AutoPOVQuery) (moments.Direction, error) {
	values := url.Values{}
	if q.TargetSeconds > 0 {
		values.Set("target_seconds", strconv.FormatFloat(q.TargetSeconds, 'f', -1, 64))
	}
	if q.ExtraSeconds > 0 {
		values.Set("extra_seconds", strconv.FormatFloat(q.ExtraSeconds, 'f', -1, 64))
	}
	path := jobPath(q.JobID) + "/auto-pov"
	if len(values) > 0 {
		path += "?" + values.Encode()
	}
	var out moments.Direction
	err := c.getJSON(ctx, path, &out)
	return out, err
}
