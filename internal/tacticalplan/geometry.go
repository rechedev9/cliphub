package tacticalplan

import (
	"math"
	"sort"

	"github.com/rechedev9/fragforge/internal/radarmap"
)

// GeometrySourceOccupancy marks geometry derived from where players actually
// stood during the demo, which is the only map shape a demo can prove.
const GeometrySourceOccupancy = "occupancy"

// MapGeometry is the drawable map, derived from play rather than from game
// assets: an occupancy grid of the walkable space plus the callout centroids
// CS2 itself labels positions with. It ships inside the document so a viewer
// can draw a faithful, licence-free radar with no external image.
type MapGeometry struct {
	Map         string               `json:"map"`
	Source      string               `json:"source"`
	Calibration radarmap.Calibration `json:"calibration"`
	Bounds      radarmap.Bounds      `json:"bounds"`
	CellSize    float64              `json:"cell_size"`
	Levels      []GeometryLevel      `json:"levels"`
	Callouts    []Callout            `json:"callouts"`
	SampleCount int                  `json:"sample_count"`
}

// GeometryLevel is one vertical section's occupancy grid.
type GeometryLevel struct {
	Name string `json:"name"`
	// Cells holds [cellX, cellY, weight] triples: grid coordinates relative to
	// MapGeometry.Origin and the number of samples observed in the cell. The
	// packed form keeps a full map under a few hundred kilobytes of JSON.
	Cells [][3]int `json:"cells"`
}

// Callout is one named position and the centre of mass of play observed in it.
// The names come from the map's own place names, so they match what players say.
type Callout struct {
	Name    string     `json:"name"`
	Level   string     `json:"level"`
	Center  [2]float64 `json:"center"`
	Samples int        `json:"samples"`
}

// OccupancyBuilder accumulates sampled positions into a map geometry. It is the
// same pass for every level: a sample lands in exactly one level and one cell.
type OccupancyBuilder struct {
	cellSize float64
	cal      radarmap.Calibration
	levels   map[string]map[[2]int]int
	callouts map[string]*calloutAcc
	bounds   radarmap.Bounds
	samples  int
	started  bool
}

type calloutAcc struct {
	level string
	sumX  float64
	sumY  float64
	count int
}

// NewOccupancyBuilder returns a builder writing cells of cellSize world units.
// The calibration decides which level a sample belongs to and travels into the
// finished geometry so a viewer needs nothing else to draw it.
func NewOccupancyBuilder(cal radarmap.Calibration, cellSize float64) *OccupancyBuilder {
	if cellSize <= 0 {
		cellSize = 64
	}
	return &OccupancyBuilder{
		cellSize: cellSize,
		cal:      cal,
		levels:   map[string]map[[2]int]int{},
		callouts: map[string]*calloutAcc{},
	}
}

// Add records one observed position. Place may be empty; callers pass the
// player's last place name when the demo provides one.
func (b *OccupancyBuilder) Add(x, y, z float64, place string) {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsNaN(z) {
		return
	}
	if !b.started {
		b.bounds = radarmap.Bounds{MinX: x, MinY: y, MaxX: x, MaxY: y}
		b.started = true
	}
	b.bounds.MinX = math.Min(b.bounds.MinX, x)
	b.bounds.MinY = math.Min(b.bounds.MinY, y)
	b.bounds.MaxX = math.Max(b.bounds.MaxX, x)
	b.bounds.MaxY = math.Max(b.bounds.MaxY, y)
	b.samples++

	level := b.cal.Level(z)
	cells, ok := b.levels[level]
	if !ok {
		cells = map[[2]int]int{}
		b.levels[level] = cells
	}
	cells[[2]int{
		int(math.Floor(x / b.cellSize)),
		int(math.Floor(y / b.cellSize)),
	}]++

	if place == "" {
		return
	}
	key := level + "\x00" + place
	acc, ok := b.callouts[key]
	if !ok {
		acc = &calloutAcc{level: level}
		b.callouts[key] = acc
	}
	acc.sumX += x
	acc.sumY += y
	acc.count++
}

// Bounds returns the world extent observed so far.
func (b *OccupancyBuilder) Bounds() radarmap.Bounds { return b.bounds }

// Samples returns how many positions have been recorded.
func (b *OccupancyBuilder) Samples() int { return b.samples }

// Build returns the geometry. minSamples drops cells and callouts seen fewer
// times, which removes the specks left by a player clipping through a corner or
// by a single spectator-camera artefact; the default of 1 keeps everything.
//
// The calibration passed to Build wins over the builder's, so a caller that
// derived a calibration from the final bounds can install it here.
func (b *OccupancyBuilder) Build(mapName string, cal radarmap.Calibration, minSamples int) MapGeometry {
	if minSamples < 1 {
		minSamples = 1
	}
	geo := MapGeometry{
		Map:         mapName,
		Source:      GeometrySourceOccupancy,
		Calibration: cal,
		Bounds:      b.bounds,
		CellSize:    b.cellSize,
		Levels:      []GeometryLevel{},
		Callouts:    []Callout{},
		SampleCount: b.samples,
	}

	names := make([]string, 0, len(b.levels))
	for name := range b.levels {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		cells := b.levels[name]
		packed := make([][3]int, 0, len(cells))
		for cell, weight := range cells {
			if weight < minSamples {
				continue
			}
			packed = append(packed, [3]int{cell[0], cell[1], weight})
		}
		sort.Slice(packed, func(i, j int) bool {
			if packed[i][0] != packed[j][0] {
				return packed[i][0] < packed[j][0]
			}
			return packed[i][1] < packed[j][1]
		})
		if len(packed) == 0 {
			continue
		}
		geo.Levels = append(geo.Levels, GeometryLevel{Name: name, Cells: packed})
	}

	keys := make([]string, 0, len(b.callouts))
	for key := range b.callouts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		acc := b.callouts[key]
		if acc.count < minSamples {
			continue
		}
		geo.Callouts = append(geo.Callouts, Callout{
			Name:    key[len(acc.level)+1:],
			Level:   acc.level,
			Center:  [2]float64{acc.sumX / float64(acc.count), acc.sumY / float64(acc.count)},
			Samples: acc.count,
		})
	}
	sort.Slice(geo.Callouts, func(i, j int) bool {
		if geo.Callouts[i].Samples != geo.Callouts[j].Samples {
			return geo.Callouts[i].Samples > geo.Callouts[j].Samples
		}
		return geo.Callouts[i].Name < geo.Callouts[j].Name
	})
	return geo
}

// CellCenter returns the world centre of a grid cell, which is what a renderer
// needs to place the cell without re-deriving the grid convention.
func (g MapGeometry) CellCenter(cellX, cellY int) (float64, float64) {
	return (float64(cellX) + 0.5) * g.CellSize, (float64(cellY) + 0.5) * g.CellSize
}
