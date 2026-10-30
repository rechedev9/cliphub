package tacticalplan

import (
	"reflect"
	"testing"
	"time"
)

var playGeometry = MapGeometry{Callouts: []Callout{
	{Name: "BombsiteA", Level: "default", Center: [2]float64{1000, 1000}},
	{Name: "BombsiteB", Level: "default", Center: [2]float64{-1000, -1000}},
	{Name: "Middle", Level: "default", Center: [2]float64{0, 0}},
}}

// playRoundFixture is a T round of the t-start team (slots 2 and 3) at a
// given base tick, with the grenades it threw.
func playRoundFixture(number, base int, buy BuyType, site Site, pattern TPattern, events ...Event) Round {
	return Round{
		Number: number, Half: 1, TickStart: base, TickFreezeEnd: base + 640, TickEnd: base + 7000,
		Winner: SideT, Economy: Economy{TBuy: buy, CTBuy: BuyFull},
		Class: Class{TSide: pattern, CTSide: CTHold, Site: site, Tags: []string{}, Reasons: []string{}},
		Players: []PlayerRound{
			{Slot: 0, Side: SideCT}, {Slot: 1, Side: SideCT},
			{Slot: 2, Side: SideT}, {Slot: 3, Side: SideT},
		},
		Events: events,
	}
}

func grenade(kind EventKind, tick int, slot uint8, x, y float64) Event {
	return Event{Tick: tick, Kind: kind, ActorSlot: slotPtr(slot), Side: SideT, Pos: [3]float64{x, y, 0}}
}

// aExecute is the repeated play: a smoke and a flash on A from both players.
func aExecute(number, base int) Round {
	return playRoundFixture(number, base, BuyFull, SiteA, TExecute,
		grenade(EventSmoke, base+1600, 2, 990, 1010),
		grenade(EventFlash, base+1700, 3, 1020, 980),
	)
}

func playDocument(rounds ...Round) Document {
	d := testDocument()
	d.Geometry = playGeometry
	d.Rounds = rounds
	return d
}

func atA(frameTick int) Frame {
	return Frame{Tick: frameTick, Samples: []Sample{
		{Slot: 2, X: 1000, Y: 990, Flags: FlagAlive | FlagSideT},
		{Slot: 3, X: 990, Y: 1000, Flags: FlagAlive | FlagSideT},
		{Slot: 0, X: -1000, Y: -1000, Flags: FlagAlive},
	}}
}

func playMatches() []ScoutMatch {
	one := playDocument(
		aExecute(1, 0),
		aExecute(2, 10000),
		playRoundFixture(3, 20000, BuyFull, SiteB, TExecute,
			grenade(EventSmoke, 21600, 2, -990, -1000), grenade(EventMolotov, 21700, 3, -1010, -990)),
		// An eco with the same utility is not compared.
		playRoundFixture(4, 30000, BuyEco, SiteA, TEcoRush,
			grenade(EventSmoke, 31600, 2, 990, 1010), grenade(EventFlash, 31700, 3, 1020, 980)),
	)
	two := playDocument(
		aExecute(1, 0),
		// One grenade is not a play.
		playRoundFixture(2, 10000, BuyFull, SiteA, TFast, grenade(EventFlash, 11600, 2, 1000, 1000)),
	)
	return []ScoutMatch{
		{
			MatchID: scoutMatchOne, TeamKey: "t-start", PlayedAt: time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC), Document: one,
			Setup: map[int]Frame{1: atA(1600), 2: atA(11600)},
		},
		{
			MatchID: scoutMatchTwo, TeamKey: "t-start", PlayedAt: time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC), Document: two,
			Setup: map[int]Frame{1: atA(1600)},
		},
	}
}

func TestDetectPlaysGroupsRepeatedRoundsAcrossMatches(t *testing.T) {
	det := DetectPlays(playMatches(), ScoutFilter{})
	if det.Matches != 2 || det.Rounds != 6 || det.Compared != 4 {
		t.Fatalf("matches/rounds/compared = %d/%d/%d, want 2/6/4", det.Matches, det.Rounds, det.Compared)
	}
	if len(det.Plays) != 1 {
		t.Fatalf("plays = %+v, want the A execute only (the B round has no repeat)", det.Plays)
	}
	p := det.Plays[0]
	if p.Map != "de_mirage" || p.Side != SideT || p.Kind != string(TExecute) || p.Site != SiteA || p.Rounds != 3 {
		t.Fatalf("play = %+v", p)
	}
	if p.Share.Count != 3 || p.Share.Total != 6 || p.WinRate.Count != 3 {
		t.Fatalf("share/win = %+v / %+v", p.Share, p.WinRate)
	}
	// The newest match comes first, so its round seeds the play.
	want := []Evidence{
		{MatchID: scoutMatchTwo, Round: 1, Tick: 640},
		{MatchID: scoutMatchOne, Round: 1, Tick: 640},
		{MatchID: scoutMatchOne, Round: 2, Tick: 10640},
	}
	if p.Seed != want[0] || !reflect.DeepEqual(p.Examples, want) {
		t.Fatalf("seed/examples = %+v / %+v, want %+v", p.Seed, p.Examples, want)
	}
	if len(p.Grenades) != 2 || p.Grenades[0].Kind != EventSmoke || p.Grenades[0].Callout != "BombsiteA" ||
		p.Grenades[0].Share != 1 || p.Grenades[0].Seconds != 15 || p.Grenades[1].Kind != EventFlash {
		t.Fatalf("grenades = %+v", p.Grenades)
	}
	if len(p.Positions) != 1 || p.Positions[0].Callout != "BombsiteA" || p.Positions[0].Share != 1 {
		t.Fatalf("positions = %+v, want both T players on A merged into one spot", p.Positions)
	}
}

func TestDetectPlaysIsDeterministicAndFollowsTheFilter(t *testing.T) {
	ms := playMatches()
	first := DetectPlays(ms, ScoutFilter{})
	reversed := []ScoutMatch{ms[1], ms[0]}
	if again := DetectPlays(reversed, ScoutFilter{}); !reflect.DeepEqual(first, again) {
		t.Fatalf("input order changed the result:\n%+v\n%+v", first, again)
	}
	if ct := DetectPlays(ms, ScoutFilter{Side: SideCT}); ct.Rounds != 0 || len(ct.Plays) != 0 {
		t.Fatalf("CT detection = %+v, want no rounds", ct)
	}
	if one := DetectPlays(ms, ScoutFilter{Opponents: []string{"nobody"}}); one.Matches != 0 {
		t.Fatalf("rival filter = %+v, want no matches", one)
	}
}

func TestDetectPlaysSplitsRoundsThatHoldOtherSpots(t *testing.T) {
	ms := playMatches()
	// The same utility from players standing in B is a different play.
	ms[0].Setup[2] = Frame{Tick: 11600, Samples: []Sample{
		{Slot: 2, X: -1000, Y: -1000, Flags: FlagAlive | FlagSideT},
		{Slot: 3, X: -990, Y: -1000, Flags: FlagAlive | FlagSideT},
	}}
	if det := DetectPlays(ms, ScoutFilter{}); len(det.Plays) != 0 {
		t.Fatalf("plays = %+v, want none: only two rounds are still alike", det.Plays)
	}
	// Without positions only the utility is compared.
	ms[0].Setup = nil
	ms[1].Setup = nil
	if det := DetectPlays(ms, ScoutFilter{}); len(det.Plays) != 1 || len(det.Plays[0].Positions) != 0 {
		t.Fatalf("plays without positions = %+v", det.Plays)
	}
}

func TestMultisetSimilarity(t *testing.T) {
	a := map[string]int{"smoke@A": 2, "flash@A": 1}
	b := map[string]int{"smoke@A": 1, "flash@A": 1, "he@B": 1}
	if got := multisetSimilarity(a, b); got != 0.5 {
		t.Fatalf("similarity = %v, want 2 shared of 4", got)
	}
	if got := multisetSimilarity(map[string]int{}, map[string]int{}); got != 0 {
		t.Fatalf("empty similarity = %v, want 0", got)
	}
}

func TestSketchRoundStopsAtThePlantAndReadsTheFrame(t *testing.T) {
	r := aExecute(1, 0)
	r.Bomb = &Bomb{PlantTick: 2000, Site: SiteA}
	r.Events = append(r.Events,
		grenade(EventMolotov, 2500, 2, 1000, 1000), // post-plant
		grenade(EventSmoke, 1800, 0, 0, 0),         // the rival's
		grenade(EventFlash, 1700, 3, 1020, 980),    // the same flash reported twice
		Event{Tick: 1900, Kind: EventKill, ActorSlot: slotPtr(2)},
	)
	doc := playDocument(r)
	frame := atA(1600)
	s, err := SketchRound(doc, r, "t-start", &frame)
	if err != nil {
		t.Fatal(err)
	}
	if s.Side != SideT || s.Kind != string(TExecute) || s.Site != SiteA || s.PositionSeconds != 15 {
		t.Fatalf("sketch = %+v", s)
	}
	if len(s.Grenades) != 2 || s.Grenades[0].Player != "b1" || s.Grenades[1].Player != "b2" {
		t.Fatalf("grenades = %+v, want the two pre-plant T grenades", s.Grenades)
	}
	if len(s.Positions) != 2 || s.Positions[0].SteamID64 != "3" || s.Positions[0].Callout != "BombsiteA" {
		t.Fatalf("positions = %+v, want the two T players", s.Positions)
	}
	if _, err := SketchRound(doc, r, "spectators", nil); err == nil {
		t.Fatal("SketchRound(unknown team) = nil error")
	}
	if ct, _ := SketchRound(doc, r, "ct-start", nil); ct.Side != SideCT || len(ct.Grenades) != 1 || len(ct.Positions) != 0 {
		t.Fatalf("CT sketch = %+v, want its one smoke and no positions", ct)
	}
}
