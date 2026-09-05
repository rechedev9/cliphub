package workers

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
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
	f.Rounds = append(f.Rounds, recapplan.RoundFacts{ID: "round-002", Number: 2, StartTick: 1200, FreezeEndTick: 1400, RoundEndTick: 1800, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}})
}

func TestFullDemoCaptureCoverageReuseAndOrigins(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*recapplan.Facts, *recapplan.Options)
		missing []string
	}{
		{"unchanged", func(_ *recapplan.Facts, _ *recapplan.Options) {}, nil},
		{"narrower", func(_ *recapplan.Facts, o *recapplan.Options) { o.Editorial.FreezeSeconds = 1 }, nil},
		{"expand only second round", func(_ *recapplan.Facts, o *recapplan.Options) {
			o.Editorial.FreezeSeconds = 1
			o.Editorial.RoundTailSeconds = 2
			o.Editorial.ManualRanges = []recapplan.ManualRange{{RoundID: "round-001", StartTick: 336, EndTick: 1060}}
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
