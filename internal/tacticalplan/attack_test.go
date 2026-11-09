package tacticalplan

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func attackOf(t *testing.T) AttackPrep {
	t.Helper()
	return BuildAttack([]FramedMatch{pbMatch(t)}, AttackOptions{Filter: ScoutFilter{Maps: []string{"de_mirage"}}})
}

func cellRounds(cells []AttackCell) map[string][]int {
	out := map[string][]int{}
	for _, c := range cells {
		out[c.Key] = refRounds(c.Refs)
	}
	return out
}

func TestAttackCrossesSitesAndTempoWithBuyMatchups(t *testing.T) {
	prep := attackOf(t)
	if prep.Map != "de_mirage" || prep.RoundCount != 7 || prep.Framed != 6 {
		t.Fatalf("totals = map %s, %d rounds, %d framed", prep.Map, prep.RoundCount, prep.Framed)
	}
	wantSites := map[string][]int{AttackSiteA: {2, 6}, AttackSiteB: {1, 3, 7}, AttackSiteBoth: {4}, AttackSiteNone: {5}}
	if got := cellRounds(prep.Sites); !reflect.DeepEqual(got, wantSites) {
		t.Errorf("sites = %v, want %v", got, wantSites)
	}
	wantTempo := map[string][]int{TempoRush: {1}, TempoSetStrat: {2, 4, 7}, TempoMidRound: {3, 6}, TempoNoSite: {5}}
	if got := cellRounds(prep.Tempo); !reflect.DeepEqual(got, wantTempo) {
		t.Errorf("tempo = %v, want %v", got, wantTempo)
	}
	if len(prep.Matchups) != 2 {
		t.Fatalf("matchups = %+v", prep.Matchups)
	}
	eco, full := prep.Matchups[0], prep.Matchups[1]
	if eco.Buy != BandEco || eco.OpponentBuy != BandFull || eco.Rounds != 1 || eco.Share.Total != 7 {
		t.Errorf("first matchup = %+v, want eco against full", eco.PrepRounds)
	}
	if full.Buy != BandFull || full.OpponentBuy != BandFull || full.Rounds != 6 || full.WinRate.Count != 4 {
		t.Errorf("second matchup = %+v, want full against full, 4 of 6 won", full.PrepRounds)
	}
	fullSites := map[string][]int{AttackSiteA: {2, 6}, AttackSiteB: {1, 3, 7}, AttackSiteBoth: {4}, AttackSiteNone: {}}
	if got := cellRounds(full.Sites); !reflect.DeepEqual(got, fullSites) {
		t.Errorf("full buy sites = %v, want %v", got, fullSites)
	}
	if b := full.Sites[1]; b.Share.Count != 3 || b.Share.Total != 6 || b.WinRate.Count != 3 {
		t.Errorf("full buy on B = %+v, want 3 of 6 rounds, all won", b.PrepRounds)
	}
	if got := cellRounds(eco.Tempo); !reflect.DeepEqual(got[TempoNoSite], []int{5}) || len(got[TempoRush]) != 0 {
		t.Errorf("eco tempo = %v, want the save as no site", got)
	}
}

func TestAttackBuyBandsFoldHalfBuys(t *testing.T) {
	for buy, want := range map[BuyType]string{BuyPistol: BandPistol, BuyEco: BandEco, BuySemi: BandHalf, BuyForce: BandHalf, BuyFull: BandFull, BuyUnknown: ""} {
		if got := buyBand(buy); got != want {
			t.Errorf("buyBand(%s) = %q, want %q", buy, got, want)
		}
	}
}

func TestAttackReadsDefaultsAndExecutes(t *testing.T) {
	prep := attackOf(t)
	if len(prep.Defaults) != 1 {
		t.Fatalf("defaults = %+v, want the map control round only (round 6 has no frames)", prep.Defaults)
	}
	d := prep.Defaults[0]
	if d.Formation != "0-2-0" || d.Rounds != 1 || d.Share.Total != 6 || d.WinRate.Count != 1 {
		t.Errorf("default = %+v, want both players mid at 25 s over the 6 framed rounds", d)
	}
	if len(d.Zones) != 1 || d.Zones[0].Zone != "Middle" || d.Zones[0].Label != "Middle" || d.Zones[0].Share.Total != 1 {
		t.Errorf("default zones = %+v", d.Zones)
	}
	if got := refRounds(d.Refs); !reflect.DeepEqual(got, []int{3}) || d.Refs[0].Tick != pbTick(3, 25) {
		t.Errorf("default refs = %+v, want round 3 at the read second", d.Refs)
	}

	type row struct {
		site, route string
		rounds      []int
	}
	var got []row
	for _, e := range prep.Executes {
		if e.Buy != BandFull || e.OpponentBuy != BandFull || e.Share.Total != 6 {
			t.Errorf("execute %+v is outside the full buy matchup", e)
		}
		got = append(got, row{e.Site, e.Route, refRounds(e.Refs)})
	}
	want := []row{{"A", "Middle", []int{2}}, {"B", "", []int{7}}, {"B", "TSpawn", []int{1}}, {"both", "", []int{4}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("executes = %+v, want %+v", got, want)
	}
	rush := prep.Executes[2]
	if rush.Rushes.Count != 1 || rush.ContactSeconds != 15 || rush.RouteLabel != "T Spawn" {
		t.Errorf("rush execute = %+v", rush)
	}
	if split := prep.Executes[3]; split.Plants.Count != 1 || split.ContactSeconds != 30 {
		t.Errorf("split execute = %+v, want the plant and the A contact at 30 s", split)
	}
}

func TestAttackPlayersPerWindow(t *testing.T) {
	prep := attackOf(t)
	if len(prep.Players) != 2 || prep.Players[0].SteamID64 != "steam2" || prep.Players[0].Rounds != 6 || prep.Players[0].Role != "unknown" {
		t.Fatalf("players = %+v", prep.Players)
	}
	early := prep.Players[0].Windows[0]
	if early.Rounds != 6 || len(early.Spots) != 4 {
		t.Fatalf("early window = %+v", early)
	}
	b, mid := early.Spots[0], early.Spots[1]
	if b.Callout != "BombsiteB" || b.Share.Count != 2 || b.ArrivalSeconds != 32.5 || !reflect.DeepEqual(refRounds(b.Refs), []int{1, 7}) {
		t.Errorf("first early spot = %+v, want B in rounds 1 and 7, reached at 15 and 50 s", b)
	}
	if mid.Callout != "Middle" || mid.ArrivalSeconds != 5 || mid.Pos != [3]float64{0, 0, 0} {
		t.Errorf("second early spot = %+v", mid)
	}
	if p := early.Pace; p.Static.Count != 3 || p.Static.Total != 6 || p.Fast.Count != 0 {
		t.Errorf("early pace = %+v, want rounds 4, 5 and 7 held still", p)
	}
	if late := prep.Players[0].Windows[1]; late.Rounds != 5 || late.Spots[0].Callout != "BombsiteB" || late.Spots[0].Share.Count != 4 {
		t.Errorf("1:00 to 0:40 window = %+v", late)
	}
}

func TestAttackWarnsAboutThinStaleSamples(t *testing.T) {
	m := pbMatch(t)
	prep := BuildAttack([]FramedMatch{m}, AttackOptions{Now: m.PlayedAt.Add(90 * 24 * time.Hour)})
	want := []PrepWarning{{Kind: WarningFewRounds, Rounds: 7}, {Kind: WarningStale, Days: 90}}
	if !reflect.DeepEqual(prep.Warnings, want) || prep.DaysSince != 90 || prep.LastPlayedAt == nil {
		t.Errorf("warnings = %+v, days %d, want %+v", prep.Warnings, prep.DaysSince, want)
	}
}

func TestAttackWarnsAboutLineupChanges(t *testing.T) {
	day := func(n int) time.Time { return time.Date(2026, 9, n, 18, 0, 0, 0, time.UTC) }
	match := func(n int, nicks ...string) FramedMatch {
		doc := NewDocument()
		for i, nick := range nicks {
			doc.Players = append(doc.Players, Player{Slot: uint8(i), SteamID64: "id-" + nick, Name: nick, TeamKey: "team"})
		}
		doc.Players = append(doc.Players, Player{Slot: 9, SteamID64: "rival", Name: "rival", TeamKey: "other"})
		return FramedMatch{ScoutMatch: ScoutMatch{MatchID: uuid.New(), TeamKey: "team", PlayedAt: day(n), Document: doc}}
	}
	selected := []FramedMatch{
		match(4, "core", "new", "sub"), match(3, "core", "old", "new"), match(2, "core", "old", "new", "sub"), match(1, "core", "old"),
	}
	days, warnings := sampleWarnings(selected, day(10))
	want := []PrepWarning{{Kind: WarningRosterChanged, Joined: []string{"new"}, Left: []string{"old"}}}
	if days != 6 || !reflect.DeepEqual(warnings, want) {
		t.Errorf("sampleWarnings = %d days, %+v, want 6 days and %+v", days, warnings, want)
	}
}
