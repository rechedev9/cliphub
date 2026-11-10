package tacticalplan

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func sideStatsRound(number, half int, winner Side, buy BuyType, opening Side) Round {
	return Round{
		Number: number, Half: half, Winner: winner,
		Economy: Economy{CTBuy: buy, TBuy: buy},
		Class:   Class{OpeningSide: opening},
	}
}

// sideStatsDocument has "home" (slots 0, 1) on CT in the first half and on T
// in the second; the map ends 2 to 2, so home has not won it.
func sideStatsDocument() Document {
	d := NewDocument()
	d.Demo.Map, d.Demo.Tickrate, d.UtilityEffects = "de_nuke", 64, true
	d.Teams = []Team{{Key: "home", StartSide: SideCT}, {Key: "away", StartSide: SideT}}
	d.Players = []Player{
		{Slot: 0, SteamID64: "10", Name: "h0", TeamKey: "home"}, {Slot: 1, SteamID64: "11", Name: "h1", TeamKey: "home"},
		{Slot: 2, SteamID64: "20", Name: "a0", TeamKey: "away"}, {Slot: 3, SteamID64: "21", Name: "a1", TeamKey: "away"},
	}
	r1 := sideStatsRound(1, 1, SideCT, BuyPistol, SideCT)
	r1.Class.Tags = []string{TagPistol}
	r1.Players = []PlayerRound{
		{Slot: 0, Side: SideCT, Kills: 1, Damage: 100}, {Slot: 1, Side: SideCT, Kills: 1, Deaths: 1, Damage: 50, Traded: true},
		{Slot: 2, Side: SideT, Deaths: 1}, {Slot: 3, Side: SideT, Kills: 1, Deaths: 1},
	}
	r1.Events = []Event{
		{Tick: 1000, Kind: EventFlash, ActorSlot: slotPtr(0), Effect: &UtilityEffect{Blinded: []BlindHit{{Slot: 2, Seconds: 2, Enemy: true}}}},
		{Tick: 1100, Kind: EventKill, ActorSlot: slotPtr(1), TargetSlot: slotPtr(2)},
		// The thrower's own kill of the blinded player is no assist.
		{Tick: 1110, Kind: EventKill, ActorSlot: slotPtr(0), TargetSlot: slotPtr(2)},
		{Tick: 1200, Kind: EventHE, ActorSlot: slotPtr(1), Effect: &UtilityEffect{EnemyDamage: 40, TeamDamage: 10}},
		{Tick: 1300, Kind: EventMolotov, ActorSlot: slotPtr(2), Effect: &UtilityEffect{EnemyDamage: 70}},
	}
	r2 := sideStatsRound(2, 1, SideT, BuyFull, SideT)
	r2.Players = []PlayerRound{{Slot: 0, Side: SideCT, Deaths: 1, Damage: 20}, {Slot: 1, Side: SideCT}, {Slot: 2, Side: SideT}, {Slot: 3, Side: SideT}}
	r3 := sideStatsRound(3, 2, SideT, BuyFull, SideT)
	r3.Players = []PlayerRound{{Slot: 0, Side: SideT, Kills: 2, Damage: 150}, {Slot: 1, Side: SideT}, {Slot: 2, Side: SideCT}, {Slot: 3, Side: SideCT}}
	r4 := sideStatsRound(4, 2, SideCT, BuyFull, "")
	r4.Players = []PlayerRound{{Slot: 0, Side: SideT}, {Slot: 1, Side: SideT}, {Slot: 2, Side: SideCT}, {Slot: 3, Side: SideCT}}
	// A round without a winner never counts.
	r5 := sideStatsRound(5, 2, "", BuyFull, SideT)
	d.Rounds = []Round{r1, r2, r3, r4, r5}
	return d
}

func sideStatsMatch(teamKey string) SideStatsMatch {
	return SideStatsMatch{
		MatchID: uuid.MustParse("00000000-0000-0000-0000-0000000000c1"), PlayedAt: time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC),
		Document: sideStatsDocument(), TeamKey: teamKey, Opponent: "Away",
	}
}

func wantStat(t *testing.T, name string, got SideStat, want float64) {
	t.Helper()
	if got.Value == nil || *got.Value != want {
		t.Fatalf("%s = %v, want %v", name, got.Value, want)
	}
}

func TestTeamSideStatsEveryFigure(t *testing.T) {
	r := TeamSideStats(SideStatsInput{Team: []SideStatsMatch{sideStatsMatch("home")}})
	if r.Matches != 1 || r.Rounds != 4 {
		t.Fatalf("sample = %d matches, %d rounds", r.Matches, r.Rounds)
	}
	o := r.Overall
	wantStat(t, "maps won", o.MapsWon, 0)
	wantStat(t, "rounds won", o.RoundsWon, 50)
	wantStat(t, "pistols won", o.PistolsWon, 100)
	wantStat(t, "opening kills", o.OpeningKills, 66.7)
	wantStat(t, "5v4 won", o.Won5v4, 100)
	wantStat(t, "4v5 won", o.Won4v5, 0)
	wantStat(t, "trades", o.Trades, 50)
	wantStat(t, "utility damage", o.UtilityDamage, 10)
	wantStat(t, "flash assists", o.FlashAssists, 0.25)
	wantStat(t, "rounds played", o.RoundsPlayed, 4)
	if r.CT.MapsWon.Value != nil {
		t.Fatalf("a side has no map result, got %v", *r.CT.MapsWon.Value)
	}
	wantStat(t, "CT rounds won", r.CT.RoundsWon, 50)
	wantStat(t, "T rounds played", r.T.RoundsPlayed, 2)
	if len(r.MatchList) != 1 || r.MatchList[0].Won || r.MatchList[0].Rounds != 4 {
		t.Fatalf("match list = %+v", r.MatchList)
	}
}

func TestTeamSideStatsBuyScopeAndEmptyFigures(t *testing.T) {
	r := TeamSideStats(SideStatsInput{Team: []SideStatsMatch{sideStatsMatch("home")}, Options: SideStatsOptions{Rounds: Filter{Buys: []BuyType{BuyFull}}, Side: SideT}})
	if r.Rounds != 2 {
		t.Fatalf("full buy T rounds = %d, want 2", r.Rounds)
	}
	// No pistol is a full buy: the figure is empty, never 0 %.
	if r.Overall.PistolsWon.Value != nil {
		t.Fatalf("pistols won = %v, want no data", *r.Overall.PistolsWon.Value)
	}
	wantStat(t, "rounds won", r.Overall.RoundsWon, 50)
	// The other side keeps every filter but the side, so a greyed CT still has numbers.
	if r.CT.RoundsPlayed.Value == nil || *r.CT.RoundsPlayed.Value == 0 {
		t.Fatalf("CT rounds with the T side chosen = %+v, want figures", r.CT.RoundsPlayed)
	}
	if r.CT.RoundsWon.Value == nil {
		t.Fatalf("CT rounds won = no data, want a figure")
	}
	if r.T.RoundsPlayed.Value == nil || *r.T.RoundsPlayed.Value != float64(r.Rounds) {
		t.Fatalf("T rounds %+v do not match overall %d", r.T.RoundsPlayed, r.Rounds)
	}
}

func TestTeamSideStatsKeepsOnlyThePlayRounds(t *testing.T) {
	m := sideStatsMatch("home")
	opts := SideStatsOptions{PlayRounds: map[uuid.UUID][]int{m.MatchID: {2}}}
	r := TeamSideStats(SideStatsInput{Team: []SideStatsMatch{m}, Options: opts})
	if r.Rounds != 1 || r.Matches != 1 {
		t.Fatalf("play rounds sample = %d matches, %d rounds, want round 2 alone", r.Matches, r.Rounds)
	}
	opts.PlayRounds = map[uuid.UUID][]int{}
	if r := TeamSideStats(SideStatsInput{Team: []SideStatsMatch{m}, Options: opts}); r.Rounds != 0 {
		t.Fatalf("a match without play rounds kept %d rounds", r.Rounds)
	}
}

func TestTeamSideStatsRoundRange(t *testing.T) {
	all := TeamSideStats(SideStatsInput{Team: []SideStatsMatch{sideStatsMatch("home")}})
	r := TeamSideStats(SideStatsInput{Team: []SideStatsMatch{sideStatsMatch("home")}, Options: SideStatsOptions{Rounds: Filter{RoundFrom: 2}}})
	if all.Rounds < 2 || r.Rounds != all.Rounds-1 {
		t.Fatalf("rounds from 2 = %d of %d, want one fewer", r.Rounds, all.Rounds)
	}
}

func TestTeamSideStatsPlayerCards(t *testing.T) {
	r := TeamSideStats(SideStatsInput{Team: []SideStatsMatch{sideStatsMatch("home")}, Options: SideStatsOptions{Roles: map[string]string{"10": "awp"}}})
	if len(r.Players) != 2 {
		t.Fatalf("players = %+v", r.Players)
	}
	p := r.Players[0]
	if p.SteamID64 != "10" || p.Role != "awp" || p.Rounds != 4 || p.Kills != 3 || p.Deaths != 1 {
		t.Fatalf("h0 = %+v", p)
	}
	if p.KPR != 0.75 || p.DPR != 0.25 || p.ADR != 67.5 || p.KD != 3 || p.Rating != 1.16 {
		t.Fatalf("h0 rates = %+v", p)
	}
}

func TestTeamSideStatsGroupFoldsMatchesAndNeedsTwentyRounds(t *testing.T) {
	group := NewSideStatsGroup(SideStatsOptions{Lineup: Lineup{"999"}})
	for i := range 5 {
		m := sideStatsMatch("away")
		m.MatchID = uuid.New()
		group.Add("away", m)
		if i == 0 {
			// One match is 4 rounds: under GroupMinRounds no figure enters the mean.
			if r := TeamSideStats(SideStatsInput{Team: []SideStatsMatch{sideStatsMatch("home")}, Group: group}); r.Overall.RoundsWon.GroupAvg != nil {
				t.Fatalf("a 4 round team entered the mean: %+v", r.Overall.RoundsWon)
			}
		}
	}
	group.Add("thin", sideStatsMatch("home"))
	if group.Added("away") != 5 || group.Added("nobody") != 0 {
		t.Fatalf("added = %d", group.Added("away"))
	}
	r := TeamSideStats(SideStatsInput{Team: []SideStatsMatch{sideStatsMatch("home")}, Group: group})
	if r.GroupTeams != 2 || r.GroupMatches != 6 || r.GroupMinRounds != GroupMinRounds {
		t.Fatalf("group = %d teams over %d matches", r.GroupTeams, r.GroupMatches)
	}
	// The group ignores the team's lineup, and only away has 20 rounds.
	rw := r.Overall.RoundsWon
	if rw.GroupAvg == nil || *rw.GroupAvg != 50 || rw.GroupTeams != 1 {
		t.Fatalf("rounds won group = %+v", rw)
	}
	// Each map ended 2 to 2 and a draw is no win, so the group maps won is 0 %.
	if mw := r.Overall.MapsWon; mw.GroupAvg == nil || *mw.GroupAvg != 0 {
		t.Fatalf("maps won group = %+v", mw)
	}
	// Five pistol rounds are too few for the pistol mean.
	if pw := r.Overall.PistolsWon; pw.GroupAvg != nil || pw.GroupTeams != 0 {
		t.Fatalf("pistols won group = %+v", pw)
	}
}

func TestApplyGroupAverageIsThePlainMeanOfTeamsWithData(t *testing.T) {
	v := func(x float64) *float64 { return &x }
	stat := func(x float64, total int) SideStat { return SideStat{Value: v(x), Total: total} }
	group := []groupTeam{
		{stats: SideStats{RoundsWon: stat(40, 30), PistolsWon: stat(100, 20)}, rounds: 30},
		{stats: SideStats{RoundsWon: stat(60, 25), PistolsWon: stat(0, 2)}, rounds: 25},
		{stats: SideStats{RoundsWon: stat(55, 40)}, rounds: 40},
	}
	var target SideStats
	applyGroupAverage(&target, group)
	if g := target.RoundsWon; g.GroupAvg == nil || *g.GroupAvg != 51.67 || g.GroupTeams != 3 {
		t.Fatalf("rounds won = %+v", g)
	}
	if g := target.PistolsWon; g.GroupAvg == nil || *g.GroupAvg != 100 || g.GroupTeams != 1 {
		t.Fatalf("pistols won = %+v", g)
	}
	if g := target.Trades; g.GroupAvg != nil || g.GroupTeams != 0 {
		t.Fatalf("trades without data = %+v", g)
	}
}
