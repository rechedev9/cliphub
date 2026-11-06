package tuiclient

import (
	"context"
	"net/url"

	"github.com/rechedev9/cliphub/internal/mapgeo"
)

// GetMapSceneSummary reports what the map's real 3D scene holds, reading it
// from the CS2 install first when Studio has not cached it yet.
func (c *Client) GetMapSceneSummary(ctx context.Context, mapName string) (mapgeo.SceneSummary, error) {
	var out mapgeo.SceneSummary
	err := c.getJSON(ctx, "/api/maps/"+url.PathEscape(mapName)+"/scene/summary", &out)
	return out, err
}
