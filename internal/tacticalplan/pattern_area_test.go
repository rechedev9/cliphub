package tacticalplan

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

// crowdMatch is three T rounds of team "a" with three players. Round 1: all three stand at
// (300,0) from 3 s. Round 2: two of them stand there from 2 to 4 s and the third only from
// 6 to 8 s, so never three at once. Round 3: one of them walks through at 5 s.
func crowdMatch(t *testing.T) SpatialMatch {
	t.Helper()
	doc := Document{
		Demo:  Demo{Map: "de_ancient", Tickrate: 64},
		Teams: []Team{{Key: "a", StartSide: SideT}, {Key: "b", StartSide: SideCT}},
		Players: []Player{{Slot: 0, SteamID64: "76561198000000010", TeamKey: "a"}, {Slot: 1, SteamID64: "76561198000000011", TeamKey: "a"},
			{Slot: 2, SteamID64: "76561198000000012", TeamKey: "a"}, {Slot: 3, SteamID64: "76561198000000013", TeamKey: "b"}},
		Geometry: MapGeometry{Calibration: radarmap.Calibration{Map: "de_ancient", Scale: 5, Size: 1024}},
	}
	inside := func(round, slot, s int) bool {
		switch round {
		case 0:
			return s >= 3
		case 1:
			if slot == 2 {
				return s >= 6 && s <= 8
			}
			return s >= 2 && s <= 4
		default:
			return slot == 0 && s == 5
		}
	}
	var rounds []RoundFrames
	for n := range 3 {
		r := Round{Number: n + 1, Half: 1, TickFreezeEnd: patternFreeze, TickEnd: patternFreeze + 64*30, Winner: SideT,
			Players: []PlayerRound{{Slot: 0, Side: SideT}, {Slot: 1, Side: SideT}, {Slot: 2, Side: SideT}, {Slot: 3, Side: SideCT}}}
		var frames []Frame
		for s := 0; s <= 10; s++ {
			samples := []Sample{{Slot: 3, X: 900, Y: 900, Flags: FlagAlive}}
			for slot := range 3 {
				x := -800.0
				if inside(n, slot, s) {
					x = 300
				}
				samples = append(samples, Sample{Slot: uint8(slot), X: x, Y: float64(slot) * 10, Flags: FlagAlive | FlagSideT})
			}
			frames = append(frames, Frame{Tick: patternFreeze + s*64, Samples: samples})
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

func TestPatternMinPlayersCountsTogether(t *testing.T) {
	inputs := []PatternInput{{Match: crowdMatch(t)}}
	siteA := PatternArea{MinX: 250, MinY: -50, MaxX: 350, MaxY: 50}
	for _, c := range []struct {
		need int
		want []int
	}{{0, []int{1, 2, 3}}, {2, []int{1, 2}}, {3, []int{1}}} {
		got := runOne(t, inputs, Pattern{Side: SideT, Blocks: []PatternBlock{{Kind: PatternPlayer, MinPlayers: c.need, Area: &siteA}}})
		if rounds := matchedRounds(got); !equalInts(rounds, c.want) {
			t.Errorf("min_players %d: rounds %v, want %v", c.need, rounds, c.want)
		}
	}
	// "A Finish": three or more T in A before 1:00 on the round clock (55 s after freeze end).
	finish := runOne(t, inputs, Pattern{Side: SideT, Blocks: []PatternBlock{{Kind: PatternPlayer, MinPlayers: 3, Area: &siteA, To: 55}}})
	if row := finish.Matches[0].Matched[0]; finish.Matched != 1 || row.Seconds == nil || *row.Seconds != 3 {
		t.Fatalf("A Finish = %+v, want round 1 entered at 3 s", finish)
	}
	late := runOne(t, inputs, Pattern{Side: SideT, Blocks: []PatternBlock{{Kind: PatternPlayer, MinPlayers: 3, Area: &siteA, To: 2}}})
	if late.Matched != 0 {
		t.Fatalf("three in A by 2 s = %d rounds, want none", late.Matched)
	}
}

func TestPatternMinPlayersSkipsRoundsWithoutEnoughNamedPlayers(t *testing.T) {
	m := crowdMatch(t)
	// Round 3 is played without the third named player, so "the three of them" cannot hold in it.
	m.Document.Rounds[2].Players = slices.DeleteFunc(slices.Clone(m.Document.Rounds[2].Players), func(p PlayerRound) bool { return p.Slot == 2 })
	inputs := []PatternInput{{Match: m}}
	siteA := PatternArea{MinX: 250, MinY: -50, MaxX: 350, MaxY: 50}
	named := []string{"76561198000000010", "76561198000000011", "76561198000000012"}
	got := runOne(t, inputs, Pattern{Side: SideT, Blocks: []PatternBlock{{Kind: PatternPlayer, Players: named, MinPlayers: 3, Area: &siteA}}})
	if got.Rounds != 2 || !equalInts(matchedRounds(got), []int{1}) {
		t.Fatalf("three named in A = %d rounds read, matched %v; want 2 read, round 1", got.Rounds, matchedRounds(got))
	}
	inverted := runOne(t, inputs, Pattern{Side: SideT, Blocks: []PatternBlock{{Kind: PatternPlayer, Players: named, MinPlayers: 3, Area: &siteA, Invert: true}}})
	if !equalInts(matchedRounds(inverted), []int{2}) {
		t.Fatalf("inverted = %v, want round 2 only", matchedRounds(inverted))
	}
}

func TestPatternPolygonArea(t *testing.T) {
	// An L: the vertical arm covers x 150..230 at y 0, the foot runs east below y -50.
	ell := [][2]float64{{150, -100}, {400, -100}, {400, -50}, {230, -50}, {230, 100}, {150, 100}}
	for _, c := range []struct {
		x, y float64
		want bool
	}{{200, 0, true}, {300, -75, true}, {300, 0, false}, {100, 0, false}} {
		if got := inPolygon(ell, c.x, c.y); got != c.want {
			t.Errorf("inPolygon(%v, %v) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
	inputs := []PatternInput{{Match: patternMatch(t)}}
	got := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, Players: []string{patternWalker}, Area: &PatternArea{Polygon: ell}}}})
	if rounds := matchedRounds(got); !equalInts(rounds, []int{1, 2}) {
		t.Fatalf("L polygon rounds = %v, want 1 and 2", rounds)
	}
	if row := got.Matches[0].Matched[0]; row.Seconds == nil || *row.Seconds != 2 {
		t.Fatalf("entry = %+v, want 2 s: the walker reaches x 200 then and x 300 is outside the L", row)
	}
	upper := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, Area: &PatternArea{Polygon: ell, Level: radarmap.LevelDefault}}}})
	if rounds := matchedRounds(upper); !equalInts(rounds, []int{1}) {
		t.Fatalf("upper floor polygon rounds = %v, want 1", rounds)
	}
}

func TestPatternCalloutArea(t *testing.T) {
	m := patternMatch(t)
	m.Document.Geometry.CellSize = 100
	m.Document.Geometry.Callouts = []Callout{
		{Name: "BombsiteA", Level: radarmap.LevelDefault, Center: [2]float64{350, 50}, Cells: [][2]int{{3, 0}}},
		{Name: "Ramp", Level: radarmap.LevelLower, Center: [2]float64{350, 50}, Cells: [][2]int{{3, 0}}},
		{Name: "TSpawn", Level: radarmap.LevelDefault, Center: [2]float64{-500, 0}, Cells: [][2]int{{-5, 0}}},
	}
	inputs := []PatternInput{{Match: m}}
	siteA := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, Players: []string{patternWalker}, Area: &PatternArea{Callouts: []string{"BombsiteA"}}}}})
	if rounds := matchedRounds(siteA); !equalInts(rounds, []int{1}) {
		t.Fatalf("BombsiteA rounds = %v, want 1: round 2 is the floor below, round 3 in T spawn", rounds)
	}
	both := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, Area: &PatternArea{Callouts: []string{"BombsiteA", "Ramp"}}}}})
	if rounds := matchedRounds(both); !equalInts(rounds, []int{1, 2}) {
		t.Fatalf("BombsiteA or Ramp rounds = %v, want 1 and 2", rounds)
	}
	smoke := runOne(t, inputs, Pattern{Blocks: []PatternBlock{{Kind: PatternUtility, Rival: true, Area: &PatternArea{Callouts: []string{"TSpawn"}}}}})
	if smoke.Matched != 0 {
		t.Fatalf("a smoke landing by BombsiteA counted in TSpawn: %+v", smoke)
	}
}

func TestPatternCheckRejectsBadAreasAndCounts(t *testing.T) {
	tri := [][2]float64{{0, 0}, {10, 0}, {0, 10}}
	for name, b := range map[string]PatternBlock{
		"count on utility":      {Kind: PatternUtility, MinPlayers: 2},
		"count past a side":     {Kind: PatternPlayer, MinPlayers: 6},
		"count past the named":  {Kind: PatternPlayer, MinPlayers: 2, Players: []string{patternWalker}},
		"two-point polygon":     {Kind: PatternPlayer, Area: &PatternArea{Polygon: tri[:2]}},
		"polygon and callouts":  {Kind: PatternPlayer, Area: &PatternArea{Polygon: tri, Callouts: []string{"BombsiteA"}}},
		"callouts with a floor": {Kind: PatternPlayer, Area: &PatternArea{Callouts: []string{"BombsiteA"}, Level: radarmap.LevelLower}},
		"empty callout":         {Kind: PatternPlayer, Area: &PatternArea{Callouts: []string{""}}},
	} {
		if err := (Pattern{Blocks: []PatternBlock{b}}).Check(); err == nil {
			t.Errorf("%s: Check accepted %+v", name, b)
		}
	}
	if err := (Pattern{Blocks: []PatternBlock{{Kind: PatternPlayer, MinPlayers: 3, Area: &PatternArea{Polygon: tri}}}}).Check(); err != nil {
		t.Fatalf("a polygon with a count: %v", err)
	}
}

// TestPatternReadsAddUpLikeARun is what lets a stored read stand in for a fresh run.
func TestPatternReadsAddUpLikeARun(t *testing.T) {
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	inputs := []PatternInput{{Match: patternMatch(t), PlayedAt: now.AddDate(0, 0, -2)}, {Match: crowdMatch(t), PlayedAt: now.AddDate(0, -2, 0)}}
	p := Pattern{Side: SideT, Blocks: []PatternBlock{{Kind: PatternPlayer, Area: &PatternArea{MinX: 250, MinY: -50, MaxX: 350, MaxY: 50}}}}
	runner := NewPatternRunner([]Pattern{p}, now)
	for _, in := range inputs {
		runner.Add(in)
	}
	want := runner.Finish()[0]
	raw, err := json.Marshal(runner.Reads()[0])
	if err != nil {
		t.Fatal(err)
	}
	var stored []PatternMatchRead
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	got := CombinePatternReads(PatternCombine{Reads: stored, Blocks: 1, Now: now})
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if !reflect.DeepEqual(gotJSON, wantJSON) || want.Matched != 5 {
		t.Fatalf("stored reads = %s\nwant %s", gotJSON, wantJSON)
	}
}
