package tacticalplan

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func aliveSample(x, y float64) Sample { return Sample{Slot: 0, X: x, Y: y, Flags: FlagAlive} }

func windowFrames(samples ...Sample) []Frame {
	out := make([]Frame, len(samples))
	for i, s := range samples {
		out[i] = Frame{Tick: i * 8, Samples: []Sample{s}}
	}
	return out
}

// positionsMatch has player "p1" in slot 0 on CT for rounds 1 to 4 and on T in round 5.
func positionsMatch() PositionMatch {
	d := NewDocument()
	d.Demo.Map = "de_mirage"
	d.Players = []Player{{Slot: 0, SteamID64: "p1"}, {Slot: 1, SteamID64: "p2"}}
	d.Geometry = MapGeometry{Callouts: []Callout{
		{Name: "BombsiteA", Level: "default", Center: [2]float64{1000, 1000}},
		{Name: "BombsiteB", Level: "default", Center: [2]float64{-1000, -1000}},
		{Name: "Middle", Level: "default", Center: [2]float64{0, 0}},
	}}
	round := func(n int, side Side, events ...Event) Round {
		return Round{Number: n, Players: []PlayerRound{{Slot: 0, Side: side}, {Slot: 1, Side: side.Opponent()}}, Events: events}
	}
	d.Rounds = []Round{
		round(1, SideCT, Event{Kind: EventPlant, Site: SiteA, Pos: [3]float64{900, 900, 0}}),
		round(2, SideCT),
		round(3, SideCT),
		round(4, SideCT),
		round(5, SideT,
			Event{Kind: EventKill, ActorSlot: slotPtr(0), Weapon: "awp"},
			Event{Kind: EventKill, ActorSlot: slotPtr(0), Weapon: "ak47"},
			Event{Kind: EventKill, ActorSlot: slotPtr(1), Weapon: "awp"},
		),
	}
	dead := Sample{Slot: 0, X: 1000, Y: 1000}
	return PositionMatch{
		MatchID: uuid.MustParse("00000000-0000-0000-0000-0000000000c1"), PlayedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Document: d,
		Opening: map[int][]Frame{
			1: windowFrames(aliveSample(-1000, -1000), aliveSample(-990, -1000), aliveSample(-1000, -990), aliveSample(0, 0)),
			2: windowFrames(aliveSample(-1000, -1000), aliveSample(-1000, -1000)),
			// A tie between Middle and A goes to the place reached later.
			3: windowFrames(aliveSample(0, 0), aliveSample(1000, 1000)),
			5: windowFrames(aliveSample(0, 0), aliveSample(10, 0), dead, dead, dead),
		},
	}
}

func TestPlayerPositionsReadsTheMostSeenPlacePerRound(t *testing.T) {
	report := PlayerPositions("p1", []PositionMatch{positionsMatch()})
	if report.MatchCount != 1 || len(report.Maps) != 1 || report.WindowSeconds != PositionWindowSeconds {
		t.Fatalf("report = %+v", report)
	}
	m := report.Maps[0]
	if m.Map != "de_mirage" || m.Matches != 1 || len(m.Sides) != 2 {
		t.Fatalf("map = %+v", m)
	}
	ct, tSide := m.Sides[0], m.Sides[1]
	if ct.Side != SideCT || ct.Rounds != 4 || ct.Sampled != 3 {
		t.Fatalf("CT = %d rounds, %d sampled; round 4 has no frames", ct.Rounds, ct.Sampled)
	}
	if ct.Role != PositionAnchorB {
		t.Fatalf("CT role = %q, want B anchor with 2 of 3 rounds on B", ct.Role)
	}
	wantRate(t, "zone A", ct.Zones[0].Rounds, 1, 3)
	wantRate(t, "zone B", ct.Zones[1].Rounds, 2, 3)
	wantRate(t, "zone other", ct.Zones[2].Rounds, 0, 3)
	if len(ct.Places) != 2 || ct.Places[0].Place != "BombsiteB" || ct.Places[0].Label != "Bombsite B" || ct.Places[1].Place != "BombsiteA" {
		t.Fatalf("CT places = %+v", ct.Places)
	}
	wantRate(t, "B place", ct.Places[0].Rounds, 2, 3)
	if len(ct.Places[0].Examples) != 2 || ct.Places[0].Examples[0].Round != 1 {
		t.Fatalf("B examples = %+v", ct.Places[0].Examples)
	}

	if tSide.Role != "" || tSide.Sampled != 1 || len(tSide.Places) != 1 || tSide.Places[0].Place != "Middle" {
		t.Fatalf("T = %+v, want Middle with no role; dead samples on A do not count", tSide)
	}
	wantRate(t, "T AWP kills", tSide.AWPKills, 1, 2)
	wantRate(t, "T AWP rounds", tSide.AWPRounds, 1, 1)
	wantRate(t, "CT AWP kills", ct.AWPKills, 0, 0)

	if none := PlayerPositions("p9", []PositionMatch{positionsMatch()}); none.MatchCount != 0 || len(none.Maps) != 0 {
		t.Fatalf("a player outside the matches = %+v", none)
	}
}

func TestPlayerPositionsRotatorAndPlaceLimit(t *testing.T) {
	m := positionsMatch()
	callouts := []string{"Jungle", "Connector", "Palace", "Ramp", "Underpass", "Apartments"}
	xs := []float64{3000, 4000, 5000, 6000, 7000, 8000}
	m.Document.Rounds = nil
	m.Opening = map[int][]Frame{}
	for i, name := range callouts {
		m.Document.Geometry.Callouts = append(m.Document.Geometry.Callouts, Callout{Name: name, Level: "default", Center: [2]float64{xs[i], 0}})
		m.Document.Rounds = append(m.Document.Rounds, Round{Number: i + 1, Players: []PlayerRound{{Slot: 0, Side: SideCT}}})
		m.Opening[i+1] = windowFrames(aliveSample(xs[i], 0))
	}
	ct := PlayerPositions("p1", []PositionMatch{m}).Maps[0].Sides[0]
	if ct.Role != PositionRotator || len(ct.Places) != PositionPlaceLimit || ct.Places[0].Place != "Apartments" {
		t.Fatalf("CT = role %q with %d places, first %+v", ct.Role, len(ct.Places), ct.Places)
	}
}

func TestOpeningFramesKeepsTheWindowAfterFreezeEnd(t *testing.T) {
	end := 1000 + PositionWindowSeconds*64
	blob, err := EncodePositions([]RoundFrames{{Round: 1, Frames: []Frame{
		{Tick: 992, Samples: []Sample{aliveSample(0, 0)}},
		{Tick: 1000, Samples: []Sample{aliveSample(1, 0)}},
		{Tick: end, Samples: []Sample{aliveSample(2, 0)}},
		{Tick: end + 8, Samples: []Sample{aliveSample(3, 0)}},
	}}}, 8, 64)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDocument()
	d.Demo.Tickrate = 64
	d.Rounds = []Round{{Number: 1, TickFreezeEnd: 1000, TickEnd: end + 100}, {Number: 2, TickFreezeEnd: 5000}}
	d.Positions = blob.Descriptor
	got, err := OpeningFrames(blob.Data, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[1]) != 2 || got[1][0].Tick != 1000 || got[1][1].Tick != end {
		t.Fatalf("opening frames = %+v", got)
	}
	if _, err := OpeningFrames([]byte("nope"), d); err == nil {
		t.Fatal("a broken blob must fail")
	}
}
