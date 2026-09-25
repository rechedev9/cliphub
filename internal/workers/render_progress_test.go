package workers

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/storage"
)

type countingStorage struct {
	storage.Storage
	puts int
}

func (s *countingStorage) Put(key string, r io.Reader) error {
	s.puts++
	return s.Storage.Put(key, r)
}

func TestRenderProgressReporterWritesArtifact(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	jobID := uuid.New()
	progressPath := filepath.Join(t.TempDir(), "editor-progress.json")
	tracker := editor.NewProgressTracker(progressPath)
	tracker.Set("Montando cortes y ritmo", 42)

	reporter := newRenderProgressReporter(store, jobID, progressPath)
	if err := reporter.report(); err != nil {
		t.Fatal(err)
	}

	rc, err := store.Open(artifacts.RenderProgressKey(jobID))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	var got editor.EditorProgress
	if err := json.NewDecoder(rc).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Percent != 42 || got.Stage != "Montando cortes y ritmo" {
		t.Fatalf("progress = %+v, want 42%% Montando cortes y ritmo", got)
	}
}

func TestRenderProgressReporterWatchFinalWrite(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	jobID := uuid.New()
	progressPath := filepath.Join(t.TempDir(), "editor-progress.json")
	reporter := newRenderProgressReporter(store, jobID, progressPath)
	tracker := editor.NewProgressTracker(progressPath)
	tracker.Flush("Preparando", 10)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		reporter.watch(ctx)
	}()
	// Wait for watch's initial report so the 88 below can only be published by
	// the final report on cancellation, well before the first 1 s tick.
	deadline := time.Now().Add(500 * time.Millisecond)
	for storedRenderPercent(t, store, jobID) != 10 {
		if time.Now().After(deadline) {
			t.Fatal("watch did not publish its initial report")
		}
		time.Sleep(5 * time.Millisecond)
	}

	tracker.Flush("Montando cortes y ritmo", 88)
	cancel()
	<-done

	if got := storedRenderPercent(t, store, jobID); got != 88 {
		t.Fatalf("percent = %d, want 88 from the final report on cancel", got)
	}
}

// storedRenderPercent returns the published render progress percent, or -1
// when no complete document is published yet.
func storedRenderPercent(t *testing.T, store storage.Storage, jobID uuid.UUID) int {
	t.Helper()
	rc, err := store.Open(artifacts.RenderProgressKey(jobID))
	if err != nil {
		return -1
	}
	defer rc.Close()
	var got editor.EditorProgress
	if err := json.NewDecoder(rc).Decode(&got); err != nil {
		return -1
	}
	return got.Percent
}

func TestRenderProgressReporterSkipsUnchangedPut(t *testing.T) {
	inner, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store := &countingStorage{Storage: inner}
	jobID := uuid.New()
	progressPath := filepath.Join(t.TempDir(), "editor-progress.json")
	tracker := editor.NewProgressTracker(progressPath)
	tracker.Flush("Montando cortes y ritmo", 42)

	reporter := newRenderProgressReporter(store, jobID, progressPath)
	if err := reporter.report(); err != nil {
		t.Fatal(err)
	}
	if err := reporter.report(); err != nil {
		t.Fatal(err)
	}
	if store.puts != 1 {
		t.Fatalf("puts = %d, want 1 for identical progress", store.puts)
	}

	tracker.Flush("Verificando fotogramas, vídeo y audio", 94)
	if err := reporter.report(); err != nil {
		t.Fatal(err)
	}
	if store.puts != 2 {
		t.Fatalf("puts = %d, want 2 after progress changed", store.puts)
	}
}
