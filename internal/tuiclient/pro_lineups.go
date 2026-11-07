package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/rechedev9/cliphub/internal/prolineups"
)

// ProLineupMaps is the list of maps with pro lineups. Fresh is false when the
// community server did not answer and this is the last copy Studio kept.
type ProLineupMaps struct {
	Version int               `json:"version"`
	BuiltAt time.Time         `json:"built_at"`
	Maps    []ProLineupMapRow `json:"maps"`
	Fresh   bool              `json:"fresh"`
}

// ProLineupMapRow counts one map's pro lineups, combos and the pro maps read.
type ProLineupMapRow struct {
	Map        string `json:"map"`
	Lineups    int    `json:"lineups"`
	Combos     int    `json:"combos"`
	Maps       int    `json:"maps"`
	RecentMaps int    `json:"recent_maps"`
}

// ProLineupIndex is one map's pro lineups and combos.
type ProLineupIndex struct {
	prolineups.MapIndex
	Fresh bool `json:"fresh"`
}

// ListProLineupMaps lists the maps with pro lineups.
func (c *Client) ListProLineupMaps(ctx context.Context) (ProLineupMaps, error) {
	var out ProLineupMaps
	err := c.getJSON(ctx, "/api/library/pro-lineups", &out)
	return out, err
}

// GetProLineups returns every pro lineup and combo of a map.
func (c *Client) GetProLineups(ctx context.Context, mapName string) (ProLineupIndex, error) {
	var out ProLineupIndex
	err := c.getJSON(ctx, "/api/library/pro-lineups/"+url.PathEscape(mapName), &out)
	return out, err
}

// SaveProLineup copies a pro lineup into the Libreta.
func (c *Client) SaveProLineup(ctx context.Context, mapName, id string) (Lineup, error) {
	var out Lineup
	err := c.doJSON(ctx, http.MethodPost, "/api/library/pro-lineups/"+url.PathEscape(mapName)+"/lineups/"+url.PathEscape(id)+"/save", nil, &out)
	return out, err
}
