package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/fragforge/internal/artifacts"
	"github.com/rechedev9/fragforge/internal/job"
	"github.com/rechedev9/fragforge/internal/recording"
	"github.com/rechedev9/fragforge/internal/storage"
)

type captureProgressObservingRepo struct {
	*fakeRepo
	store       storage.Storage
	atRecording recording.CaptureProgress
	observeErr  error
}

type failRecordingStatusRepo struct {
	*fakeRepo
	err   error
	calls int
}

func (r *failRecordingStatusRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status job.Status, reason string) error {
	if status == job.StatusRecording {
		r.calls++
		return r.err
	}
	return r.fakeRepo.UpdateStatus(ctx, id, status, reason)
}

func (r *captureProgressObservingRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status job.Status, reason string) error {
	if status == job.StatusRecording {
		rc, err := r.store.Open(artifacts.CaptureProgressKey(id))
		if err == nil {
			err = json.NewDecoder(rc).Decode(&r.atRecording)
			err = errors.Join(err, rc.Close())
		}
		r.observeErr = err
	}
	return r.fakeRepo.UpdateStatus(ctx, id, status, reason)
}

func storedCaptureProgress(t *testing.T, store storage.Storage, id uuid.UUID) recording.CaptureProgress {
	t.Helper()
	rc, err := store.Open(artifacts.CaptureProgressKey(id))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	var progress recording.CaptureProgress
	if err := json.NewDecoder(rc).Decode(&progress); err != nil {
		t.Fatal(err)
	}
	return progress
}

func TestCaptureProgressReporterNeverPublishesPartialClips(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	jobID := uuid.New()
	segments := t.TempDir()
	if err := os.WriteFile(filepath.Join(segments, "s1.mp4"), []byte("partial-local-output"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segments, "unselected.mp4"), []byte("ignore"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segments, "s2.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	attemptID := uuid.New()
	reporter := newCaptureProgressReporter(store, jobID, attemptID, segments, []string{"s1", "s2"})
	if err := reporter.report(); err != nil {
		t.Fatal(err)
	}
	rc, err := store.Open(artifacts.CaptureProgressKey(jobID))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	var progress recording.CaptureProgress
	if err := json.NewDecoder(rc).Decode(&progress); err != nil {
		t.Fatal(err)
	}
	if err := progress.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(progress.CompletedSegmentIDs) != 1 || progress.CompletedSegmentIDs[0] != "s1" {
		t.Fatalf("completed segment ids = %v, want [s1]", progress.CompletedSegmentIDs)
	}
	if progress.AttemptID != attemptID {
		t.Fatalf("attempt id = %s, want %s", progress.AttemptID, attemptID)
	}
	for _, segmentID := range []string{"s1", "s2"} {
		key, err := recording.SegmentClipArtifactKey(jobID, segmentID)
		if err != nil {
			t.Fatal(err)
		}
		if exists, err := store.Exists(key); err != nil || exists {
			t.Fatalf("progress committed %s: exists=%v err=%v", segmentID, exists, err)
		}
	}
}

func TestStartCaptureProgressAttemptReplacesStaleAttemptBeforeCapture(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	jobID := uuid.New()
	oldAttemptID := uuid.New()
	old := newCaptureProgressReporter(store, jobID, oldAttemptID, "", []string{"s1", "s2"})
	if err := old.write([]string{"s1", "s2"}); err != nil {
		t.Fatal(err)
	}

	attemptID, err := startCaptureProgressAttempt(store, jobID, []string{"s1", "s2"})
	if err != nil {
		t.Fatal(err)
	}
	if attemptID == oldAttemptID {
		t.Fatal("new capture reused the stale attempt id")
	}
	rc, err := store.Open(artifacts.CaptureProgressKey(jobID))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	var progress recording.CaptureProgress
	if err := json.NewDecoder(rc).Decode(&progress); err != nil {
		t.Fatal(err)
	}
	if progress.AttemptID != attemptID ||
		len(progress.CompletedSegmentIDs) != 0 ||
		len(progress.SegmentIDs) != 2 {
		t.Fatalf("fresh progress = %#v, want new attempt with 0/2 completed", progress)
	}
}

func TestRecordWorkerPublishesFreshAttemptBeforeRecordingStatus(t *testing.T) {
	store := newFakeStorage()
	base := newFakeRepo()
	jobID := uuid.New()
	plan := minimalKillPlan()
	base.jobs[jobID] = &job.Job{
		ID:       jobID,
		Status:   job.StatusParsed,
		DemoPath: "demos/test.dem",
		KillPlan: &plan,
	}
	if err := store.Put("demos/test.dem", bytes.NewReader([]byte("demo"))); err != nil {
		t.Fatal(err)
	}
	segmentIDs := killPlanSegmentIDs(&plan)
	oldAttemptID := uuid.New()
	stale := newCaptureProgressReporter(store, jobID, oldAttemptID, "", segmentIDs)
	if err := stale.write(segmentIDs); err != nil {
		t.Fatal(err)
	}
	repo := &captureProgressObservingRepo{fakeRepo: base, store: store}
	worker := NewRecordWorker(repo, store, RecordWorkerConfig{
		WorkDir:      t.TempDir(),
		RecorderPath: "zv-recorder",
		HLAEPath:     "HLAE.exe",
		CS2Path:      "cs2.exe",
	})
	worker.runner = &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("stop after recording starts")
	}}

	if err := worker.HandleRecordDemo(context.Background(), recordTask(t, jobID)); err == nil {
		t.Fatal("HandleRecordDemo error = nil, want recorder failure")
	}
	if repo.observeErr != nil {
		t.Fatalf("progress at recording transition: %v", repo.observeErr)
	}
	if repo.atRecording.AttemptID == oldAttemptID ||
		len(repo.atRecording.CompletedSegmentIDs) != 0 ||
		len(repo.atRecording.SegmentIDs) != len(segmentIDs) {
		t.Fatalf("progress at recording transition = %#v, want a fresh 0/%d attempt", repo.atRecording, len(segmentIDs))
	}
	final := storedCaptureProgress(t, store, jobID)
	if final.AttemptID != repo.atRecording.AttemptID {
		t.Fatalf("watcher attempt id = %s, want status-visible attempt %s", final.AttemptID, repo.atRecording.AttemptID)
	}
}

func TestRecordWorkerCacheHitKeepsExistingCaptureProgressAttempt(t *testing.T) {
	store := newFakeStorage()
	repo := newFakeRepo()
	jobID := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[jobID] = &job.Job{
		ID:       jobID,
		Status:   job.StatusParsed,
		DemoPath: "demos/test.dem",
		KillPlan: &plan,
	}
	putJSON(t, store, recording.ResultArtifactKey(jobID), recordingResultWithSegment("", "stale-local.mp4"))
	if err := store.Put(recording.ScriptArtifactKey(jobID), bytes.NewReader([]byte("script"))); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(mustSegmentClipKey(t, jobID, "seg-001"), bytes.NewReader([]byte("clip"))); err != nil {
		t.Fatal(err)
	}
	segmentIDs := killPlanSegmentIDs(&plan)
	oldAttemptID := uuid.New()
	stale := newCaptureProgressReporter(store, jobID, oldAttemptID, "", segmentIDs)
	if err := stale.write(segmentIDs); err != nil {
		t.Fatal(err)
	}
	worker := NewRecordWorker(repo, store, RecordWorkerConfig{})
	worker.runner = &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("runner should not be called on a recording cache hit")
		return nil, nil
	}}

	if err := worker.HandleRecordDemo(context.Background(), recordTask(t, jobID)); err != nil {
		t.Fatalf("HandleRecordDemo error = %v", err)
	}
	progress := storedCaptureProgress(t, store, jobID)
	if progress.AttemptID != oldAttemptID || len(progress.CompletedSegmentIDs) != len(segmentIDs) {
		t.Fatalf("capture progress after cache hit = %#v, want unchanged completed attempt %s", progress, oldAttemptID)
	}
}

func TestRecordWorkerSetupFailureKeepsExistingCaptureProgressAttempt(t *testing.T) {
	store := newFakeStorage()
	repo := newFakeRepo()
	jobID := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[jobID] = &job.Job{
		ID:       jobID,
		Status:   job.StatusParsed,
		DemoPath: "demos/test.dem",
		KillPlan: &plan,
	}
	segmentIDs := killPlanSegmentIDs(&plan)
	oldAttemptID := uuid.New()
	stale := newCaptureProgressReporter(store, jobID, oldAttemptID, "", segmentIDs)
	if err := stale.write(segmentIDs); err != nil {
		t.Fatal(err)
	}
	worker := NewRecordWorker(repo, store, RecordWorkerConfig{})

	if err := worker.HandleRecordDemo(context.Background(), recordTask(t, jobID)); err == nil {
		t.Fatal("HandleRecordDemo error = nil, want missing capture configuration")
	}
	progress := storedCaptureProgress(t, store, jobID)
	if progress.AttemptID != oldAttemptID || len(progress.CompletedSegmentIDs) != len(segmentIDs) {
		t.Fatalf("capture progress after setup failure = %#v, want unchanged attempt %s", progress, oldAttemptID)
	}
}

func TestRecordWorkerStatusFailureRollsBackNewCaptureProgressAttempt(t *testing.T) {
	store := newFakeStorage()
	base := newFakeRepo()
	jobID := uuid.New()
	plan := minimalKillPlan()
	base.jobs[jobID] = &job.Job{
		ID:       jobID,
		Status:   job.StatusParsed,
		DemoPath: "demos/test.dem",
		KillPlan: &plan,
	}
	if err := store.Put("demos/test.dem", bytes.NewReader([]byte("demo"))); err != nil {
		t.Fatal(err)
	}
	statusErr := errors.New("recording status rejected")
	repo := &failRecordingStatusRepo{fakeRepo: base, err: statusErr}
	worker := NewRecordWorker(repo, store, RecordWorkerConfig{
		WorkDir:      t.TempDir(),
		RecorderPath: "zv-recorder",
		HLAEPath:     "HLAE.exe",
		CS2Path:      "cs2.exe",
	})
	worker.runner = &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("runner should not start after a rejected recording status")
		return nil, nil
	}}

	err := worker.HandleRecordDemo(context.Background(), recordTask(t, jobID))
	if !errors.Is(err, statusErr) {
		t.Fatalf("HandleRecordDemo error = %v, want %v", err, statusErr)
	}
	if repo.calls != 1 {
		t.Fatalf("recording status calls = %d, want 1", repo.calls)
	}
	if exists, existsErr := store.Exists(artifacts.CaptureProgressKey(jobID)); existsErr != nil || exists {
		t.Fatalf("capture progress exists after rejected status: exists=%v err=%v", exists, existsErr)
	}
	if exists, existsErr := store.Exists(artifacts.CaptureSelectionKey(jobID)); existsErr != nil || exists {
		t.Fatalf("capture selection exists after rejected status: exists=%v err=%v", exists, existsErr)
	}
}
