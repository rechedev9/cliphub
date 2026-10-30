package tacticalplan

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-4 }

func TestWinProbabilityCT(t *testing.T) {
	even := WinState{Map: "de_mirage", AliveCT: 5, AliveT: 5, EquipCT: 20000, EquipT: 20000}
	if got := WinProbabilityCT(even); !near(got, 0.5) {
		t.Fatalf("even state = %v, want 0.5", got)
	}
	up := even
	up.AliveT, up.EquipT = 4, 16000
	if got := WinProbabilityCT(up); !near(got, 1/(1+math.Exp(-winAliveCoef))) {
		t.Fatalf("5v4 = %v", got)
	}
	nuke := even
	nuke.Map = "de_nuke"
	if WinProbabilityCT(nuke) <= 0.5 {
		t.Fatal("the nuke prior must favour CT")
	}
	eco := even
	eco.EquipT = 5000
	if WinProbabilityCT(eco) <= WinProbabilityCT(even) {
		t.Fatal("richer CT must be favoured")
	}
	late := even
	late.Seconds = 100
	if WinProbabilityCT(late) <= WinProbabilityCT(even) {
		t.Fatal("the clock must favour CT before a plant")
	}
	planted := even
	planted.Planted = true
	later := planted
	later.BombSeconds = 30
	if p, q := WinProbabilityCT(planted), WinProbabilityCT(later); p >= 0.5 || q >= p {
		t.Fatalf("plant = %v, later = %v: the plant and its timer must favour T", p, q)
	}
	terminal := []struct {
		name  string
		state WinState
		want  float64
	}{
		{"no CT alive", WinState{AliveT: 1}, 0},
		{"no T alive, no plant", WinState{AliveCT: 1}, 1},
		{"no T alive, early plant", WinState{AliveCT: 1, Planted: true, BombSeconds: 10}, 1 - winFloor},
		{"no T alive, late plant", WinState{AliveCT: 1, Planted: true, BombSeconds: 38}, winFloor},
		{"no T alive, mid plant", WinState{AliveCT: 1, Planted: true, BombSeconds: 30}, 0.5},
	}
	for _, tt := range terminal {
		if got := WinProbabilityCT(tt.state); !near(got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
		}
	}
}

var (
	metricMatchOne = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	metricMatchTwo = uuid.MustParse("00000000-0000-0000-0000-0000000000a2")
)

// metricsDocument is a 2v2 round won by CT: slot 0 flashes slot 2 (and its own
// teammate), slot 1 kills the blind slot 2, the eco slot 3 kills slot 1, and
// slot 0 trades it. A plant after the round end follows.
func metricsDocument() Document {
	return Document{
		UtilityEffects: true,
		Demo:           Demo{Map: "de_mirage", Tickrate: 64},
		Teams: []Team{
			{Key: "ct-start", Name: "Alpha", StartSide: SideCT, Slots: []uint8{0, 1}},
			{Key: "t-start", Name: "Beta", StartSide: SideT, Slots: []uint8{2, 3}},
		},
		Players: []Player{
			{Slot: 0, SteamID64: "10", Name: "a0", TeamKey: "ct-start", StartSide: SideCT},
			{Slot: 1, SteamID64: "11", Name: "a1", TeamKey: "ct-start", StartSide: SideCT},
			{Slot: 2, SteamID64: "20", Name: "b0", TeamKey: "t-start", StartSide: SideT},
			{Slot: 3, SteamID64: "21", Name: "b1", TeamKey: "t-start", StartSide: SideT},
		},
		Geometry: MapGeometry{Callouts: []Callout{
			{Name: "BombsiteA", Level: "default", Center: [2]float64{1000, 1000}},
			{Name: "Middle", Level: "default", Center: [2]float64{0, 0}},
		}},
		Rounds: []Round{{
			Number: 1, TickStart: 900, TickFreezeEnd: 1000, TickEnd: 1500, Winner: SideCT, Half: 1,
			Players: []PlayerRound{
				{Slot: 0, Side: SideCT, EquipValue: 4000, Kills: 1},
				{Slot: 1, Side: SideCT, EquipValue: 4000, Kills: 1, Deaths: 1},
				{Slot: 2, Side: SideT, EquipValue: 4000, Deaths: 1},
				{Slot: 3, Side: SideT, EquipValue: 1000, Kills: 1, Deaths: 1},
			},
			Events: []Event{
				{Tick: 1100, Kind: EventFlash, ActorSlot: slotPtr(0), Side: SideCT, Effect: &UtilityEffect{Blinded: []BlindHit{
					{Slot: 2, Seconds: 2, Enemy: true}, {Slot: 1, Seconds: 1.5},
				}}},
				{Tick: 1100, Kind: EventFlash, ActorSlot: slotPtr(0), Side: SideCT},
				{Tick: 1120, Kind: EventHE, ActorSlot: slotPtr(3), Side: SideT, Effect: &UtilityEffect{EnemyDamage: 30, TeamDamage: 5}},
				{Tick: 1150, Kind: EventKill, ActorSlot: slotPtr(1), TargetSlot: slotPtr(2), Side: SideCT},
				{Tick: 1300, Kind: EventKill, ActorSlot: slotPtr(3), TargetSlot: slotPtr(1), Side: SideT},
				{Tick: 1400, Kind: EventKill, ActorSlot: slotPtr(0), TargetSlot: slotPtr(3), Side: SideCT},
				// A plant after the round end moves nothing.
				{Tick: 1600, Kind: EventPlant, ActorSlot: slotPtr(2), Side: SideT},
			},
		}},
	}
}

func metricsMatches() []MetricsMatch {
	return []MetricsMatch{
		{
			MatchID: metricMatchOne, PlayedAt: time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC), Document: metricsDocument(),
			Setup: map[int]Frame{1: {Tick: 1960, Samples: []Sample{
				{Slot: 0, X: 990, Y: 1000, Flags: FlagAlive},
				{Slot: 3, X: 10, Y: 0, Flags: FlagAlive | FlagSideT},
			}}},
		},
		{MatchID: metricMatchTwo, PlayedAt: time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC), Document: metricsDocument()},
	}
}

func playerBy(t *testing.T, r MetricsReport, steamID string) PlayerMetrics {
	t.Helper()
	for _, p := range r.Players {
		if p.SteamID64 == steamID {
			return p
		}
	}
	t.Fatalf("no player %s in %d lines", steamID, len(r.Players))
	return PlayerMetrics{}
}

func TestMetricsSwingFollowsTheModel(t *testing.T) {
	r := Metrics(metricsMatches()[:1], MetricsOptions{Timelines: true})
	state := WinState{Map: "de_mirage", AliveCT: 2, AliveT: 2, EquipCT: 8000, EquipT: 5000, Seconds: 150.0 / 64}
	before := WinProbabilityCT(state)
	state.AliveT, state.EquipT = 1, 1000
	after := WinProbabilityCT(state)

	a1 := playerBy(t, r, "11")
	if len(a1.Swing.Best) == 0 || !near(a1.Swing.Best[0].Delta, after-before) {
		t.Fatalf("a1 best swing = %+v, want %v", a1.Swing.Best, after-before)
	}
	if b0 := playerBy(t, r, "20"); !near(b0.Swing.WPA, before-after) {
		t.Fatalf("b0 wpa = %v, want %v", b0.Swing.WPA, before-after)
	}
	total := 0.0
	for _, p := range r.Players {
		total += p.Swing.WPA
	}
	if !near(total, 0) {
		t.Fatalf("kills move both sides by the same amount, total wpa = %v", total)
	}
	if len(r.Timelines) != 1 || len(r.Timelines[0].Points) != 5 {
		t.Fatalf("timelines = %+v", r.Timelines)
	}
	points := r.Timelines[0].Points
	if points[0].Kind != WinPointStart || points[4].Kind != WinPointEnd || points[4].ProbabilityCT != 1 {
		t.Fatalf("timeline ends = %+v %+v", points[0], points[4])
	}
	if !near(points[1].ProbabilityCT, after) || points[1].AliveT != 1 {
		t.Fatalf("first kill point = %+v, want %v", points[1], after)
	}
}

func TestMetricsDuelsTradesAndUtility(t *testing.T) {
	r := Metrics(metricsMatches()[:1], MetricsOptions{})
	a0, a1, b1 := playerBy(t, r, "10"), playerBy(t, r, "11"), playerBy(t, r, "21")

	if a1.Duels.Even.Won.Count != 1 || a1.Duels.Advantage.Lost.Count != 1 {
		t.Fatalf("a1 duels = %+v", a1.Duels)
	}
	if b1.Duels.Disadvantage.Won.Count != 1 || a0.Duels.Advantage.Won.Count != 1 {
		t.Fatalf("b1 duels = %+v, a0 duels = %+v", b1.Duels, a0.Duels)
	}
	if !near(b1.Eco.Kills, 1.6) || !near(a1.Eco.Deaths, 1.6) || !near(a0.Eco.Kills, 0.4) || !near(a1.Eco.Kills, 1) {
		t.Fatalf("eco kills: b1 %v, a1 deaths %v, a0 %v, a1 %v", b1.Eco.Kills, a1.Eco.Deaths, a0.Eco.Kills, a1.Eco.Kills)
	}
	if b1.Eco.Examples[0].KillerEquip != 1000 || b1.Eco.Examples[0].Tick != 1300 {
		t.Fatalf("b1 eco example = %+v", b1.Eco.Examples)
	}
	if a0.Trades.TradeKills.Count != 1 || a0.Trades.TradeKills.Examples[0].Tick != 1400 {
		t.Fatalf("a0 trades = %+v", a0.Trades)
	}
	if a1.Trades.TradedDeaths.Count != 1 || a1.Trades.UntradedDeaths.Count != 0 {
		t.Fatalf("a1 trades = %+v", a1.Trades)
	}
	if b1.Trades.UntradedDeaths.Count != 1 {
		t.Fatalf("b1 trades = %+v", b1.Trades)
	}

	f := a0.Utility.Flash
	if !a0.Utility.Measured || f.Thrown != 1 || f.EnemiesBlinded.Count != 1 || f.TeamBlinded.Count != 1 || f.EnemyBlindSeconds != 2 {
		t.Fatalf("a0 flash = %+v (the duplicated detonation counts once)", f)
	}
	if f.Assists.Count != 1 || f.Assists.Examples[0].Tick != 1150 {
		t.Fatalf("a0 flash assists = %+v", f.Assists)
	}
	if he := b1.Utility.HE; he.Thrown != 1 || he.EnemyDamage != 30 || he.TeamDamage != 5 || he.Examples[0].Tick != 1120 {
		t.Fatalf("b1 HE = %+v", he)
	}

	if len(a0.Positions) != 1 || a0.Positions[0].Callout != "BombsiteA" || a0.Positions[0].Won != 1 || a0.Positions[0].Examples[0].Tick != 1960 {
		t.Fatalf("a0 positions = %+v", a0.Positions)
	}
	if a0.Rounds != 1 || a0.Kills != 1 || a0.Deaths != 0 || a1.Deaths != 1 {
		t.Fatalf("a0 %d rounds %d kills %d deaths, a1 %d deaths", a0.Rounds, a0.Kills, a0.Deaths, a1.Deaths)
	}
}

func TestMetricsIsDeterministicAndFollowsTheOptions(t *testing.T) {
	ms := metricsMatches()
	reversed := []MetricsMatch{ms[1], ms[0]}
	if a, b := Metrics(ms, MetricsOptions{}), Metrics(reversed, MetricsOptions{}); !reflect.DeepEqual(a, b) {
		t.Fatal("metrics depend on the input order")
	}

	one := Metrics(ms, MetricsOptions{SteamIDs: []string{"10"}, Roles: map[string]string{"10": "awp"}})
	if len(one.Players) != 1 || one.Players[0].Matches != 2 || one.Players[0].Role != "awp" {
		t.Fatalf("one player = %+v", one.Players)
	}
	if lines := one.Players[0].MatchLines; len(lines) != 2 || lines[0].MatchID != metricMatchTwo {
		t.Fatalf("match lines = %+v, want the newest first", lines)
	}
	if len(one.Roles) != 1 || one.Roles[0].Role != "awp" || one.Roles[0].Rounds != 2 {
		t.Fatalf("roles = %+v", one.Roles)
	}

	tSide := Metrics(ms, MetricsOptions{Side: SideT})
	for _, p := range tSide.Players {
		if p.SteamID64 == "10" || p.SteamID64 == "11" {
			t.Fatalf("side T kept a CT player: %+v", p)
		}
	}

	ms[0].TeamKey, ms[1].TeamKey = "t-start", "t-start"
	team := Metrics(ms, MetricsOptions{})
	if len(team.Players) != 2 || team.Rounds != 2 {
		t.Fatalf("team key kept %d players over %d rounds", len(team.Players), team.Rounds)
	}

	ms[0].Document.UtilityEffects, ms[1].Document.UtilityEffects = false, false
	if r := Metrics(ms, MetricsOptions{}); r.UtilityMatches != 0 || r.Players[0].Utility.Measured {
		t.Fatal("documents without utility effects must say so")
	}
}
