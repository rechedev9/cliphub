package workers

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

func TestRadarWindowsFollowEachPlaysPOVAndSkipVictimReplays(t *testing.T) {
	t.Parallel()
	plan := recording.RecordingPlan{TargetSteamID64: "1", Tickrate: 64, DemoDurationTicks: 1540, Segments: []recording.RecordingSegment{
		{ID: "seg-001", TickStart: 1000, TickEnd: 1500, ExactWindow: true},
		{ID: "seg-001" + recording.VictimReplaySuffix, TickStart: 1300, TickEnd: 1500, ExactWindow: true},
		{ID: "seg-002", TickStart: 200, TickEnd: 400, ExactWindow: true, Observe: &killplan.Target{SteamID64: "2"}},
	}}
	windows, err := radarWindows(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(windows) != 2 || windows[0].ID != "seg-001" || windows[1].ID != "seg-002" {
		t.Fatalf("windows = %+v, want both plays without the victim replay", windows)
	}
	if windows[0].POVSteamID64 != "1" || windows[1].POVSteamID64 != "2" {
		t.Errorf("POVs = %q %q, want the target and the observed teammate", windows[0].POVSteamID64, windows[1].POVSteamID64)
	}
	if windows[0].TickEnd != 1540 || windows[1].TickEnd != 400+64 {
		t.Errorf("ends = %d %d, want a second of tail capped at the demo's end", windows[0].TickEnd, windows[1].TickEnd)
	}
	plan.Segments = []recording.RecordingSegment{{ID: "long", TickStart: 0, TickEnd: 64 * 700, ExactWindow: true}}
	plan.DemoDurationTicks = 0
	if _, err := radarWindows(plan); err == nil {
		t.Error("a capture past the sampling bound was accepted")
	}
}

func TestShortRadarRendersWithoutItWhenTheDemoCannotBeRead(t *testing.T) {
	t.Parallel()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	j := job.Job{ID: uuid.New(), DemoPath: "demos/match.dem"}
	if err := store.Put(j.DemoPath, bytes.NewReader([]byte("not a demo"))); err != nil {
		t.Fatal(err)
	}
	plan := recording.RecordingPlan{DemoSHA256: "mismatch", DemoMap: "de_mirage", Tickrate: 64, Segments: []recording.RecordingSegment{{ID: "seg-001", TickStart: 1000, TickEnd: 1500, ExactWindow: true}}}
	w := &RenderWorker{storage: store}
	in := shortDataInput{job: j, edit: renderplan.EditRequest{Radar: true}, result: recording.RecordingResult{Plan: plan}, workDir: t.TempDir()}
	if args := w.writeShortRadar(context.Background(), in); args != nil {
		t.Fatalf("args = %q, want none: the demo does not match the capture", args)
	}
}

func TestShortOverlayArgsCarryTheRadarOptions(t *testing.T) {
	t.Parallel()
	got := shortOverlayArgs(renderplan.EditRequest{Radar: true, RadarPosition: "top-right", RadarSize: "large", RadarOpacity: 0.6})
	want := []string{"--radar", "--radar-position", "top-right", "--radar-size", "large", "--radar-opacity", "0.6"}
	if !slices.Equal(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
	if got := shortOverlayArgs(renderplan.EditRequest{Radar: true}); !slices.Equal(got, []string{"--radar"}) {
		t.Fatalf("default radar args = %q", got)
	}
}
