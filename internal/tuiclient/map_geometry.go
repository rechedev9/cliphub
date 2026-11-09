package tuiclient

import (
	"context"
	"net/url"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// MapGeometryOf reads the reference geometry of a map, the one its 2D radar draws;
// nil when no match of the map is analysed.
func (c *Client) MapGeometryOf(ctx context.Context, mapName string) (*tacticalplan.MapGeometry, error) {
	var out struct {
		Geometry *tacticalplan.MapGeometry `json:"geometry"`
	}
	err := c.getJSON(ctx, "/api/library/maps/"+url.PathEscape(mapName)+"/callouts?geometry=true", &out)
	return out.Geometry, err
}
