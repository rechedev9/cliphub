package parser

import (
	"testing"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func buy(side common.Team, firstID uint64, equips ...int) []alivePlayer {
	out := make([]alivePlayer, len(equips))
	for i, equip := range equips {
		out[i] = alivePlayer{SteamID64: firstID + uint64(i), Team: side, Equip: equip}
	}
	return out
}

func TestEcoRoundNeedsAClearlySmallerBuyAgainstARealOne(t *testing.T) {
	fullCT := buy(common.TeamCounterTerrorists, 1, 5200, 4900, 5100, 6000, 4700)
	pistolCT := buy(common.TeamCounterTerrorists, 1, 800, 800, 800, 800, 800)
	ecoT := buy(common.TeamTerrorists, 10, 850, 1000, 800, 1200, 700)
	tests := []struct {
		name    string
		players []alivePlayer
		side    common.Team
		want    bool
	}{
		{"eco against a full buy", append(append([]alivePlayer{}, fullCT...), ecoT...), common.TeamTerrorists, true},
		{"force buy over half", append(append([]alivePlayer{}, fullCT...), buy(common.TeamTerrorists, 10, 3000, 2800, 3200, 2600, 2900)...), common.TeamTerrorists, false},
		{"the full buy is no eco", append(append([]alivePlayer{}, fullCT...), ecoT...), common.TeamCounterTerrorists, false},
		{"pistol round", append(append([]alivePlayer{}, pistolCT...), buy(common.TeamTerrorists, 10, 800, 800, 800, 800, 800)...), common.TeamTerrorists, false},
		{"no enemy listed", buy(common.TeamTerrorists, 10, 0, 0), common.TeamTerrorists, false},
		{"no equipment values", append(buy(common.TeamCounterTerrorists, 1, 0, 0), buy(common.TeamTerrorists, 10, 0, 0)...), common.TeamTerrorists, false},
	}
	for _, tt := range tests {
		if got := ecoRound(tt.players, tt.side); got != tt.want {
			t.Fatalf("%s: ecoRound = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestViewTrailFlagsABigTurnInsideTheWindowOnly(t *testing.T) {
	var trail viewTrail
	for tick := 100; tick < 110; tick++ {
		trail.record(viewSample{tick: tick, yaw: 90, pitch: 0})
	}
	if !trail.flick(viewSample{tick: 110, yaw: 150, pitch: 0}, 13) {
		t.Fatal("a 60 degree turn in 10 ticks is a flick")
	}
	if trail.flick(viewSample{tick: 110, yaw: 120, pitch: 0}, 13) {
		t.Fatal("a 30 degree turn is not a flick")
	}
	if trail.flick(viewSample{tick: 140, yaw: 150, pitch: 0}, 13) {
		t.Fatal("a turn older than the window is not a flick")
	}

	trail.reset()
	trail.record(viewSample{tick: 200, yaw: 350, pitch: 0})
	if trail.flick(viewSample{tick: 205, yaw: 10, pitch: 0}, 13) {
		t.Fatal("crossing 360 is a 20 degree turn, not 340")
	}
	trail.record(viewSample{tick: 201, yaw: 10, pitch: 300})
	if !trail.flick(viewSample{tick: 205, yaw: 10, pitch: 10}, 13) {
		t.Fatal("pitch 300 to 10 is a 70 degree turn")
	}

	trail.reset()
	trail.record(viewSample{tick: 300, yaw: 0})
	if trail.flick(viewSample{tick: 300, yaw: 90}, 13) {
		t.Fatal("a sample on the kill tick itself is not before the kill")
	}
}

func TestViewTrailKeepsOnlyTheLatestSamples(t *testing.T) {
	var trail viewTrail
	trail.record(viewSample{tick: 1, yaw: 0})
	for tick := 2; tick <= flickTrailSize+1; tick++ {
		trail.record(viewSample{tick: tick, yaw: 100})
	}
	if trail.n != flickTrailSize || trail.flick(viewSample{tick: flickTrailSize + 2, yaw: 100}, 1000) {
		t.Fatal("the oldest sample, 100 degrees away, must have been dropped")
	}
}

func TestMarkRoundResultsFindsRetakesAndEcoWins(t *testing.T) {
	ct := killplan.Player{TeamAtKill: "CT"}
	tSide := killplan.Player{TeamAtKill: "T"}
	kills := []RawKill{
		{Round: 1, Killer: ct, AfterPlant: true},
		{Round: 2, Killer: ct, AfterPlant: true},
		{Round: 3, Killer: ct},
		{Round: 4, Killer: tSide, Eco: true},
		{Round: 5, Killer: tSide, Eco: true},
		{Round: 6, Killer: ct, AfterPlant: true, Eco: true},
	}
	markRoundResults(kills, []RoundEnd{{Round: 1, Winner: "CT"}, {Round: 2, Winner: "T"}, {Round: 3, Winner: "CT"}, {Round: 4, Winner: "T"}, {Round: 5, Winner: "CT"}})
	want := []struct{ retake, eco bool }{{true, false}, {false, false}, {false, false}, {false, true}, {false, false}, {false, false}}
	for i, w := range want {
		if kills[i].Retake != w.retake || kills[i].EcoWin != w.eco {
			t.Fatalf("round %d retake/eco = %v/%v, want %v/%v", kills[i].Round, kills[i].Retake, kills[i].EcoWin, w.retake, w.eco)
		}
	}
}

func TestRoundResultsKeepTheFirstKnownWinnerInRoundOrder(t *testing.T) {
	got := roundResults([]RoundEnd{
		{Round: 2, Winner: "T", TargetSide: "CT"},
		{Round: 1, Winner: "CT", TargetSide: "CT"},
		{Round: 2, Winner: "CT", TargetSide: "CT"},
		{Round: 3},
		{Round: 0, Winner: "T"},
		{Round: 4, Winner: "T"},
	})
	want := []killplan.RoundResult{{Number: 1, Winner: "CT", TargetSide: "CT"}, {Number: 2, Winner: "T", TargetSide: "CT"}, {Number: 4, Winner: "T"}}
	if len(got) != len(want) {
		t.Fatalf("roundResults = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("round %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestPostPlantKillOfTheAttackersIsNoRetake(t *testing.T) {
	kills := []RawKill{{Round: 1, Killer: killplan.Player{TeamAtKill: "T"}, AfterPlant: true}}
	markRoundResults(kills, []RoundEnd{{Round: 1, Winner: "T"}})
	if kills[0].Retake {
		t.Fatal("a T kill after the plant in a round T won is not a retake")
	}
	if got := buildKillPlanKills(kills); !got[0].PostPlant {
		t.Fatal("the kill plan must keep the post-plant flag of either side")
	}
}

func TestOpeningCandidateNeedsTwoEnemies(t *testing.T) {
	ct := &common.Player{SteamID64: 1, Team: common.TeamCounterTerrorists}
	ct2 := &common.Player{SteamID64: 2, Team: common.TeamCounterTerrorists}
	tp := &common.Player{SteamID64: 3, Team: common.TeamTerrorists}
	tests := []struct {
		name string
		e    events.Kill
		want bool
	}{
		{"enemy kill", events.Kill{Killer: ct, Victim: tp}, true},
		{"team kill", events.Kill{Killer: ct, Victim: ct2}, false},
		{"suicide", events.Kill{Killer: ct, Victim: ct}, false},
		{"world kill", events.Kill{Victim: tp}, false},
	}
	for _, tt := range tests {
		if got := isOpeningCandidate(tt.e); got != tt.want {
			t.Fatalf("%s: isOpeningCandidate = %v, want %v", tt.name, got, tt.want)
		}
	}
}
