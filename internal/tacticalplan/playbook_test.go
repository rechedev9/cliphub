package tacticalplan

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

const pbRate = 64

type spot struct{ x, y float64 }

var (
	pbA      = spot{1000, 1000}
	pbB      = spot{-1000, -1000}
	pbMid    = spot{0, 0}
	pbTSpawn = spot{0, -3000}
)

// leg is where a player stands from one second after freeze end to another.
type leg struct {
	from, to float64
	at       spot
}

func pbGeometry() MapGeometry {
	return MapGeometry{Callouts: []Callout{
		{Name: "BombsiteA", Level: "default", Center: [2]float64{1000, 1000}},
		{Name: "BombsiteB", Level: "default", Center: [2]float64{-1000, -1000}},
		{Name: "Middle", Level: "default", Center: [2]float64{0, 0}},
		{Name: "TSpawn", Level: "default", Center: [2]float64{0, -3000}},
		{Name: "CTSpawn", Level: "default", Center: [2]float64{0, 3000}},
	}}
}

func pbFreezeEnd(n int) int { return n*10000 + 640 }

func pbTick(n int, secs float64) int { return pbFreezeEnd(n) + int(secs*pbRate) }

// pbRound is round n of the t-start team (slots 2 and 3): T in half 1, CT in half 2.
func pbRound(n, half int, class Class, buy BuyType, won bool) Round {
	teamSide, rival := SideT, SideCT
	if half == 2 {
		teamSide, rival = SideCT, SideT
	}
	winner := rival
	if won {
		winner = teamSide
	}
	r := Round{
		Number: n, Half: half, TickStart: n * 10000, TickFreezeEnd: pbFreezeEnd(n), TickEnd: pbTick(n, 90),
		Winner: winner, Class: class,
		Players: []PlayerRound{{Slot: 0, Side: rival}, {Slot: 1, Side: rival}, {Slot: 2, Side: teamSide}, {Slot: 3, Side: teamSide}},
	}
	if teamSide == SideT {
		r.Economy = Economy{TBuy: buy, CTBuy: BuyFull}
	} else {
		r.Economy = Economy{CTBuy: buy, TBuy: BuyFull}
	}
	return r
}

// pbFrames samples every half second from freeze end; a slot with no leg at
// a second is not sampled then.
func pbFrames(n int, seconds float64, paths map[uint8][]leg) []Frame {
	var out []Frame
	for s := 0.0; s <= seconds; s += 0.5 {
		f := Frame{Tick: pbTick(n, s)}
		for _, slot := range []uint8{2, 3} {
			for _, l := range paths[slot] {
				if s >= l.from && s < l.to {
					f.Samples = append(f.Samples, Sample{Slot: slot, X: l.at.x, Y: l.at.y, Flags: FlagAlive})
					break
				}
			}
		}
		out = append(out, f)
	}
	return out
}

func both(legs ...leg) map[uint8][]leg { return map[uint8][]leg{2: legs, 3: legs} }

func smokeAt(n int, secs float64, at spot) Event {
	return Event{Tick: pbTick(n, secs), Kind: EventSmoke, ActorSlot: slotPtr(2), Side: SideT, Pos: [3]float64{at.x, at.y, 0}}
}

// pbMatch is one match with the playbook rounds: seven T rounds, one of each
// category, and two CT rounds for the prep.
func pbMatch(t *testing.T) FramedMatch {
	t.Helper()
	doc := NewDocument()
	doc.Demo = Demo{Map: "de_mirage", Tickrate: pbRate}
	doc.Geometry = pbGeometry()
	doc.Teams = []Team{{Key: "t-start", Name: "Rival", StartSide: SideT}, {Key: "ct-start", Name: "Us", StartSide: SideCT}}
	doc.Players = []Player{
		{Slot: 0, SteamID64: "steam0", Name: "c0", TeamKey: "ct-start"}, {Slot: 1, SteamID64: "steam1", Name: "c1", TeamKey: "ct-start"},
		{Slot: 2, SteamID64: "steam2", Name: "t2", TeamKey: "t-start"}, {Slot: 3, SteamID64: "steam3", Name: "t3", TeamKey: "t-start"},
	}
	r4 := pbRound(4, 1, Class{TSide: TDefault, Site: SiteB}, BuyFull, false)
	r4.Bomb = &Bomb{PlantTick: pbTick(4, 70), Site: SiteB}
	r6 := pbRound(6, 1, Class{TSide: TDefault, Site: SiteA}, BuyFull, true)
	r6.Bomb = &Bomb{PlantTick: pbTick(6, 80), Site: SiteA}
	r7 := pbRound(7, 1, Class{TSide: TExecute, Site: SiteB}, BuyFull, true)
	r7.Events = []Event{smokeAt(7, 20, pbA), smokeAt(7, 22, pbA)}
	r8 := pbRound(8, 2, Class{CTSide: CTHold, Site: SiteA, FirstContactTick: pbTick(8, 30)}, BuyFull, true)
	doc.Rounds = []Round{
		pbRound(1, 1, Class{TSide: TDefault, Site: SiteB}, BuyFull, true),
		pbRound(2, 1, Class{TSide: TExecute, Site: SiteA}, BuyFull, false),
		pbRound(3, 1, Class{TSide: TDefault, Site: SiteB}, BuyFull, true),
		r4,
		pbRound(5, 1, Class{TSide: TSave, Site: SiteNone}, BuyEco, false),
		r6, r7, r8,
		pbRound(9, 2, Class{CTSide: CTStack, Site: SiteB}, BuyFull, false),
	}
	rounds := []RoundFrames{
		{Round: 1, Frames: pbFrames(1, 60, both(leg{0, 10, pbTSpawn}, leg{15, 90, pbB}))},
		{Round: 2, Frames: pbFrames(2, 60, both(leg{0, 5, pbTSpawn}, leg{5, 20, pbMid}, leg{50, 90, pbA}))},
		{Round: 3, Frames: pbFrames(3, 70, both(leg{0, 5, pbTSpawn}, leg{5, 30, pbMid}, leg{60, 90, pbB}))},
		{Round: 4, Frames: pbFrames(4, 70, both(leg{30, 35.5, pbA}, leg{60, 90, pbB}))},
		{Round: 5, Frames: pbFrames(5, 30, both(leg{0, 90, pbTSpawn}))},
		{Round: 7, Frames: pbFrames(7, 60, both(leg{50, 90, pbB}))},
		{Round: 8, Frames: pbFrames(8, 70, map[uint8][]leg{2: {{0, 90, pbA}}, 3: {{0, 25, pbMid}, {25, 90, pbB}}})},
		{Round: 9, Frames: pbFrames(9, 10, map[uint8][]leg{2: {{0, 90, pbMid}}})},
	}
	blob, err := EncodePositions(rounds, 32, pbRate)
	if err != nil {
		t.Fatal(err)
	}
	doc.Positions = blob.Descriptor
	return FramedMatch{
		ScoutMatch: ScoutMatch{MatchID: uuid.MustParse("00000000-0000-4000-8000-000000000001"), TeamKey: "t-start", PlayedAt: time.Unix(1, 0), Document: doc},
		Blob:       func() ([]byte, error) { return blob.Data, nil },
	}
}

func pbCategory(t *testing.T, pb Playbook, name string) PlaybookCategory {
	t.Helper()
	for _, c := range pb.Categories {
		if c.Category == name {
			return c
		}
	}
	t.Fatalf("category %s missing from %+v", name, pb.Categories)
	return PlaybookCategory{}
}

func refRounds(refs []Evidence) []int {
	out := []int{}
	for _, e := range refs {
		out = append(out, e.Round)
	}
	return out
}

func TestPlaybookClassifiesEachTRoundOnce(t *testing.T) {
	pb := BuildPlaybook([]FramedMatch{pbMatch(t)}, PlaybookOptions{Filter: ScoutFilter{Maps: []string{"de_mirage"}, Side: SideT}})
	if pb.Map != "de_mirage" || pb.RoundCount != 7 || pb.Framed != 6 || pb.WinRate.Count != 4 {
		t.Fatalf("playbook totals = map %s, %d rounds, %d framed, %d won", pb.Map, pb.RoundCount, pb.Framed, pb.WinRate.Count)
	}
	want := map[string][]int{
		PlaybookSave: {5}, PlaybookFake: {4, 7}, PlaybookRush: {1},
		PlaybookExecute: {2}, PlaybookMapControl: {3}, PlaybookDefault: {6},
	}
	if len(pb.Categories) != len(want) {
		t.Fatalf("categories = %+v", pb.Categories)
	}
	for name, rounds := range want {
		if got := refRounds(pbCategory(t, pb, name).Refs); !reflect.DeepEqual(got, rounds) {
			t.Errorf("%s rounds = %v, want %v", name, got, rounds)
		}
	}
	if fake := pbCategory(t, pb, PlaybookFake); fake.Share.Count != 2 || fake.Share.Total != 7 || fake.WinRate.Count != 1 {
		t.Errorf("fake group = %+v", fake.PlaybookGroup)
	}
	byRound := map[int]PlaybookRound{}
	for _, r := range pb.Rounds {
		byRound[r.Round] = r
	}
	if byRound[4].FakeSite != SiteA || byRound[7].FakeSite != SiteA || byRound[6].Framed {
		t.Errorf("rounds 4/6/7 = %+v / %+v / %+v", byRound[4], byRound[6], byRound[7])
	}
	if byRound[5].TempoSeconds != nil || *byRound[1].TempoSeconds != 15 || *byRound[6].TempoSeconds != 80 {
		t.Errorf("tempo of rounds 1/5/6, want 15 s from presence, none, 80 s from the plant")
	}
	if pb.Tempo.Samples != 6 || pb.Tempo.BucketSeconds != 5 || pb.Tempo.Buckets[3].Count != 1 {
		t.Errorf("tempo = %+v", pb.Tempo)
	}
}

func TestPlaybookCountsSitesAndZones(t *testing.T) {
	pb := BuildPlaybook([]FramedMatch{pbMatch(t)}, PlaybookOptions{Filter: ScoutFilter{Maps: []string{"de_mirage"}, Side: SideT}})
	sites := map[Site][]int{}
	for _, s := range pb.Sites {
		sites[s.Site] = refRounds(s.Refs)
	}
	if !reflect.DeepEqual(sites, map[Site][]int{SiteA: {2, 6}, SiteB: {1, 3, 4, 7}, SiteMid: {}}) {
		t.Errorf("sites = %v", sites)
	}
	if len(pb.Zones) != 3 || pb.Zones[0].Zone != "Middle" || pb.Zones[0].Rounds != 2 || pb.Zones[0].Share.Total != 6 {
		t.Fatalf("zones = %+v, want Middle first over the 6 framed rounds", pb.Zones)
	}
	if pb.Zones[1].Zone != "BombsiteA" || pb.Zones[1].Label != "Bombsite A" || !reflect.DeepEqual(refRounds(pb.Zones[1].Refs), []int{4}) {
		t.Errorf("second zone = %+v", pb.Zones[1])
	}
}

func TestPlaybookCTCategoriesFollowTheClassifier(t *testing.T) {
	pb := BuildPlaybook([]FramedMatch{pbMatch(t)}, PlaybookOptions{Filter: ScoutFilter{Side: SideCT}})
	if pb.RoundCount != 2 || pb.Map != "" {
		t.Fatalf("CT rounds = %d, map %q", pb.RoundCount, pb.Map)
	}
	if got := refRounds(pbCategory(t, pb, PlaybookStack).Refs); !reflect.DeepEqual(got, []int{9}) {
		t.Errorf("stack rounds = %v", got)
	}
	if got := refRounds(pbCategory(t, pb, PlaybookDefault).Refs); !reflect.DeepEqual(got, []int{8}) {
		t.Errorf("default rounds = %v", got)
	}
	if *pb.Rounds[0].TempoSeconds != 30 {
		t.Errorf("CT tempo = %v, want the first contact at 30 s", *pb.Rounds[0].TempoSeconds)
	}
}

func TestPlaybookWithoutBlobFallsBackToEvents(t *testing.T) {
	m := pbMatch(t)
	m.Blob = func() ([]byte, error) { return []byte("broken"), nil }
	pb := BuildPlaybook([]FramedMatch{m}, PlaybookOptions{Filter: ScoutFilter{Side: SideT}})
	if pb.Framed != 0 || len(pb.Zones) != 0 {
		t.Fatalf("a broken blob read %d framed rounds and zones %+v", pb.Framed, pb.Zones)
	}
	if got := refRounds(pbCategory(t, pb, PlaybookFake).Refs); !reflect.DeepEqual(got, []int{7}) {
		t.Errorf("fake rounds = %v, want only the utility fake", got)
	}
}
