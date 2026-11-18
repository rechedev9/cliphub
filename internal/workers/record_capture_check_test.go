package workers

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/rules"
)

// The orchestrator asks this before a page load's generate: it must answer as the record task
// would decide, and run nothing.
func TestCaptureNeededAnswersAsTheRecordTaskWould(t *testing.T) {
	repo, store, id := newFakeRepo(), newFakeStorage(), uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002")
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/test.dem", Rules: rules.Default(), KillPlan: &plan}
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))
	var seen []string
	w := newRecordWorkerForTest(repo, store, t)
	w.runner = planRecorderRunner(t, &seen)
	task := recordTaskFor(t, id, []string{"seg-001"})
	needed := func(task *asynq.Task) bool {
		t.Helper()
		got, err := w.CaptureNeeded(context.Background(), task)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	if !needed(task) || len(seen) != 0 || repo.jobs[id].Status != job.StatusParsed {
		t.Fatalf("before any capture: want a capture, and the check runs nothing (recorder runs %d, status %s)", len(seen), repo.jobs[id].Status)
	}
	if err := w.HandleRecordDemo(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if needed(task) {
		t.Fatal("the clip is stored: want it reused without CS2")
	}
	if !needed(recordTaskFor(t, id, []string{"seg-001", "seg-002"})) || !needed(recordTaskWithCaptureProfile(t, id, "gameplay", []string{"seg-001"}, true)) {
		t.Fatal("a play the capture lacks, or another capture profile: want a capture")
	}
	key, err := storedRecordingResult(t, store, id).SegmentClipKey(id, "seg-001")
	if err != nil {
		t.Fatal(err)
	}
	delete(store.files, key)
	if !needed(task) {
		t.Fatal("the clip was freed: want a capture")
	}
}
