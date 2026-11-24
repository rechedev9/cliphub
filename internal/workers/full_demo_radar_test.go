package workers

import (
	"slices"
	"testing"

	"github.com/rechedev9/cliphub/internal/radaroverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

func TestFullDemoRadarSamplesEveryCapturedRoundOfTheTimeline(t *testing.T) {
	d := recapplan.Document{
		Input: recapplan.Input{TargetSteamID64: "76561198000000001"},
		Clock: recapplan.Clock{TickRate: 64},
		Rounds: []recapplan.Round{
			{ID: "round-001", RequestedStartTick: 1000, CaptureStartTick: 936, RequestedEndTick: 4000, EffectiveEndTick: 3900, CaptureEndTick: 4064},
			{ID: "round-002", RequestedStartTick: 5000, CaptureStartTick: 4936, RequestedEndTick: 8000, EffectiveEndTick: 8000, CaptureEndTick: 8064},
			{ID: "round-003", RequestedStartTick: 9000, CaptureStartTick: 8936, RequestedEndTick: 9900, EffectiveEndTick: 9900, CaptureEndTick: 9964},
		},
		Timeline: []recapplan.TimelineItem{
			{Role: "bumper", SourceRef: "intro"},
			{Role: "round", SourceRef: "round-001"},
			{Role: "round", SourceRef: "round-003"},
		},
	}
	got := fullDemoRadarWindows(d, 10000)
	want := []radaroverlay.Window{
		{ID: "round-001", POVSteamID64: "76561198000000001", TickStart: 936, TickEnd: 4064 + 64},
		{ID: "round-003", POVSteamID64: "76561198000000001", TickStart: 8936, TickEnd: 10000},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("windows = %+v, want the timeline's rounds from capture start to a second past the end, inside the demo", got)
	}
}
