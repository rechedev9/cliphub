package workers

import (
	"slices"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func victimReplayResult(t *testing.T, kp killplan.Plan, ids ...string) recording.RecordingResult {
	t.Helper()
	selected, err := filterKillPlanSegments(&kp, ids)
	if err != nil {
		t.Fatal(err)
	}
	stream := recording.DefaultStreamConfig()
	stream.VictimReplay = true
	plan, err := recording.NewPlanFromKillPlan(*selected, "demo.dem", "out", stream)
	if err != nil {
		t.Fatal(err)
	}
	result := recording.RecordingResult{Plan: plan}
	for _, capture := range plan.CaptureSegments() {
		result.Artifacts = append(result.Artifacts, recording.RecordingArtifact{SegmentID: capture.ID, Role: "segment", Type: "video", Path: capture.ID + ".mp4"})
	}
	return result
}

func TestMergeRecordingResultsCarriesVictimReplaysWithTheirSegments(t *testing.T) {
	kp := killplan.NewPlan()
	kp.Demo = killplan.Demo{SHA256: strings.Repeat("b", 64), Tickrate: 64, DurationTicks: 60000}
	kp.Target.SteamID64 = "76561198148986856"
	victim := killplan.Player{SteamID64: "76561198000000001"}
	kp.Segments = []killplan.Segment{
		{ID: "seg-001", TickStart: 10000, TickEnd: 11000, Kills: []killplan.Kill{{Tick: 10500, Victim: victim}}},
		{ID: "seg-002", TickStart: 20000, TickEnd: 21000, Kills: []killplan.Kill{{Tick: 20500, Victim: victim}}},
	}
	prev := victimReplayResult(t, kp, "seg-001", "seg-002")
	// The new run recaptures seg-002 only, without a replay for it.
	next := victimReplayResult(t, kp, "seg-002")
	next.Plan.Segments[0].VictimReplay = nil
	next.Artifacts = next.Artifacts[:1]

	merged, err := mergeRecordingResults(prev, next, &kp)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var clips []string
	for _, artifact := range merged.Artifacts {
		clips = append(clips, artifact.SegmentID)
	}
	slices.Sort(clips)
	if want := []string{"seg-001", "seg-001-victim", "seg-002"}; !slices.Equal(clips, want) {
		t.Fatalf("merged clips = %v, want %v (stale seg-002 replay dropped)", clips, want)
	}
}
