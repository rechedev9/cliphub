package parser

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func TestSegmentDeathsEndsEachClipOnTheDeath(t *testing.T) {
	killer := killplan.Player{SteamID64: "76561198000000002", NameInDemo: "enemy", TeamAtKill: "T"}
	deaths := []TargetDeath{
		{Round: 4, Tick: 9000, Killer: killer, Weapon: "awp", Headshot: true},
		{Round: 2, Tick: 3000},
		{Round: 2, Tick: 3100},
		{Round: 6, Tick: 12950},
	}
	liveStarts := []RoundLiveStart{{Round: 2, Tick: 2800}, {Round: 4, Tick: 7000}, {Round: 6, Tick: 12000}}
	roundStarts := []RoundStart{{Round: 2, Tick: 2000}, {Round: 4, Tick: 6000}, {Round: 6, Tick: 11000}, {Round: 7, Tick: 12990}}
	segs := SegmentDeaths(DeathSegmentsInput{Deaths: deaths, LiveStarts: liveStarts, RoundStarts: roundStarts, Tickrate: testTickrate})
	if len(segs) != 3 {
		t.Fatalf("got %d segments, want one per round with a death: %+v", len(segs), segs)
	}
	type window struct {
		id                    string
		round, start, end, at int
	}
	want := []window{
		// Starts at the live start: the death came 3 s after freeze end.
		{"death-001", 2, 2800, 3000 + testTickrate, 3000},
		{"death-002", 4, 9000 - 8*testTickrate, 9000 + testTickrate, 9000},
		// The tail stops before the next round starts.
		{"death-003", 6, 12950 - 8*testTickrate, 12989, 12950},
	}
	for i, w := range want {
		s := segs[i]
		if s.ID != w.id || s.Round != w.round || s.TickStart != w.start || s.TickEnd != w.end || s.LiveEndTick != w.at || len(s.Kills) != 0 {
			t.Errorf("segment %d = %s r%d [%d,%d] live %d kills %d, want %+v", i, s.ID, s.Round, s.TickStart, s.TickEnd, s.LiveEndTick, len(s.Kills), w)
		}
		if s.Death == nil || s.Death.Tick != w.at {
			t.Errorf("segment %d death = %+v, want tick %d", i, s.Death, w.at)
		}
	}
	if d := segs[1].Death; d.Killer != killer || d.Weapon != "awp" || !d.Headshot {
		t.Errorf("death of round 4 = %+v, want the AWP headshot by enemy", d)
	}
}
