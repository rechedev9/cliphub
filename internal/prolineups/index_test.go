package prolineups

import (
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

var now = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

func smoke(mapKey, player string, round int, origin, landing [3]float64, yaw float64, daysAgo int) Throw {
	return Throw{
		Map: "de_mirage", Kind: tacticalplan.EventSmoke, Side: tacticalplan.SideT, Match: mapKey, MapKey: mapKey, PlayedAt: now.AddDate(0, 0, -daysAgo),
		Round: round, Team: "team-" + player, SteamID64: player, Player: player, Origin: origin, Yaw: yaw, Pitch: -30, Movement: "standing", Jump: true,
		Landing: landing, ThrowPlace: "T Spawn", LandPlace: "Window", Clock: 2, AirTime: 7, RoundWon: round%2 == 0,
	}
}

func TestBuildIndexGroupsRepeatedThrowsAndKeepsOneOffsOut(t *testing.T) {
	spot, land := [3]float64{100, 100, 0}, [3]float64{2000, 500, 0}
	throws := []Throw{
		smoke("m1", "a", 1, spot, land, 179.5, 10),
		smoke("m1", "a", 2, [3]float64{110, 95, 0}, [3]float64{2050, 520, 0}, -179.8, 10),
		smoke("m2", "b", 1, [3]float64{95, 105, 2}, [3]float64{1980, 470, 0}, 179.9, 40),
		// Same spot and aim but it flew elsewhere: an attempt, not a member.
		smoke("m2", "b", 2, spot, [3]float64{900, 900, 0}, 179.6, 40),
		// Aim five degrees off: a different lineup, thrown once, left out.
		smoke("m3", "c", 1, spot, land, 174, 200),
	}
	idx := BuildIndex("de_mirage", throws, now)
	if len(idx.Lineups) != 1 {
		t.Fatalf("lineups = %d, want 1", len(idx.Lineups))
	}
	l := idx.Lineups[0]
	if l.Throws.All != 3 || l.Throws.Recent != 3 || l.Matches.All != 2 {
		t.Fatalf("counts = %+v %+v", l.Throws, l.Matches)
	}
	if l.Attempts != 4 || l.Landed != 3 {
		t.Fatalf("attempts %d landed %d, want 4 and 3", l.Attempts, l.Landed)
	}
	if l.Title != "Humo a Window desde T Spawn (jumpthrow)" || l.Slug != "smoke/window-from-t-spawn-jumpthrow" || !l.Insta {
		t.Fatalf("named %q %q insta=%v", l.Title, l.Slug, l.Insta)
	}
	if l.Yaw < 179 && l.Yaw > -179 {
		t.Fatalf("yaw %v must average across the wrap", l.Yaw)
	}
	if idx.Maps.All != 3 || idx.RoundsT.Recent != 4 {
		t.Fatalf("maps %+v rounds %+v", idx.Maps, idx.RoundsT)
	}
}

func TestCombosNeedSeveralRoundsAndTeams(t *testing.T) {
	throws := []Throw{}
	for i, team := range []string{"x", "y", "x", "y"} {
		mapKey := "m" + string(rune('a'+i))
		for _, p := range []string{"p1", "p2"} {
			a := smoke(mapKey, p, 1, [3]float64{0, 0, 0}, [3]float64{1000, 0, 0}, 10, 5)
			b := smoke(mapKey, p, 1, [3]float64{500, 500, 0}, [3]float64{0, 1500, 0}, 90, 5)
			a.Team, b.Team = team, team
			b.Clock = 8
			if p == "p2" {
				a.Round, b.Round = 2, 2
			}
			throws = append(throws, a, b)
		}
	}
	idx := BuildIndex("de_mirage", throws, now)
	if len(idx.Lineups) != 2 {
		t.Fatalf("lineups = %d, want 2", len(idx.Lineups))
	}
	if len(idx.Combos) != 1 || idx.Combos[0].Rounds.Recent != 8 || idx.Combos[0].TeamCount != 2 {
		t.Fatalf("combos = %+v", idx.Combos)
	}
	if idx.Lineups[0].With[0].Share != 1 {
		t.Fatalf("partner share = %+v", idx.Lineups[0].With)
	}
}
