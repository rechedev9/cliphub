package workers

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func victimReplayResult(t *testing.T, kp killplan.Plan, ids ...string) recording.RecordingResult {
	t.Helper()
	return captureResult(t, captureFixture{kp: kp, ids: ids, victimReplay: true})
}

type captureFixture struct {
	kp           killplan.Plan
	ids          []string
	victimReplay bool
}

func captureResult(t *testing.T, f captureFixture) recording.RecordingResult {
	t.Helper()
	kp, ids := f.kp, f.ids
	selected, err := filterKillPlanSegments(&kp, ids)
	if err != nil {
		t.Fatal(err)
	}
	stream := recording.DefaultStreamConfig()
	stream.VictimReplay = f.victimReplay
	plan, err := recording.NewPlanFromKillPlan(*selected, "demo.dem", "out", stream)
	if err != nil {
		t.Fatal(err)
	}
	result := recording.RecordingResult{Plan: plan, CaptureMode: recording.CaptureModeReal, CaptureVerified: true}
	for _, capture := range plan.CaptureSegments() {
		result.Artifacts = append(result.Artifacts, recording.RecordingArtifact{SegmentID: capture.ID, Role: "segment", Type: "video", Path: capture.ID + ".mp4"})
	}
	return result
}

func replayKillPlan() killplan.Plan {
	kp := killplan.NewPlan()
	kp.Demo = killplan.Demo{SHA256: strings.Repeat("b", 64), Tickrate: 64, DurationTicks: 60000}
	kp.Target.SteamID64 = "76561198148986856"
	victim := killplan.Player{SteamID64: "76561198000000001"}
	kp.Segments = []killplan.Segment{
		{ID: "seg-001", TickStart: 10000, TickEnd: 11000, Kills: []killplan.Kill{{Tick: 10500, Victim: victim}}},
		{ID: "seg-002", TickStart: 20000, TickEnd: 21000, Kills: []killplan.Kill{{Tick: 20500, Victim: victim}}},
	}
	return kp
}

func TestMergeRecordingResultsCarriesVictimReplaysWithTheirSegments(t *testing.T) {
	kp := replayKillPlan()
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

func TestRecordingOutputsReadyReusesMainClipsAcrossTheReplayToggle(t *testing.T) {
	kp := replayKillPlan()
	both := []string{"seg-001", "seg-002"}
	cases := []struct {
		name             string
		stored, expected bool
		wantMissing      int
	}{
		{name: "replays recorded, now off", stored: true, expected: false, wantMissing: 0},
		{name: "replays recorded, still on", stored: true, expected: true, wantMissing: 0},
		{name: "no replays recorded, now on", stored: false, expected: true, wantMissing: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stored := captureResult(t, captureFixture{kp: kp, ids: both, victimReplay: tc.stored})
			stored.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(stored.Plan)
			expected := captureResult(t, captureFixture{kp: kp, ids: both, victimReplay: tc.expected}).Plan
			store := newFakeStorage()
			id := uuid.New()
			if err := putRecordingResult(store, id, stored); err != nil {
				t.Fatal(err)
			}
			if err := store.Put(recording.ScriptArtifactKey(id), bytes.NewReader([]byte("script"))); err != nil {
				t.Fatal(err)
			}
			for _, capture := range stored.Plan.CaptureSegments() {
				if err := store.Put(mustSegmentClipKey(t, id, capture.ID), bytes.NewReader([]byte("clip"))); err != nil {
					t.Fatal(err)
				}
			}
			missing, _, err := recordingOutputsReady(t.Context(), store, id, both, expected)
			if err != nil {
				t.Fatal(err)
			}
			if len(missing) != tc.wantMissing {
				t.Fatalf("missing = %v, want %d segments", missing, tc.wantMissing)
			}
		})
	}
}

func TestMergeRecordingResultsKeepsReplaysAfterTheOptionIsTurnedOff(t *testing.T) {
	kp := replayKillPlan()
	prev := captureResult(t, captureFixture{kp: kp, ids: []string{"seg-001", "seg-002"}, victimReplay: true})
	next := captureResult(t, captureFixture{kp: kp, ids: []string{"seg-002"}})
	if !captureProfilesCompatible(prev.Plan, next.Plan) {
		t.Fatal("captures that differ only in victim replay must stay compatible")
	}
	merged, err := mergeRecordingResults(prev, next, &kp)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !merged.Plan.Stream.VictimReplay {
		t.Fatal("merged plan carries seg-001's replay but its stream flag is off")
	}
	if _, ok := recording.ResolveVictimReplayClip(merged, "seg-001"); !ok {
		t.Fatal("seg-001 replay clip was dropped")
	}
}
