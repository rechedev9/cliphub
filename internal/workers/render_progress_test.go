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

	ctx, cancel := context.WithCancel(context.Background())
	go reporter.watch(ctx)

	tracker := editor.NewProgressTracker(progressPath)
	tracker.Flush("Montando cortes y ritmo", 88)
	time.Sleep(1200 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)

	rc, err := store.Open(artifacts.RenderProgressKey(jobID))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	var got editor.EditorProgress
	if err := json.NewDecoder(rc).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Percent != 88 {
		t.Fatalf("percent = %d, want 88", got.Percent)
	}
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
