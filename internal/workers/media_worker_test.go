package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/tickcut/internal/artifacts"
	"github.com/rechedev9/tickcut/internal/composition"
	"github.com/rechedev9/tickcut/internal/editor"
	"github.com/rechedev9/tickcut/internal/job"
	"github.com/rechedev9/tickcut/internal/killplan"
	"github.com/rechedev9/tickcut/internal/recording"
	"github.com/rechedev9/tickcut/internal/renderplan"
	"github.com/rechedev9/tickcut/internal/rules"
	"github.com/rechedev9/tickcut/internal/tasks"
)

type runnerCall struct {
	exe  string
	args []string
}

type fakeRunner struct {
	mu               sync.Mutex
	calls            []runnerCall
	fn               func(context.Context, string, ...string) ([]byte, error)
	recordCoverCalls bool
}

func (f *fakeRunner) Run(ctx context.Context, exe string, args ...string) ([]byte, error) {
	if !f.recordCoverCalls && argValue(args, "-vf") == "scale=720:-2" {
		return nil, os.WriteFile(args[len(args)-1], []byte("jpeg"), 0o600)
	}
	// The render worker probes shorts concurrently, so guard the call log.
	f.mu.Lock()
	f.calls = append(f.calls, runnerCall{exe: exe, args: append([]string(nil), args...)})
	f.mu.Unlock()
	if f.fn == nil {
		return nil, nil
	}
	return f.fn(ctx, exe, args...)
}

func TestRecordWorkerStoresOutputsAndMarksRecorded(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{
		ID:       id,
		Status:   job.StatusParsed,
		DemoPath: "demos/test.dem",
		Rules:    rules.Default(),
		KillPlan: &plan,
	}
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))

	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outDir := argValue(args, "--out")
		if outDir == "" {
			t.Fatal("runner args missing --out")
		}
		scriptPath := filepath.Join(outDir, "recording.js")
		segmentPath := filepath.Join(outDir, "segments", "seg-001.mp4")
		if err := os.MkdirAll(filepath.Dir(segmentPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(scriptPath, []byte("script"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(segmentPath, []byte("clip"), 0o644); err != nil {
			t.Fatal(err)
		}
		result := recordingResultForRunnerArgs(t, args, scriptPath, segmentPath)
		if err := writeJSONFile(filepath.Join(outDir, "recording-result.json"), result); err != nil {
			t.Fatal(err)
		}
		return []byte("recorded"), nil
	}}
	w := NewRecordWorker(repo, store, RecordWorkerConfig{
		WorkDir:      t.TempDir(),
		RecorderPath: "zv-recorder",
		HLAEPath:     "HLAE.exe",
		CS2Path:      "cs2.exe",
	})
	w.runner = runner

	task := recordTask(t, id)
	if err := w.HandleRecordDemo(context.Background(), task); err != nil {
		t.Fatalf("HandleRecordDemo error = %v", err)
	}

	if repo.jobs[id].Status != job.StatusRecorded {
		t.Fatalf("Status = %s, want recorded", repo.jobs[id].Status)
	}
	for _, key := range []string{
		recording.ResultArtifactKey(id),
		recording.ScriptArtifactKey(id),
		mustSegmentClipKey(t, id, "seg-001"),
	} {
		if _, ok := store.files[key]; !ok {
			t.Fatalf("storage missing %s", key)
		}
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %d, want 1", len(runner.calls))
	}
	if got := argValue(runner.calls[0].args, "--timeout"); got != defaultMediaWorkerTimeout {
		t.Fatalf("--timeout = %q, want %q", got, defaultMediaWorkerTimeout)
	}
}

func TestRecordWorkerHUDFromPayloadOverridesDefault(t *testing.T) {
	cases := []struct {
		name                 string
		hud                  string
		portraitSafeKillfeed bool
		wantHUD              string
		wantPortraitFlag     bool
	}{
		{name: "preset clean overrides default", hud: "clean", wantHUD: "clean"},
		{name: "empty payload keeps worker default", hud: "", wantHUD: "deathnotices"},
		{name: "vertical killfeed configures portrait safe capture", hud: "deathnotices", portraitSafeKillfeed: true, wantHUD: "deathnotices", wantPortraitFlag: true},
		{name: "vertical full HUD configures portrait safe capture", hud: "gameplay", portraitSafeKillfeed: true, wantHUD: "gameplay", wantPortraitFlag: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			id := uuid.New()
			plan := minimalKillPlan()
			repo.jobs[id] = &job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/test.dem", Rules: rules.Default(), KillPlan: &plan}
			_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))

			runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
				outDir := argValue(args, "--out")
				scriptPath := filepath.Join(outDir, "recording.js")
				segmentPath := filepath.Join(outDir, "segments", "seg-001.mp4")
				_ = os.MkdirAll(filepath.Dir(segmentPath), 0o755)
				_ = os.WriteFile(scriptPath, []byte("script"), 0o644)
				_ = os.WriteFile(segmentPath, []byte("clip"), 0o644)
				_ = writeJSONFile(filepath.Join(outDir, "recording-result.json"), recordingResultForRunnerArgs(t, args, scriptPath, segmentPath))
				return []byte("recorded"), nil
			}}
			// Worker default HUD is "deathnotices" (withDefaults); the payload may override it.
			w := NewRecordWorker(repo, store, RecordWorkerConfig{WorkDir: t.TempDir(), RecorderPath: "zv-recorder", HLAEPath: "HLAE.exe", CS2Path: "cs2.exe"})
			w.runner = runner

			task, err := tasks.NewRecordDemoTask(id, tc.hud, nil, tc.portraitSafeKillfeed)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.HandleRecordDemo(context.Background(), task); err != nil {
				t.Fatalf("HandleRecordDemo error = %v", err)
			}
			if got := argValue(runner.calls[0].args, "--hud"); got != tc.wantHUD {
				t.Fatalf("--hud = %q, want %q", got, tc.wantHUD)
			}
			if got := hasArg(runner.calls[0].args, "--portrait-safe-killfeed"); got != tc.wantPortraitFlag {
				t.Fatalf("--portrait-safe-killfeed present = %v, want %v", got, tc.wantPortraitFlag)
			}
		})
	}
}

func TestRecordWorkerFailsWithoutKillPlan(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	repo.jobs[id] = &job.Job{
		ID:       id,
		Status:   job.StatusParsed,
		DemoPath: "demos/test.dem",
		Rules:    rules.Default(),
	}

	w := NewRecordWorker(repo, store, RecordWorkerConfig{
		WorkDir:      t.TempDir(),
		RecorderPath: "zv-recorder",
		HLAEPath:     "HLAE.exe",
		CS2Path:      "cs2.exe",
	})
	err := w.HandleRecordDemo(context.Background(), recordTask(t, id))
	if err == nil || !strings.Contains(err.Error(), "no kill plan") {
		t.Fatalf("HandleRecordDemo error = %v, want no kill plan", err)
	}
	if repo.jobs[id].Status != job.StatusFailed {
		t.Fatalf("Status = %s, want failed", repo.jobs[id].Status)
	}
}

func TestRecordWorkerSkipsWhenOutputsAlreadyExist(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{
		ID:       id,
		Status:   job.StatusParsed,
		DemoPath: "demos/test.dem",
		Rules:    rules.Default(),
		KillPlan: &plan,
	}
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "stale-local.mp4"))
	_ = store.Put(recording.ScriptArtifactKey(id), bytes.NewReader([]byte("script")))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))

	runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("runner should not be called when recording outputs already exist")
		return nil, nil
	}}
	w := NewRecordWorker(repo, store, RecordWorkerConfig{})
	w.runner = runner

	if err := w.HandleRecordDemo(context.Background(), recordTask(t, id)); err != nil {
		t.Fatalf("HandleRecordDemo error = %v", err)
	}
	if repo.jobs[id].Status != job.StatusRecorded {
		t.Fatalf("Status = %s, want recorded", repo.jobs[id].Status)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %d, want 0", len(runner.calls))
	}
}

func TestPrepareStageDirCleansTempWorkDirWhenRootEmpty(t *testing.T) {
	dir, cleanup, err := prepareStageDir("", uuid.New(), "record")
	if err != nil {
		t.Fatalf("prepareStageDir error = %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("work dir not created: %v", err)
	}

	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("work dir still exists after cleanup, err=%v", err)
	}
}

func TestPrepareStageDirKeepsExplicitWorkDir(t *testing.T) {
	root := t.TempDir()
	dir, cleanup, err := prepareStageDir(root, uuid.New(), "record")
	if err != nil {
		t.Fatalf("prepareStageDir error = %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("work dir not created: %v", err)
	}

	cleanup()
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("explicit work dir removed, err=%v", err)
	}
}

func TestComposeWorkerLocalizesSegmentsAndStoresFinal(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default()}
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))

	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		recordingResultPath := argValue(args, "--recording-result")
		outPath := argValue(args, "--out")
		var result recording.RecordingResult
		if err := readJSONFile(recordingResultPath, &result); err != nil {
			t.Fatal(err)
		}
		gotPath := result.Artifacts[0].Path
		if strings.Contains(gotPath, "stale") {
			t.Fatalf("segment path was not localized: %s", gotPath)
		}
		b, err := os.ReadFile(gotPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "clip" {
			t.Fatalf("localized segment = %q, want clip", b)
		}
		if err := os.WriteFile(outPath, []byte("final"), 0o644); err != nil {
			t.Fatal(err)
		}
		composed := composition.Result{
			RecordingResult: recordingResultPath,
			Output:          outPath,
			OutputArtifact: recording.RecordingArtifact{
				Role:      "final",
				Type:      "video",
				Path:      outPath,
				SizeBytes: 5,
			},
		}
		if err := writeJSONFile(filepath.Join(filepath.Dir(outPath), "composition-result.json"), composed); err != nil {
			t.Fatal(err)
		}
		return []byte("composed"), nil
	}}
	w := NewComposeWorker(repo, store, ComposeWorkerConfig{
		WorkDir:      t.TempDir(),
		ComposerPath: "zv-composer",
	})
	w.runner = runner

	if err := w.HandleComposeFinal(context.Background(), composeTask(t, id)); err != nil {
		t.Fatalf("HandleComposeFinal error = %v", err)
	}
	if repo.jobs[id].Status != job.StatusComposed {
		t.Fatalf("Status = %s, want composed", repo.jobs[id].Status)
	}
	for _, key := range []string{composition.ResultArtifactKey(id), composition.FinalArtifactKey(id)} {
		if _, ok := store.files[key]; !ok {
			t.Fatalf("storage missing %s", key)
		}
	}
}

func TestComposeWorkerMarksFailedOnResultError(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default()}
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))

	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outPath := argValue(args, "--out")
		result := composition.Result{Output: outPath, Error: "bad compose"}
		if err := writeJSONFile(filepath.Join(filepath.Dir(outPath), "composition-result.json"), result); err != nil {
			t.Fatal(err)
		}
		return []byte("bad"), nil
	}}
	w := NewComposeWorker(repo, store, ComposeWorkerConfig{
		WorkDir:      t.TempDir(),
		ComposerPath: "zv-composer",
	})
	w.runner = runner

	err := w.HandleComposeFinal(context.Background(), composeTask(t, id))
	if err == nil || !strings.Contains(err.Error(), "bad compose") {
		t.Fatalf("HandleComposeFinal error = %v, want bad compose", err)
	}
	if repo.jobs[id].Status != job.StatusFailed {
		t.Fatalf("Status = %s, want failed", repo.jobs[id].Status)
	}
	if _, ok := store.files[composition.ResultArtifactKey(id)]; !ok {
		t.Fatalf("storage missing failed composition result")
	}
}

func TestComposeWorkerPersistsStructuredResultWhenRunnerAlsoFails(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default()}
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))

	runErr := errors.New("composer process failed")
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outPath := argValue(args, "--out")
		result := composition.Result{Output: outPath, Error: "ffmpeg failed at frame 42"}
		if err := writeJSONFile(filepath.Join(filepath.Dir(outPath), "composition-result.json"), result); err != nil {
			t.Fatal(err)
		}
		return nil, runErr
	}}
	w := NewComposeWorker(repo, store, ComposeWorkerConfig{
		WorkDir:      t.TempDir(),
		ComposerPath: "zv-composer",
	})
	w.runner = runner

	err := w.HandleComposeFinal(context.Background(), composeTask(t, id))
	if !errors.Is(err, runErr) {
		t.Fatalf("HandleComposeFinal error = %v, want runner error", err)
	}
	var stored composition.Result
	if err := json.Unmarshal(store.files[composition.ResultArtifactKey(id)], &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Error != "ffmpeg failed at frame 42" {
		t.Fatalf("stored diagnostic error = %q", stored.Error)
	}
	ready, _, _, err := compositionOutputsReady(store, id)
	if err != nil {
		t.Fatal(err)
	}
	if ready {
		t.Fatal("failed diagnostic must not be a reusable composition result")
	}
}

func TestComposeWorkerSerializesSameJobComposition(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default()}
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))

	started := make(chan struct{})
	release := make(chan struct{})
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		close(started)
		<-release
		outPath := argValue(args, "--out")
		if err := os.WriteFile(outPath, []byte("final"), 0o600); err != nil {
			return nil, err
		}
		result := composition.Result{
			Output: outPath,
			OutputArtifact: recording.RecordingArtifact{
				Role:      "final",
				Type:      "video",
				Path:      outPath,
				SizeBytes: 5,
			},
		}
		return nil, writeJSONFile(filepath.Join(filepath.Dir(outPath), "composition-result.json"), result)
	}}
	w := NewComposeWorker(repo, store, ComposeWorkerConfig{
		WorkDir:      t.TempDir(),
		ComposerPath: "zv-composer",
	})
	w.runner = runner

	errs := make(chan error, 2)
	go func() { errs <- w.HandleComposeFinal(context.Background(), composeTask(t, id)) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first composition did not start")
	}
	go func() { errs <- w.HandleComposeFinal(context.Background(), composeTask(t, id)) }()

	time.Sleep(25 * time.Millisecond)
	runner.mu.Lock()
	callsWhileBlocked := len(runner.calls)
	runner.mu.Unlock()
	if callsWhileBlocked != 1 {
		t.Fatalf("composer calls while first run blocked = %d, want 1", callsWhileBlocked)
	}
	close(release)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("HandleComposeFinal error = %v", err)
		}
	}
	runner.mu.Lock()
	totalCalls := len(runner.calls)
	runner.mu.Unlock()
	if totalCalls != 1 {
		t.Fatalf("composer calls = %d, want one committed run plus one cache skip", totalCalls)
	}
}

func TestComposeWorkerJoinsRunnerAndResultReadErrors(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default()}
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))

	runErr := errors.New("composer exited")
	runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		return nil, runErr
	}}
	w := NewComposeWorker(repo, store, ComposeWorkerConfig{
		WorkDir:      t.TempDir(),
		ComposerPath: "zv-composer",
	})
	w.runner = runner

	err := w.HandleComposeFinal(context.Background(), composeTask(t, id))
	if !errors.Is(err, runErr) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("HandleComposeFinal error = %v, want runner and missing-result errors", err)
	}
	if !strings.Contains(err.Error(), "read composition result") {
		t.Fatalf("HandleComposeFinal error = %v, want causal read context", err)
	}
}

func TestComposeWorkerJoinsRunnerAndResultValidationErrors(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default()}
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))

	runErr := errors.New("composer exited")
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outPath := argValue(args, "--out")
		if err := writeJSONFile(filepath.Join(filepath.Dir(outPath), "composition-result.json"), composition.Result{}); err != nil {
			return nil, err
		}
		return nil, runErr
	}}
	w := NewComposeWorker(repo, store, ComposeWorkerConfig{
		WorkDir:      t.TempDir(),
		ComposerPath: "zv-composer",
	})
	w.runner = runner

	err := w.HandleComposeFinal(context.Background(), composeTask(t, id))
	if !errors.Is(err, runErr) {
		t.Fatalf("HandleComposeFinal error = %v, want runner error", err)
	}
	if !strings.Contains(err.Error(), "composition result has no output") {
		t.Fatalf("HandleComposeFinal error = %v, want validation cause", err)
	}
}

type failSuccessfulCompositionResultStorage struct {
	*fakeStorage
	failNextSuccessfulResult bool
}

func (s *failSuccessfulCompositionResultStorage) Put(key string, r io.Reader) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if key == composition.ResultArtifactKey(uuid.Nil) {
		panic("test storage requires a concrete job id")
	}
	if strings.HasSuffix(key, "/composition-result.json") && s.failNextSuccessfulResult {
		var result composition.Result
		if json.Unmarshal(body, &result) == nil && result.Error == "" {
			s.failNextSuccessfulResult = false
			return errors.New("result commit failed")
		}
	}
	return s.fakeStorage.Put(key, bytes.NewReader(body))
}

func TestComposeWorkerInvalidatesOldCommitMarkerBeforeRepair(t *testing.T) {
	repo := newFakeRepo()
	baseStore := newFakeStorage()
	store := &failSuccessfulCompositionResultStorage{
		fakeStorage:              baseStore,
		failNextSuccessfulResult: true,
	}
	id := uuid.New()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default()}
	putJSON(t, baseStore, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	_ = baseStore.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))
	putJSON(t, baseStore, composition.ResultArtifactKey(id), composition.Result{Output: "old-final.mp4"})

	runs := 0
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		runs++
		outPath := argValue(args, "--out")
		if err := os.WriteFile(outPath, []byte(fmt.Sprintf("final-%d", runs)), 0o644); err != nil {
			t.Fatal(err)
		}
		result := composition.Result{
			Output: outPath,
			OutputArtifact: recording.RecordingArtifact{
				Role:      "final",
				Type:      "video",
				Path:      outPath,
				SizeBytes: int64(len(fmt.Sprintf("final-%d", runs))),
			},
		}
		if err := writeJSONFile(filepath.Join(filepath.Dir(outPath), "composition-result.json"), result); err != nil {
			t.Fatal(err)
		}
		return nil, nil
	}}
	w := NewComposeWorker(repo, store, ComposeWorkerConfig{
		WorkDir:      t.TempDir(),
		ComposerPath: "zv-composer",
	})
	w.runner = runner

	if err := w.HandleComposeFinal(context.Background(), composeTask(t, id)); err == nil ||
		!strings.Contains(err.Error(), "result commit failed") {
		t.Fatalf("first HandleComposeFinal error = %v, want result commit failure", err)
	}
	var pending composition.Result
	if err := json.Unmarshal(baseStore.files[composition.ResultArtifactKey(id)], &pending); err != nil {
		t.Fatal(err)
	}
	if pending.Error == "" {
		t.Fatal("failed result commit left a reusable successful marker")
	}
	ready, _, _, err := compositionOutputsReady(store, id)
	if err != nil {
		t.Fatal(err)
	}
	if ready {
		t.Fatal("mixed old result and new final must not be reusable")
	}

	if err := w.HandleComposeFinal(context.Background(), composeTask(t, id)); err != nil {
		t.Fatalf("retry HandleComposeFinal error = %v", err)
	}
	if runs != 2 {
		t.Fatalf("composer runs = %d, want 2", runs)
	}
	ready, _, _, err = compositionOutputsReady(store, id)
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Fatal("successful retry did not commit the repaired composition pair")
	}
}

func TestCompositionCompletionStatusRequiresReviewForWarnings(t *testing.T) {
	if got := compositionCompletionStatus(false); got != job.StatusComposed {
		t.Fatalf("clean composition status = %s, want composed", got)
	}
	if got := compositionCompletionStatus(true); got != job.StatusReviewRequired {
		t.Fatalf("warning composition status = %s, want review_required", got)
	}
}

func TestComposeWorkerSkipsWhenOutputsAlreadyExist(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default()}
	putJSON(t, store, composition.ResultArtifactKey(id), composition.Result{Output: "final.mp4"})
	_ = store.Put(composition.FinalArtifactKey(id), bytes.NewReader([]byte("final")))

	runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("runner should not be called when composition outputs already exist")
		return nil, nil
	}}
	w := NewComposeWorker(repo, store, ComposeWorkerConfig{})
	w.runner = runner

	if err := w.HandleComposeFinal(context.Background(), composeTask(t, id)); err != nil {
		t.Fatalf("HandleComposeFinal error = %v", err)
	}
	if repo.jobs[id].Status != job.StatusComposed {
		t.Fatalf("Status = %s, want composed", repo.jobs[id].Status)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %d, want 0", len(runner.calls))
	}
}

func TestRenderWorkerLocalizesSegmentsAndStoresVariantOutputs(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))

	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		recordingResultPath := argValue(args, "--recording-result")
		outDir := argValue(args, "--out")
		publishDir := argValue(args, "--publish-dir")
		if got := argValue(args, "--preset"); got != editor.PresetViral60Clean {
			t.Fatalf("--preset = %q, want %q", got, editor.PresetViral60Clean)
		}
		for _, check := range []struct {
			key  string
			want string
		}{
			{"--output-format", renderplan.FormatLandscape16x9},
			{"--kill-effect", renderplan.KillEffectFreezeFlash},
			{"--transition", renderplan.TransitionDip},
		} {
			if got := argValue(args, check.key); got != check.want {
				t.Fatalf("%s = %q, want %q", check.key, got, check.want)
			}
		}
		if !hasArg(args, "--hook=true") || !hasArg(args, "--kill-counter=false") {
			t.Fatalf("editor args missing explicit automatic text values: %#v", args)
		}
		if !hasArg(args, "--intro=true") || !hasArg(args, "--outro=true") {
			t.Fatalf("editor args missing intro/outro flags: %#v", args)
		}
		if got := argValue(args, "--intro-text"); got != "Watch this ace" {
			t.Fatalf("--intro-text = %q, want %q", got, "Watch this ace")
		}
		if got := argValue(args, "--outro-text"); got != "follow for more" {
			t.Fatalf("--outro-text = %q, want %q", got, "follow for more")
		}
		var result recording.RecordingResult
		if err := readJSONFile(recordingResultPath, &result); err != nil {
			t.Fatal(err)
		}
		gotPath := result.Artifacts[0].Path
		if strings.Contains(gotPath, "stale") {
			t.Fatalf("segment path was not localized: %s", gotPath)
		}
		b, err := os.ReadFile(gotPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "clip" {
			t.Fatalf("localized segment = %q, want clip", b)
		}

		videoPath := filepath.Join(publishDir, "seg-001.mp4")
		coverPath := filepath.Join(publishDir, "seg-001.cover.jpg")
		captionPath := filepath.Join(publishDir, "seg-001.caption.txt")
		logPath := filepath.Join(outDir, "logs", "seg-001-render.log")
		for _, file := range []struct {
			path string
			body string
		}{
			{filepath.Join(outDir, "edit-manifest.json"), `{"shorts":[{"segment_id":"seg-001"}]}`},
			{filepath.Join(publishDir, "pack-manifest.json"), `{"items":[{"segment_id":"seg-001"}]}`},
			{filepath.Join(publishDir, "index.html"), `<html></html>`},
			{filepath.Join(publishDir, "publish-summary.md"), `summary`},
			{videoPath, "video"},
			{coverPath, "cover"},
			{captionPath, "caption"},
			{logPath, "log"},
		} {
			if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file.path, []byte(file.body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		rendered := editor.Result{
			Preset:      editor.PresetViral60Clean,
			OutputDir:   outDir,
			PublishDir:  publishDir,
			GalleryPath: filepath.Join(publishDir, "index.html"),
			SummaryPath: filepath.Join(publishDir, "publish-summary.md"),
			Shorts: []editor.ShortResult{{
				SegmentID:     "seg-001",
				Output:        videoPath,
				PublishPath:   videoPath,
				CoverPath:     coverPath,
				CaptionPath:   captionPath,
				RenderLogPath: logPath,
			}},
		}
		if err := writeJSONFile(filepath.Join(outDir, "shorts-result.json"), rendered); err != nil {
			t.Fatal(err)
		}
		return []byte("rendered"), nil
	}}
	w := NewRenderWorker(repo, store, RenderWorkerConfig{
		WorkDir:    t.TempDir(),
		EditorPath: "zv-editor",
		FFmpegPath: "ffmpeg",
	})
	w.runner = runner

	task, err := tasks.NewRenderVariantTask(id, editor.PresetViral60Clean, "", 0, renderplan.EditRequest{
		Format:      renderplan.FormatLandscape16x9,
		KillEffect:  renderplan.KillEffectFreezeFlash,
		Transition:  renderplan.TransitionDip,
		Intro:       true,
		Outro:       true,
		IntroText:   "Watch this ace",
		OutroText:   "follow for more",
		HookText:    true,
		KillCounter: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.HandleRenderVariant(context.Background(), task); err != nil {
		t.Fatalf("HandleRenderVariant error = %v", err)
	}
	if repo.jobs[id].Status != job.StatusRecorded {
		t.Fatalf("Status = %s, want unchanged recorded", repo.jobs[id].Status)
	}
	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean)], &state); err != nil {
		t.Fatal(err)
	}
	if got, want := state.Status, renderplan.RenderVariantStatusReady; got != want {
		t.Fatalf("render state = %q, want %q", got, want)
	}
	videoRef, err := renderplan.NewRenderVariantArtifactRefForState(state, renderplan.RenderVariantArtifactVideo, "seg-001")
	if err != nil {
		t.Fatal(err)
	}
	coverRef, err := renderplan.NewRenderVariantArtifactRefForState(state, renderplan.RenderVariantArtifactCover, "seg-001")
	if err != nil {
		t.Fatal(err)
	}
	captionRef, err := renderplan.NewRenderVariantArtifactRefForState(state, renderplan.RenderVariantArtifactCaption, "seg-001")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean),
		state.RenderResultKey,
		state.EditDocumentKey,
		state.EditManifestKey,
		state.PackManifestKey,
		state.PublishSummaryKey,
		state.GalleryKey,
		videoRef.Key,
		coverRef.Key,
		captionRef.Key,
		state.ArtifactPrefix + "/logs/seg-001-render.log",
	} {
		if _, ok := store.files[key]; !ok {
			t.Fatalf("storage missing %s", key)
		}
	}
	revisionID := state.ArtifactPrefix[strings.LastIndex(state.ArtifactPrefix, "/")+1:]
	revisionBase := fmt.Sprintf(
		"/api/jobs/%s/renders/%s/revisions/%s",
		id,
		editor.PresetViral60Clean,
		revisionID,
	)
	gallery := string(store.files[state.GalleryKey])
	for _, want := range []string{
		revisionBase + "/videos/seg-001",
		revisionBase + "/covers/seg-001",
		revisionBase + "/captions/seg-001",
	} {
		if !strings.Contains(gallery, want) {
			t.Fatalf("durable gallery missing revision-scoped URL %q: %s", want, gallery)
		}
	}
	mutableVideoURL := fmt.Sprintf(
		`src="/api/jobs/%s/renders/%s/videos/seg-001"`,
		id,
		editor.PresetViral60Clean,
	)
	if strings.Contains(gallery, mutableVideoURL) {
		t.Fatalf("durable gallery retained mutable video URL %q: %s", mutableVideoURL, gallery)
	}
	var doc renderplan.EditDocument
	if err := json.Unmarshal(store.files[state.EditDocumentKey], &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Edit.Format != renderplan.FormatLandscape16x9 || doc.LoadoutSnapshot.Output.AspectRatio != "16:9" || doc.LoadoutSnapshot.Output.Width != 1920 || doc.LoadoutSnapshot.Output.Height != 1080 {
		t.Fatalf("edit document = %#v", doc)
	}
	if !doc.Edit.HookText || doc.Edit.KillCounter {
		t.Fatalf("edit document automatic text = hook %v / counter %v, want true / false", doc.Edit.HookText, doc.Edit.KillCounter)
	}
	wantDocumentOutputs := renderplan.Outputs{
		Prefix:          state.ArtifactPrefix,
		RenderResult:    state.RenderResultKey,
		EditManifest:    state.EditManifestKey,
		PackManifest:    state.PackManifestKey,
		Gallery:         state.GalleryKey,
		PublishSummary:  state.PublishSummaryKey,
		UploadReadyRoot: state.ArtifactPrefix,
	}
	if doc.Outputs != wantDocumentOutputs {
		t.Fatalf("edit document outputs = %#v, want revision-scoped %#v", doc.Outputs, wantDocumentOutputs)
	}
	var storedResult editor.Result
	if err := json.Unmarshal(store.files[state.RenderResultKey], &storedResult); err != nil {
		t.Fatal(err)
	}
	if storedResult.InputFingerprint == "" {
		t.Fatal("stored render result is missing input fingerprint")
	}
}

func TestWriteDurableRenderDocumentsRejectsMalformedOrMismatchedManifestSegments(t *testing.T) {
	tests := []struct {
		name    string
		editIDs []string
		packIDs []string
		want    string
	}{
		{
			name:    "unsafe edit manifest id",
			editIDs: []string{"../escape"},
			packIDs: []string{"seg-001"},
			want:    "edit manifest entry 0",
		},
		{
			name:    "duplicate pack manifest id",
			editIDs: []string{"seg-001"},
			packIDs: []string{"seg-001", "seg-001"},
			want:    `pack manifest contains duplicate segment id "seg-001"`,
		},
		{
			name:    "pack manifest differs from result",
			editIDs: []string{"seg-001"},
			packIDs: []string{"seg-002"},
			want:    `pack manifest is missing render result segment id "seg-001"`,
		},
		{
			name:    "edit manifest omits result",
			editIDs: nil,
			packIDs: []string{"seg-001"},
			want:    "edit manifest contains 0 segment ids, want 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			outDir := filepath.Join(root, "out")
			publishDir := filepath.Join(outDir, "shortslistosparasubir")
			resultPath := filepath.Join(outDir, "shorts-result.json")
			if err := writeJSONFile(filepath.Join(outDir, "edit-document.json"), renderplan.EditDocument{}); err != nil {
				t.Fatal(err)
			}
			manifest := editor.Manifest{Shorts: make([]editor.ShortEdit, len(tt.editIDs))}
			for i, segmentID := range tt.editIDs {
				manifest.Shorts[i].SegmentID = segmentID
			}
			if err := writeJSONFile(filepath.Join(outDir, "edit-manifest.json"), manifest); err != nil {
				t.Fatal(err)
			}
			pack := editor.PackManifest{Items: make([]editor.PublishItem, len(tt.packIDs))}
			for i, segmentID := range tt.packIDs {
				pack.Items[i].SegmentID = segmentID
			}
			if err := writeJSONFile(filepath.Join(publishDir, "pack-manifest.json"), pack); err != nil {
				t.Fatal(err)
			}
			const originalResult = "not yet committed"
			if err := os.WriteFile(resultPath, []byte(originalResult), 0o600); err != nil {
				t.Fatal(err)
			}
			local := editor.Result{
				GalleryPath: filepath.Join(publishDir, "index.html"),
				SummaryPath: filepath.Join(publishDir, "publish-summary.md"),
				Shorts:      []editor.ShortResult{{SegmentID: "seg-001"}},
			}

			err := writeDurableRenderDocuments(
				uuid.New(),
				editor.PresetViral60Clean,
				uuid.New(),
				outDir,
				publishDir,
				resultPath,
				local,
			)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("writeDurableRenderDocuments error = %v, want %q", err, tt.want)
			}
			body, readErr := os.ReadFile(resultPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(body) != originalResult {
				t.Fatalf("result marker changed before manifest validation: %q", body)
			}
		})
	}
}

func TestWriteDurableRenderDocumentsIgnoresRendererPublishDocumentPaths(t *testing.T) {
	tests := []struct {
		name  string
		paths func(root, publishDir string) (string, string)
	}{
		{
			name: "external absolute paths",
			paths: func(root, _ string) (string, string) {
				return filepath.Join(root, "outside-gallery.html"), filepath.Join(root, "outside-summary.md")
			},
		},
		{
			name: "aliased traversal path",
			paths: func(_, publishDir string) (string, string) {
				alias := filepath.Clean(filepath.Join(publishDir, "..", "..", "aliased-output.txt"))
				return alias, alias
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			outDir := filepath.Join(root, "work", "out")
			publishDir := filepath.Join(outDir, "shortslistosparasubir")
			resultPath := filepath.Join(outDir, "shorts-result.json")
			if err := writeJSONFile(filepath.Join(outDir, "edit-document.json"), renderplan.EditDocument{}); err != nil {
				t.Fatal(err)
			}
			if err := writeJSONFile(filepath.Join(outDir, "edit-manifest.json"), editor.Manifest{
				Shorts: []editor.ShortEdit{{SegmentID: "seg-001"}},
			}); err != nil {
				t.Fatal(err)
			}
			if err := writeJSONFile(filepath.Join(publishDir, "pack-manifest.json"), editor.PackManifest{
				Items: []editor.PublishItem{{SegmentID: "seg-001"}},
			}); err != nil {
				t.Fatal(err)
			}
			rendererGallery, rendererSummary := tt.paths(t.TempDir(), publishDir)
			const sentinel = "must remain untouched"
			poisoned := map[string]struct{}{
				rendererGallery: {},
				rendererSummary: {},
			}
			for path := range poisoned {
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(sentinel), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			err := writeDurableRenderDocuments(
				uuid.New(),
				editor.PresetViral60Clean,
				uuid.New(),
				outDir,
				publishDir,
				resultPath,
				editor.Result{
					GalleryPath: rendererGallery,
					SummaryPath: rendererSummary,
					Shorts:      []editor.ShortResult{{SegmentID: "seg-001"}},
				},
			)
			if err != nil {
				t.Fatalf("writeDurableRenderDocuments error = %v", err)
			}
			for path := range poisoned {
				body, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if string(body) != sentinel {
					t.Fatalf("renderer-controlled path %s changed to %q", path, body)
				}
			}
			gallery, err := os.ReadFile(filepath.Join(publishDir, "index.html"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(gallery, []byte("TickCut publish pack")) {
				t.Fatalf("canonical gallery = %q", gallery)
			}
			summary, err := os.ReadFile(filepath.Join(publishDir, "publish-summary.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(summary, []byte("# TickCut publish pack")) {
				t.Fatalf("canonical summary = %q", summary)
			}
		})
	}
}

func TestDurableRenderProjectorsReturnArtifactReferenceErrors(t *testing.T) {
	id := uuid.New()
	revisionID := uuid.New()
	tests := []struct {
		name    string
		project func() error
	}{
		{
			name: "render result",
			project: func() error {
				short := editor.ShortResult{SegmentID: "../escape"}
				return projectDurableShort(id, editor.PresetViral60Clean, revisionID, &short)
			},
		},
		{
			name: "edit manifest",
			project: func() error {
				short := editor.ShortEdit{SegmentID: "../escape"}
				return projectDurableEdit(id, editor.PresetViral60Clean, revisionID, &short)
			},
		},
		{
			name: "pack manifest",
			project: func() error {
				item := editor.PublishItem{SegmentID: "../escape"}
				return projectDurablePublishItem(id, editor.PresetViral60Clean, revisionID, &item)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.project()
			if err == nil || !strings.Contains(err.Error(), "artifact name") {
				t.Fatalf("project error = %v, want artifact reference validation", err)
			}
		})
	}
}

func TestExplicitCoverArgsCarriesEveryBooleanDecision(t *testing.T) {
	tests := []struct {
		name string
		edit renderplan.EditRequest
		want string
	}{
		{name: "generated gameplay candidates", edit: renderplan.EditRequest{CoverStrategy: renderplan.CoverStrategyGenerated}, want: "--covers=true"},
		{name: "no cover", edit: renderplan.EditRequest{CoverStrategy: renderplan.CoverStrategyNone}, want: "--covers=false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := explicitCoverArgs(renderplan.Loadout{CoverSheets: false}, tt.edit)
			for _, want := range []string{tt.want, "--cover-sheets=false", "--cover-first-frame=false"} {
				if !slices.Contains(args, want) {
					t.Fatalf("args = %#v, want %q", args, want)
				}
			}
		})
	}
}

func TestRenderVariantCompletionStatusRequiresReviewForWarnings(t *testing.T) {
	if got := renderVariantCompletionStatus(editor.Result{}); got != renderplan.RenderVariantStatusReady {
		t.Fatalf("clean result status = %q, want ready", got)
	}
	if got := renderVariantCompletionStatus(editor.Result{Warnings: []string{"frozen frame"}}); got != renderplan.RenderVariantStatusReview {
		t.Fatalf("warning result status = %q, want review_required", got)
	}
}

func TestPreserveRenderArtifactPointerKeepsCanonicalPrefixForLegacyState(t *testing.T) {
	id := uuid.New()
	loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:   id,
		Loadout: loadout,
		Status:  renderplan.RenderVariantStatusReady,
	})
	if err != nil {
		t.Fatal(err)
	}
	legacy.ArtifactPrefix = ""
	migrated, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:    id,
		Loadout:  loadout,
		Status:   renderplan.RenderVariantStatusRendering,
		Previous: &legacy,
	})
	if err != nil {
		t.Fatal(err)
	}
	canonicalPrefix := migrated.ArtifactPrefix

	preserveRenderArtifactPointer(&migrated, &legacy)

	if migrated.ArtifactPrefix != canonicalPrefix || migrated.ArtifactPrefix == "" {
		t.Fatalf("artifact prefix = %q, want canonical %q", migrated.ArtifactPrefix, canonicalPrefix)
	}
	ref, err := renderplan.NewRenderVariantArtifactRefForState(
		migrated,
		renderplan.RenderVariantArtifactVideo,
		"seg-001",
	)
	if err != nil {
		t.Fatal(err)
	}
	if want := canonicalPrefix + "/videos/seg-001.mp4"; ref.Key != want {
		t.Fatalf("video key = %q, want %q", ref.Key, want)
	}
}

func TestCompileSegmentsArgs(t *testing.T) {
	tests := []struct {
		name       string
		segmentIDs []string
		want       []string
	}{
		{
			name:       "no segments",
			segmentIDs: nil,
			want:       nil,
		},
		{
			name:       "single segment keeps today's per-segment render",
			segmentIDs: []string{"seg-001"},
			want:       nil,
		},
		{
			name:       "two segments compile into one short in plan order",
			segmentIDs: []string{"seg-001", "seg-004"},
			want:       []string{"--compile-segments", "--segments", "seg-001,seg-004"},
		},
		{
			name:       "three segments join all ids in order",
			segmentIDs: []string{"seg-003", "seg-001", "seg-002"},
			want:       []string{"--compile-segments", "--segments", "seg-003,seg-001,seg-002"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compileSegmentsArgs(tt.segmentIDs)
			if len(got) != len(tt.want) {
				t.Fatalf("compileSegmentsArgs(%v) = %v, want %v", tt.segmentIDs, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("compileSegmentsArgs(%v) = %v, want %v", tt.segmentIDs, got, tt.want)
				}
			}
		})
	}
}

func TestRenderWorkerCompilesMultipleSegmentsIntoOneShort(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002")
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	recordingPlan, err := recording.NewPlanFromKillPlan(plan, "demo.dem", "out", recording.DefaultStreamConfig())
	if err != nil {
		t.Fatal(err)
	}
	rec := recording.RecordingResult{
		Plan:            recordingPlan,
		CaptureMode:     recording.CaptureModeReal,
		CaptureVerified: true,
		Artifacts: []recording.RecordingArtifact{
			{SegmentID: "seg-001", Role: "segment", Type: "video", Path: "C:/stale/seg-001.mp4", SizeBytes: 4},
			{SegmentID: "seg-002", Role: "segment", Type: "video", Path: "C:/stale/seg-002.mp4", SizeBytes: 4},
		},
	}
	rec.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(rec.Plan)
	putJSON(t, store, recording.ResultArtifactKey(id), rec)
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip1")))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-002"), bytes.NewReader([]byte("clip2")))

	const compiledID = "demo-compilation"
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if !hasArg(args, "--compile-segments") {
			t.Fatalf("editor args missing --compile-segments for a 2-segment render: %#v", args)
		}
		if got, want := argValue(args, "--segments"), "seg-001,seg-002"; got != want {
			t.Fatalf("--segments = %q, want %q", got, want)
		}
		outDir := argValue(args, "--out")
		publishDir := argValue(args, "--publish-dir")
		videoPath := filepath.Join(publishDir, compiledID+".mp4")
		coverPath := filepath.Join(publishDir, compiledID+".cover.jpg")
		captionPath := filepath.Join(publishDir, compiledID+".caption.txt")
		logPath := filepath.Join(outDir, "logs", compiledID+"-render.log")
		for _, file := range []struct {
			path string
			body string
		}{
			{filepath.Join(outDir, "edit-manifest.json"), `{"shorts":[{"segment_id":"demo-compilation"}]}`},
			{filepath.Join(publishDir, "pack-manifest.json"), `{"items":[{"segment_id":"demo-compilation"}]}`},
			{filepath.Join(publishDir, "index.html"), `<html></html>`},
			{filepath.Join(publishDir, "publish-summary.md"), `summary`},
			{videoPath, "video"},
			{coverPath, "cover"},
			{captionPath, "caption"},
			{logPath, "log"},
		} {
			if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file.path, []byte(file.body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		rendered := editor.Result{
			Preset:      editor.PresetViral60Clean,
			OutputDir:   outDir,
			PublishDir:  publishDir,
			GalleryPath: filepath.Join(publishDir, "index.html"),
			SummaryPath: filepath.Join(publishDir, "publish-summary.md"),
			// A compiled render emits exactly one short covering every selected
			// segment, not one short per segment.
			Shorts: []editor.ShortResult{{
				SegmentID:     compiledID,
				Output:        videoPath,
				PublishPath:   videoPath,
				CoverPath:     coverPath,
				CaptionPath:   captionPath,
				RenderLogPath: logPath,
			}},
		}
		if err := writeJSONFile(filepath.Join(outDir, "shorts-result.json"), rendered); err != nil {
			t.Fatal(err)
		}
		return []byte("rendered"), nil
	}}
	w := NewRenderWorker(repo, store, RenderWorkerConfig{
		WorkDir:    t.TempDir(),
		EditorPath: "zv-editor",
		FFmpegPath: "ffmpeg",
	})
	w.runner = runner

	if err := w.HandleRenderVariant(context.Background(), renderTask(t, id, editor.PresetViral60Clean)); err != nil {
		t.Fatalf("HandleRenderVariant error = %v", err)
	}

	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean)], &state); err != nil {
		t.Fatal(err)
	}
	if got, want := state.Status, renderplan.RenderVariantStatusReady; got != want {
		t.Fatalf("render state = %q, want %q", got, want)
	}
	// The published output is one compiled reel, not per-segment shorts: only
	// the "demo-compilation" video/cover/caption keys exist.
	for _, kind := range []renderplan.RenderVariantArtifactKind{
		renderplan.RenderVariantArtifactVideo,
		renderplan.RenderVariantArtifactCover,
		renderplan.RenderVariantArtifactCaption,
	} {
		ref, err := renderplan.NewRenderVariantArtifactRefForState(state, kind, compiledID)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := store.files[ref.Key]; !ok {
			t.Fatalf("storage missing compiled short artifact %s", ref.Key)
		}
	}
	for _, segmentID := range []string{"seg-001", "seg-002"} {
		ref, err := renderplan.NewRenderVariantArtifactRefForState(state, renderplan.RenderVariantArtifactVideo, segmentID)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := store.files[ref.Key]; ok {
			t.Fatalf("storage has a per-segment short %s; multi-segment renders must compile into one reel", ref.Key)
		}
	}

	// The result artifact is the source the videos-listing endpoints
	// (GetRenderPublishBoard, GetRenderVariant) read from; confirm it reports
	// the single compiled short so the API/web exposes exactly one video.
	var result editor.Result
	if err := json.Unmarshal(store.files[state.RenderResultKey], &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Shorts) != 1 || result.Shorts[0].SegmentID != compiledID {
		t.Fatalf("render result shorts = %#v, want exactly one %q short", result.Shorts, compiledID)
	}
}

func TestRenderWorkerWritesFailedStateWhenEditorFails(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))

	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outDir := argValue(args, "--out")
		publishDir := argValue(args, "--publish-dir")
		if hasArg(args, "--intro-text") || hasArg(args, "--outro-text") {
			t.Fatalf("editor args = %#v, want no bookend text flags when unset", args)
		}
		if err := os.MkdirAll(publishDir, 0o750); err != nil {
			t.Fatal(err)
		}
		videoPath := filepath.Join(publishDir, "seg-001.mp4")
		if err := os.WriteFile(videoPath, []byte("mp4"), 0o600); err != nil {
			t.Fatal(err)
		}
		result := editor.Result{
			Preset: editor.PresetViral60Clean,
			Error:  "encoder failed",
			Shorts: []editor.ShortResult{{
				SegmentID:   "seg-001",
				PublishPath: videoPath,
				PublishArtifact: recording.RecordingArtifact{
					Path:      videoPath,
					SizeBytes: 3,
				},
			}},
		}
		if err := writeJSONFile(filepath.Join(outDir, "shorts-result.json"), result); err != nil {
			t.Fatal(err)
		}
		return nil, errors.New("zv-editor failed")
	}}
	w := NewRenderWorker(repo, store, RenderWorkerConfig{
		WorkDir:    t.TempDir(),
		EditorPath: "zv-editor",
	})
	w.runner = runner

	err := w.HandleRenderVariant(context.Background(), renderTask(t, id, editor.PresetViral60Clean))
	if err == nil {
		t.Fatal("HandleRenderVariant error = nil, want failure")
	}
	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean)], &state); err != nil {
		t.Fatal(err)
	}
	if got, want := state.Status, renderplan.RenderVariantStatusFailed; got != want {
		t.Fatalf("render state = %q, want %q", got, want)
	}
	if state.Error != "encoder failed" {
		t.Fatalf("state error = %q, want encoder failed", state.Error)
	}
}

func TestRenderWorkerRejectsUnknownVariant(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("runner should not be called for an unknown variant")
		return nil, nil
	}}
	w := NewRenderWorker(repo, store, RenderWorkerConfig{WorkDir: t.TempDir(), EditorPath: "zv-editor"})
	w.runner = runner

	err := w.HandleRenderVariant(context.Background(), renderTask(t, id, "made-up-preset"))
	if err == nil {
		t.Fatal("HandleRenderVariant error = nil, want unknown variant error")
	}
	if !strings.Contains(err.Error(), "unknown render variant") || !strings.Contains(err.Error(), editor.PresetViral60Clean) {
		t.Fatalf("error = %v, want unknown render variant listing valid presets", err)
	}
}

func TestRenderWorkerDefaultsToViral60WhenVariantEmpty(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	defaultVariant := editor.DefaultPreset().Name
	recordingResult := recordingResultWithSegment("", "C:/stale/seg-001.mp4")
	recordingResult.CaptureRevision = "capture-1"
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResult)
	fingerprint, err := renderInputFingerprint(recordingResult, &plan, defaultVariant, "", "", 0, renderplan.DefaultEditRequest())
	if err != nil {
		t.Fatal(err)
	}
	seedLegacyRenderVariantReady(t, store, id, defaultVariant, editor.Result{
		Preset:           defaultVariant,
		InputFingerprint: fingerprint,
		Shorts:           []editor.ShortResult{{SegmentID: "seg-001"}},
	})

	runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("runner should not be called when default variant outputs already exist")
		return nil, nil
	}}
	w := NewRenderWorker(repo, store, RenderWorkerConfig{WorkDir: t.TempDir(), EditorPath: "zv-editor"})
	w.runner = runner

	payload, err := json.Marshal(tasks.RenderVariantPayload{JobID: id})
	if err != nil {
		t.Fatal(err)
	}
	task := asynq.NewTask(tasks.TypeRenderVariant, payload)
	if err := w.HandleRenderVariant(context.Background(), task); err != nil {
		t.Fatalf("HandleRenderVariant error = %v", err)
	}
	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, defaultVariant)], &state); err != nil {
		t.Fatal(err)
	}
	if got, want := state.Variant, defaultVariant; got != want {
		t.Fatalf("state variant = %q, want %q", got, want)
	}
	if got, want := state.Status, renderplan.RenderVariantStatusReady; got != want {
		t.Fatalf("render state = %q, want %q", got, want)
	}
}

func TestProbeRenderResultUpdatesPublishArtifact(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "seg-001.mp4")
	if err := os.WriteFile(videoPath, []byte("mp4"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := editor.Result{
		Shorts: []editor.ShortResult{{
			SegmentID:   "seg-001",
			PublishPath: videoPath,
		}},
	}
	runner := &fakeRunner{fn: func(_ context.Context, exe string, args ...string) ([]byte, error) {
		if exe != "ffprobe" {
			t.Fatalf("exe = %q, want ffprobe", exe)
		}
		if got := args[len(args)-1]; got != videoPath {
			t.Fatalf("last arg = %q, want %q", got, videoPath)
		}
		return []byte(`{"streams":[{"codec_name":"h264","width":1080,"height":1920,"r_frame_rate":"60/1","duration":"12.5"}],"format":{"duration":"12.5","size":"12345"}}`), nil
	}}

	if err := probeRenderResult(context.Background(), runner, "ffprobe", &result); err != nil {
		t.Fatalf("probeRenderResult error = %v", err)
	}
	got := result.Shorts[0].PublishArtifact
	if got.Codec != "h264" || got.Width != 1080 || got.Height != 1920 || got.DurationSeconds != 12.5 || got.SizeBytes != 12345 {
		t.Fatalf("artifact = %#v", got)
	}
}

func TestRenderWorkerSkipsWhenVariantOutputsAlreadyExist(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	recordingResult := recordingResultWithSegment("", "C:/stale/seg-001.mp4")
	recordingResult.CaptureRevision = "capture-1"
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResult)
	fingerprint, err := renderInputFingerprint(recordingResult, &plan, editor.PresetViral60Clean, "", "", 0, renderplan.DefaultEditRequest())
	if err != nil {
		t.Fatal(err)
	}
	seedLegacyRenderVariantReady(t, store, id, editor.PresetViral60Clean, editor.Result{
		Preset:           editor.PresetViral60Clean,
		InputFingerprint: fingerprint,
		Shorts: []editor.ShortResult{{
			SegmentID: "seg-001",
		}},
	})

	runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("runner should not be called when render variant outputs already exist")
		return nil, nil
	}}
	w := NewRenderWorker(repo, store, RenderWorkerConfig{})
	w.runner = runner

	if err := w.HandleRenderVariant(context.Background(), renderTask(t, id, editor.PresetViral60Clean)); err != nil {
		t.Fatalf("HandleRenderVariant error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %d, want 0", len(runner.calls))
	}
}

func TestRenderWorkerMigratesCachedWarningsToReviewRequired(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	recordingResult := recordingResultWithSegment("", "C:/stale/seg-001.mp4")
	recordingResult.CaptureRevision = "capture-1"
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResult)
	fingerprint, err := renderInputFingerprint(recordingResult, &plan, editor.PresetViral60Clean, "", "", 0, renderplan.DefaultEditRequest())
	if err != nil {
		t.Fatal(err)
	}
	warnings := []string{"frozen frame at 00:07"}
	seedLegacyRenderVariantReady(t, store, id, editor.PresetViral60Clean, editor.Result{
		Preset:           editor.PresetViral60Clean,
		InputFingerprint: fingerprint,
		Warnings:         warnings,
		Shorts:           []editor.ShortResult{{SegmentID: "seg-001"}},
	})

	runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("runner should not be called when cached render outputs are complete")
		return nil, nil
	}}
	worker := NewRenderWorker(repo, store, RenderWorkerConfig{})
	worker.runner = runner
	if err := worker.HandleRenderVariant(context.Background(), renderTask(t, id, editor.PresetViral60Clean)); err != nil {
		t.Fatalf("HandleRenderVariant error = %v", err)
	}

	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean)], &state); err != nil {
		t.Fatal(err)
	}
	if state.Status != renderplan.RenderVariantStatusReview {
		t.Fatalf("render state = %q, want %q", state.Status, renderplan.RenderVariantStatusReview)
	}
	if !slices.Equal(state.Warnings, warnings) {
		t.Fatalf("warnings = %#v, want %#v", state.Warnings, warnings)
	}
}

func TestRenderWorkerMigratesCachedArtifactWarningsToReviewRequired(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	recordingResult := recordingResultWithSegment("", "C:/stale/seg-001.mp4")
	recordingResult.CaptureRevision = "capture-1"
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResult)
	fingerprint, err := renderInputFingerprint(recordingResult, &plan, editor.PresetViral60Clean, "", "", 0, renderplan.DefaultEditRequest())
	if err != nil {
		t.Fatal(err)
	}
	seedLegacyRenderVariantReady(t, store, id, editor.PresetViral60Clean, editor.Result{
		Preset:           editor.PresetViral60Clean,
		InputFingerprint: fingerprint,
		Shorts: []editor.ShortResult{{
			SegmentID: "seg-001",
			PublishArtifact: recording.RecordingArtifact{
				Path:   "seg-001.mp4",
				Width:  1080,
				Height: 1920,
			},
		}},
	})

	runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("runner should not be called when cached render outputs are complete")
		return nil, nil
	}}
	worker := NewRenderWorker(repo, store, RenderWorkerConfig{})
	worker.runner = runner
	if err := worker.HandleRenderVariant(context.Background(), renderTask(t, id, editor.PresetViral60Clean)); err != nil {
		t.Fatalf("HandleRenderVariant error = %v", err)
	}

	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean)], &state); err != nil {
		t.Fatal(err)
	}
	if state.Status != renderplan.RenderVariantStatusReview {
		t.Fatalf("render state = %q, want %q", state.Status, renderplan.RenderVariantStatusReview)
	}
	want := []string{"quality seg-001: missing_or_empty_video"}
	if !slices.Equal(state.Warnings, want) {
		t.Fatalf("warnings = %#v, want %#v", state.Warnings, want)
	}
}

func TestRenderWorkerPreservesResolvedReviewForSameCachedRevision(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	recordingResult := recordingResultWithSegment("", "C:/stale/seg-001.mp4")
	recordingResult.CaptureRevision = "capture-1"
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResult)
	fingerprint, err := renderInputFingerprint(recordingResult, &plan, editor.PresetViral60Clean, "", "", 0, renderplan.DefaultEditRequest())
	if err != nil {
		t.Fatal(err)
	}
	warnings := []string{"intentional freeze at 00:07"}
	seedLegacyRenderVariantReady(t, store, id, editor.PresetViral60Clean, editor.Result{
		Preset:           editor.PresetViral60Clean,
		InputFingerprint: fingerprint,
		Warnings:         warnings,
		Shorts:           []editor.ShortResult{{SegmentID: "seg-001"}},
	})
	loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:    id,
		Loadout:  loadout,
		Status:   renderplan.RenderVariantStatusReady,
		Warnings: warnings,
	})
	if err != nil {
		t.Fatal(err)
	}
	state.ReviewResolution = &renderplan.RenderReviewResolution{
		ArtifactPrefix: state.ArtifactPrefix,
		Warnings:       warnings,
		Note:           "Freeze inspected and intentional.",
		ReviewedAt:     time.Now().UTC(),
	}
	putJSON(t, store, mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean), state)

	runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("runner should not be called for the same resolved cached revision")
		return nil, nil
	}}
	worker := NewRenderWorker(repo, store, RenderWorkerConfig{})
	worker.runner = runner
	if err := worker.HandleRenderVariant(context.Background(), renderTask(t, id, editor.PresetViral60Clean)); err != nil {
		t.Fatalf("HandleRenderVariant error = %v", err)
	}
	var stored renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean)], &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status != renderplan.RenderVariantStatusReady || !stored.ReviewResolvedFor(warnings) {
		t.Fatalf("resolved cached state regressed: %#v", stored)
	}
}

func TestRenderWorkerRerunsWhenCachedInputsChange(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*recording.RecordingResult, *renderplan.EditRequest)
		musicKey  string
		withMusic bool
	}{
		{
			name: "capture revision",
			mutate: func(result *recording.RecordingResult, _ *renderplan.EditRequest) {
				result.CaptureRevision = "capture-2"
			},
		},
		{
			name: "edit treatment",
			mutate: func(_ *recording.RecordingResult, edit *renderplan.EditRequest) {
				edit.Transition = renderplan.TransitionWhip
			},
		},
		{name: "music", musicKey: "phonk", withMusic: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			id := uuid.New()
			plan := minimalKillPlan()
			repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
			rec := recordingResultWithSegment("", "C:/stale/seg-001.mp4")
			rec.CaptureRevision = "capture-1"
			cachedFingerprint, err := renderInputFingerprint(rec, &plan, editor.PresetViral60Clean, "", "", 0, renderplan.DefaultEditRequest())
			if err != nil {
				t.Fatal(err)
			}
			putJSON(t, store, mustRenderVariantResultKey(t, id, editor.PresetViral60Clean), editor.Result{
				Preset:           editor.PresetViral60Clean,
				InputFingerprint: cachedFingerprint,
				Shorts:           []editor.ShortResult{{SegmentID: "seg-001"}},
			})
			_ = store.Put(mustRenderVariantPackManifestKey(t, id, editor.PresetViral60Clean), bytes.NewReader([]byte("pack")))
			_ = store.Put(mustRenderVariantGalleryKey(t, id, editor.PresetViral60Clean), bytes.NewReader([]byte("gallery")))

			edit := renderplan.DefaultEditRequest()
			if tc.mutate != nil {
				tc.mutate(&rec, &edit)
			}
			putJSON(t, store, recording.ResultArtifactKey(id), rec)
			_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))
			musicDir := t.TempDir()
			if tc.withMusic {
				if err := os.WriteFile(filepath.Join(musicDir, tc.musicKey+".wav"), []byte("music"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			wantErr := errors.New("rerender invoked")
			runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
				return nil, wantErr
			}}
			w := NewRenderWorker(repo, store, RenderWorkerConfig{
				WorkDir:    t.TempDir(),
				EditorPath: "zv-editor",
				MusicDir:   musicDir,
			})
			w.runner = runner
			task, err := tasks.NewRenderVariantTask(id, editor.PresetViral60Clean, tc.musicKey, 0, edit)
			if err != nil {
				t.Fatal(err)
			}
			err = w.HandleRenderVariant(context.Background(), task)
			if !errors.Is(err, wantErr) {
				t.Fatalf("HandleRenderVariant error = %v, want rerender sentinel", err)
			}
			if len(runner.calls) != 1 {
				t.Fatalf("runner calls = %d, want 1 for stale cache", len(runner.calls))
			}
		})
	}
}

func TestRenderWorkerPassesMusicVolume(t *testing.T) {
	cases := []struct {
		name           string
		volume         float64
		musicAvailable bool
		wantMusic      renderplan.MusicSnapshot
	}{
		{
			name:           "custom volume threads to every consumer",
			volume:         0.35,
			musicAvailable: true,
			wantMusic:      renderplan.MusicSnapshot{Key: "phonk", Volume: 0.35},
		},
		{
			name:           "default volume is explicit unity everywhere",
			volume:         0,
			musicAvailable: true,
			wantMusic:      renderplan.MusicSnapshot{Key: "phonk", Volume: 1},
		},
		{
			name:           "unavailable music stays music free",
			volume:         1,
			musicAvailable: false,
			wantMusic:      renderplan.MusicSnapshot{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			id := uuid.New()
			plan := minimalKillPlan()
			repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
			putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
			_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))

			musicDir := t.TempDir()
			if tc.musicAvailable {
				if err := os.WriteFile(filepath.Join(musicDir, "phonk.wav"), []byte("music"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var gotArgs []string
			var gotDocument renderplan.EditDocument
			wantErr := errors.New("stop after args")
			runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
				gotArgs = append([]string(nil), args...)
				documentPath := filepath.Join(argValue(args, "--out"), "edit-document.json")
				body, err := os.ReadFile(documentPath)
				if err != nil {
					t.Fatalf("read effective edit document: %v", err)
				}
				if err := json.Unmarshal(body, &gotDocument); err != nil {
					t.Fatalf("decode effective edit document: %v", err)
				}
				return nil, wantErr
			}}
			w := NewRenderWorker(repo, store, RenderWorkerConfig{
				WorkDir:    t.TempDir(),
				EditorPath: "zv-editor",
				MusicDir:   musicDir,
			})
			w.runner = runner

			task, err := tasks.NewRenderVariantTask(id, editor.PresetViral60Clean, "phonk", tc.volume, renderplan.DefaultEditRequest())
			if err != nil {
				t.Fatal(err)
			}
			if err := w.HandleRenderVariant(context.Background(), task); !errors.Is(err, wantErr) {
				t.Fatalf("HandleRenderVariant error = %v, want stop sentinel", err)
			}
			if got := hasArg(gotArgs, "--music"); got != tc.musicAvailable {
				t.Fatalf("--music present = %v, want %v: %#v", got, tc.musicAvailable, gotArgs)
			}
			if got := hasArg(gotArgs, "--music-volume"); got != tc.musicAvailable {
				t.Fatalf("--music-volume present = %v, want %v: %#v", got, tc.musicAvailable, gotArgs)
			}
			if tc.musicAvailable {
				wantValue := strconv.FormatFloat(tc.wantMusic.Volume, 'f', -1, 64)
				if got := argValue(gotArgs, "--music-volume"); got != wantValue {
					t.Fatalf("--music-volume = %q, want %q", got, wantValue)
				}
			}
			if gotDocument.Music == nil ||
				*gotDocument.Music != tc.wantMusic {
				t.Fatalf("effective music snapshot = %#v, want %#v", gotDocument.Music, tc.wantMusic)
			}
		})
	}
}

func TestRenderWorkerTreatsDefaultAndUnityMusicVolumeAsSameCacheIdentity(t *testing.T) {
	tests := []struct {
		name         string
		firstVolume  float64
		secondVolume float64
	}{
		{name: "default then explicit unity", firstVolume: 0, secondVolume: 1},
		{name: "explicit unity then default", firstVolume: 1, secondVolume: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			id := uuid.New()
			plan := minimalKillPlan()
			repo.jobs[id] = &job.Job{
				ID:       id,
				Status:   job.StatusRecorded,
				Rules:    rules.Default(),
				KillPlan: &plan,
			}
			putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
			if err := store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip"))); err != nil {
				t.Fatal(err)
			}
			musicDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(musicDir, "phonk.wav"), []byte("music"), 0o600); err != nil {
				t.Fatal(err)
			}

			runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
				if got := argValue(args, "--music-volume"); got != "1" {
					t.Fatalf("--music-volume = %q, want normalized unity", got)
				}
				documentPath := filepath.Join(argValue(args, "--out"), "edit-document.json")
				var document renderplan.EditDocument
				if err := readJSONFile(documentPath, &document); err != nil {
					t.Fatal(err)
				}
				if document.Music == nil || *document.Music != (renderplan.MusicSnapshot{Key: "phonk", Volume: 1}) {
					t.Fatalf("effective music snapshot = %#v, want phonk/1", document.Music)
				}
				writeSuccessfulSingleShortRenderOutput(t, args)
				return []byte("rendered"), nil
			}}
			worker := NewRenderWorker(repo, store, RenderWorkerConfig{
				WorkDir:    t.TempDir(),
				EditorPath: "zv-editor",
				MusicDir:   musicDir,
			})
			worker.runner = runner

			run := func(volume float64) {
				t.Helper()
				task, err := tasks.NewRenderVariantTask(
					id,
					editor.PresetViral60Clean,
					"phonk",
					volume,
					renderplan.DefaultEditRequest(),
				)
				if err != nil {
					t.Fatal(err)
				}
				if err := worker.HandleRenderVariant(context.Background(), task); err != nil {
					t.Fatalf("HandleRenderVariant(%v) error = %v", volume, err)
				}
			}

			run(tt.firstVolume)
			var firstState renderplan.RenderVariantState
			if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean)], &firstState); err != nil {
				t.Fatal(err)
			}
			var firstResult editor.Result
			if err := json.Unmarshal(store.files[firstState.RenderResultKey], &firstResult); err != nil {
				t.Fatal(err)
			}
			if firstResult.InputFingerprint == "" {
				t.Fatal("first render fingerprint is empty")
			}

			run(tt.secondVolume)
			if len(runner.calls) != 1 {
				t.Fatalf("runner calls = %d, want one render plus one equivalent cache hit", len(runner.calls))
			}
			var secondState renderplan.RenderVariantState
			if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean)], &secondState); err != nil {
				t.Fatal(err)
			}
			if secondState.ArtifactPrefix != firstState.ArtifactPrefix ||
				secondState.RenderResultKey != firstState.RenderResultKey {
				t.Fatalf("cache identity changed: first=%#v second=%#v", firstState, secondState)
			}
		})
	}
}

func writeSuccessfulSingleShortRenderOutput(t *testing.T, args []string) {
	t.Helper()
	outDir := argValue(args, "--out")
	publishDir := argValue(args, "--publish-dir")
	videoPath := filepath.Join(publishDir, "seg-001.mp4")
	captionPath := filepath.Join(publishDir, "seg-001.caption.txt")
	if err := writeJSONFile(filepath.Join(outDir, "edit-manifest.json"), editor.Manifest{
		Shorts: []editor.ShortEdit{{SegmentID: "seg-001"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(publishDir, "pack-manifest.json"), editor.PackManifest{
		Items: []editor.PublishItem{{SegmentID: "seg-001"}},
	}); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		videoPath:   "video",
		captionPath: "caption",
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeJSONFile(filepath.Join(outDir, "shorts-result.json"), editor.Result{
		Preset:      editor.PresetViral60Clean,
		OutputDir:   outDir,
		PublishDir:  publishDir,
		GalleryPath: filepath.Join(publishDir, "index.html"),
		SummaryPath: filepath.Join(publishDir, "publish-summary.md"),
		Shorts: []editor.ShortResult{{
			SegmentID:   "seg-001",
			Output:      videoPath,
			PublishPath: videoPath,
			CaptionPath: captionPath,
		}},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestIsTerminalAttempt(t *testing.T) {
	cases := []struct {
		name              string
		retried, maxRetry int
		inTask            bool
		want              bool
	}{
		{"outside asynq task context", 0, 0, false, true},
		{"no-retry task first attempt", 0, 0, true, true},
		{"retryable task mid-flight", 3, 25, true, false},
		{"retryable task final attempt", 25, 25, true, true},
		{"retryable task past max", 26, 25, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTerminalAttempt(tc.retried, tc.maxRetry, tc.inTask); got != tc.want {
				t.Errorf("isTerminalAttempt(%d, %d, %v) = %v, want %v", tc.retried, tc.maxRetry, tc.inTask, got, tc.want)
			}
		})
	}
}

func TestTaskIsTerminalUsesInlineAttemptContext(t *testing.T) {
	tests := []struct {
		name     string
		retried  int
		maxRetry int
		want     bool
	}{
		{name: "intermediate attempt", retried: 0, maxRetry: 1, want: false},
		{name: "final attempt", retried: 1, maxRetry: 1, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tasks.WithTaskAttempt(context.Background(), tt.retried, tt.maxRetry)
			if got := taskIsTerminal(ctx); got != tt.want {
				t.Errorf("taskIsTerminal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func minimalKillPlan() killplan.Plan {
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Demo.SHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	plan.Demo.DurationTicks = 100000
	plan.Target.SteamID64 = "76561197960265729"
	plan.Rules = rules.Default()
	plan.Segments = []killplan.Segment{{
		ID:        "seg-001",
		Round:     1,
		TickStart: 64,
		TickEnd:   128,
	}}
	return plan
}

func recordingResultWithSegment(scriptPath, segmentPath string) recording.RecordingResult {
	plan := minimalKillPlan()
	stream := recording.DefaultStreamConfig()
	stream.HUDMode = recording.HUDModeDeathnotices
	outDir := "out"
	demoPath := "demo.dem"
	if segmentPath != "" {
		outDir = filepath.Dir(filepath.Dir(segmentPath))
		demoPath = filepath.Join(filepath.Dir(outDir), "demo.dem")
	} else if scriptPath != "" {
		outDir = filepath.Dir(scriptPath)
		demoPath = filepath.Join(filepath.Dir(outDir), "demo.dem")
	}
	recordingPlan, err := recording.NewPlanFromKillPlan(plan, demoPath, outDir, stream)
	if err != nil {
		panic(fmt.Sprintf("build test recording plan: %v", err))
	}
	result := recording.RecordingResult{
		Plan:            recordingPlan,
		Script:          scriptPath,
		CaptureMode:     recording.CaptureModeReal,
		CaptureVerified: true,
		Artifacts: []recording.RecordingArtifact{{
			SegmentID: "seg-001",
			Role:      "segment",
			Type:      "video",
			Path:      segmentPath,
			SizeBytes: 4,
		}},
	}
	result.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(result.Plan)
	return result
}

func recordingResultForRunnerArgs(t *testing.T, args []string, scriptPath, segmentPath string) recording.RecordingResult {
	t.Helper()
	var plan killplan.Plan
	if err := readJSONFile(argValue(args, "--killplan"), &plan); err != nil {
		t.Fatalf("read runner kill plan: %v", err)
	}
	stream := recording.DefaultStreamConfig()
	stream.HUDMode = recording.HUDMode(argValue(args, "--hud"))
	stream.PortraitSafeKillfeed = hasArg(args, "--portrait-safe-killfeed")
	recordingPlan, err := recording.NewPlanFromKillPlan(plan, argValue(args, "--demo"), argValue(args, "--out"), stream)
	if err != nil {
		t.Fatalf("build runner recording plan: %v", err)
	}
	info, err := os.Stat(segmentPath)
	if err != nil {
		t.Fatal(err)
	}
	result := recording.RecordingResult{
		Plan:            recordingPlan,
		Script:          scriptPath,
		CaptureMode:     recording.CaptureModeReal,
		CaptureVerified: true,
		Artifacts: []recording.RecordingArtifact{{
			SegmentID: plan.Segments[0].ID,
			Role:      "segment",
			Type:      "video",
			Path:      segmentPath,
			SizeBytes: info.Size(),
		}},
	}
	result.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(result.Plan)
	return result
}

func recordTask(t *testing.T, id uuid.UUID) *asynq.Task {
	t.Helper()
	return recordTaskFor(t, id, nil)
}

func recordTaskFor(t *testing.T, id uuid.UUID, segmentIDs []string) *asynq.Task {
	t.Helper()
	return recordTaskWithCaptureProfile(t, id, "", segmentIDs, false)
}

func recordTaskWithCaptureProfile(t *testing.T, id uuid.UUID, hudMode string, segmentIDs []string, portraitSafeKillfeed bool) *asynq.Task {
	t.Helper()
	task, err := tasks.NewRecordDemoTask(id, hudMode, segmentIDs, portraitSafeKillfeed)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func composeTask(t *testing.T, id uuid.UUID) *asynq.Task {
	t.Helper()
	task, err := tasks.NewComposeFinalTask(id)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func renderTask(t *testing.T, id uuid.UUID, variant string) *asynq.Task {
	t.Helper()
	task, err := tasks.NewRenderVariantTask(id, variant, "", 0, renderplan.EditRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func argValue(args []string, key string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == key {
			return args[i+1]
		}
	}
	return ""
}

func hasArg(args []string, key string) bool {
	for _, arg := range args {
		if arg == key {
			return true
		}
	}
	return false
}

func putJSON(t *testing.T, store *fakeStorage, key string, value any) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(key, bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}

func mustSegmentClipKey(t *testing.T, id uuid.UUID, segmentID string) string {
	t.Helper()
	key, err := recording.SegmentClipArtifactKey(id, segmentID)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustRenderVariantResultKey(t *testing.T, id uuid.UUID, variant string) string {
	t.Helper()
	key, err := artifacts.RenderVariantResultKey(id, variant)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustRenderVariantStatusKey(t *testing.T, id uuid.UUID, variant string) string {
	t.Helper()
	key, err := artifacts.RenderVariantStatusKey(id, variant)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustRenderVariantEditDocumentKey(t *testing.T, id uuid.UUID, variant string) string {
	t.Helper()
	key, err := artifacts.RenderVariantEditDocumentKey(id, variant)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustRenderVariantEditManifestKey(t *testing.T, id uuid.UUID, variant string) string {
	t.Helper()
	key, err := artifacts.RenderVariantEditManifestKey(id, variant)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustRenderVariantPackManifestKey(t *testing.T, id uuid.UUID, variant string) string {
	t.Helper()
	key, err := artifacts.RenderVariantPackManifestKey(id, variant)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustRenderVariantPublishSummaryKey(t *testing.T, id uuid.UUID, variant string) string {
	t.Helper()
	key, err := artifacts.RenderVariantPublishSummaryKey(id, variant)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustRenderVariantGalleryKey(t *testing.T, id uuid.UUID, variant string) string {
	t.Helper()
	key, err := artifacts.RenderVariantGalleryKey(id, variant)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustRenderVariantVideoKey(t *testing.T, id uuid.UUID, variant, name string) string {
	t.Helper()
	key, err := artifacts.RenderVariantVideoKey(id, variant, name)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustRenderVariantCoverKey(t *testing.T, id uuid.UUID, variant, name string) string {
	t.Helper()
	key, err := artifacts.RenderVariantCoverKey(id, variant, name)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustRenderVariantCaptionKey(t *testing.T, id uuid.UUID, variant, name string) string {
	t.Helper()
	key, err := artifacts.RenderVariantCaptionKey(id, variant, name)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func seedLegacyRenderVariantReady(
	t *testing.T,
	store *fakeStorage,
	id uuid.UUID,
	variant string,
	result editor.Result,
) {
	t.Helper()
	putJSON(t, store, mustRenderVariantResultKey(t, id, variant), result)
	for _, key := range []string{
		mustRenderVariantEditDocumentKey(t, id, variant),
		mustRenderVariantEditManifestKey(t, id, variant),
		mustRenderVariantPackManifestKey(t, id, variant),
		mustRenderVariantPublishSummaryKey(t, id, variant),
		mustRenderVariantGalleryKey(t, id, variant),
	} {
		if err := store.Put(key, bytes.NewReader([]byte("artifact"))); err != nil {
			t.Fatal(err)
		}
	}
	for _, short := range result.Shorts {
		for _, key := range []string{
			mustRenderVariantVideoKey(t, id, variant, short.SegmentID),
			mustRenderVariantCaptionKey(t, id, variant, short.SegmentID),
		} {
			if err := store.Put(key, bytes.NewReader([]byte("artifact"))); err != nil {
				t.Fatal(err)
			}
		}
		if result.CoversEnabled {
			if err := store.Put(
				mustRenderVariantCoverKey(t, id, variant, short.SegmentID),
				bytes.NewReader([]byte("artifact")),
			); err != nil {
				t.Fatal(err)
			}
		}
	}
}
