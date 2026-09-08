package workers

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func twoFullDemoFixtureRounds(f *recapplan.Facts, o *recapplan.Options) {
	o.Editorial.RoundTailSeconds = 1
	f.Rounds[0].NextStartTick = 1200
	f.Rounds = append(f.Rounds, recapplan.RoundFacts{ID: "round-002", Number: 2, StartTick: 1200, FreezeEndTick: 1500, RoundEndTick: 1800, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}})
}

func TestFullDemoCaptureCoverageReuseAndOrigins(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*recapplan.Facts, *recapplan.Options)
		missing []string
	}{
		{"unchanged", func(_ *recapplan.Facts, _ *recapplan.Options) {}, nil},
		{"narrower tail", func(_ *recapplan.Facts, o *recapplan.Options) { o.Editorial.RoundTailSeconds = 0.5 }, nil},
		{"expand only second round", func(_ *recapplan.Facts, o *recapplan.Options) {
			o.Editorial.RoundTailSeconds = 2
			o.Editorial.ManualRanges = []recapplan.ManualRange{{RoundID: "round-001", StartTick: 272, EndTick: 1060}}
		}, []string{"round-002"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, id := newFakeStorage(), uuid.New()
			old, dir, resultPath := fullDemoPublicationFixture(t, "original", twoFullDemoFixtureRounds)
			if _, err := uploadFullDemoRecordingOutputs(store, id, dir, resultPath, old, old, false); err != nil {
				t.Fatal(err)
			}
			old, err := decodeStoredRecordingResult(store, id)
			if err != nil {
				t.Fatal(err)
			}
			next, dir, resultPath := fullDemoPublicationFixture(t, "new second round", twoFullDemoFixtureRounds, tc.mutate)
			requested := []string{"round-001", "round-002"}
			missing, _, err := recordingOutputsReady(store, id, requested, next.Plan, context.Background())
			if err != nil || !reflect.DeepEqual(missing, tc.missing) {
				t.Fatalf("missing = %v, err = %v", missing, err)
			}
			if len(missing) == 0 {
				return
			}
			fullPlan := next.Plan.ToKillPlan()
			next.Plan.Segments = slices.DeleteFunc(next.Plan.Segments, func(segment recording.RecordingSegment) bool { return !slices.Contains(missing, segment.ID) })
			next.Plan.EditorialSegmentIDs = slices.Clone(missing)
			next.Artifacts = slices.DeleteFunc(next.Artifacts, func(a recording.RecordingArtifact) bool { return !slices.Contains(missing, a.SegmentID) })
			for id := range next.FullDemoEvidence.CertifiedEnds {
				if !slices.Contains(missing, id) {
					delete(next.FullDemoEvidence.CertifiedEnds, id)
				}
			}
			next.CaptureInputFingerprint, err = recording.CaptureInputFingerprint(next.Plan)
			if err != nil {
				t.Fatal(err)
			}
			merged, err := mergeRecordingResults(old, next, &fullPlan)
			if err != nil {
				t.Fatal(err)
			}
			if len(merged.FullDemoRuns) != 2 {
				t.Fatalf("original launches not retained: %d", len(merged.FullDemoRuns))
			}
			if err := recording.ValidateUploadResult(merged); err != nil {
				t.Fatal(err)
			}
			if _, err := uploadFullDemoRecordingOutputs(store, id, dir, resultPath, next, merged, true); err != nil {
				t.Fatal(err)
			}
			published, err := decodeStoredRecordingResult(store, id)
			if err != nil {
				t.Fatal(err)
			}
			for _, segmentID := range requested {
				key, err := published.SegmentClipKey(id, segmentID)
				if err != nil {
					t.Fatal(err)
				}
				want := "original"
				if segmentID == "round-002" {
					want = "new second round"
				}
				if string(store.files[key]) != want {
					t.Fatalf("%s lost original media", segmentID)
				}
			}
			for _, corruption := range []string{"end", "source player", "clip origin"} {
				t.Run(corruption, func(t *testing.T) {
					b, _ := json.Marshal(published)
					var broken recording.RecordingResult
					if err := json.Unmarshal(b, &broken); err != nil {
						t.Fatal(err)
					}
					switch corruption {
					case "end":
						broken.FullDemoEvidence.CertifiedEnds["round-001"]++
					case "source player":
						broken.FullDemoRuns[0].Plan.TargetSteamID64 = "76561198000000002"
					case "clip origin":
						broken.Artifacts[0].CaptureRevision = uuid.NewString()
					}
					if err := recording.ValidateUploadResult(broken); err == nil {
						t.Fatal("corrupted origin accepted")
					}
				})
			}
		})
	}
}

func TestFullDemoRenderHashIgnoresAttemptLocationsAndApprovalTime(t *testing.T) {
	result, _, _ := fullDemoPublicationFixture(t, "clip")
	snapshot := recapplan.Snapshot{Document: *result.Plan.FullDemo, Approval: recapplan.Approval{PlanHash: result.Plan.FullDemo.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()}}
	before, err := fullDemoRenderFingerprint(result, "gameplay-pov-60", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Approval.Timestamp = snapshot.Approval.Timestamp.Add(time.Minute)
	snapshot.Document.PlanID = uuid.NewString()
	result.Plan.DemoPath, result.Plan.OutputDir, result.Script = "another.dem", "other", "other.js"
	result.CaptureRevision = uuid.NewString()
	for i := range result.Artifacts {
		result.Artifacts[i].Path = "elsewhere.mp4"
	}
	after, err := fullDemoRenderFingerprint(result, "gameplay-pov-60", snapshot)
	if err != nil || before != after {
		t.Fatalf("volatile fields changed canonical render hash: %v", err)
	}
}

func TestFullDemoRenderHashInvalidatesLegacyIntroOnlyWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		result, _, _ := fullDemoPublicationFixture(t, "clip", func(_ *recapplan.Facts, o *recapplan.Options) {
			o.Overlays.Roster = enabled
		})
		snapshot := recapplan.Snapshot{Document: *result.Plan.FullDemo, Approval: recapplan.Approval{PlanHash: result.Plan.FullDemo.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()}}
		effective, err := recapplan.ApplyCertifiedEnds(snapshot, result.FullDemoEvidence.CertifiedEnds)
		if err != nil {
			t.Fatal(err)
		}
		// The old cache identity omitted timing and could reuse a 5-14s intro.
		type legacyInput struct {
			SegmentID, ContentSHA256 string
			StartTick, EndTick       int
		}
		segment, artifact := result.Plan.Segments[0], result.Artifacts[0]
		legacyHash, err := recapplan.HashValue(struct {
			Policy, Variant, EffectivePlanHash string
			Captures                           []legacyInput
		}{"full-demo-render-v1", "gameplay-pov-60", effective.PlanHash, []legacyInput{{segment.ID, artifact.ContentSHA256, segment.TickStart, result.FullDemoEvidence.CertifiedEnds[segment.ID]}}})
		if err != nil {
			t.Fatal(err)
		}
		got, err := fullDemoRenderFingerprint(result, "gameplay-pov-60", snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if (got != legacyHash) != enabled {
			t.Fatalf("roster=%t: legacy render reuse changed=%t", enabled, got != legacyHash)
		}
	}
}

func TestFullDemoShortFramesRequireOnlyAffectedRoundsToBeRecaptured(t *testing.T) {
	store, id := newFakeStorage(), uuid.New()
	result, dir, resultPath := fullDemoPublicationFixture(t, "original", twoFullDemoFixtureRounds)
	if _, err := uploadFullDemoRecordingOutputs(store, id, dir, resultPath, result, result, false); err != nil {
		t.Fatal(err)
	}
	stored, err := decodeStoredRecordingResult(store, id)
	if err != nil {
		t.Fatal(err)
	}
	stored.Artifacts[0].FrameCount-- // Old muxed clip; its duration still passed the former 250ms tolerance.
	if err := putRecordingResult(store, id, stored); err != nil {
		t.Fatal(err)
	}
	missing, _, err := recordingOutputsReady(store, id, []string{"round-001", "round-002"}, result.Plan, context.Background())
	if err != nil || !slices.Equal(missing, []string{"round-001"}) {
		t.Fatalf("only the short round should be recaptured: missing=%v, err=%v", missing, err)
	}
	snapshot := recapplan.Snapshot{Document: *stored.Plan.FullDemo, Approval: recapplan.Approval{PlanHash: stored.Plan.FullDemo.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()}}
	if _, err := fullDemoRenderFingerprint(stored, "gameplay-pov-60", snapshot); err == nil || !recording.IsNotReusableMessage(err.Error()) || !strings.Contains(err.Error(), "round-001") {
		t.Fatalf("render must request recapture before materializing media: %v", err)
	}
	// The corrected capture replaces the bad clip and retains the complete
	// round's original bytes and attestation through the normal merge/publication.
	next, nextDir, nextPath := fullDemoPublicationFixture(t, "recaptured", twoFullDemoFixtureRounds)
	next.Plan.Segments = next.Plan.Segments[:1]
	next.Plan.EditorialSegmentIDs = []string{"round-001"}
	next.Artifacts = next.Artifacts[:1]
	delete(next.FullDemoEvidence.CertifiedEnds, "round-002")
	next.CaptureInputFingerprint, err = recording.CaptureInputFingerprint(next.Plan)
	if err != nil {
		t.Fatal(err)
	}
	fullPlan := result.Plan.ToKillPlan()
	merged, err := mergeRecordingResults(stored, next, &fullPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uploadFullDemoRecordingOutputs(store, id, nextDir, nextPath, next, merged, true); err != nil {
		t.Fatal(err)
	}
	missing, _, err = recordingOutputsReady(store, id, []string{"round-001", "round-002"}, result.Plan, context.Background())
	if err != nil || len(missing) != 0 {
		t.Fatalf("repaired capture should be reusable: missing=%v, err=%v", missing, err)
	}
	repaired, err := decodeStoredRecordingResult(store, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fullDemoRenderFingerprint(repaired, "gameplay-pov-60", snapshot); err != nil {
		t.Fatalf("repaired capture should be renderable: %v", err)
	}
	for id, want := range map[string]string{"round-001": "recaptured", "round-002": "original"} {
		for _, a := range repaired.Artifacts {
			if a.SegmentID == id && string(store.files[a.StorageKey]) != want {
				t.Fatalf("%s did not retain the expected media", id)
			}
		}
	}
}

func TestFullDemoRecordingAttemptRejectsShortMuxedClip(t *testing.T) {
	result, dir, _ := fullDemoPublicationFixture(t, "clip")
	result.Artifacts[0].FrameCount--
	if err := recording.ValidateRecordingAttempt(result.Plan, dir, result); err == nil || !strings.Contains(err.Error(), "round-001") {
		t.Fatalf("new capture must reject missing frames before publication: %v", err)
	}
}
