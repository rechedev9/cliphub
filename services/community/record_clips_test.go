package main

import "testing"

func TestClipKillPlanFilmsEachThrowerFromTheirOwnPOV(t *testing.T) {
	d := clipDemo{Demo: "/d/1__a-m1-mirage.dem", Map: "de_mirage", TickRate: 64, DurationTicks: 90000, Clips: []clipShot{
		{Name: "1/a-r3-t20000-76561198000000001.mp4", SteamID64: "76561198000000001", Round: 3, ThrowTick: 20000, StartTick: 19808, EndTick: 20224},
		{Name: "1/a-r5-t40000-76561198000000002.mp4", SteamID64: "76561198000000002", Round: 5, ThrowTick: 40000, StartTick: 39808, EndTick: 40224},
	}}
	p := clipKillPlan(d, "abc")
	if p.SchemaVersion != "1.2" || p.Demo.SHA256 != "abc" || p.Demo.DurationTicks != 90000 || p.Target.SteamID64 != "76561198000000001" {
		t.Fatalf("plan head = %+v %+v", p.Demo, p.Target)
	}
	if len(p.Segments) != 2 || p.Segments[0].POV != nil || p.Segments[1].POV == nil || p.Segments[1].POV.SteamID64 != "76561198000000002" {
		t.Fatalf("segments = %+v", p.Segments)
	}
	s := p.Segments[1]
	if s.ID != "clip-002" || s.Round != 5 || s.TickStart != 39808 || s.TickEnd != 40224 || len(s.Kills) != 0 {
		t.Fatalf("segment = %+v", s)
	}
}

func TestClipHUDFramesFollowTheWindow(t *testing.T) {
	if got := clipFrames(clipShot{StartTick: 1000, EndTick: 1416}, 64); got != 390 {
		t.Fatalf("frames = %d, want 390", got)
	}
}
