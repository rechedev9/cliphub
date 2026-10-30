package tacticalplan

import (
	"fmt"
	"math"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

// Spatial rules for the multi-round replay. They travel with every heatmap and
// overlay, so a reader sees the rule next to the picture it produced.
const (
	// HeatmapCellSize is the heatmap grid in world units, the occupancy grid's default.
	HeatmapCellSize = 64
	// HeatmapStepSeconds is how often an alive player adds to the position heatmap.
	HeatmapStepSeconds = 1
	// HeatmapExampleLimit caps the example rounds kept for each cell.
	HeatmapExampleLimit = 3
	// OverlayStepSeconds is the spacing of the points of an overlaid track.
	OverlayStepSeconds = 0.5
	// OverlayRoundLimit caps how many rounds one overlay draws.
	OverlayRoundLimit = 40
)

// HeatmapKind is what a heatmap counts.
type HeatmapKind string

// Heatmap kinds: where the players stood, where they died, where their
// grenades landed, and how soon after freeze end they first reached a cell.
const (
	HeatPosition HeatmapKind = "position"
	HeatDeath    HeatmapKind = "death"
	HeatUtility  HeatmapKind = "utility"
	HeatTiming   HeatmapKind = "timing"
)

// HeatmapKinds lists the kinds in the order the UI offers them.
var HeatmapKinds = []HeatmapKind{HeatPosition, HeatDeath, HeatUtility, HeatTiming}

// SpatialMatch is one match a heatmap or an overlay reads: its document, the
// round filter (TeamKey set when one team is followed), and its position blob.
type SpatialMatch struct {
	MatchID  uuid.UUID
	Document Document
	Filter   Filter
	// Blob reads the zvpos1 sidecar when the builder reaches the match, so only
	// one blob is in memory at a time. Nil, or nil bytes, means the match has
	// none, which leaves the positions, timings and overlay tracks without it.
	Blob func() ([]byte, error)
}

// positions reads and decodes the header of a match's blob; ok is false when
// the match has none.
func (m SpatialMatch) positions() (blob []byte, header Positions, ok bool, err error) {
	if m.Blob == nil || m.Document.Demo.Tickrate <= 0 {
		return nil, Positions{}, false, nil
	}
	if blob, err = m.Blob(); err != nil || len(blob) == 0 {
		return nil, Positions{}, false, err
	}
	if header, err = DecodeHeader(blob); err != nil {
		return nil, Positions{}, false, fmt.Errorf("match %s: %w", m.MatchID, err)
	}
	return blob, header, true, nil
}

// SpatialWindow limits a heatmap to a span of seconds after freeze end. A zero
// To has no upper bound.
type SpatialWindow struct {
	From float64 `json:"from"`
	To   float64 `json:"to"`
}

func (w SpatialWindow) contains(seconds float64) bool {
	return seconds >= w.From && (w.To <= 0 || seconds <= w.To)
}

// HeatmapOptions selects the kind and, for utility, the grenades counted.
type HeatmapOptions struct {
	Kind     HeatmapKind
	Window   SpatialWindow
	Grenades []EventKind
}

// HeatmapCell is one cell of the grid on one side and one level. X and Y index
// the world grid: the cell covers [X*cell_size, (X+1)*cell_size) on each axis.
type HeatmapCell struct {
	Side   Side   `json:"side"`
	Level  string `json:"level"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Count  int    `json:"count"`
	Rounds int    `json:"rounds"`
	// Seconds is the median time after freeze end of what the cell counted;
	// for the timing kind it is the median first arrival.
	Seconds  float64    `json:"seconds"`
	Examples []Evidence `json:"examples"`
}

// Heatmap is a grid of counts over the selected rounds, split by side and level.
type Heatmap struct {
	Kind         HeatmapKind   `json:"kind"`
	Window       SpatialWindow `json:"window"`
	Grenades     []EventKind   `json:"grenades,omitempty"`
	CellSize     float64       `json:"cell_size"`
	StepSeconds  float64       `json:"step_seconds"`
	ExampleLimit int           `json:"example_limit"`
	Matches      int           `json:"matches"`
	Rounds       int           `json:"rounds"`
	// MissingPositions counts the matches with no position blob; they add
	// deaths and grenades but no positions or timings.
	MissingPositions int           `json:"missing_positions"`
	Sides            []HeatmapSide `json:"sides"`
	Cells            []HeatmapCell `json:"cells"`
}

// HeatmapSide is the total of one side and level, the scale its cells are read on.
type HeatmapSide struct {
	Side     Side   `json:"side"`
	Level    string `json:"level"`
	Total    int    `json:"total"`
	Rounds   int    `json:"rounds"`
	MaxCount int    `json:"max_count"`
}

type heatKey struct {
	side  Side
	level string
	x, y  int
}

type heatAcc struct {
	count     int
	rounds    int
	lastRound int
	seconds   []float64
	examples  []timedEvidence
}

type timedEvidence struct {
	seconds float64
	ev      Evidence
}

type heatmapBuilder struct {
	opts      HeatmapOptions
	cells     map[heatKey]*heatAcc
	sideTotal map[heatKey]*HeatmapSide
	sideRound map[heatKey]int
	roundSeq  int
}

// subjects maps the slots a round counts to the side each played: the team the
// filter follows, the side it fixes, or every player.
func subjects(d Document, f Filter, r Round) map[uint8]Side {
	side := f.Perspective(d, r)
	out := map[uint8]Side{}
	for _, pr := range r.Players {
		if side == "" || pr.Side == side {
			out[pr.Slot] = pr.Side
		}
	}
	return out
}

func roundEndTick(r Round) int {
	if r.TickEnd > 0 {
		return r.TickEnd
	}
	return r.TickOfficial
}

func cellIndex(v float64) int {
	return int(math.Floor(v / HeatmapCellSize))
}

func (b *heatmapBuilder) add(cal radarmap.Calibration, side Side, pos [3]float64, seconds float64, ev Evidence) {
	key := heatKey{side: side, level: cal.Level(pos[2]), x: cellIndex(pos[0]), y: cellIndex(pos[1])}
	acc, ok := b.cells[key]
	if !ok {
		acc = &heatAcc{}
		b.cells[key] = acc
	}
	acc.count++
	acc.seconds = append(acc.seconds, seconds)
	if acc.lastRound != b.roundSeq {
		acc.lastRound = b.roundSeq
		acc.rounds++
		if b.opts.Kind != HeatTiming && len(acc.examples) < HeatmapExampleLimit {
			acc.examples = append(acc.examples, timedEvidence{seconds, ev})
		}
	}
	if b.opts.Kind == HeatTiming {
		acc.examples = keepEarliest(acc.examples, timedEvidence{seconds, ev})
	}

	total := heatKey{side: side, level: key.level}
	sum, ok := b.sideTotal[total]
	if !ok {
		sum = &HeatmapSide{Side: side, Level: key.level}
		b.sideTotal[total] = sum
	}
	sum.Total++
	if b.sideRound[total] != b.roundSeq {
		b.sideRound[total] = b.roundSeq
		sum.Rounds++
	}
}

// keepEarliest keeps the HeatmapExampleLimit earliest arrivals; on a tie the
// one seen first stays, so the order is the matches' order.
func keepEarliest(list []timedEvidence, e timedEvidence) []timedEvidence {
	at := len(list)
	for i, item := range list {
		if e.seconds < item.seconds {
			at = i
			break
		}
	}
	if at >= HeatmapExampleLimit {
		return list
	}
	list = append(list, timedEvidence{})
	copy(list[at+1:], list[at:])
	list[at] = e
	if len(list) > HeatmapExampleLimit {
		list = list[:HeatmapExampleLimit]
	}
	return list
}

func (b *heatmapBuilder) grenade(kind EventKind) bool {
	if !isScoutUtility(kind) {
		return false
	}
	if len(b.opts.Grenades) == 0 {
		return true
	}
	for _, g := range b.opts.Grenades {
		if g == kind {
			return true
		}
	}
	return false
}

func (b *heatmapBuilder) addEvents(m SpatialMatch, r Round, team map[uint8]Side) {
	doc := m.Document
	cal := doc.Geometry.Calibration
	for _, ev := range r.Events {
		seconds, ok := secondsSince(r, ev.Tick, doc.Demo.Tickrate)
		if !ok || !b.opts.Window.contains(seconds) {
			continue
		}
		e := Evidence{MatchID: m.MatchID, Round: r.Number, Tick: ev.Tick}
		switch b.opts.Kind {
		case HeatDeath:
			if ev.Kind != EventKill || ev.TargetSlot == nil {
				continue
			}
			if side, ok := team[*ev.TargetSlot]; ok {
				b.add(cal, side, ev.TargetPos, seconds, e)
			}
		case HeatUtility:
			if !b.grenade(ev.Kind) || ev.ActorSlot == nil {
				continue
			}
			if side, ok := team[*ev.ActorSlot]; ok {
				b.add(cal, side, ev.Pos, seconds, e)
			}
		}
	}
}

// addFrames reads the round's samples: one per HeatmapStepSeconds for the
// position kind, and every frame for the first arrival of the timing kind.
func (b *heatmapBuilder) addFrames(m SpatialMatch, r Round, team map[uint8]Side, frames []Frame) {
	doc := m.Document
	cal := doc.Geometry.Calibration
	tickrate := doc.Demo.Tickrate
	end := roundEndTick(r)
	nextTick := r.TickFreezeEnd
	step := int(math.Round(HeatmapStepSeconds * tickrate))
	type arrival struct {
		slot uint8
		cell heatKey
	}
	arrived := map[arrival]bool{}
	for _, frame := range frames {
		if end > 0 && frame.Tick > end {
			break
		}
		seconds, ok := secondsSince(r, frame.Tick, tickrate)
		if !ok || !b.opts.Window.contains(seconds) {
			continue
		}
		if b.opts.Kind == HeatPosition {
			if frame.Tick < nextTick {
				continue
			}
			for nextTick <= frame.Tick {
				nextTick += max(step, 1)
			}
		}
		e := Evidence{MatchID: m.MatchID, Round: r.Number, Tick: frame.Tick}
		for _, s := range frame.Samples {
			side, ok := team[s.Slot]
			if !ok || !s.Flags.Has(FlagAlive) {
				continue
			}
			pos := [3]float64{s.X, s.Y, s.Z}
			if b.opts.Kind == HeatTiming {
				key := arrival{slot: s.Slot, cell: heatKey{side: side, level: cal.Level(s.Z), x: cellIndex(s.X), y: cellIndex(s.Y)}}
				if arrived[key] {
					continue
				}
				arrived[key] = true
			}
			b.add(cal, side, pos, seconds, e)
		}
	}
}

// roundFrames decodes one round of a match's blob, or nil when it has none.
func roundFrames(blob []byte, header Positions, doc Document, round int) ([]Frame, error) {
	offset, ok := doc.Positions.Offset(round)
	if !ok || offset.FrameCount == 0 {
		return nil, nil
	}
	return DecodeFrames(blob, offset.ByteOffset, offset.FrameCount, header)
}

// BuildHeatmap counts the selected rounds of every match into one grid. The
// matches are read in the given order, which is the order of the examples.
func BuildHeatmap(matches []SpatialMatch, opts HeatmapOptions) (Heatmap, error) {
	b := &heatmapBuilder{opts: opts, cells: map[heatKey]*heatAcc{}, sideTotal: map[heatKey]*HeatmapSide{}, sideRound: map[heatKey]int{}}
	out := Heatmap{
		Kind: opts.Kind, Window: opts.Window, Grenades: opts.Grenades, CellSize: HeatmapCellSize,
		StepSeconds: HeatmapStepSeconds, ExampleLimit: HeatmapExampleLimit, Matches: len(matches),
		Sides: []HeatmapSide{}, Cells: []HeatmapCell{},
	}
	positional := opts.Kind == HeatPosition || opts.Kind == HeatTiming
	for _, m := range matches {
		var blob []byte
		var header Positions
		hasBlob := false
		if positional {
			var err error
			if blob, header, hasBlob, err = m.positions(); err != nil {
				return Heatmap{}, err
			}
			if !hasBlob {
				out.MissingPositions++
			}
		}
		for _, r := range m.Filter.Apply(m.Document) {
			b.roundSeq++
			out.Rounds++
			team := subjects(m.Document, m.Filter, r)
			if !positional {
				b.addEvents(m, r, team)
				continue
			}
			if !hasBlob {
				continue
			}
			frames, err := roundFrames(blob, header, m.Document, r.Number)
			if err != nil {
				return Heatmap{}, fmt.Errorf("match %s round %d: %w", m.MatchID, r.Number, err)
			}
			b.addFrames(m, r, team, frames)
		}
	}

	for key, acc := range b.cells {
		cell := HeatmapCell{
			Side: key.side, Level: key.level, X: key.x, Y: key.y, Count: acc.count, Rounds: acc.rounds,
			Seconds: median(acc.seconds), Examples: make([]Evidence, 0, len(acc.examples)),
		}
		for _, e := range acc.examples {
			cell.Examples = append(cell.Examples, e.ev)
		}
		out.Cells = append(out.Cells, cell)
		if sum := b.sideTotal[heatKey{side: key.side, level: key.level}]; acc.count > sum.MaxCount {
			sum.MaxCount = acc.count
		}
	}
	sort.Slice(out.Cells, func(i, j int) bool {
		a, c := out.Cells[i], out.Cells[j]
		switch {
		case a.Side != c.Side:
			return a.Side < c.Side
		case a.Level != c.Level:
			return a.Level < c.Level
		case a.Y != c.Y:
			return a.Y < c.Y
		default:
			return a.X < c.X
		}
	})
	for _, sum := range b.sideTotal {
		out.Sides = append(out.Sides, *sum)
	}
	sort.Slice(out.Sides, func(i, j int) bool {
		if out.Sides[i].Side != out.Sides[j].Side {
			return out.Sides[i].Side < out.Sides[j].Side
		}
		return out.Sides[i].Level < out.Sides[j].Level
	})
	return out, nil
}

// OverlayTrack is one player's path through a round. Points[i] is the
// position Start+i steps after freeze end; the track ends where the player died.
type OverlayTrack struct {
	Slot      uint8    `json:"slot"`
	SteamID64 string   `json:"steamid64"`
	Name      string   `json:"name"`
	Side      Side     `json:"side"`
	Start     int      `json:"start"`
	Points    [][3]int `json:"points"`
	Died      bool     `json:"died"`
}

// OverlayEvent is a kill, grenade or plant placed on the shared clock.
type OverlayEvent struct {
	Seconds float64   `json:"seconds"`
	Tick    int       `json:"tick"`
	Kind    EventKind `json:"kind"`
	Side    Side      `json:"side,omitempty"`
	// Pos is where it happened: the victim for a kill, the landing for a grenade.
	Pos [3]int `json:"pos"`
	// Subject reports whether the followed players did it (or died, for a kill).
	Subject bool `json:"subject"`
}

// OverlayRound is one round on the shared clock that starts at freeze end.
type OverlayRound struct {
	MatchID         uuid.UUID      `json:"match_id"`
	Round           int            `json:"round"`
	Map             string         `json:"map"`
	Side            Side           `json:"side,omitempty"`
	Winner          Side           `json:"winner"`
	Buy             BuyType        `json:"buy,omitempty"`
	TPattern        TPattern       `json:"t_pattern"`
	CTPattern       CTPattern      `json:"ct_pattern"`
	Site            Site           `json:"site"`
	TickFreezeEnd   int            `json:"tick_freeze_end"`
	Tickrate        float64        `json:"tickrate"`
	DurationSeconds float64        `json:"duration_seconds"`
	Tracks          []OverlayTrack `json:"tracks"`
	Events          []OverlayEvent `json:"events"`
}

// Overlay is a set of rounds synchronised from freeze end.
type Overlay struct {
	StepSeconds float64 `json:"step_seconds"`
	Limit       int     `json:"limit"`
	// Total is how many rounds matched before the limit cut the list.
	Total            int            `json:"total"`
	MissingPositions int            `json:"missing_positions"`
	Rounds           []OverlayRound `json:"rounds"`
}

func intPos(p [3]float64) [3]int {
	return [3]int{int(math.Round(p[0])), int(math.Round(p[1])), int(math.Round(p[2]))}
}

// BuildOverlay lays the selected rounds on one clock, at most limit of them, in
// the matches' order and then by round number.
func BuildOverlay(matches []SpatialMatch, limit int) (Overlay, error) {
	if limit <= 0 || limit > OverlayRoundLimit {
		limit = OverlayRoundLimit
	}
	out := Overlay{StepSeconds: OverlayStepSeconds, Limit: limit, Rounds: []OverlayRound{}}
	for _, m := range matches {
		doc := m.Document
		blob, header, hasBlob, err := m.positions()
		if err != nil {
			return Overlay{}, err
		}
		if !hasBlob {
			out.MissingPositions++
		}
		for _, r := range m.Filter.Apply(doc) {
			out.Total++
			if len(out.Rounds) >= limit {
				continue
			}
			round := overlayRound(m, r)
			if hasBlob {
				frames, err := roundFrames(blob, header, doc, r.Number)
				if err != nil {
					return Overlay{}, fmt.Errorf("match %s round %d: %w", m.MatchID, r.Number, err)
				}
				round.Tracks = overlayTracks(m, r, frames)
			}
			out.Rounds = append(out.Rounds, round)
		}
	}
	return out, nil
}

func overlayRound(m SpatialMatch, r Round) OverlayRound {
	doc := m.Document
	side := m.Filter.Perspective(doc, r)
	out := OverlayRound{
		MatchID: m.MatchID, Round: r.Number, Map: doc.Demo.Map, Side: side, Winner: r.Winner,
		TPattern: r.Class.TSide, CTPattern: r.Class.CTSide, Site: r.Class.Site,
		TickFreezeEnd: r.TickFreezeEnd, Tickrate: doc.Demo.Tickrate,
		Tracks: []OverlayTrack{}, Events: []OverlayEvent{},
	}
	if side != "" {
		out.Buy = r.Economy.Buy(side)
	}
	if doc.Demo.Tickrate > 0 && roundEndTick(r) > r.TickFreezeEnd {
		out.DurationSeconds = math.Round(float64(roundEndTick(r)-r.TickFreezeEnd)/doc.Demo.Tickrate*10) / 10
	}
	team := subjects(doc, m.Filter, r)
	for _, ev := range r.Events {
		seconds, ok := secondsSince(r, ev.Tick, doc.Demo.Tickrate)
		if !ok {
			continue
		}
		item := OverlayEvent{Seconds: math.Round(seconds*100) / 100, Tick: ev.Tick, Kind: ev.Kind, Side: ev.Side}
		switch {
		case ev.Kind == EventKill && ev.TargetSlot != nil:
			item.Pos = intPos(ev.TargetPos)
			_, item.Subject = team[*ev.TargetSlot]
			if victim, ok := playerSide(r, *ev.TargetSlot); ok {
				item.Side = victim
			}
		case isScoutUtility(ev.Kind) || ev.Kind == EventPlant:
			item.Pos = intPos(ev.Pos)
			if ev.ActorSlot != nil {
				_, item.Subject = team[*ev.ActorSlot]
			}
		default:
			continue
		}
		out.Events = append(out.Events, item)
	}
	return out
}

func playerSide(r Round, slot uint8) (Side, bool) {
	for _, pr := range r.Players {
		if pr.Slot == slot {
			return pr.Side, true
		}
	}
	return "", false
}

// overlayTracks samples each followed player every OverlayStepSeconds from
// freeze end: the first frame at or after each step, until death or round end.
func overlayTracks(m SpatialMatch, r Round, frames []Frame) []OverlayTrack {
	doc := m.Document
	team := subjects(doc, m.Filter, r)
	slots := make([]uint8, 0, len(team))
	for slot := range team {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
	tracks := make([]OverlayTrack, 0, len(slots))
	index := map[uint8]int{}
	for _, slot := range slots {
		steamID, name := "", fmt.Sprintf("slot %d", slot)
		if p, ok := doc.PlayerBySlot(slot); ok {
			steamID, name = p.SteamID64, p.Name
		}
		index[slot] = len(tracks)
		tracks = append(tracks, OverlayTrack{Slot: slot, SteamID64: steamID, Name: name, Side: team[slot], Start: -1, Points: [][3]int{}})
	}

	end := roundEndTick(r)
	stepTicks := OverlayStepSeconds * doc.Demo.Tickrate
	step := 0
	for _, frame := range frames {
		if end > 0 && frame.Tick > end {
			break
		}
		if float64(frame.Tick) < float64(r.TickFreezeEnd)+float64(step)*stepTicks {
			continue
		}
		for float64(frame.Tick) >= float64(r.TickFreezeEnd)+float64(step+1)*stepTicks {
			step++
		}
		for _, s := range frame.Samples {
			i, ok := index[s.Slot]
			if !ok || tracks[i].Died {
				continue
			}
			t := &tracks[i]
			if !s.Flags.Has(FlagAlive) {
				t.Died = t.Start >= 0
				continue
			}
			if t.Start < 0 {
				t.Start = step
			}
			for len(t.Points) < step-t.Start {
				// A gap in the samples repeats the last point, so index stays time.
				t.Points = append(t.Points, t.Points[len(t.Points)-1])
			}
			t.Points = append(t.Points, intPos([3]float64{s.X, s.Y, s.Z}))
		}
		step++
	}
	out := tracks[:0]
	for _, t := range tracks {
		if t.Start >= 0 {
			out = append(out, t)
		}
	}
	return out
}
