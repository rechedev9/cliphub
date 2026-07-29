package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/fragforge/internal/artifacts"
	"github.com/rechedev9/fragforge/internal/editor"
	"github.com/rechedev9/fragforge/internal/job"
	"github.com/rechedev9/fragforge/internal/killplan"
	"github.com/rechedev9/fragforge/internal/recording"
	"github.com/rechedev9/fragforge/internal/renderplan"
	"github.com/rechedev9/fragforge/internal/rules"
	"github.com/rechedev9/fragforge/internal/storage"
)

type failPutAtStorage struct {
	*fakeStorage
	key    string
	failAt int
	puts   int
	err    error
}

func (s *failPutAtStorage) Put(key string, r io.Reader) error {
	if key == s.key {
		s.puts++
		if s.puts == s.failAt {
			return s.err
		}
	}
	return s.fakeStorage.Put(key, r)
}

func publicationRecordWorker(t *testing.T, repo *fakeRepo, store storage.Storage, clip string) *RecordWorker {
	t.Helper()
	w := NewRecordWorker(repo, store, RecordWorkerConfig{
		WorkDir:      t.TempDir(),
		RecorderPath: "zv-recorder",
		HLAEPath:     "HLAE.exe",
		CS2Path:      "cs2.exe",
	})
	w.runner = &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outDir := argValue(args, "--out")
		scriptPath := filepath.Join(outDir, "recording.js")
		segmentPath := filepath.Join(outDir, "segments", requestedSegmentID(t, args)+".mp4")
		if err := os.MkdirAll(filepath.Dir(segmentPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(scriptPath, []byte("script-"+clip), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(segmentPath, []byte(clip), 0o644); err != nil {
			t.Fatal(err)
		}
		result := recordingResultForRunnerArgs(t, args, scriptPath, segmentPath)
		if err := writeJSONFile(filepath.Join(outDir, "recording-result.json"), result); err != nil {
			t.Fatal(err)
		}
		return []byte("recorded"), nil
	}}
	return w
}

func requestedSegmentID(t *testing.T, args []string) string {
	t.Helper()
	var plan killplan.Plan
	if err := readJSONFile(argValue(args, "--killplan"), &plan); err != nil {
		t.Fatalf("read recording plan: %v", err)
	}
	if len(plan.Segments) != 1 {
		t.Fatalf("recording plan segments = %d, want 1", len(plan.Segments))
	}
	return plan.Segments[0].ID
}

func seededRecordingPublicationJob(t *testing.T) (*fakeRepo, *fakeStorage, uuid.UUID) {
	t.Helper()
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002")
	repo.jobs[id] = &job.Job{
		ID:       id,
		Status:   job.StatusParsed,
		DemoPath: "demos/test.dem",
		Rules:    rules.Default(),
		KillPlan: &plan,
	}
	if err := store.Put("demos/test.dem", bytes.NewReader([]byte("demo"))); err != nil {
		t.Fatal(err)
	}

	w := publicationRecordWorker(t, repo, store, "old-seg-001")
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"})); err != nil {
		t.Fatalf("seed seg-001: %v", err)
	}
	w = publicationRecordWorker(t, repo, store, "old-seg-002")
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-002"})); err != nil {
		t.Fatalf("seed seg-002: %v", err)
	}

	// Changing a selected segment's bounds forces a real replacement at the same
	// durable clip key while leaving seg-002 compatible and reusable.
	repo.jobs[id].KillPlan.Segments[0].TickStart--
	repo.jobs[id].KillPlan.Segments[0].TickEnd--
	return repo, store, id
}

func seededSingleRecordingPublicationJob(t *testing.T) (*fakeRepo, *fakeStorage, uuid.UUID) {
	t.Helper()
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002")
	repo.jobs[id] = &job.Job{
		ID:       id,
		Status:   job.StatusParsed,
		DemoPath: "demos/test.dem",
		Rules:    rules.Default(),
		KillPlan: &plan,
	}
	if err := store.Put("demos/test.dem", bytes.NewReader([]byte("demo"))); err != nil {
		t.Fatal(err)
	}
	w := publicationRecordWorker(t, repo, store, "old-seg-001")
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"})); err != nil {
		t.Fatalf("seed seg-001: %v", err)
	}
	return repo, store, id
}

func expectedRecordingPlan(t *testing.T, plan *killplan.Plan, segmentID string) recording.RecordingPlan {
	t.Helper()
	selected, err := filterKillPlanSegments(plan, []string{segmentID})
	if err != nil {
		t.Fatal(err)
	}
	stream := recording.DefaultStreamConfig()
	stream.HUDMode = recording.HUDModeDeathnotices
	expected, err := recording.NewPlanFromKillPlan(*selected, "profile.dem", "profile", stream)
	if err != nil {
		t.Fatal(err)
	}
	return expected
}

func TestRecordingAttemptRunnerFailurePreservesCompletePreviousCommitWithoutRenderChain(t *testing.T) {
	repo, store, id := seededRecordingPublicationJob(t)
	resultKey := recording.ResultArtifactKey(id)
	scriptKey := recording.ScriptArtifactKey(id)
	clipKey := mustSegmentClipKey(t, id, "seg-001")
	progressKey := artifacts.CaptureProgressKey(id)
	selectionKey := artifacts.CaptureSelectionKey(id)
	beforeResult := bytes.Clone(store.files[resultKey])
	beforeScript := bytes.Clone(store.files[scriptKey])
	beforeClip := bytes.Clone(store.files[clipKey])
	beforeProgress := bytes.Clone(store.files[progressKey])
	beforeSelection := bytes.Clone(store.files[selectionKey])

	runnerErr := errors.New("recorder crashed before producing a result")
	w := NewRecordWorker(repo, store, RecordWorkerConfig{
		WorkDir:      t.TempDir(),
		RecorderPath: "zv-recorder",
		HLAEPath:     "HLAE.exe",
		CS2Path:      "cs2.exe",
	})
	w.runner = &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		return nil, runnerErr
	}}
	intent := renderplan.GenerateIntent{
		Variant:     editor.PresetViral60Clean,
		Edit:        renderplan.DefaultEditRequest(),
		ActiveRunID: uuid.New(),
	}
	putJSON(t, store, artifacts.GenerateIntentKey(id), intent)
	enqueuer := &fakeEnqueuer{}
	w.UseEnqueuer(enqueuer)

	err := w.HandleRecordDemo(context.Background(), generateRecordTask(t, id, intent))
	if !errors.Is(err, runnerErr) {
		t.Fatalf("HandleRecordDemo error = %v, want runner error", err)
	}
	if got := repo.jobs[id]; got.Status != job.StatusRecorded || got.FailureReason != "" {
		t.Fatalf("job after failed recapture = status %s reason %q, want recorded without failure reason", got.Status, got.FailureReason)
	}
	if !bytes.Equal(store.files[resultKey], beforeResult) ||
		!bytes.Equal(store.files[scriptKey], beforeScript) ||
		!bytes.Equal(store.files[clipKey], beforeClip) {
		t.Fatal("failed runner mutated the previous recording commit")
	}
	if !bytes.Equal(store.files[progressKey], beforeProgress) {
		t.Fatal("failed runner did not restore the previous capture progress document")
	}
	if !bytes.Equal(store.files[selectionKey], beforeSelection) {
		t.Fatal("failed runner did not restore the previous capture selection document")
	}
	if len(enqueuer.tasks) != 0 {
		t.Fatalf("failed recapture enqueued %d render task(s), want 0", len(enqueuer.tasks))
	}
	if _, ok := store.files[mustRenderVariantStatusKey(t, id, intent.Variant)]; ok {
		t.Fatal("failed recapture created a render state for stale media")
	}
	var completed renderplan.GenerateIntent
	if err := json.Unmarshal(store.files[artifacts.GenerateIntentKey(id)], &completed); err != nil {
		t.Fatalf("decode completed generate intent: %v", err)
	}
	if completed.ActiveRunID != uuid.Nil {
		t.Fatalf("active generate run = %s, want cleared after failed recapture", completed.ActiveRunID)
	}
}

func TestRecordingAttemptRunnerFailureRestoresAbsentCaptureSelection(t *testing.T) {
	repo, store, id := seededRecordingPublicationJob(t)
	selectionKey := artifacts.CaptureSelectionKey(id)
	progressKey := artifacts.CaptureProgressKey(id)
	if err := store.Delete(selectionKey); err != nil {
		t.Fatal(err)
	}
	beforeProgress := bytes.Clone(store.files[progressKey])
	runnerErr := errors.New("recorder failed with no prior capture selection")
	w := NewRecordWorker(repo, store, RecordWorkerConfig{
		WorkDir:      t.TempDir(),
		RecorderPath: "zv-recorder",
		HLAEPath:     "HLAE.exe",
		CS2Path:      "cs2.exe",
	})
	w.runner = &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		return nil, runnerErr
	}}

	err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"}))
	if !errors.Is(err, runnerErr) {
		t.Fatalf("HandleRecordDemo error = %v, want runner error", err)
	}
	if got := repo.jobs[id]; got.Status != job.StatusRecorded || got.FailureReason != "" {
		t.Fatalf("job after failed recapture = status %s reason %q, want recorded without failure reason", got.Status, got.FailureReason)
	}
	if exists, existsErr := store.Exists(selectionKey); existsErr != nil || exists {
		t.Fatalf("capture selection after rollback: exists=%v err=%v, want absent", exists, existsErr)
	}
	if !bytes.Equal(store.files[progressKey], beforeProgress) {
		t.Fatal("failed runner did not restore progress while deleting the new selection")
	}
}

func TestRecordingAttemptValidationFailurePreservesCompletePreviousCommit(t *testing.T) {
	repo, store, id := seededRecordingPublicationJob(t)
	resultKey := recording.ResultArtifactKey(id)
	scriptKey := recording.ScriptArtifactKey(id)
	clipKey := mustSegmentClipKey(t, id, "seg-001")
	beforeResult := bytes.Clone(store.files[resultKey])
	beforeScript := bytes.Clone(store.files[scriptKey])
	beforeClip := bytes.Clone(store.files[clipKey])

	w := NewRecordWorker(repo, store, RecordWorkerConfig{
		WorkDir:      t.TempDir(),
		RecorderPath: "zv-recorder",
		HLAEPath:     "HLAE.exe",
		CS2Path:      "cs2.exe",
	})
	w.runner = &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outDir := argValue(args, "--out")
		scriptPath := filepath.Join(outDir, "recording.js")
		segmentPath := filepath.Join(outDir, "segments", requestedSegmentID(t, args)+".mp4")
		if err := os.MkdirAll(filepath.Dir(segmentPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(scriptPath, []byte("invalid replacement script"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(segmentPath, []byte("invalid replacement clip"), 0o644); err != nil {
			t.Fatal(err)
		}
		result := recordingResultForRunnerArgs(t, args, scriptPath, segmentPath)
		result.Plan.Segments[0].TickEnd++
		fingerprint, err := recording.CaptureInputFingerprint(result.Plan)
		if err != nil {
			t.Fatal(err)
		}
		result.CaptureInputFingerprint = fingerprint
		if err := writeJSONFile(filepath.Join(outDir, "recording-result.json"), result); err != nil {
			t.Fatal(err)
		}
		return []byte("recorded"), nil
	}}

	err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"}))
	if err == nil || !strings.Contains(err.Error(), "plan does not match the launched attempt") {
		t.Fatalf("HandleRecordDemo error = %v, want attempt-plan validation failure", err)
	}
	if got := repo.jobs[id]; got.Status != job.StatusRecorded || got.FailureReason != "" {
		t.Fatalf("job after invalid recapture = status %s reason %q, want recorded without failure reason", got.Status, got.FailureReason)
	}
	if !bytes.Equal(store.files[resultKey], beforeResult) ||
		!bytes.Equal(store.files[scriptKey], beforeScript) ||
		!bytes.Equal(store.files[clipKey], beforeClip) {
		t.Fatal("invalid recording attempt mutated the previous recording commit")
	}
}

func TestRecordingAttemptFailureDoesNotPreserveIncompletePreviousCommit(t *testing.T) {
	repo, store, id := seededRecordingPublicationJob(t)
	if err := store.Delete(recording.ScriptArtifactKey(id)); err != nil {
		t.Fatal(err)
	}
	runnerErr := errors.New("recorder failed with an incomplete previous commit")
	w := NewRecordWorker(repo, store, RecordWorkerConfig{
		WorkDir:      t.TempDir(),
		RecorderPath: "zv-recorder",
		HLAEPath:     "HLAE.exe",
		CS2Path:      "cs2.exe",
	})
	w.runner = &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		return nil, runnerErr
	}}

	err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"}))
	if !errors.Is(err, runnerErr) {
		t.Fatalf("HandleRecordDemo error = %v, want runner error", err)
	}
	if got := repo.jobs[id]; got.Status != job.StatusFailed || !strings.Contains(got.FailureReason, runnerErr.Error()) {
		t.Fatalf("job with incomplete previous commit = status %s reason %q, want failed runner error", got.Status, got.FailureReason)
	}
}

func TestRecordingPublicationDoesNotMutateOutputsWhenInvalidationFails(t *testing.T) {
	repo, base, id := seededRecordingPublicationJob(t)
	resultKey := recording.ResultArtifactKey(id)
	scriptKey := recording.ScriptArtifactKey(id)
	clipKey := mustSegmentClipKey(t, id, "seg-001")
	beforeResult := bytes.Clone(base.files[resultKey])
	beforeScript := bytes.Clone(base.files[scriptKey])
	beforeClip := bytes.Clone(base.files[clipKey])
	invalidationErr := errors.New("invalidation unavailable")
	store := &failPutAtStorage{
		fakeStorage: base,
		key:         resultKey,
		failAt:      1,
		err:         invalidationErr,
	}
	w := publicationRecordWorker(t, repo, store, "replacement-seg-001")

	err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"}))
	if err == nil || !strings.Contains(err.Error(), invalidationErr.Error()) {
		t.Fatalf("HandleRecordDemo error = %v, want invalidation failure", err)
	}
	if !bytes.Equal(base.files[resultKey], beforeResult) {
		t.Fatal("canonical result mutated after its invalidation write failed")
	}
	if !bytes.Equal(base.files[scriptKey], beforeScript) {
		t.Fatal("recording script mutated after result invalidation failed")
	}
	if !bytes.Equal(base.files[clipKey], beforeClip) {
		t.Fatal("segment clip mutated after result invalidation failed")
	}
	if got := repo.jobs[id].Status; got != job.StatusRecorded {
		t.Fatalf("job status = %s, want recorded while the previous commit remains intact", got)
	}
}

func TestRecordingPublicationLeavesCanonicalResultPendingWhenCommitFails(t *testing.T) {
	repo, base, id := seededRecordingPublicationJob(t)
	resultKey := recording.ResultArtifactKey(id)
	commitErr := errors.New("commit unavailable")
	store := &failPutAtStorage{
		fakeStorage: base,
		key:         resultKey,
		failAt:      2,
		err:         commitErr,
	}
	w := publicationRecordWorker(t, repo, store, "replacement-seg-001")

	err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"}))
	if err == nil || !strings.Contains(err.Error(), commitErr.Error()) {
		t.Fatalf("HandleRecordDemo error = %v, want final commit failure", err)
	}
	if got := string(base.files[mustSegmentClipKey(t, id, "seg-001")]); got != "replacement-seg-001" {
		t.Fatalf("overwritten clip = %q, want replacement bytes", got)
	}
	pending := storedRecordingResult(t, base, id)
	if !pending.PublicationPending {
		t.Fatal("canonical recording result is ready after clip overwrite and failed commit")
	}
	expected := expectedRecordingPlan(t, repo.jobs[id].KillPlan, "seg-001")
	ready, _, readyErr := recordingOutputsReady(base, id, []string{"seg-001"}, expected)
	if readyErr != nil {
		t.Fatal(readyErr)
	}
	if ready {
		t.Fatal("pending canonical recording result was reported ready")
	}
	if _, readErr := readStoredRecordingResult(base, id); readErr == nil {
		t.Fatal("pending canonical recording result was accepted by a downstream reader")
	}
	if got := repo.jobs[id].Status; got != job.StatusFailed {
		t.Fatalf("job status = %s, want failed after mutating a referenced clip", got)
	}
}

func TestRecordingPublicationRestoresPreviousCommitWhenArtifactUploadFailsBeforeClipMutation(t *testing.T) {
	repo, base, id := seededRecordingPublicationJob(t)
	resultKey := recording.ResultArtifactKey(id)
	scriptKey := recording.ScriptArtifactKey(id)
	beforeResult := bytes.Clone(base.files[resultKey])
	beforeScript := bytes.Clone(base.files[scriptKey])
	uploadErr := errors.New("script upload unavailable")
	store := &failPutAtStorage{
		fakeStorage: base,
		key:         scriptKey,
		failAt:      1,
		err:         uploadErr,
	}
	w := publicationRecordWorker(t, repo, store, "replacement-seg-001")

	err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"}))
	if err == nil || !strings.Contains(err.Error(), uploadErr.Error()) {
		t.Fatalf("HandleRecordDemo error = %v, want script upload failure", err)
	}
	if !bytes.Equal(base.files[resultKey], beforeResult) {
		t.Fatal("failed pre-clip upload did not restore the previous recording result")
	}
	if !bytes.Equal(base.files[scriptKey], beforeScript) {
		t.Fatal("failed pre-clip upload did not preserve the previous recording script")
	}
	if err := recording.ValidateUploadResult(storedRecordingResult(t, base, id)); err != nil {
		t.Fatalf("restored previous commit validation: %v", err)
	}
	if got := repo.jobs[id].Status; got != job.StatusRecorded {
		t.Fatalf("job status = %s, want recorded after restoring the previous commit", got)
	}
}

func TestRecordingPublicationRestoresPreviousCommitWhenOnlyNewClipKeysChanged(t *testing.T) {
	repo, base, id := seededSingleRecordingPublicationJob(t)
	resultKey := recording.ResultArtifactKey(id)
	scriptKey := recording.ScriptArtifactKey(id)
	beforeResult := bytes.Clone(base.files[resultKey])
	beforeScript := bytes.Clone(base.files[scriptKey])
	commitErr := errors.New("commit unavailable")
	store := &failPutAtStorage{
		fakeStorage: base,
		key:         resultKey,
		failAt:      2,
		err:         commitErr,
	}
	w := publicationRecordWorker(t, repo, store, "new-seg-002")

	err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-002"}))
	if err == nil || !strings.Contains(err.Error(), commitErr.Error()) {
		t.Fatalf("HandleRecordDemo error = %v, want final commit failure", err)
	}
	if !bytes.Equal(base.files[resultKey], beforeResult) {
		t.Fatal("failed new-segment commit did not restore the previous recording result")
	}
	if !bytes.Equal(base.files[scriptKey], beforeScript) {
		t.Fatal("failed new-segment commit did not restore the previous recording script")
	}
	restored := storedRecordingResult(t, base, id)
	if err := recording.ValidateUploadResult(restored); err != nil {
		t.Fatalf("restored previous commit validation: %v", err)
	}
	expected := expectedRecordingPlan(t, repo.jobs[id].KillPlan, "seg-001")
	ready, _, readyErr := recordingOutputsReady(base, id, []string{"seg-001"}, expected)
	if readyErr != nil {
		t.Fatal(readyErr)
	}
	if !ready {
		t.Fatal("previous segment was not reusable after a failed new-segment commit")
	}
	if got := string(base.files[mustSegmentClipKey(t, id, "seg-002")]); got != "new-seg-002" {
		t.Fatalf("orphaned new clip = %q, want harmless unreferenced new bytes", got)
	}
	if got := repo.jobs[id].Status; got != job.StatusRecorded {
		t.Fatalf("job status = %s, want recorded after restoring the previous commit", got)
	}
}

func TestRecordingPublicationRetryCommitsAndPreservesCompatibleSegments(t *testing.T) {
	repo, base, id := seededRecordingPublicationJob(t)
	resultKey := recording.ResultArtifactKey(id)
	store := &failPutAtStorage{
		fakeStorage: base,
		key:         resultKey,
		failAt:      2,
		err:         errors.New("commit unavailable"),
	}
	task := recordTaskFor(t, id, []string{"seg-001"})
	w := publicationRecordWorker(t, repo, store, "replacement-seg-001")
	if err := w.HandleRecordDemo(context.Background(), task); err == nil {
		t.Fatal("first publication unexpectedly committed")
	}

	store.failAt = 0
	if err := w.HandleRecordDemo(context.Background(), task); err != nil {
		t.Fatalf("retry publication: %v", err)
	}
	committed := storedRecordingResult(t, base, id)
	if committed.PublicationPending {
		t.Fatal("successful retry left publication pending")
	}
	if err := recording.ValidateUploadResult(committed); err != nil {
		t.Fatalf("committed result validation: %v", err)
	}
	if got := recording.SegmentIDs(committed); len(got) != 2 || got[0] != "seg-001" || got[1] != "seg-002" {
		t.Fatalf("committed segments = %v, want [seg-001 seg-002]", got)
	}
	if got := string(base.files[mustSegmentClipKey(t, id, "seg-001")]); got != "replacement-seg-001" {
		t.Fatalf("seg-001 clip = %q, want replacement bytes", got)
	}
	if got := string(base.files[mustSegmentClipKey(t, id, "seg-002")]); got != "old-seg-002" {
		t.Fatalf("unrelated seg-002 clip = %q, want preserved bytes", got)
	}
	for _, segmentID := range []string{"seg-001", "seg-002"} {
		expected := expectedRecordingPlan(t, repo.jobs[id].KillPlan, segmentID)
		ready, _, err := recordingOutputsReady(base, id, []string{segmentID}, expected)
		if err != nil {
			t.Fatalf("recordingOutputsReady(%s): %v", segmentID, err)
		}
		if !ready {
			t.Fatalf("recordingOutputsReady(%s) = false after successful retry", segmentID)
		}
	}
}
