package workers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func cameraSnapshot(d recapplan.Document) recapplan.Snapshot {
	return recapplan.Snapshot{Document: d, Approval: recapplan.Approval{PlanHash: d.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()}}
}

func publishedCameraResult(t *testing.T, end func(recording.RecordingSegment) int) (recording.RecordingResult, recording.RecordingResult, recapplan.Document) {
	t.Helper()
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
	next, nextDir, nextPath, _ := cameraOnlyCapture(t, end)
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
	return old, published, *on.Plan.FullDemo
}

func TestFullDemoRenderHashGainsTheSecondCameraOnlyWhenItIsOn(t *testing.T) {
	verified := func(s recording.RecordingSegment) int { return s.TickEnd }
	old, published, on := publishedCameraResult(t, verified)
	today, err := fullDemoRenderFingerprint(old, "gameplay-pov-60", cameraSnapshot(*old.Plan.FullDemo))
	if err != nil {
		t.Fatal(err)
	}
	off, err := fullDemoRenderFingerprint(published, "gameplay-pov-60", cameraSnapshot(*old.Plan.FullDemo))
	if err != nil || off != today {
		t.Fatalf("with the option off a stored camera clip changed the render hash: %v", err)
	}
	drawn, err := fullDemoRenderFingerprint(published, "gameplay-pov-60", cameraSnapshot(on))
	if err != nil || drawn == today {
		t.Fatalf("the second camera must change the render hash: %v", err)
	}
	again, err := fullDemoRenderFingerprint(published, "gameplay-pov-60", cameraSnapshot(on))
	if err != nil || again != drawn {
		t.Fatalf("the camera render hash is not stable: %v", err)
	}

	_, unverified, _ := publishedCameraResult(t, func(s recording.RecordingSegment) int { return s.TickStart })
	omitted, err := fullDemoRenderFingerprint(unverified, "gameplay-pov-60", cameraSnapshot(on))
	if err != nil || omitted == drawn || omitted == today {
		t.Fatalf("an omitted camera must render to its own hash: %v", err)
	}
	effective, err := recapplan.ApplyCertifiedEnds(cameraSnapshot(on), unverified.FullDemoEvidence.CertifiedEnds)
	if err != nil {
		t.Fatal(err)
	}
	if effective, err = recapplan.WithCameraEvidence(effective, unverified.UsableFullDemoCameras(effective)); err != nil || effective.SecondCamera != nil {
		t.Fatalf("the unverified camera must be left out of the effective plan: %v", err)
	}
}
