package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/placename"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const (
	zoneMapAppURI = "ui://cliphub/zone-map"
	// zoneMapMetaKey carries the drawing in the result's _meta: the view reads it, the model does not need it.
	zoneMapMetaKey = "cliphub/zone-map"
	// silhouetteStep merges this many occupancy cells per side, so the map outline stays a few kilobytes.
	silhouetteStep = 2
	// lowerLevel is the radar level name of a two-floor map's lower floor.
	lowerLevel = "lower"
)

type showZonesIn struct {
	Map        string   `json:"map" jsonschema:"the map, such as de_mirage"`
	Zones      []string `json:"zones" jsonschema:"CS2 place tokens such as BombsiteA or TopofMid, from team_attack, team_grenades, player_positions or list_callouts; at most 12"`
	Title      string   `json:"title,omitempty" jsonschema:"what the zones are, shown over the map, such as the default 2-1-2 on full buys"`
	StudioPath string   `json:"studio_path,omitempty" jsonschema:"the finding's studio_path, shown under the map so the user can open its rounds in Studio"`
}

type zoneMarkOut struct {
	Callout string `json:"callout"`
	Label   string `json:"label"`
	Found   bool   `json:"found" jsonschema:"false when the map's geometry has no such place"`
}

type showZonesOut struct {
	Map        string        `json:"map"`
	Title      string        `json:"title,omitempty"`
	Zones      []zoneMarkOut `json:"zones"`
	StudioPath string        `json:"studio_path,omitempty"`
}

// zoneDrawing is the map in world units with y pointing down: the walkable
// outline and each zone's cells, ready for one SVG.
type zoneDrawing struct {
	ViewBox    string       `json:"view_box"`
	Silhouette string       `json:"silhouette"`
	Zones      []zoneShapes `json:"zones"`
}

type zoneShapes struct {
	Callout string  `json:"callout"`
	Label   string  `json:"label"`
	Path    string  `json:"path"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Lower   bool    `json:"lower,omitempty"`
}

func registerZoneMapTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "show_zones", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}, Meta: appToolMeta(zoneMapAppURI),
		Description: "Show the user a map with some zones marked, inside the conversation in hosts with MCP Apps: the walkable outline of the map " +
			"Studio's 2D radar draws, and each zone's area. Use it for the zones a finding names (a default's map control, an execute's route, " +
			"a player's spot, where smokes land). Without a UI the answer still says which zones the map has."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in showZonesIn) (*mcp.CallToolResult, showZonesOut, error) {
			if !mapNamePattern.MatchString(in.Map) {
				return nil, showZonesOut{}, fmt.Errorf("map must be a map name such as de_mirage, not %q", cleanName(in.Map))
			}
			if len(in.Zones) == 0 || len(in.Zones) > maxZoneMarks {
				return nil, showZonesOut{}, fmt.Errorf("zones takes from 1 to %d place tokens", maxZoneMarks)
			}
			for _, z := range in.Zones {
				if !library.ValidCalloutToken(z) {
					return nil, showZonesOut{}, fmt.Errorf("%q is not a CS2 place token", cleanName(z))
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, showZonesOut{}, toolError(err)
			}
			geometry, err := c.MapGeometryOf(ctx, in.Map)
			if err != nil {
				return nil, showZonesOut{}, toolError(err)
			}
			if geometry == nil || geometry.CellSize <= 0 {
				return nil, showZonesOut{}, errors.New("no match of this map is analysed yet, so Studio cannot draw it")
			}
			drawing := drawZones(*geometry, in.Zones)
			out := showZonesOut{Map: in.Map, Title: cleanName(strings.TrimSpace(in.Title)), Zones: []zoneMarkOut{}, StudioPath: cleanName(in.StudioPath)}
			found := map[string]bool{}
			for _, z := range drawing.Zones {
				found[z.Callout] = true
			}
			labels := []string{}
			for _, z := range in.Zones {
				out.Zones = append(out.Zones, zoneMarkOut{Callout: cleanName(z), Label: cleanName(placename.Label(z)), Found: found[z]})
				labels = append(labels, placename.Label(z))
			}
			text := fmt.Sprintf("Map %s with %s marked.", in.Map, strings.Join(cleanNames(labels), ", "))
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, Meta: mcp.Meta{zoneMapMetaKey: drawing}}, out, nil
		})
}

type cellRun struct{ x0, x1, y int }

// runs merges cells into horizontal runs, row by row, so a path needs one rectangle per run.
func runs(cells map[[2]int]bool) []cellRun {
	keys := make([][2]int, 0, len(cells))
	for k := range cells {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][1] != keys[j][1] {
			return keys[i][1] < keys[j][1]
		}
		return keys[i][0] < keys[j][0]
	})
	out := []cellRun{}
	for _, k := range keys {
		if n := len(out); n > 0 && out[n-1].y == k[1] && out[n-1].x1 == k[0]-1 {
			out[n-1].x1 = k[0]
			continue
		}
		out = append(out, cellRun{x0: k[0], x1: k[0], y: k[1]})
	}
	return out
}

// runPath draws runs of cells of side units; world y grows up, SVG y grows down.
func runPath(list []cellRun, side float64) string {
	var b strings.Builder
	for _, r := range list {
		w := float64(r.x1-r.x0+1) * side
		fmt.Fprintf(&b, "M%g %gh%gv%gh%gz", float64(r.x0)*side, float64(-(r.y+1))*side, w, side, -w)
	}
	return b.String()
}

// drawZones draws the default floor's outline and the asked zones on any floor.
func drawZones(g tacticalplan.MapGeometry, zones []string) zoneDrawing {
	side := g.CellSize * silhouetteStep
	blocks := map[[2]int]bool{}
	for _, level := range g.Levels {
		if level.Name == lowerLevel {
			continue
		}
		for _, cell := range level.Cells {
			if cell[2] > 0 {
				blocks[[2]int{floorDiv(cell[0], silhouetteStep), floorDiv(cell[1], silhouetteStep)}] = true
			}
		}
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for k := range blocks {
		minX, maxX = math.Min(minX, float64(k[0])*side), math.Max(maxX, float64(k[0]+1)*side)
		minY, maxY = math.Min(minY, -float64(k[1]+1)*side), math.Max(maxY, -float64(k[1])*side)
	}
	out := zoneDrawing{Silhouette: runPath(runs(blocks), side), Zones: []zoneShapes{}}
	if len(blocks) > 0 {
		pad := side * 2
		out.ViewBox = fmt.Sprintf("%g %g %g %g", minX-pad, minY-pad, maxX-minX+2*pad, maxY-minY+2*pad)
	}
	for _, zone := range zones {
		for _, c := range g.Callouts {
			if c.Name != zone || len(c.Cells) == 0 {
				continue
			}
			cells := map[[2]int]bool{}
			for _, cell := range c.Cells {
				cells[cell] = true
			}
			out.Zones = append(out.Zones, zoneShapes{Callout: zone, Label: placename.Label(zone), Path: runPath(runs(cells), g.CellSize),
				X: math.Round(c.Center[0]), Y: 0 - math.Round(c.Center[1]), Lower: c.Level == lowerLevel})
		}
	}
	return out
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}
