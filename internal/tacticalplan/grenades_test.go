package tacticalplan

import (
	"reflect"
	"testing"
	"time"
)

func grenadeMatches() []ScoutMatch {
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
	return []ScoutMatch{
		{MatchID: scoutMatchOne, TeamKey: "t-start", PlayedAt: time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC), Document: older},
		{MatchID: scoutMatchTwo, TeamKey: "t-start", PlayedAt: time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC), Document: newer},
	}
}

func TestCollectTeamGrenadesListsTheTeamsGrenadesNewestMatchFirst(t *testing.T) {
	got := CollectTeamGrenades(grenadeMatches(), ScoutFilter{}, nil)
	if got.Rounds != 3 {
		t.Fatalf("rounds = %d, want 3", got.Rounds)
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
	origin, tick, view := [3]float64{-500, 200, 16}, 1400, [2]float64{45, 350}
	want := TeamGrenade{
		MatchID: scoutMatchOne, Round: 1, Tick: 1600, Seconds: 15, Kind: EventSmoke, Side: SideT,
		ThrowerSteamID64: "3", ThrowerName: "b1", Origin: &origin, ThrowTick: &tick, View: &view,
		Landing: [3]float64{990, 1010, 0}, Buy: BuyPistol, RoundWon: true,
	}
	if !reflect.DeepEqual(smoke, want) {
		t.Fatalf("smoke = %+v, want %+v", smoke, want)
	}
	if decoy := got.Grenades[2]; decoy.Origin != nil || decoy.ThrowTick != nil || decoy.View != nil {
		t.Fatalf("a grenade without a recorded throw has an origin: %+v", decoy)
	}
	// After half time the team is CT: its side, buy and result follow.
	if molly := got.Grenades[3]; molly.Side != SideCT || molly.Buy != BuyForce || molly.RoundWon || molly.Seconds != 10 {
		t.Fatalf("second-half molotov = %+v", molly)
	}
}

func TestCollectTeamGrenadesFiltersBySideAndKind(t *testing.T) {
	ct := CollectTeamGrenades(grenadeMatches(), ScoutFilter{Side: SideCT}, nil)
	if ct.Rounds != 1 || len(ct.Grenades) != 1 || ct.Grenades[0].Kind != EventMolotov {
		t.Fatalf("CT side = %+v, want the one molotov of one round", ct)
	}
	smokes := CollectTeamGrenades(grenadeMatches(), ScoutFilter{}, []EventKind{EventSmoke, EventHE})
	if smokes.Rounds != 3 || len(smokes.Grenades) != 2 {
		t.Fatalf("smokes and HEs = %+v, want 2 grenades over 3 rounds", smokes)
	}
	other := CollectTeamGrenades(grenadeMatches(), ScoutFilter{Maps: []string{"de_inferno"}}, nil)
	if other.Rounds != 0 || other.Grenades == nil || len(other.Grenades) != 0 {
		t.Fatalf("another map = %+v, want an empty list", other)
	}
}
