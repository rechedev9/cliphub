package tacticalplan

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

const (
	patternWalker = "76561198000000001"
	patternRival  = "76561198000000002"
	patternFreeze = 1000
)

// patternMatch is three T rounds of team "a" on a two-floor map (lower below
// z 0). In round 1 the walker crosses x 0..300 at 100 u/s on the upper floor,
// in round 2 the same line on the lower floor, in round 3 stays at x -500.
// The CT rival smokes (200,0) in round 1, thrown from (-400,0). The T win round 1 and 3.
func patternMatch(t *testing.T) SpatialMatch {
	t.Helper()
	lower := 0.0
	doc := Document{
		Demo:     Demo{Map: "de_nuke", Tickrate: 64},
		Teams:    []Team{{Key: "a", StartSide: SideT}, {Key: "b", StartSide: SideCT}},
		Players:  []Player{{Slot: 0, SteamID64: patternWalker, TeamKey: "a"}, {Slot: 1, SteamID64: patternRival, TeamKey: "b"}},
		Geometry: MapGeometry{Calibration: radarmap.Calibration{Map: "de_nuke", Scale: 7, Size: 1024, LowerAltitudeMax: &lower}},
	}
	var rounds []RoundFrames
	for n, z := range []float64{100, -100, 100} {
		r := Round{Number: n + 1, Half: 1, TickFreezeEnd: patternFreeze, TickEnd: patternFreeze + 64*30, Winner: SideT,
			Players: []PlayerRound{{Slot: 0, Side: SideT}, {Slot: 1, Side: SideCT}}}
		if n == 1 {
			r.Winner = SideCT
		}
		var frames []Frame
		for s := 0; s <= 10; s++ {
			x := float64(s) * 100
			if n == 2 {
				x = -500
			}
			frames = append(frames, Frame{Tick: patternFreeze + s*64, Samples: []Sample{
				{Slot: 0, X: x, Y: 0, Z: z, Flags: FlagAlive | FlagSideT},
				{Slot: 1, X: 900, Y: 900, Z: 100, Flags: FlagAlive},
			}})
		}
		if n == 0 {
			r.Events = []Event{{Tick: patternFreeze + 64*6, Kind: EventSmoke, ActorSlot: slotPtr(1), Pos: [3]float64{200, 0, 100},
				Throw: &Throw{Tick: patternFreeze + 64*4, Pos: [3]float64{-400, 0, 100}}}}
		}
		doc.Rounds = append(doc.Rounds, r)
		rounds = append(rounds, RoundFrames{Round: n + 1, Frames: frames})
	}
	blob, err := EncodePositions(rounds, 8, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	doc.Positions = blob.Descriptor
	return SpatialMatch{MatchID: uuid.New(), Document: doc, Filter: Filter{TeamKey: "a"}, Blob: func() ([]byte, error) { return blob.Data, nil }}
}

func runOne(t *testing.T, inputs []PatternInput, p Pattern) PatternResult {
	t.Helper()
	if err := p.Check(); err != nil {
		t.Fatalf("Check: %v", err)
	}
	return RunPatterns(inputs, []Pattern{p}, time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))[0]
}

func matchedRounds(r PatternResult) []int {
	var out []int
	for _, m := range r.Matches {
		for _, row := range m.Matched {
			out = append(out, row.Round)
		}
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPatternAreaKeepsItsFloor(t *testing.T) {
	m := patternMatch(t)
	inputs := []PatternInput{{Match: m}}
	box := PatternArea{MinX: 250, MinY: -50, MaxX: 350, MaxY: 50}
	anyFloor := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, Players: []string{patternWalker}, Area: &box}}})
	if got := matchedRounds(anyFloor); !equalInts(got, []int{1, 2}) {
		t.Fatalf("any floor rounds = %v, want 1 and 2", got)
	}
	upper := box
	upper.Level = radarmap.LevelDefault
	onUpper := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, Area: &upper}}})
	if got := matchedRounds(onUpper); !equalInts(got, []int{1}) {
		t.Fatalf("upper floor rounds = %v, want 1", got)
	}
	row := onUpper.Matches[0].Matched[0]
	if row.Seconds == nil || *row.Seconds != 3 || row.Tick != patternFreeze+3*64 {
		t.Fatalf("entry = %+v, want 3 s after freeze end", row)
	}
}

func TestPatternClockWindowAndInversion(t *testing.T) {
	inputs := []PatternInput{{Match: patternMatch(t)}}
	box := PatternArea{MinX: 250, MinY: -50, MaxX: 350, MaxY: 50}
	early := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, Area: &box, From: 0, To: 2}}})
	if early.Matched != 0 || early.Rounds != 3 {
		t.Fatalf("window 0-2 s = %d of %d, want 0 of 3: the walker reaches the box at 3 s", early.Matched, early.Rounds)
	}
	inWindow := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, Area: &box, From: 2, To: 5}}})
	if got := matchedRounds(inWindow); !equalInts(got, []int{1, 2}) {
		t.Fatalf("window 2-5 s rounds = %v", got)
	}
	inverted := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, Area: &box, Invert: true}}})
	if got := matchedRounds(inverted); !equalInts(got, []int{3}) {
		t.Fatalf("Rondas en que no pasa = %v, want 3", got)
	}
	if hit := inverted.Matches[0].Matched[0]; hit.Seconds != nil || !hit.Won {
		t.Fatalf("inverted row = %+v, want no entry time and a win", hit)
	}
}

func TestPatternBlocksAddUpAndCountApart(t *testing.T) {
	inputs := []PatternInput{{Match: patternMatch(t)}}
	box := PatternArea{MinX: 250, MinY: -50, MaxX: 350, MaxY: 50}
	smokeBox := PatternArea{MinX: 150, MinY: -50, MaxX: 250, MaxY: 50}
	player := PatternBlock{Kind: PatternPlayer, Area: &box}
	smoke := PatternBlock{Kind: PatternUtility, Rival: true, Grenade: EventSmoke, Area: &smokeBox, From: 0, To: 5}
	both := runOne(t, inputs, Pattern{Blocks: []PatternBlock{player, smoke}})
	if got := matchedRounds(both); !equalInts(got, []int{1}) {
		t.Fatalf("player and rival smoke = %v, want round 1", got)
	}
	if both.Blocks[0] != (PatternCount{Matched: 2, Rounds: 3}) || both.Blocks[1] != (PatternCount{Matched: 1, Rounds: 3}) {
		t.Fatalf("block counts = %+v", both.Blocks)
	}
	// The smoke lands at 6 s but leaves the hand at 4 s: the window reads the throw.
	if hits := both.Matches[0].Matched[0].Hits; hits[1].Seconds == nil || *hits[1].Seconds != 4 {
		t.Fatalf("smoke hit = %+v, want the throw at 4 s", hits[1])
	}
	ownSmoke := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternUtility, Area: &smokeBox}}})
	if ownSmoke.Matched != 0 {
		t.Fatalf("the team's own smokes = %d, want none: the rival threw it", ownSmoke.Matched)
	}
	fromThrow := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternUtility, Rival: true, FromThrow: true,
		Area: &PatternArea{MinX: -450, MinY: -50, MaxX: -350, MaxY: 50}}}})
	if fromThrow.Matched != 1 {
		t.Fatalf("thrown from the box = %d, want 1", fromThrow.Matched)
	}
}

func TestPatternWinRateAndTrend(t *testing.T) {
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	recent, old := patternMatch(t), patternMatch(t)
	inputs := []PatternInput{{Match: recent, PlayedAt: now.AddDate(0, 0, -3)}, {Match: old, PlayedAt: now.AddDate(0, -3, 0)}}
	box := PatternArea{MinX: 250, MinY: -50, MaxX: 350, MaxY: 50}
	got := runOne(t, inputs, Pattern{Side: SideT, Blocks: []PatternBlock{{Kind: PatternPlayer, Area: &box}}})
	if got.Matched != 4 || got.Won != 2 || got.WinRate != 0.5 {
		t.Fatalf("matched %d won %d rate %v, want 4, 2 and 0.5", got.Matched, got.Won, got.WinRate)
	}
	if ct := runOne(t, inputs, Pattern{Side: SideCT, Blocks: []PatternBlock{{Kind: PatternPlayer}}}); ct.Rounds != 0 || len(ct.Matches) != 0 {
		t.Fatalf("a CT pattern over T rounds = %+v, want nothing in scope", ct)
	}
	// Three rounds on each side of the cut are too few to read a trend.
	if got.RecentRounds != 3 || got.RecentMatched != 2 || got.Trend != "" {
		t.Fatalf("recent = %d of %d, trend %q", got.RecentMatched, got.RecentRounds, got.Trend)
	}
	if len(got.Months) != 4 || got.Months[0] != (PatternMonth{Month: "2026-08", Rounds: 3, Matched: 2}) || got.Months[1] != (PatternMonth{Month: "2026-09"}) {
		t.Fatalf("months = %+v, want August to November with the empty months at zero", got.Months)
	}
	if only := OnlyMatched(inputs, got); len(only) != 2 || len(only[0].Document.Rounds) != 2 || len(recent.Document.Rounds) != 3 {
		t.Fatalf("OnlyMatched kept %+v and must not touch the input", only)
	}
	for _, c := range []struct {
		recent, earlier PatternCount
		want            string
	}{
		{PatternCount{Matched: 5, Rounds: 10}, PatternCount{Matched: 2, Rounds: 10}, TrendUp},
		{PatternCount{Matched: 1, Rounds: 10}, PatternCount{Matched: 4, Rounds: 10}, TrendDown},
		{PatternCount{Matched: 3, Rounds: 10}, PatternCount{Matched: 3, Rounds: 11}, TrendFlat},
		{PatternCount{Matched: 3, Rounds: 10}, PatternCount{}, ""},
		{PatternCount{Matched: 9, Rounds: 9}, PatternCount{Matched: 0, Rounds: 30}, ""},
	} {
		if got := PatternTrend(c.recent, c.earlier); got != c.want {
			t.Errorf("PatternTrend(%+v, %+v) = %q, want %q", c.recent, c.earlier, got, c.want)
		}
	}
}

func TestPatternCheckRejectsBadBlocks(t *testing.T) {
	for name, p := range map[string]Pattern{
		"no blocks":     {},
		"bad side":      {Side: "X", Blocks: []PatternBlock{{Kind: PatternPlayer}}},
		"bad kind":      {Blocks: []PatternBlock{{Kind: "kill"}}},
		"bad player":    {Blocks: []PatternBlock{{Kind: PatternPlayer, Players: []string{"nick"}}}},
		"grenade":       {Blocks: []PatternBlock{{Kind: PatternPlayer, Grenade: EventSmoke}}},
		"empty area":    {Blocks: []PatternBlock{{Kind: PatternPlayer, Area: &PatternArea{MinX: 5, MaxX: 5, MaxY: 1}}}},
		"reversed time": {Blocks: []PatternBlock{{Kind: PatternPlayer, From: 20, To: 10}}},
	} {
		if err := p.Check(); err == nil {
			t.Errorf("%s: Check accepted %+v", name, p)
		}
	}
}

func TestPatternNamedPlayersAndPositionsSetTheScope(t *testing.T) {
	m := patternMatch(t)
	// Round 3 is played without the walker.
	m.Document.Rounds[2].Players = []PlayerRound{{Slot: 1, Side: SideCT}}
	inputs := []PatternInput{{Match: m}}
	box := PatternArea{MinX: 250, MinY: -50, MaxX: 350, MaxY: 50}
	notThere := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, Players: []string{patternWalker}, Area: &box, From: 0, To: 2, Invert: true}}})
	if notThere.Rounds != 2 || notThere.Matched != 2 {
		t.Fatalf("inverted block = %d of %d, want 2 of 2: the round without the walker is out of scope", notThere.Matched, notThere.Rounds)
	}
	stranger := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, Players: []string{patternRival}, Invert: true}}})
	if stranger.Rounds != 0 || len(stranger.Matches) != 0 {
		t.Fatalf("a player of the other team = %+v, want the match out of scope", stranger)
	}
	blind := m
	blind.Blob = nil
	noFrames := runOne(t, []PatternInput{{Match: blind}}, Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, Invert: true}}})
	if noFrames.Rounds != 0 || noFrames.MissingPositions != 1 {
		t.Fatalf("no positions = %+v, want no rounds and one match without positions", noFrames)
	}
	if plants := runOne(t, []PatternInput{{Match: blind}}, Pattern{Blocks: []PatternBlock{{Kind: PatternPlant, Invert: true}}}); plants.Rounds != 3 || plants.MissingPositions != 0 {
		t.Fatalf("an event block without positions = %+v, want every round", plants)
	}
	m.Document.Rounds[0].Events[0].Throw = nil
	lost := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternUtility, Rival: true, FromThrow: true}}})
	if lost.NoThrow != 1 || lost.Matched != 0 {
		t.Fatalf("a smoke without throw data = %+v, want it counted apart", lost)
	}
}
