package parser

import (
	"testing"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func team(n int, side common.Team, firstID uint64) []alivePlayer {
	out := make([]alivePlayer, n)
	for i := range out {
		out[i] = alivePlayer{SteamID64: firstID + uint64(i), Team: side, Equip: 4500}
	}
	return out
}

func TestKillWinSwingGrowsWithTheStakesAndReadsTheKillerSide(t *testing.T) {
	cts := team(5, common.TeamCounterTerrorists, 1)
	ts := team(5, common.TeamTerrorists, 10)
	opening := killWinSwing(killSwingInput{Map: "de_mirage", Alive: append(append([]alivePlayer{}, cts...), ts...), Killer: cts[0], Victim: ts[0], Seconds: 20})
	if opening <= 0.1 || opening >= 0.3 {
		t.Fatalf("opening kill swing = %v, want a clear gain under 0.3", opening)
	}
	tSide := killWinSwing(killSwingInput{Map: "de_mirage", Alive: append(append([]alivePlayer{}, cts...), ts...), Killer: ts[0], Victim: cts[0], Seconds: 20})
	if tSide <= 0.1 {
		t.Fatalf("T opening kill swing = %v, want a gain for the T side", tSide)
	}
	// 1v1, the victim already left the alive list: the last kill settles the round.
	last := killWinSwing(killSwingInput{Map: "de_mirage", Alive: []alivePlayer{cts[0]}, Killer: cts[0], Victim: ts[0], Seconds: 80})
	if last <= opening {
		t.Fatalf("round-winning kill swing = %v, want more than the opening %v", last, opening)
	}
	if got := killWinSwing(killSwingInput{Alive: cts, Killer: alivePlayer{Team: common.TeamSpectators}, Victim: ts[0]}); got != 0 {
		t.Fatalf("spectator killer swing = %v, want 0", got)
	}
}

func TestMarkClutchResultsUsesTheRoundWinner(t *testing.T) {
	ct := killplan.Player{TeamAtKill: "CT"}
	kills := []RawKill{
		{Round: 3, ClutchVs: 2, Killer: ct},
		{Round: 4, ClutchVs: 1, Killer: ct},
		{Round: 5, ClutchVs: 3, Killer: ct},
		{Round: 3, Killer: ct},
	}
	markClutchResults(kills, []RoundEnd{{Round: 3, Winner: "CT"}, {Round: 4, Winner: "T"}, {Round: 4, Winner: "CT"}, {Round: 5}})
	want := []string{killplan.ClutchWon, killplan.ClutchLost, "", ""}
	for i, k := range kills {
		if k.ClutchResult != want[i] {
			t.Fatalf("kill %d clutch result = %q, want %q", i, k.ClutchResult, want[i])
		}
	}
}

func TestBuildRawKillCarriesKillFlagsAndSwing(t *testing.T) {
	e := events.Kill{
		Killer: mkPlayer(killerID, "target", common.TeamCounterTerrorists), Victim: mkPlayer(victimID, "enemy", common.TeamTerrorists),
		Weapon: mkEquipment(common.EqAWP, "weapon_awp"), NoScope: true, ThroughSmoke: true, AttackerBlind: true,
	}
	rk, ok := BuildRawKill(e, gameInfo{Tick: 10, Round: 1, WinProbSwing: 0.21}, killerID, true)
	if !ok || !rk.NoScope || !rk.ThroughSmoke || !rk.AttackerBlind || rk.WinProbSwing != 0.21 {
		t.Fatalf("raw kill = %+v, want the noscope, smoke and blind flags and the swing", rk)
	}
	got := buildKillPlanKills([]RawKill{rk})[0]
	if !got.NoScope || !got.ThroughSmoke || !got.AttackerBlind || got.WinProbSwing != 0.21 {
		t.Fatalf("plan kill = %+v, want the flags and the swing", got)
	}
}
