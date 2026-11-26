package workers

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

const cameraEnemy = "76561198000000101"

func withSecondCamera(f *recapplan.Facts, o *recapplan.Options) {
	f.Rounds[0].Kills = []killplan.Kill{{Tick: 800, Weapon: "ak47",
		Killer: killplan.Player{SteamID64: f.TargetSteamID64, NameInDemo: "pov", TeamAtKill: "CT"},
		Victim: killplan.Player{SteamID64: cameraEnemy, NameInDemo: "enemy", TeamAtKill: "T"}}}
	f.Holds = &recapplan.HoldEvidence{Version: recapplan.HoldEvidenceVersion, Kills: []recapplan.KillHold{{Tick: 800, VictimSteamID64: cameraEnemy, MoveUnits: 6, Covered: true}}}
	o.SecondCamera = &recapplan.SecondCameraOptions{Enabled: true, MaxWindows: 5}
}

func withHoldsOnly(f *recapplan.Facts, o *recapplan.Options) {
	withSecondCamera(f, o)
	o.SecondCamera = nil
}

func cameraOnlyCapture(t *testing.T, end func(recording.RecordingSegment) int, mutations ...func(*recapplan.Facts, *recapplan.Options)) (recording.RecordingResult, string, string, recapplan.CameraWindow) {
	t.Helper()
	result, dir, resultPath := fullDemoPublicationFixture(t, "recaptured", append([]func(*recapplan.Facts, *recapplan.Options){withSecondCamera}, mutations...)...)
	if result.Plan.FullDemo.SecondCamera == nil {
		t.Fatal("fixture planned no second camera")
	}
	w := result.Plan.FullDemo.SecondCamera.Windows[0]
	if err := result.Plan.MarkReplayOnly([]string{w.RoundID}); err != nil {
		t.Fatal(err)
	}
	captures := result.Plan.CaptureSegments()
	camera := captures[slices.IndexFunc(captures, func(s recording.RecordingSegment) bool { return s.ID == w.ID })]
	clip := filepath.Join(dir, "segments", camera.ID+".mp4")
	if err := os.WriteFile(clip, []byte("camera"), 0600); err != nil {
		t.Fatal(err)
	}
	frames, err := recapplan.TickFrames(camera.TickEnd-camera.TickStart, result.Plan.Tickrate)
	if err != nil {
		t.Fatal(err)
	}
	result.Artifacts = slices.DeleteFunc(result.Artifacts, func(a recording.RecordingArtifact) bool { return a.SegmentID == w.RoundID })
	result.Artifacts = append(result.Artifacts, recording.RecordingArtifact{SegmentID: camera.ID, Role: "segment", Type: "video", Path: clip, SizeBytes: 6, FrameCount: frames, FrameRate: "60/1", DurationSeconds: float64(frames) / 60})
	delete(result.FullDemoEvidence.CertifiedEnds, w.RoundID)
	result.FullDemoEvidence.CertifiedEnds[camera.ID] = end(camera)
	if err := result.DigestSegmentFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	result.CaptureInputFingerprint, err = recording.CaptureInputFingerprint(result.Plan)
	if err != nil {
		t.Fatal(err)
	}
	return result, dir, resultPath, w
}

func TestFullDemoSecondCameraRecordsOnlyTheWindowsOfARecordedVideo(t *testing.T) {
	for _, tc := range []struct {
		name   string
		end    func(recording.RecordingSegment) int
		usable bool
	}{
		{"camera verified", func(s recording.RecordingSegment) int { return s.TickEnd }, true},
		{"camera unverified", func(s recording.RecordingSegment) int { return s.TickStart }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, id := newFakeStorage(), uuid.New()
			old, dir, resultPath := fullDemoPublicationFixture(t, "original", withHoldsOnly)
			if _, err := uploadFullDemoRecordingOutputs(store, id, dir, resultPath, old, old, false); err != nil {
				t.Fatal(err)
			}
			old, err := decodeStoredRecordingResult(store, id)
			if err != nil {
				t.Fatal(err)
			}
			on, _, _ := fullDemoPublicationFixture(t, "unused", withSecondCamera)
			missing, reuse, err := recordingOutputsReady(context.Background(), store, id, []string{"round-001"}, on.Plan)
			if err != nil || !slices.Equal(missing, []string{"round-001"}) || !slices.Equal(reuse.replayOnly, []string{"round-001"}) {
				t.Fatalf("turning the camera on: missing=%v replay only=%v err=%v, want only the window", missing, reuse.replayOnly, err)
			}
			doc := *on.Plan.FullDemo
			w := doc.SecondCamera.Windows[0]
			if cameras, omitted, err := recording.MissingFullDemoCameras(store, id, doc); err != nil || !slices.Equal(cameras, []string{w.ID}) || len(omitted) != 0 {
				t.Fatalf("capture check before recording: missing=%v omitted=%v err=%v", cameras, omitted, err)
			}
			if rounds, err := recording.MissingFullDemoRounds(store, id, doc); err != nil || len(rounds) != 0 {
				t.Fatalf("the camera must not recapture its round: %v %v", rounds, err)
			}

			next, nextDir, nextPath, _ := cameraOnlyCapture(t, tc.end)
			fullPlan := on.Plan.ToKillPlan()
			merged, err := mergeRecordingResults(old, next, &fullPlan)
			if err != nil {
				t.Fatal(err)
			}
			if len(merged.Plan.FullDemoSources) != 1 || merged.Plan.FullDemoSources[0].SecondCamera != nil {
				t.Fatalf("the source that differs only by its windows was dropped: %d sources", len(merged.Plan.FullDemoSources))
			}
			if _, err := uploadFullDemoRecordingOutputs(store, id, nextDir, nextPath, next, merged, true); err != nil {
				t.Fatal(err)
			}
			published, err := decodeStoredRecordingResult(store, id)
			if err != nil {
				t.Fatal(err)
			}
			if err := recording.ValidateUploadResult(published); err != nil {
				t.Fatalf("merged camera from a later run rejected: %v", err)
			}
			for segmentID, want := range map[string]string{"round-001": "original", w.ID: "camera"} {
				key, err := published.SegmentClipKey(id, segmentID)
				if err != nil || string(store.files[key]) != want {
					t.Fatalf("%s lost its media: %v", segmentID, err)
				}
			}
			if got := published.UsableFullDemoCameras(doc)[w.ID]; got != tc.usable {
				t.Fatalf("usable = %t, want %t", got, tc.usable)
			}
			cameras, omitted, err := recording.MissingFullDemoCameras(store, id, doc)
			wantOmitted := []string{}
			if !tc.usable {
				wantOmitted = []string{w.ID}
			}
			if err != nil || len(cameras) != 0 || !slices.Equal(omitted, wantOmitted) {
				t.Fatalf("capture check after recording: missing=%v omitted=%v err=%v", cameras, omitted, err)
			}
			for name, expected := range map[string]recording.RecordingPlan{"on": on.Plan, "removed": old.Plan} {
				missing, _, err := recordingOutputsReady(context.Background(), store, id, []string{"round-001"}, expected)
				if err != nil || len(missing) != 0 {
					t.Fatalf("%s: a recorded window is never recaptured: missing=%v err=%v", name, missing, err)
				}
			}

			broken := published
			broken.Artifacts = slices.Clone(published.Artifacts)
			for i := range broken.Artifacts {
				if broken.Artifacts[i].SegmentID == w.ID {
					broken.Artifacts[i].CaptureRevision = old.CaptureRevision
				}
			}
			if err := recording.ValidateUploadResult(broken); err == nil {
				t.Fatal("a camera clip attributed to a run that never recorded it was accepted")
			}
		})
	}
}

func TestFullDemoSecondCameraKeepsARecordedRoundThatOutlastsThePlan(t *testing.T) {
	store, id := newFakeStorage(), uuid.New()
	old, dir, resultPath := fullDemoPublicationFixture(t, "original", withHoldsOnly)
	if _, err := uploadFullDemoRecordingOutputs(store, id, dir, resultPath, old, old, false); err != nil {
		t.Fatal(err)
	}
	old, err := decodeStoredRecordingResult(store, id)
	if err != nil {
		t.Fatal(err)
	}
	withALaterRound := func(f *recapplan.Facts, _ *recapplan.Options) {
		f.Rounds = append(f.Rounds, recapplan.RoundFacts{ID: "round-002", Number: 2, StartTick: 1480, FreezeEndTick: 1790, RoundEndTick: 1900, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}})
	}
	on, _, _ := fullDemoPublicationFixture(t, "unused", withSecondCamera, withALaterRound)
	recordedEnd := old.Plan.Segments[0].TickEnd
	if recordedEnd <= on.Plan.Segments[0].TickEnd {
		t.Fatalf("the fixture needs a recorded round longer than the plan: %d against %d", recordedEnd, on.Plan.Segments[0].TickEnd)
	}
	rounds := []string{"round-001", "round-002"}
	missing, reuse, err := recordingOutputsReady(context.Background(), store, id, rounds, on.Plan)
	if err != nil || !slices.Equal(missing, rounds) || !slices.Equal(reuse.replayOnly, []string{"round-001"}) {
		t.Fatalf("missing=%v replay only=%v err=%v, want the new round and only the window of the recorded one", missing, reuse.replayOnly, err)
	}
	next, nextDir, nextPath, w := cameraOnlyCapture(t, func(s recording.RecordingSegment) int { return s.TickEnd }, withALaterRound)
	fullPlan := on.Plan.ToKillPlan()
	merged, err := mergeRecordingResults(old, next, &fullPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uploadFullDemoRecordingOutputs(store, id, nextDir, nextPath, next, merged, true); err != nil {
		t.Fatal(err)
	}
	published, err := decodeStoredRecordingResult(store, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := recording.ValidateUploadResult(published); err != nil {
		t.Fatalf("a window added to a longer recorded round was rejected: %v", err)
	}
	if got := published.Plan.Segments[0]; got.TickEnd != recordedEnd || got.SecondCamera == nil || got.SecondCamera.ID != w.ID {
		t.Fatalf("round = %+v, want the recorded round up to %d with window %s", got, recordedEnd, w.ID)
	}
	if !published.UsableFullDemoCameras(*on.Plan.FullDemo)[w.ID] {
		t.Fatal("the recorded window is not usable")
	}
	if missing, _, err := recordingOutputsReady(context.Background(), store, id, rounds, on.Plan); err != nil || len(missing) != 0 {
		t.Fatalf("nothing is left to record: missing=%v err=%v", missing, err)
	}
}
