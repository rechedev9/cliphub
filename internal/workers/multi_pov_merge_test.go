package workers

import (
	"slices"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

// A second Short of a multi-POV job can film a teammate's play at the same time
// as a play the first Short filmed; the new capture must still be stored.
func TestMergeRecordingResultsDropsStoredPlaysTheNewPOVOverlaps(t *testing.T) {
	kp := replayKillPlan()
	anchor, mate := kp.Target, killplan.Target{SteamID64: "76561198000000077", NameInDemo: "mate"}
	victim := killplan.Player{SteamID64: "76561198000000001"}
	kp.Segments = []killplan.Segment{
		{ID: "seg-001", TickStart: 10000, TickEnd: 11000, POV: &anchor, Kills: []killplan.Kill{{Tick: 10500, Victim: victim}}},
		{ID: "seg-002", TickStart: 10500, TickEnd: 11500, POV: &mate, Kills: []killplan.Kill{{Tick: 11000, Victim: victim}}},
		{ID: "seg-003", TickStart: 20000, TickEnd: 21000, POV: &anchor, Kills: []killplan.Kill{{Tick: 20500, Victim: victim}}},
	}
	prev := captureResult(t, captureFixture{kp: kp, ids: []string{"seg-001", "seg-003"}})
	next := captureResult(t, captureFixture{kp: kp, ids: []string{"seg-002"}})
	merged, err := mergeRecordingResults(prev, next, &kp)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var segments, clips []string
	for _, s := range merged.Plan.Segments {
		segments = append(segments, s.ID)
	}
	for _, a := range merged.Artifacts {
		clips = append(clips, a.SegmentID)
	}
	slices.Sort(clips)
	if want := []string{"seg-002", "seg-003"}; !slices.Equal(segments, want) || !slices.Equal(clips, want) {
		t.Fatalf("merged segments %v clips %v, want %v: the overlapped seg-001 gives way", segments, clips, want)
	}
}
