package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/tactical"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func TestTacticalWorkerSkipsScanWhenArtifactsExist(t *testing.T) {
	id := uuid.New()
	repo := newFakeJobRepo(job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/missing.dem"})
	store := newFakeStorage()
	// Both durable artifacts are already present; the demo deliberately is not,
	// so a worker that rescanned instead of skipping would fail.
	store.files[artifacts.TacticalIndexKey(id)] = []byte(`{"schema_version":"1.0"}`)
	store.files[artifacts.TacticalPositionsKey(id)] = []byte("ZVPOS1")
	statusRaw, err := json.Marshal(artifacts.TacticalStatus{
		State:         artifacts.TacticalStateReady,
		SchemaVersion: tacticalplan.SchemaVersion,
		SampleHZ:      tactical.DefaultSampleHZ,
	})
	if err != nil {
		t.Fatal(err)
	}
	store.files[artifacts.TacticalStatusKey(id)] = statusRaw

	w := NewTacticalWorker(repo, store)
	payload, _ := json.Marshal(tasks.AnalyzeTacticalPayload{JobID: id})
	if err := w.HandleAnalyzeTactical(context.Background(), asynq.NewTask(tasks.TypeAnalyzeTactical, payload)); err != nil {
		t.Fatalf("HandleAnalyzeTactical error = %v", err)
	}

	if !bytes.Equal(store.files[artifacts.TacticalStatusKey(id)], statusRaw) {
		t.Error("status artifact changed, want the retry to skip without touching the analysis")
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

func TestTacticalWorkerRejectsNonCanonicalSampleRate(t *testing.T) {
	id := uuid.New()
	repo := newFakeJobRepo(job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/missing.dem"})
	store := newFakeStorage()
	store.files[artifacts.TacticalIndexKey(id)] = []byte(`{"schema_version":"1.0"}`)
	store.files[artifacts.TacticalPositionsKey(id)] = []byte("ZVPOS1")

	err := NewTacticalWorker(repo, store).ProcessAnalyzeTactical(context.Background(), id, 20)
	if err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("ProcessAnalyzeTactical error = %v, want canonical sample-rate rejection", err)
	}
	if _, exists := store.files[artifacts.TacticalStatusKey(id)]; exists {
		t.Fatal("noncanonical task wrote tactical status")
	}
}

func TestTacticalArtifactsRequireCanonicalReadyStatus(t *testing.T) {
	id := uuid.New()
	store := newFakeStorage()
	indexKey := artifacts.TacticalIndexKey(id)
	positionsKey := artifacts.TacticalPositionsKey(id)
	statusKey := artifacts.TacticalStatusKey(id)
	store.files[indexKey] = []byte(`{"schema_version":"1.0"}`)
	store.files[positionsKey] = []byte("ZVPOS1")
	w := NewTacticalWorker(nil, store)

	ready, err := w.artifactsReady(statusKey, tactical.DefaultSampleHZ, indexKey, positionsKey)
	if err != nil || ready {
		t.Fatalf("artifactsReady without status = (%v, %v), want false, nil", ready, err)
	}

	for name, status := range map[string]artifacts.TacticalStatus{
		"running": {
			State:         artifacts.TacticalStateRunning,
			SchemaVersion: tacticalplan.SchemaVersion,
			SampleHZ:      tactical.DefaultSampleHZ,
		},
		"old schema": {
			State:         artifacts.TacticalStateReady,
			SchemaVersion: "old",
			SampleHZ:      tactical.DefaultSampleHZ,
		},
		"old sample rate": {
			State:         artifacts.TacticalStateReady,
			SchemaVersion: tacticalplan.SchemaVersion,
			SampleHZ:      tactical.DefaultSampleHZ / 2,
		},
	} {
		t.Run(name, func(t *testing.T) {
			raw, marshalErr := json.Marshal(status)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			store.files[statusKey] = raw
			ready, readyErr := w.artifactsReady(statusKey, tactical.DefaultSampleHZ, indexKey, positionsKey)
			if readyErr != nil || ready {
				t.Fatalf("artifactsReady = (%v, %v), want false, nil", ready, readyErr)
			}
		})
	}
}

// The scan is idempotent, so it stays retryable: only the terminal attempt
// journals the failure, and the job's own status is never written either way.
func TestTacticalWorkerJournalsOnlyTheTerminalFailure(t *testing.T) {
	for _, tc := range []struct {
		name        string
		retried     int
		wantJournal int
	}{
		{name: "retries remain", retried: 0, wantJournal: 0},
		{name: "retries exhausted", retried: 2, wantJournal: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			repo := newFakeJobRepo(job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/missing.dem"})
			store := newFakeStorage()

			ctx := tasks.WithTaskAttempt(context.Background(), tc.retried, 2)
			err := NewTacticalWorker(repo, store).ProcessAnalyzeTactical(ctx, id, 0)
			if err == nil {
				t.Fatal("ProcessAnalyzeTactical error = nil, want a failure for the missing demo")
			}
			if got := repo.jobs[id].Status; got != job.StatusParsed {
				t.Errorf("Status = %v, want the job left at StatusParsed", got)
			}
			events := journalEventsFor(t, obs.Default(), id.String(), tacticalClassDemoUnreadable)
			if len(events) != tc.wantJournal {
				t.Fatalf("journal events = %#v, want %d", events, tc.wantJournal)
			}
			if tc.wantJournal == 1 && (events[0].Stage != obs.StageTactical || events[0].Task != tasks.TypeAnalyzeTactical) {
				t.Fatalf("journal event = %+v, want stage %q task %q", events[0], obs.StageTactical, tasks.TypeAnalyzeTactical)
			}
		})
	}
}

// A demo the parser rejects fails the same way on a retry, so the first attempt journals it and ends the task.
func TestTacticalWorkerEndsAnIncompatibleDemoOnTheFirstAttempt(t *testing.T) {
	id := uuid.New()
	repo := newFakeJobRepo(job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/incompatible.dem"})
	store := newFakeStorage()
	store.files["demos/incompatible.dem"] = incompatibleDemo()

	err := NewTacticalWorker(repo, store).ProcessAnalyzeTactical(tasks.WithTaskAttempt(context.Background(), 0, 2), id, 0)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessAnalyzeTactical error = %v, want it to wrap asynq.SkipRetry", err)
	}
	if events := journalEventsFor(t, obs.Default(), id.String(), tacticalClassDemoIncompatible); len(events) != 1 {
		t.Fatalf("journal events = %#v, want the one terminal failure", events)
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
		{"wrapped parser error", fmt.Errorf("scan tactical: parsing demo: demo_incompatible: nextByte: insufficient buffer (701 of 701)"), tacticalClassDemoIncompatible},
		{"wrapped truncated demo", fmt.Errorf("scan tactical: parsing demo: truncated_demo: cut before the first round"), obs.ClassTruncatedDemo},
		{"positions encoder", fmt.Errorf("scan tactical: %w", relabelled{"blob refused", tactical.ErrEncodePositions}), tacticalClassInternal},
		{"stored positions encoder message", errors.New("scan tactical: encode positions: slot 1 appears twice in the frame at tick 59020"), tacticalClassInternal},
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

// relabelled hides an error's text behind another message, so only errors.Is can find it.
type relabelled struct {
	msg string
	err error
}

func (r relabelled) Error() string { return r.msg }

func (r relabelled) Unwrap() error { return r.err }

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

// A read error from the OS may clear on the next attempt; a demo the parser rejects never does.
func TestTacticalScanRetriesAReadErrorAndEndsAFormatError(t *testing.T) {
	readErr := &fs.PathError{Op: "read", Path: "demo.dem", Err: errors.New("the process cannot access the file")}
	for _, tc := range []struct {
		name  string
		demo  io.Reader
		class string
		final bool
	}{
		{"read error", io.MultiReader(bytes.NewReader(incompatibleDemo()[:16]), failingReader{readErr}), tacticalClassDemoUnreadable, false},
		{"read error on the first read", failingReader{readErr}, tacticalClassDemoUnreadable, false},
		{"empty demo", bytes.NewReader(nil), obs.ClassTruncatedDemo, true},
		{"format error", bytes.NewReader(incompatibleDemo()), obs.ClassDemoIncompatible, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tactical.ScanReader(context.Background(), tc.demo, tactical.Options{})
			if err == nil {
				t.Fatal("ScanReader error = nil, want a failure")
			}
			class := tacticalFailureClass(fmt.Errorf("scan tactical: %w", err))
			if class != tc.class {
				t.Fatalf("class of %v = %q, want %q", err, class, tc.class)
			}
			if got := tacticalClassFinal(class); got != tc.final {
				t.Fatalf("class %q of %v: final = %v, want %v", class, err, got, tc.final)
			}
			if !tc.final && !errors.Is(err, readErr) {
				t.Fatalf("ScanReader error = %v, want the read error on the chain", err)
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

	// A second run with the same ready status must be a no-op skip rather than a
	// rescan.
	before := len(store.files[artifacts.TacticalPositionsKey(id)])
	statusBefore := append([]byte(nil), store.files[artifacts.TacticalStatusKey(id)]...)
	if err := w.ProcessAnalyzeTactical(context.Background(), id, 0); err != nil {
		t.Fatalf("second ProcessAnalyzeTactical error = %v", err)
	}
	if !bytes.Equal(store.files[artifacts.TacticalStatusKey(id)], statusBefore) {
		t.Error("status rewritten on retry, want the completed artifacts to short-circuit the scan")
	}
	if got := len(store.files[artifacts.TacticalPositionsKey(id)]); got != before {
		t.Errorf("position blob size = %d, want the existing %d", got, before)
	}
}
