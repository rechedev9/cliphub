package tacticalplan

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

// Area limits: a polygon drawn point by point and a handful of the map's callouts.
const (
	PatternMaxPolygonPoints = 32
	PatternMaxCallouts      = 12
	// PatternMaxCalloutBytes matches the longest CS2 place name the Libreta stores.
	PatternMaxCalloutBytes = 64
)

// PatternArea is a zone of the map, in one of three shapes: a box in world units, a polygon
// (the box fields then unused), or CS2 places, the callouts the map names a position with.
// Level keeps a box or a polygon to one floor of a two-floor map; empty reads both.
type PatternArea struct {
	MinX     float64      `json:"min_x"`
	MinY     float64      `json:"min_y"`
	MaxX     float64      `json:"max_x"`
	MaxY     float64      `json:"max_y"`
	Polygon  [][2]float64 `json:"polygon,omitempty"`
	Callouts []string     `json:"callouts,omitempty"`
	Level    string       `json:"level,omitempty"`
}

// Contains reports whether a world position is inside the zone and on its floor.
func (a PatternArea) Contains(g MapGeometry, pos [3]float64) bool {
	switch {
	case len(a.Callouts) > 0:
		return slices.Contains(a.Callouts, NearestCallout(g, pos))
	case len(a.Polygon) > 0:
		if !inPolygon(a.Polygon, pos[0], pos[1]) {
			return false
		}
	case pos[0] < a.MinX || pos[0] > a.MaxX || pos[1] < a.MinY || pos[1] > a.MaxY:
		return false
	}
	return a.Level == "" || !g.Calibration.MultiLevel() || g.Calibration.Level(pos[2]) == a.Level
}

// inPolygon is the even-odd rule over the polygon's edges, closed from the last point to the first.
func inPolygon(poly [][2]float64, x, y float64) bool {
	in := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if (a[1] > y) != (b[1] > y) && x < (b[0]-a[0])*(y-a[1])/(b[1]-a[1])+a[0] {
			in = !in
		}
	}
	return in
}

func finite(values ...float64) bool {
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

func (a PatternArea) check() error {
	if a.Level != "" && a.Level != radarmap.LevelDefault && a.Level != radarmap.LevelLower {
		return errors.New("area level must be default or lower")
	}
	switch {
	case len(a.Callouts) > 0 && len(a.Polygon) > 0:
		return errors.New("an area is a box, a polygon or callouts, not several")
	case len(a.Callouts) > 0:
		if len(a.Callouts) > PatternMaxCallouts {
			return fmt.Errorf("an area takes at most %d callouts", PatternMaxCallouts)
		}
		if a.Level != "" {
			return errors.New("a callout area takes its floors from its places, so it has no level")
		}
		for _, c := range a.Callouts {
			if c == "" || len(c) > PatternMaxCalloutBytes {
				return fmt.Errorf("callouts must be CS2 place names of 1 to %d characters", PatternMaxCalloutBytes)
			}
		}
		return nil
	case len(a.Polygon) > 0:
		if len(a.Polygon) < 3 || len(a.Polygon) > PatternMaxPolygonPoints {
			return fmt.Errorf("a polygon takes 3 to %d points", PatternMaxPolygonPoints)
		}
		for _, p := range a.Polygon {
			if !finite(p[0], p[1]) {
				return errors.New("polygon points must be numbers")
			}
		}
		return nil
	}
	if a.MinX >= a.MaxX || a.MinY >= a.MaxY || !finite(a.MinX, a.MinY, a.MaxX, a.MaxY) {
		return errors.New("area needs min_x below max_x and min_y below max_y")
	}
	return nil
}

// zoneTest answers whether positions fall in a block's zone. A callout zone remembers each
// cell's answer, since naming a place reads every callout of the map.
type zoneTest struct {
	geometry MapGeometry
	area     *PatternArea
	cells    map[[3]int]bool
}

// zoneCalloutStep is the height band a cell's remembered answer holds for: stacked places differ in z.
const zoneCalloutStep = 32

func newZoneTest(g MapGeometry, area *PatternArea) *zoneTest {
	return &zoneTest{geometry: g, area: area}
}

func (z *zoneTest) contains(pos [3]float64) bool {
	if z.area == nil {
		return true
	}
	if len(z.area.Callouts) == 0 || z.geometry.CellSize <= 0 {
		return z.area.Contains(z.geometry, pos)
	}
	size := z.geometry.CellSize
	key := [3]int{int(math.Floor(pos[0] / size)), int(math.Floor(pos[1] / size)), int(math.Floor(pos[2] / zoneCalloutStep))}
	if z.cells == nil {
		z.cells = map[[3]int]bool{}
	}
	in, ok := z.cells[key]
	if !ok {
		in = z.area.Contains(z.geometry, pos)
		z.cells[key] = in
	}
	return in
}
