package workers

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/anticheat"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func TestFailAnticheatMarksTerminalCancellationFailed(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	doc := anticheat.NewRunningDocument(id.String(), time.Now())
	ctx, cancel := context.WithCancel(tasks.WithTaskAttempt(context.Background(), 1, 1))
	cancel()

	err := NewParserWorker(newFakeRepo(), store).failAnticheat(ctx, id, doc, errors.New("analysis interrupted"))
	if err != nil {
		t.Fatalf("failAnticheat error = %v", err)
	}

	raw, ok := store.files[artifacts.AnticheatKey(id)]
	if !ok {
		t.Fatal("anticheat document was not written")
	}
	decoded, err := anticheat.DecodeDocument(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode document: %v", err)
	}
	if decoded.Status != anticheat.StatusFailed {
		t.Fatalf("document status = %q, want %q", decoded.Status, anticheat.StatusFailed)
	}
}

// Side-lane contract: a failed screening only writes anticheat.json and must
// never flip the demo job status (parse/clip pipeline stays independent).
func TestProcessAnalyzeAnticheatFailureDoesNotMutateJobStatus(t *testing.T) {
	id := uuid.New()
	repo := newFakeJobRepo(job.Job{
		ID:       id,
		Status:   job.StatusParsed,
		DemoPath: "demos/missing-for-anticheat.dem",
	})
	store := newFakeStorage()

	if err := NewParserWorker(repo, store).ProcessAnalyzeAnticheat(context.Background(), id); err != nil {
		t.Fatalf("ProcessAnalyzeAnticheat error = %v, want nil (failure is in the document)", err)
	}

	got := repo.jobs[id]
	if got == nil {
		t.Fatal("job disappeared")
	}
	if got.Status != job.StatusParsed {
		t.Fatalf("job status = %v, want StatusParsed (anticheat must not mutate job status)", got.Status)
	}
	if got.FailureReason != "" {
		t.Fatalf("job failure_reason = %q, want empty (anticheat failures stay in the document)", got.FailureReason)
	}

	raw, ok := store.files[artifacts.AnticheatKey(id)]
	if !ok {
		t.Fatal("anticheat document was not written")
	}
	decoded, err := anticheat.DecodeDocument(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode document: %v", err)
	}
	if decoded.Status != anticheat.StatusFailed {
		t.Fatalf("document status = %q, want %q", decoded.Status, anticheat.StatusFailed)
	}
}

func TestFailAnticheatLeavesIntermediateCancellationRunning(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	doc := anticheat.NewRunningDocument(id.String(), time.Now())
	ctx, cancel := context.WithCancel(tasks.WithTaskAttempt(context.Background(), 0, 1))
	cancel()

	err := NewParserWorker(newFakeRepo(), store).failAnticheat(ctx, id, doc, errors.New("analysis interrupted"))
	if err == nil {
		t.Fatal("failAnticheat error = nil, want the queue retry to receive the cancellation")
	}
	if _, ok := store.files[artifacts.AnticheatKey(id)]; ok {
		t.Fatal("intermediate cancellation wrote a terminal document")
	}
}
