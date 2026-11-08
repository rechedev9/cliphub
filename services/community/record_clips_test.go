package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

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

func TestClipCaptureKeepsTheThrowersCrosshair(t *testing.T) {
	args := recorderArgs(recordTools{hlae: "h", cs2: "c"}, recorderFiles{plan: "p", demo: "d", out: "o"})
	i := slices.Index(args, "--hud")
	if i < 0 || args[i+1] != "deathnotices" {
		t.Fatalf("a clip without the crosshair hides the aim: %v", args)
	}
}

func TestClipAimMarkIsCentredAndLeavesTheCentreOpen(t *testing.T) {
	f := clipFilter("a.ass", "fonts")
	if !strings.HasPrefix(f, "scale=1920:1080,drawbox=") || !strings.Contains(f, "drawbox=x=938:y=539:w=16:h=3:color=white") || !strings.Contains(f, "drawbox=x=966:y=539:w=16:h=3:color=white") {
		t.Fatalf("aim mark not centred: %s", f)
	}
	if strings.Contains(f, "x=959:y=539") {
		t.Fatalf("the mark covers the pro's own crosshair: %s", f)
	}
}

func TestComboConcatListKeepsOrderAndQuotes(t *testing.T) {
	got := concatList([]string{filepath.FromSlash("C:/clips/1/a.mp4"), "/c/it's.mp4"})
	if got != "file 'C:/clips/1/a.mp4'\nfile '/c/it'\\''s.mp4'\n" {
		t.Fatalf("list = %q", got)
	}
}

func TestClipFilterEndsAt720pAndClipFPS(t *testing.T) {
	if f := clipFilter("a.ass", "fonts"); !strings.HasSuffix(f, ",scale=1280:720:flags=lanczos,fps=30,format=yuv420p") {
		t.Fatalf("filter tail = %s", f)
	}
}

func TestFailedClipReadsTheSegmentTheRecorderNames(t *testing.T) {
	out := "error: capture POV verification failed: observer target remained unknown during clip-008; check CS2 console log"
	if got := failedClip(out, 10); got != 7 {
		t.Fatalf("failedClip = %d, want 7", got)
	}
	if failedClip(out, 5) != -1 || failedClip("cs2.exe is already running", 10) != -1 {
		t.Fatal("only a named segment inside the plan counts")
	}
}
