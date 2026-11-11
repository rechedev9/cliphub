package tacticalplan

import (
	"testing"

	"github.com/google/uuid"
)

func TestFirstMovementIsTheFirstPointAWholeCellAway(t *testing.T) {
	track := OverlayTrack{Start: 2, Points: [][3]int{{0, 0, 0}, {10, 0, 0}, {30, 0, 0}, {70, 0, 0}}}
	if s, ok := FirstMovementSeconds(track, 0.5); !ok || s != 2.5 {
		t.Errorf("moved at %v (%v), want 2.5", s, ok)
	}
	if _, ok := FirstMovementSeconds(OverlayTrack{Points: [][3]int{{0, 0, 0}, {5, 5, 0}}}, 0.5); ok {
		t.Error("a player who stayed put counts as moved")
	}
	if _, ok := FirstMovementSeconds(OverlayTrack{}, 0.5); ok {
		t.Error("a track without points counts as moved")
	}
}

func TestFirstMovementsKeepPlayersWindowAndZone(t *testing.T) {
	match := uuid.New()
	run := func(id string, start int, x int) OverlayTrack {
		return OverlayTrack{SteamID64: id, Name: id, Start: start, Points: [][3]int{{0, 0, 0}, {x, 0, 0}}}
	}
	overlay := Overlay{StepSeconds: 0.5, Rounds: []OverlayRound{
		{MatchID: match, Round: 3, TickFreezeEnd: 1000, Tickrate: 64, Tracks: []OverlayTrack{run("a", 20, 100), run("b", 80, 500)}},
		{MatchID: match, Round: 4, TickFreezeEnd: 5000, Tickrate: 64, Tracks: []OverlayTrack{run("a", 4, 100)}},
	}}
	all := FirstMovements(overlay, FirstMovementFilter{})
	if len(all) != 3 || all[0].SteamID64 != "a" || all[0].Seconds != 10.5 || all[1].Seconds != 40.5 || all[2].Round != 4 {
		t.Fatalf("all = %+v", all)
	}
	if want := 1000 + 672 - 192; all[0].Tick != want {
		t.Errorf("tick = %d, want %d: three seconds before the move", all[0].Tick, want)
	}
	if got := FirstMovements(overlay, FirstMovementFilter{Players: []string{"b"}}); len(got) != 1 || got[0].SteamID64 != "b" {
		t.Errorf("player b = %+v", got)
	}
	if got := FirstMovements(overlay, FirstMovementFilter{From: 5, To: 30}); len(got) != 1 || got[0].Round != 3 || got[0].SteamID64 != "a" {
		t.Errorf("window 5 to 30 = %+v, want a's move in round 3", got)
	}
	zone := PatternArea{MinX: 400, MinY: -10, MaxX: 600, MaxY: 10}
	if got := FirstMovements(overlay, FirstMovementFilter{Area: &zone}); len(got) != 2 || got[0].Round != 3 || got[1].SteamID64 != "b" {
		t.Errorf("through the zone = %+v, want round 3 only, with both players' moves", got)
	}
}
