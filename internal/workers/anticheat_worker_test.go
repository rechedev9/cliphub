package workers

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/fragforge/internal/anticheat"
	"github.com/rechedev9/fragforge/internal/artifacts"
	"github.com/rechedev9/fragforge/internal/tasks"
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
