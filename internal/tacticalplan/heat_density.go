package tacticalplan

import (
	"hash/fnv"
	"math"
	"math/rand/v2"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

// As in cs2lens, a round is read at fixed instants of the tramo, and every mark
// also lands on a fine grid over the radar that the web turns into density contours.
const (
	// HeatmapSamplesPerRound is how many instants of a round the position heatmap reads.
	HeatmapSamplesPerRound = 50
	// HeatDensityGrid is the side of the density grid over the radar square.
	HeatDensityGrid = 150
)

// HeatDensity is one side and level of the density grid: the non-empty squares
// of a HeatDensityGrid grid over the radar square, row 0 at the radar's top.
type HeatDensity struct {
	Side  Side   `json:"side"`
	Level string `json:"level"`
	// Bins are [column, row, count].
	Bins [][3]int `json:"bins"`
	// Seconds is each bin's mean time after freeze end, in Bins order; only the timing kind sends it.
	Seconds []float64 `json:"seconds,omitempty"`
}

type densityKey struct {
	side        Side
	level       string
	column, row int
}

type densityAcc struct {
	count   int
	seconds float64
}

// densityCell is the square of the density grid under a position, false off the radar.
func (b *heatmapBuilder) densityCell(pos [3]float64) (column, row int, ok bool) {
	r := b.radar
	if !r.Valid() {
		return 0, 0, false
	}
	span := float64(r.Size) * r.Scale
	column = int(math.Floor((pos[0] - r.PosX) / span * HeatDensityGrid))
	row = int(math.Floor((r.PosY - pos[1]) / span * HeatDensityGrid))
	ok = column >= 0 && row >= 0 && column < HeatDensityGrid && row < HeatDensityGrid
	return column, row, ok
}

// density lists the grid by side and level, each in row then column order.
func (b *heatmapBuilder) density() []HeatDensity {
	layers := map[heatKey]*HeatDensity{}
	keys := make([]densityKey, 0, len(b.bins))
	for key := range b.bins {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, c := keys[i], keys[j]
		switch {
		case a.side != c.side:
			return a.side < c.side
		case a.level != c.level:
			return a.level < c.level
		case a.row != c.row:
			return a.row < c.row
		default:
			return a.column < c.column
		}
	})
	out := []HeatDensity{}
	for _, key := range keys {
		id := heatKey{side: key.side, level: key.level}
		layer, ok := layers[id]
		if !ok {
			out = append(out, HeatDensity{Side: key.side, Level: key.level, Bins: [][3]int{}})
			layer = &out[len(out)-1]
			layers[id] = layer
		}
		acc := b.bins[key]
		layer.Bins = append(layer.Bins, [3]int{key.column, key.row, acc.count})
		if b.opts.Kind == HeatTiming {
			layer.Seconds = append(layer.Seconds, math.Round(acc.seconds/float64(acc.count)*10)/10)
		}
	}
	return out
}

// densityRadar is the radar the grid covers: the caller's, else the first match's.
func densityRadar(matches []SpatialMatch, asked radarmap.Calibration) radarmap.Calibration {
	if asked.Valid() || len(matches) == 0 {
		return asked
	}
	return matches[0].Document.Geometry.Calibration
}

// span is the part of a round the window keeps, in seconds after freeze end;
// false when nothing of the round is in it.
func (w SpatialWindow) span(r Round, tickrate float64) (from, to float64, ok bool) {
	end, ok := secondsSince(r, roundEndTick(r), tickrate)
	if !ok {
		return 0, 0, false
	}
	from, to = w.From, end
	if w.FromPlant {
		if r.Bomb == nil || r.Bomb.PlantTick <= 0 {
			return 0, 0, false
		}
		if from, ok = secondsSince(r, r.Bomb.PlantTick, tickrate); !ok {
			return 0, 0, false
		}
	}
	if w.To > 0 && w.To < to {
		to = w.To
	}
	return from, to, to > from
}

// roundSpan is the part of one round a heatmap reads, in seconds after freeze end.
type roundSpan struct {
	match    uuid.UUID
	round    int
	from, to float64
}

// instants spreads HeatmapSamplesPerRound instants over the span, each nudged up to a quarter
// of the spacing by a jitter seeded by the match and round, so a heatmap never changes.
func (s roundSpan) instants() []float64 {
	n := HeatmapSamplesPerRound
	h := fnv.New64a()
	_, _ = h.Write(s.match[:])
	rng := rand.New(rand.NewPCG(h.Sum64(), uint64(s.round)))
	step := (s.to - s.from) / float64(n)
	out := make([]float64, n)
	for i := range out {
		out[i] = s.from + (float64(i)+0.5+(rng.Float64()-0.5)*0.5)*step
	}
	return out
}

// framesAt answers the two frames around a tick and how far between them it
// sits; false before the first frame or after the last.
func framesAt(frames []Frame, tick float64) (Frame, Frame, float64, bool) {
	i := sort.Search(len(frames), func(i int) bool { return float64(frames[i].Tick) >= tick })
	switch {
	case i == len(frames):
		return Frame{}, Frame{}, 0, false
	case float64(frames[i].Tick) == tick:
		return frames[i], frames[i], 0, true
	case i == 0:
		return Frame{}, Frame{}, 0, false
	}
	a, b := frames[i-1], frames[i]
	return a, b, (tick - float64(a.Tick)) / float64(b.Tick-a.Tick), true
}

// addInstants reads the position heatmap of one round: at each instant, every
// counted player alive in both frames around it, at the position between them.
func (b *heatmapBuilder) addInstants(m SpatialMatch, r Round, team map[uint8]Side, frames []Frame) {
	doc := m.Document
	tickrate := doc.Demo.Tickrate
	from, to, ok := b.opts.Window.span(r, tickrate)
	if !ok {
		return
	}
	for _, seconds := range (roundSpan{match: m.MatchID, round: r.Number, from: from, to: to}).instants() {
		tick := float64(r.TickFreezeEnd) + seconds*tickrate
		before, after, k, ok := framesAt(frames, tick)
		if !ok {
			continue
		}
		e := Evidence{MatchID: m.MatchID, Round: r.Number, Tick: int(math.Round(tick))}
		for _, s := range before.Samples {
			side, counted := team[s.Slot]
			if !counted || !s.Flags.Has(FlagAlive) {
				continue
			}
			next, found := sampleOf(after, s.Slot)
			if !found || !next.Flags.Has(FlagAlive) {
				continue
			}
			pos := [3]float64{s.X + (next.X-s.X)*k, s.Y + (next.Y-s.Y)*k, s.Z + (next.Z-s.Z)*k}
			b.add(doc.Geometry.Calibration, side, pos, seconds, e)
		}
	}
}

func sampleOf(frame Frame, slot uint8) (Sample, bool) {
	for _, s := range frame.Samples {
		if s.Slot == slot {
			return s, true
		}
	}
	return Sample{}, false
}
