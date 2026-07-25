package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"github.com/rechedev9/fragforge/internal/artifacts"
	"github.com/rechedev9/fragforge/internal/job"
	"github.com/rechedev9/fragforge/internal/tasks"
)

func TestTacticalWorkerSkipsScanWhenArtifactsExist(t *testing.T) {
	id := uuid.New()
	repo := newFakeJobRepo(job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/missing.dem"})
	store := newFakeStorage()
	// Both durable artifacts are already present; the demo deliberately is not,
	// so a worker that rescanned instead of skipping would fail.
	store.files[artifacts.TacticalIndexKey(id)] = []byte(`{"schema_version":"1.0"}`)
	store.files[artifacts.TacticalPositionsKey(id)] = []byte("ZVPOS1")

	w := NewTacticalWorker(repo, store)
	payload, _ := json.Marshal(tasks.AnalyzeTacticalPayload{JobID: id})
	if err := w.HandleAnalyzeTactical(context.Background(), asynq.NewTask(tasks.TypeAnalyzeTactical, payload)); err != nil {
		t.Fatalf("HandleAnalyzeTactical error = %v", err)
	}

	if _, ok := store.files[artifacts.TacticalStatusKey(id)]; ok {
		t.Error("status artifact written, want the retry to skip without touching the analysis")
	}
	if got := repo.jobs[id].Status; got != job.StatusParsed {
		t.Errorf("Status = %v, want the job left at StatusParsed", got)
	}
}

func TestTacticalWorkerRecordsUnreadableDemoFailure(t *testing.T) {
	id := uuid.New()
	repo := newFakeJobRepo(job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/missing.dem"})
	store := newFakeStorage()

	w := NewTacticalWorker(repo, store)
	err := w.ProcessAnalyzeTactical(context.Background(), id, 0)
	if err == nil {
		t.Fatal("ProcessAnalyzeTactical error = nil, want a failure for the missing demo")
	}
	if got := tacticalFailureClass(err); got != tacticalClassDemoUnreadable {
		t.Errorf("failure class = %q, want %q", got, tacticalClassDemoUnreadable)
	}
	// The analysis is an optional artifact: a failed scan must not flip a job
	// whose parse (or whose finished video) already succeeded.
	if got := repo.jobs[id].Status; got != job.StatusParsed {
		t.Errorf("Status = %v, want the job left at StatusParsed", got)
	}

	raw, ok := store.files[artifacts.TacticalStatusKey(id)]
	if !ok {
		t.Fatal("status artifact missing after a failed scan")
	}
	var status artifacts.TacticalStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if status.State != artifacts.TacticalStateFailed {
		t.Errorf("state = %q, want %q", status.State, artifacts.TacticalStateFailed)
	}
	if status.Error == "" {
		t.Error("status error empty, want the scan failure reason")
	}
	if _, ok := store.files[artifacts.TacticalIndexKey(id)]; ok {
		t.Error("tactical document written despite a failed scan")
	}
}

func TestTacticalWorkerLeavesTheJobStatusAloneWhileRetriesRemain(t *testing.T) {
	id := uuid.New()
	repo := newFakeJobRepo(job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/missing.dem"})
	store := newFakeStorage()

	// The scan is idempotent, so it stays retryable; either way the job's own
	// status is never written by this worker.
	ctx := tasks.WithTaskAttempt(context.Background(), 0, 2)
	err := NewTacticalWorker(repo, store).ProcessAnalyzeTactical(ctx, id, 0)
	if err == nil {
		t.Fatal("ProcessAnalyzeTactical error = nil, want a failure for the missing demo")
	}
	if got := repo.jobs[id].Status; got != job.StatusParsed {
		t.Errorf("Status = %v, want the job left at StatusParsed until retries are exhausted", got)
	}
}

func TestTacticalFailureClass(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"wrapped class", fmt.Errorf("store: %w", newTacticalFailure(tacticalClassWriteArtifact, fmt.Errorf("disk full"))), tacticalClassWriteArtifact},
		{"uncalibrated map", newTacticalFailure(tacticalClassMapUncalibrated, fmt.Errorf("no calibration")), tacticalClassMapUncalibrated},
		{"invalid file type", fmt.Errorf("scan tactical: %w", demoinfocs.ErrInvalidFileType), tacticalClassDemoIncompatible},
		{"anything else", fmt.Errorf("parsing demo: corrupt frame"), tacticalClassDemoUnreadable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tacticalFailureClass(tc.err); got != tc.want {
				t.Fatalf("tacticalFailureClass = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTacticalWorkerRunsAgainstRealDemo(t *testing.T) {
	demo := loadRealDemo(t)
	id := uuid.New()
	repo := newFakeJobRepo(job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/test.dem", DemoSHA256: "fake"})
	store := newFakeStorage()
	store.files["demos/test.dem"] = demo

	w := NewTacticalWorker(repo, store)
	if err := w.ProcessAnalyzeTactical(context.Background(), id, 0); err != nil {
		t.Fatalf("ProcessAnalyzeTactical error = %v", err)
	}

	raw, ok := store.files[artifacts.TacticalIndexKey(id)]
	if !ok {
		t.Fatal("tactical document missing after a successful scan")
	}
	var doc struct {
		SchemaVersion string `json:"schema_version"`
		Rounds        []struct {
			Number int `json:"number"`
		} `json:"rounds"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Rounds) == 0 {
		t.Error("document has no rounds, expected > 0 (regression)")
	}
	if _, ok := store.files[artifacts.TacticalPositionsKey(id)]; !ok {
		t.Error("position blob missing after a successful scan")
	}
	var status artifacts.TacticalStatus
	if err := json.Unmarshal(store.files[artifacts.TacticalStatusKey(id)], &status); err != nil {
		t.Fatal(err)
	}
	if status.State != artifacts.TacticalStateReady {
		t.Errorf("state = %q, want %q", status.State, artifacts.TacticalStateReady)
	}
	if status.SchemaVersion != doc.SchemaVersion {
		t.Errorf("status schema version = %q, want the document's %q", status.SchemaVersion, doc.SchemaVersion)
	}

	// A second run must be a no-op skip rather than a rescan.
	before := len(store.files[artifacts.TacticalPositionsKey(id)])
	store.files[artifacts.TacticalStatusKey(id)] = []byte(`{"state":"sentinel"}`)
	if err := w.ProcessAnalyzeTactical(context.Background(), id, 0); err != nil {
		t.Fatalf("second ProcessAnalyzeTactical error = %v", err)
	}
	if !bytes.Equal(store.files[artifacts.TacticalStatusKey(id)], []byte(`{"state":"sentinel"}`)) {
		t.Error("status rewritten on retry, want the completed artifacts to short-circuit the scan")
	}
	if got := len(store.files[artifacts.TacticalPositionsKey(id)]); got != before {
		t.Errorf("position blob size = %d, want the existing %d", got, before)
	}
}
