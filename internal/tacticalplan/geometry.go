package tacticalplan

import (
	"math"
	"sort"

	"github.com/rechedev9/cliphub/internal/radarmap"
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
	// Cells are the grid cells of Level where this was the place CS2 reported
	// most, sorted. A centroid alone mislabels the edges of large or bent places.
	Cells [][2]int `json:"cells,omitempty"`
	// Stacked holds sorted [cellX, cellY, mean altitude] for the cells this place
	// shares with a place on another floor, like Mirage's Underpass under Catwalk.
	Stacked [][3]int `json:"stacked,omitempty"`
}

// A place is another floor of a cell when seen there stackMinSamples times, its
// mean height stackGap or more from each place kept: above a jump, below a floor.
const (
	stackGap        = 96
	stackMinSamples = 2
)

// OccupancyBuilder accumulates sampled positions into a map geometry. It is the
// same pass for every level: a sample lands in exactly one level and one cell.
type OccupancyBuilder struct {
	cellSize float64
	cal      radarmap.Calibration
	levels   map[string]map[[2]int]int
	places   map[string]map[[2]int]map[string]cellPlace
	callouts map[string]*calloutAcc
	bounds   radarmap.Bounds
	samples  int
	started  bool
}

type cellPlace struct {
	count int
	sumZ  float64
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
		places:   map[string]map[[2]int]map[string]cellPlace{},
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
	cell := [2]int{int(math.Floor(x / b.cellSize)), int(math.Floor(y / b.cellSize))}
	cells[cell]++

	if place == "" {
		return
	}
	levelPlaces, ok := b.places[level]
	if !ok {
		levelPlaces = map[[2]int]map[string]cellPlace{}
		b.places[level] = levelPlaces
	}
	cellPlaces, ok := levelPlaces[cell]
	if !ok {
		cellPlaces = map[string]cellPlace{}
		levelPlaces[cell] = cellPlaces
	}
	tally := cellPlaces[place]
	tally.count++
	tally.sumZ += z
	cellPlaces[place] = tally
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

	owned, stacked := b.cellOwners(minSamples)
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
			Cells:   owned[key],
			Stacked: stacked[key],
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

// cellOwners maps each callout key (level + NUL + place) to the cells it is the
// most reported place in, ties to the first name, and to its stacked cells.
func (b *OccupancyBuilder) cellOwners(minSamples int) (map[string][][2]int, map[string][][3]int) {
	owned := map[string][][2]int{}
	stacked := map[string][][3]int{}
	for level, cells := range b.places {
		for cell, tallies := range cells {
			if b.levels[level][cell] < minSamples {
				continue
			}
			names := make([]string, 0, len(tallies))
			for place := range tallies {
				if acc, ok := b.callouts[level+"\x00"+place]; ok && acc.count >= minSamples {
					names = append(names, place)
				}
			}
			if len(names) == 0 {
				continue
			}
			sort.Slice(names, func(i, j int) bool {
				if tallies[names[i]].count != tallies[names[j]].count {
					return tallies[names[i]].count > tallies[names[j]].count
				}
				return names[i] < names[j]
			})
			owned[level+"\x00"+names[0]] = append(owned[level+"\x00"+names[0]], cell)

			floors := []string{names[0]}
			for _, place := range names[1:] {
				if tallies[place].count >= stackMinSamples && apartFrom(tallies, floors, place) {
					floors = append(floors, place)
				}
			}
			if len(floors) < 2 {
				continue
			}
			for _, place := range floors {
				t := tallies[place]
				key := level + "\x00" + place
				stacked[key] = append(stacked[key], [3]int{cell[0], cell[1], int(math.Round(t.sumZ / float64(t.count)))})
			}
		}
	}
	for _, cells := range owned {
		sort.Slice(cells, func(i, j int) bool { return cellLess(cells[i][0], cells[i][1], cells[j][0], cells[j][1]) })
	}
	for _, cells := range stacked {
		sort.Slice(cells, func(i, j int) bool { return cellLess(cells[i][0], cells[i][1], cells[j][0], cells[j][1]) })
	}
	return owned, stacked
}

// apartFrom reports whether place's mean height in a cell is at least stackGap
// from that of every place in floors.
func apartFrom(tallies map[string]cellPlace, floors []string, place string) bool {
	z := tallies[place].sumZ / float64(tallies[place].count)
	for _, other := range floors {
		if math.Abs(z-tallies[other].sumZ/float64(tallies[other].count)) < stackGap {
			return false
		}
	}
	return true
}

func cellLess(ax, ay, bx, by int) bool {
	return ax < bx || (ax == bx && ay < by)
}

// calloutAt names the owner of the cell holding a position, else of the closest
// owned neighbour, else ""; on a stacked cell the floor nearest z wins.
func calloutAt(g MapGeometry, level string, x, y, z float64) string {
	if g.CellSize <= 0 {
		return ""
	}
	cx, cy := int(math.Floor(x/g.CellSize)), int(math.Floor(y/g.CellSize))
	if name := cellOwner(g, level, cx, cy, z); name != "" {
		return name
	}
	best, bestDist := "", math.Inf(1)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			name := cellOwner(g, level, cx+dx, cy+dy, z)
			if name == "" {
				continue
			}
			d := math.Hypot(x-(float64(cx+dx)+0.5)*g.CellSize, y-(float64(cy+dy)+0.5)*g.CellSize)
			if d < bestDist || (d == bestDist && name < best) {
				best, bestDist = name, d
			}
		}
	}
	return best
}

func cellOwner(g MapGeometry, level string, cx, cy int, z float64) string {
	best, bestGap := "", math.Inf(1)
	for _, c := range g.Callouts {
		if c.Level != level {
			continue
		}
		i := sort.Search(len(c.Stacked), func(i int) bool { return !cellLess(c.Stacked[i][0], c.Stacked[i][1], cx, cy) })
		if i < len(c.Stacked) && c.Stacked[i][0] == cx && c.Stacked[i][1] == cy {
			if gap := math.Abs(z - float64(c.Stacked[i][2])); gap < bestGap || (gap == bestGap && c.Name < best) {
				best, bestGap = c.Name, gap
			}
		}
	}
	if best != "" {
		return best
	}
	for _, c := range g.Callouts {
		if c.Level != level {
			continue
		}
		i := sort.Search(len(c.Cells), func(i int) bool { return !cellLess(c.Cells[i][0], c.Cells[i][1], cx, cy) })
		if i < len(c.Cells) && c.Cells[i] == [2]int{cx, cy} {
			return c.Name
		}
	}
	return ""
}
