package workers

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func TestWorldWindowsFollowEachPlayAndItsPOV(t *testing.T) {
	plan := recording.RecordingPlan{
		Tickrate:          64,
		TargetSteamID64:   "1",
		DemoDurationTicks: 5000,
		Segments: []recording.RecordingSegment{
			{ID: "a", TickStart: 1000, TickEnd: 1500, ExactWindow: true},
			{ID: "b", TickStart: 4000, TickEnd: 4990, ExactWindow: true, Observe: &killplan.Target{SteamID64: "2"}},
		},
	}
	windows, err := worldWindows(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(windows) != 2 {
		t.Fatalf("windows = %+v, want one per play", windows)
	}
	// A play runs a little past its end tick, never past the demo.
	if w := windows[0]; w.ID != "a" || w.POVSteamID64 != "1" || w.TickStart != 1000 || w.TickEnd != 1500+64 {
		t.Errorf("first window = %+v", w)
	}
	if w := windows[1]; w.POVSteamID64 != "2" || w.TickEnd != 5000 {
		t.Errorf("multi-POV window = %+v, want the observed player up to the demo end", w)
	}
}

func TestWorldWindowsRefuseWhatTheyCannotSample(t *testing.T) {
	long := recording.RecordingPlan{Tickrate: 64, Segments: []recording.RecordingSegment{
		{ID: "a", TickStart: 0, TickEnd: 601 * 64, ExactWindow: true},
	}}
	for name, plan := range map[string]recording.RecordingPlan{
		"no tick rate": {Segments: long.Segments},
		"no plays":     {Tickrate: 64},
		"too long":     long,
	} {
		if _, err := worldWindows(plan); err == nil {
			t.Errorf("%s: worldWindows accepted the plan", name)
		}
	}
}
