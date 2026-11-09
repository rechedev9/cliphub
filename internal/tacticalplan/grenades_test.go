package tacticalplan

import (
	"reflect"
	"testing"
	"time"
)

func grenadeMatches() []FramedMatch {
	thrown := grenade(EventSmoke, 1600, 2, 990, 1010)
	thrown.Throw = &Throw{Tick: 1400, Pos: [3]float64{-500, 200, 16}, Yaw: 45, Pitch: 350}
	rival := grenade(EventFlash, 1650, 0, 0, 0)
	rival.Side = SideCT
	// The second half: the t-start team (slots 2 and 3) defends and loses.
	second := playRoundFixture(13, 20000, BuyFull, SiteNone, TDefault, grenade(EventMolotov, 21280, 3, 5, 5))
	second.Half, second.Winner = 2, SideT
	second.Economy = Economy{CTBuy: BuyForce, TBuy: BuyFull}
	second.Players = []PlayerRound{{Slot: 0, Side: SideT}, {Slot: 1, Side: SideT}, {Slot: 2, Side: SideCT}, {Slot: 3, Side: SideCT}}

	older := playDocument(
		playRoundFixture(1, 0, BuyPistol, SiteA, TExecute, thrown, rival, grenade(EventDecoy, 1700, 3, 1, 2)),
		second,
	)
	newer := playDocument(playRoundFixture(1, 0, BuyEco, SiteB, TEcoRush, grenade(EventHE, 704, 2, 7, 8)))
	return []FramedMatch{
		{ScoutMatch: ScoutMatch{MatchID: scoutMatchOne, TeamKey: "t-start", PlayedAt: time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC), Document: older}},
		{ScoutMatch: ScoutMatch{MatchID: scoutMatchTwo, TeamKey: "t-start", PlayedAt: time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC), Document: newer}},
	}
}

func TestCollectTeamGrenadesListsTheTeamsGrenadesNewestMatchFirst(t *testing.T) {
	got := CollectTeamGrenades(grenadeMatches(), ScoutFilter{}, GrenadeQuery{})
	if got.Rounds != 3 || len(got.RoundList) != 3 {
		t.Fatalf("rounds = %d listed %d, want 3", got.Rounds, len(got.RoundList))
	}
	kinds := make([]EventKind, 0, len(got.Grenades))
	for _, g := range got.Grenades {
		kinds = append(kinds, g.Kind)
	}
	// The rival's flash is not the team's.
	if want := []EventKind{EventHE, EventSmoke, EventDecoy, EventMolotov}; !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}

	smoke := got.Grenades[1]
	if smoke.LandPlace != "BombsiteA" || smoke.ThrowPlace != "Middle" || smoke.Spot == "" || smoke.After.Zones == nil {
		t.Fatalf("smoke places, spot and after = %q %q %q %+v", smoke.LandPlace, smoke.ThrowPlace, smoke.Spot, smoke.After)
	}
	smoke.LandPlace, smoke.ThrowPlace, smoke.Spot, smoke.After = "", "", "", GrenadeAfter{}
	origin, tick, view := [3]float64{-500, 200, 16}, 1400, [2]float64{45, 350}
	want := TeamGrenade{
		MatchID: scoutMatchOne, Round: 1, Tick: 1600, Seconds: 15, Kind: EventSmoke, Side: SideT,
		ThrowerSteamID64: "3", ThrowerName: "b1", Origin: &origin, ThrowTick: &tick, View: &view,
		Landing: [3]float64{990, 1010, 0}, Buy: BuyPistol, RoundWon: true,
	}
	if !reflect.DeepEqual(smoke, want) {
		t.Fatalf("smoke = %+v, want %+v", smoke, want)
	}
	if decoy := got.Grenades[2]; decoy.Origin != nil || decoy.ThrowTick != nil || decoy.View != nil || decoy.ThrowPlace != "" {
		t.Fatalf("a grenade without a recorded throw has an origin: %+v", decoy)
	}
	// After half time the team is CT: its side, buy and result follow.
	if molly := got.Grenades[3]; molly.Side != SideCT || molly.Buy != BuyForce || molly.RoundWon || molly.Seconds != 10 {
		t.Fatalf("second-half molotov = %+v", molly)
	}
	if r := got.RoundList[2]; r.Side != SideCT || r.Buy != BuyForce || r.OpponentBuy != BuyFull || r.Won || r.Tick != 20640 {
		t.Fatalf("second-half round = %+v", r)
	}
}

func TestCollectTeamGrenadesFiltersBySideKindAndRivals(t *testing.T) {
	ct := CollectTeamGrenades(grenadeMatches(), ScoutFilter{Side: SideCT}, GrenadeQuery{})
	if ct.Rounds != 1 || len(ct.Grenades) != 1 || ct.Grenades[0].Kind != EventMolotov {
		t.Fatalf("CT side = %+v, want the one molotov of one round", ct)
	}
	smokes := CollectTeamGrenades(grenadeMatches(), ScoutFilter{}, GrenadeQuery{Kinds: []EventKind{EventSmoke, EventHE}})
	if smokes.Rounds != 3 || len(smokes.Grenades) != 2 {
		t.Fatalf("smokes and HEs = %+v, want 2 grenades over 3 rounds", smokes)
	}
	other := CollectTeamGrenades(grenadeMatches(), ScoutFilter{Maps: []string{"de_inferno"}}, GrenadeQuery{})
	if other.Rounds != 0 || other.Grenades == nil || len(other.Grenades) != 0 {
		t.Fatalf("another map = %+v, want an empty list", other)
	}
	// The rivals' flash in the team's T rounds, from the rivals' CT side.
	rivals := CollectTeamGrenades(grenadeMatches(), ScoutFilter{Side: SideT}, GrenadeQuery{Rivals: true})
	if rivals.Rounds != 2 || len(rivals.Grenades) != 1 || rivals.Grenades[0].Kind != EventFlash || rivals.Grenades[0].Side != SideCT || rivals.Grenades[0].RoundWon {
		t.Fatalf("rivals = %+v, want the CT flash of a round they lost", rivals)
	}
	if r := rivals.RoundList[0]; r.Side != SideCT || r.Won {
		t.Fatalf("rivals' round = %+v, want CT and lost", r)
	}
}

func TestGrenadeAfterReadsWhereTheTeamStayedAndFought(t *testing.T) {
	m := pbMatch(t)
	doc := m.Document
	// Round 2: both T players stand in Middle from 5 s to 20 s; the smoke goes off at 8 s.
	r2, _ := doc.RoundByNumber(2)
	smoke := smokeAt(2, 8, pbA)
	kill := Event{Tick: pbTick(2, 12), Kind: EventKill, ActorSlot: slotPtr(0), TargetSlot: slotPtr(2), Pos: [3]float64{1000, 1000, 0}, TargetPos: [3]float64{0, 0, 0}}
	late := Event{Tick: pbTick(2, 40), Kind: EventKill, ActorSlot: slotPtr(3), TargetSlot: slotPtr(1), Pos: [3]float64{1000, 1000, 0}}
	r2.Events = []Event{smoke, late, kill}
	for i := range doc.Rounds {
		if doc.Rounds[i].Number == 2 {
			doc.Rounds[i] = r2
		}
	}
	m.Document = doc
	got := CollectTeamGrenades([]FramedMatch{m}, ScoutFilter{Side: SideT}, GrenadeQuery{Kinds: []EventKind{EventSmoke}})
	var after *GrenadeAfter
	for i, g := range got.Grenades {
		if g.Round == 2 {
			after = &got.Grenades[i].After
		}
	}
	if after == nil {
		t.Fatalf("no smoke in round 2: %+v", got.Grenades)
	}
	// The first duel of the window is the death in Middle; the kill at 40 s is outside it.
	if !reflect.DeepEqual(after.Zones, []string{"Middle"}) || after.Fight != "Middle" {
		t.Fatalf("after = %+v, want Middle held and the first duel there", *after)
	}
}

func TestStayedZonesNeedsAPlayerToStay(t *testing.T) {
	frame := func(secs float64, at map[uint8]string) teamFrame { return teamFrame{seconds: secs, callouts: at} }
	track := []teamFrame{
		frame(9, map[uint8]string{2: "Long"}),
		frame(10, map[uint8]string{2: "Middle", 3: "TSpawn"}),
		frame(11, map[uint8]string{2: "Middle", 3: "TSpawn"}),
		frame(13, map[uint8]string{2: "Middle", 3: "TSpawn"}),
		// Slot 3 dies, so the stay in Ramp starts again when it is back.
		frame(14, map[uint8]string{2: "Ramp", 3: "Ramp"}),
		frame(16, map[uint8]string{2: "Ramp"}),
		frame(17, map[uint8]string{2: "Ramp", 3: "Ramp"}),
		frame(40, map[uint8]string{2: "Long"}),
	}
	if got := stayedZones(track, 10, 30); !reflect.DeepEqual(got, []string{"Middle", "Ramp"}) {
		t.Fatalf("zones = %v, want Middle and Ramp (spawn and the zones outside the window left out)", got)
	}
	if got := stayedZones(track, 10, 12); len(got) != 0 {
		t.Fatalf("zones in 2 s = %v, want none", got)
	}
}

func TestAssignSpotsGroupsRepeatedThrows(t *testing.T) {
	at := func(round int, origin, landing [3]float64) TeamGrenade {
		return TeamGrenade{MatchID: scoutMatchOne, Round: round, Tick: round * 100, Kind: EventSmoke, ThrowerSteamID64: "1", Origin: &origin, Landing: landing}
	}
	gs := []TeamGrenade{
		at(1, [3]float64{0, 0, 10}, [3]float64{500, 500, 0}),
		at(2, [3]float64{40, 0, 10}, [3]float64{560, 520, 0}),
		// Thrown from elsewhere to the same landing: another lineup.
		at(3, [3]float64{400, 0, 10}, [3]float64{520, 500, 0}),
		// Same spot one floor down.
		at(4, [3]float64{0, 0, 10}, [3]float64{500, 500, -400}),
		{MatchID: scoutMatchOne, Round: 5, Tick: 500, Kind: EventSmoke, Landing: [3]float64{510, 490, 0}},
		{MatchID: scoutMatchOne, Round: 6, Tick: 600, Kind: EventFlash, Landing: [3]float64{500, 500, 0}},
		// Thrown from the floor below to the same landing.
		at(7, [3]float64{0, 0, -390}, [3]float64{500, 500, 0}),
	}
	assignSpots(gs)
	if gs[0].Spot != gs[1].Spot || gs[0].Spot != gs[4].Spot {
		t.Fatalf("spots = %q %q %q, want one lineup and the throw without origin in it", gs[0].Spot, gs[1].Spot, gs[4].Spot)
	}
	for _, other := range []int{2, 3, 5, 6} {
		if gs[other].Spot == gs[0].Spot {
			t.Fatalf("throw %d joined the first spot: %+v", other, gs[other])
		}
	}
	if want := "smoke:" + grenadeID(gs[0]); gs[0].Spot != want {
		t.Fatalf("key = %q, want %q", gs[0].Spot, want)
	}
}

func TestFollowUpCountsEachRoundOnceOverTheSameRounds(t *testing.T) {
	rounds := []GrenadeRound{
		{MatchID: scoutMatchOne, Round: 1, Won: true, Category: PlaybookExecute, Site: SiteA},
		{MatchID: scoutMatchOne, Round: 2, Won: false, Category: PlaybookExecute, Site: SiteA},
		{MatchID: scoutMatchOne, Round: 3, Won: true, Category: PlaybookDefault, Site: SiteB},
		{MatchID: scoutMatchOne, Round: 4, Won: true, Category: PlaybookDefault, Site: SiteB},
	}
	thrown := func(round int, fight string, zones ...string) TeamGrenade {
		return TeamGrenade{MatchID: scoutMatchOne, Round: round, After: GrenadeAfter{Zones: zones, Fight: fight}}
	}
	got := FollowUpOf([]TeamGrenade{
		thrown(1, "Middle", "Middle", "Firebox"),
		// A second throw in round 1 does not count it twice.
		thrown(1, "Ramp", "Ramp"),
		thrown(2, "Middle", "Firebox"),
		thrown(3, "Middle"),
	}, rounds)
	if got.Rounds != 3 || got.Won != 2 {
		t.Fatalf("follow-up = %d rounds, %d won; want 3 and 2", got.Rounds, got.Won)
	}
	want := []FollowUpFact{
		{Kind: FollowUpFight, Value: "Middle", Rounds: 3, Won: 2},
		{Kind: FollowUpPlay, Value: "execute@A", Rounds: 2, Won: 1},
		{Kind: FollowUpZone, Value: "Firebox", Rounds: 2, Won: 1},
		{Kind: FollowUpPlay, Value: "default@B", Rounds: 1, Won: 1},
		{Kind: FollowUpZone, Value: "Middle", Rounds: 1, Won: 1},
	}
	if !reflect.DeepEqual(got.Facts, want) {
		t.Fatalf("facts = %+v\nwant %+v", got.Facts, want)
	}
	spots := GrenadeSpots([]TeamGrenade{{Spot: "a", Round: 1, MatchID: scoutMatchOne, LandPlace: "Ramp"}, {Spot: "b", Round: 2, MatchID: scoutMatchOne}, {Spot: "b", Round: 3, MatchID: scoutMatchOne}}, rounds)
	if len(spots) != 2 || spots[0].Key != "b" || spots[0].FollowUp.Rounds != 2 || spots[1].LandPlace != "Ramp" {
		t.Fatalf("spots = %+v, want b first with its 2 rounds", spots)
	}
}
