package obs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestRecordErrorJobIDAndClassAreQueryableWithoutMessage(t *testing.T) {
	r, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cases := []struct {
		name    string
		jobID   string
		class   string
		task    string
		message string
	}{
		{
			name:    "missing_plate",
			jobID:   "11111111-1111-1111-1111-111111111111",
			class:   ClassMissingPlate,
			task:    "render:stream-clip",
			message: `composite keydrop banner code "HUASO": keydrop banner style "jcorko" plate is missing`,
		},
		{
			name:    "capture_flake",
			jobID:   "22222222-2222-2222-2222-222222222222",
			class:   ClassCaptureFlake,
			task:    "record:demo",
			message: "capture POV verification failed: observer target 76561198000000000 drifted from 76561198000000001 during seg-001",
		},
	}
	for _, tc := range cases {
		if err := r.RecordError(Event{
			JobID:   tc.jobID,
			Stage:   StageWorker,
			Task:    tc.task,
			Class:   tc.class,
			Message: tc.message,
		}); err != nil {
			t.Fatalf("%s RecordError: %v", tc.name, err)
		}
	}

	raw, err := os.ReadFile(r.JournalPath())
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	events, err := ReadJournal(r.JournalPath())
	if err != nil {
		t.Fatalf("ReadJournal: %v", err)
	}
	if got, want := len(events), 2; got != want {
		t.Fatalf("journal events: got %d want %d", got, want)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			found := Select(events, tc.jobID, tc.class)
			if len(found) != 1 {
				t.Fatalf("Select(%q, %q) = %#v, want exactly one event", tc.jobID, tc.class, found)
			}
			got := found[0]
			if got.JobID != tc.jobID || got.Class != tc.class || got.Task != tc.task {
				t.Fatalf("selected event = %+v", got)
			}
			if got.Message != tc.message {
				t.Fatalf("message = %q, want original human text", got.Message)
			}
			lookup, err := r.SelectErrors(tc.jobID, tc.class)
			if err != nil {
				t.Fatalf("SelectErrors: %v", err)
			}
			if len(lookup) != 1 || lookup[0].JobID != tc.jobID || lookup[0].Class != tc.class {
				t.Fatalf("SelectErrors = %#v", lookup)
			}
		})
	}

	if Select(events, cases[0].jobID, ClassCaptureFlake) != nil {
		t.Fatal("Select matched across job_id and class")
	}
	if strings.Contains(string(raw), `"job_id":"`+cases[0].jobID+`"`) &&
		!jsonLineHasJobAndClass(t, raw, cases[0].jobID, ClassMissingPlate) {
		t.Fatal("journal line missing job_id+class fields")
	}
}

func jsonLineHasJobAndClass(t *testing.T, raw []byte, jobID, class string) bool {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		if ev.JobID == jobID && ev.Class == class {
			return true
		}
	}
	return false
}

func TestClassOfTable(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "missing_plate",
			message: `composite keydrop banner code "HUASO": keydrop banner style "jcorko" plate is missing`,
			want:    ClassMissingPlate,
		},
		{
			name:    "observer_drift",
			message: "recorder failed: observer target 76561198000000000 drifted from 76561198000000001 during seg-001",
			want:    ClassCaptureFlake,
		},
		{
			name:    "observer_mismatch",
			message: "observer target 76561198000000000 does not match expected 76561198000000001",
			want:    ClassCaptureFlake,
		},
		{
			name:    "demo_incompatible_prefix",
			message: "demo_incompatible: cs2 cannot replay this demo",
			want:    ClassDemoIncompatible,
		},
		{
			name:    "unplayable_start_prefix",
			message: "unplayable_start: CS2 crashed rewinding playdemo to tick 0",
			want:    ClassUnplayableStart,
		},
		{
			name:    "generic_stays_empty",
			message: "ffmpeg exited with code 1",
			want:    "",
		},
		{
			name:    "empty",
			message: "",
			want:    "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassOf(tc.message); got != tc.want {
				t.Fatalf("ClassOf(%q) = %q, want %q", tc.message, got, tc.want)
			}
		})
	}
}

func TestRecordErrorWritesJournalAndCounters(t *testing.T) {
	dir := t.TempDir()
	r, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if runtime.GOOS != "windows" {
		if info, statErr := os.Stat(dir); statErr != nil {
			t.Fatalf("stat obs dir: %v", statErr)
		} else if got, want := info.Mode().Perm(), os.FileMode(0o700); got != want {
			t.Errorf("obs dir permissions: got %o want %o", got, want)
		}
	}

	if err := r.RecordError(Event{Stage: StageParse, Class: "target_not_found", Message: "no such player", Demo: "a.dem", Target: "76561198000000000"}); err != nil {
		t.Fatalf("RecordError: %v", err)
	}
	if err := r.RecordError(Event{Stage: StageParse, Class: "target_not_found", Message: "again", Demo: "b.dem"}); err != nil {
		t.Fatalf("RecordError: %v", err)
	}
	if err := r.RecordSuccess(StageParse); err != nil {
		t.Fatalf("RecordSuccess: %v", err)
	}

	events := readJournal(t, r.JournalPath())
	if got, want := len(events), 2; got != want {
		t.Fatalf("journal lines: got %d want %d", got, want)
	}
	if events[0].Demo != "a.dem" || events[0].Class != "target_not_found" {
		t.Errorf("first event: got %+v", events[0])
	}
	if events[0].Time.IsZero() {
		t.Errorf("first event time not set: %+v", events[0])
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(r.JournalPath()); err != nil {
			t.Fatalf("stat journal: %v", err)
		} else if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
			t.Errorf("journal permissions: got %o want %o", got, want)
		}
	}

	want := map[string]int64{
		`CLIPHUB_errors_total{class="target_not_found",stage="parse"}`: 2,
		`CLIPHUB_stage_runs_total{result="error",stage="parse"}`:       2,
		`CLIPHUB_stage_runs_total{result="ok",stage="parse"}`:          1,
	}
	got := counterMap(r.Snapshot())
	for k, v := range want {
		if got[k] != v {
			t.Errorf("counter %s: got %d want %d", k, got[k], v)
		}
	}
}

func TestRecordErrorRestrictsExistingJournalPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission bits")
	}
	dir := t.TempDir()
	journalPath := filepath.Join(dir, "journal.jsonl")
	if err := os.WriteFile(journalPath, nil, 0o644); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	r, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := r.RecordError(Event{Stage: StageParse, Class: "test", Message: "test"}); err != nil {
		t.Fatalf("RecordError: %v", err)
	}
	info, err := os.Stat(journalPath)
	if err != nil {
		t.Fatalf("stat journal: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Errorf("journal permissions: got %o want %o", got, want)
	}
}

func TestRecordErrorDoesNotCountAnEventTheJournalRejected(t *testing.T) {
	dir := t.TempDir()
	r, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// A directory at the journal path deterministically makes OpenFile fail
	// without relying on platform-specific permission semantics.
	if err := os.Mkdir(r.JournalPath(), 0o700); err != nil {
		t.Fatalf("block journal path: %v", err)
	}

	err = r.RecordError(Event{Stage: StageRender, Class: "journal_blocked", Message: "boom"})
	if err == nil {
		t.Fatal("RecordError error = nil, want journal failure")
	}
	if got := r.Snapshot(); len(got) != 0 {
		t.Fatalf("Snapshot = %#v, want no ghost counters for an unjournaled event", got)
	}
}

func TestInitializeDefaultPreservesAndReportsInitializationFailure(t *testing.T) {
	// This package owns the singleton and can reset it for a deterministic
	// startup-boundary test. No other obs test calls Default concurrently.
	defaultOnce = sync.Once{}
	defaultRec = nil
	defaultErr = nil
	t.Cleanup(func() {
		defaultOnce = sync.Once{}
		defaultRec = nil
		defaultErr = nil
	})

	dataDir := t.TempDir()
	t.Setenv("ZV_DATA_DIR", dataDir)
	if err := os.WriteFile(filepath.Join(dataDir, "obs"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("block obs directory: %v", err)
	}

	rec, err := InitializeDefault()
	if err == nil || rec != nil {
		t.Fatalf("InitializeDefault = (%v, %v), want explicit creation failure", rec, err)
	}
	again, againErr := InitializeDefault()
	if again != nil || againErr == nil || againErr.Error() != err.Error() {
		t.Fatalf("second InitializeDefault = (%v, %v), want stable failure %v", again, againErr, err)
	}
	if Default() != nil {
		t.Fatal("Default returned a recorder after initialization failed")
	}
}

func TestRecordErrorDefaultsStageAndClass(t *testing.T) {
	r, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := r.RecordError(Event{Message: "boom"}); err != nil {
		t.Fatalf("RecordError: %v", err)
	}
	got := counterMap(r.Snapshot())
	if got[`CLIPHUB_errors_total{class="unknown",stage="unknown"}`] != 1 {
		t.Errorf("expected unknown/unknown counter, got %v", got)
	}
}

func TestCountersPersistAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	r1, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := r1.RecordError(Event{Stage: StageRender, Class: "ffmpeg_failed", Message: "x"}); err != nil {
		t.Fatalf("RecordError: %v", err)
	}

	r2, err := New(dir)
	if err != nil {
		t.Fatalf("re-New: %v", err)
	}
	if err := r2.RecordError(Event{Stage: StageRender, Class: "ffmpeg_failed", Message: "y"}); err != nil {
		t.Fatalf("RecordError: %v", err)
	}
	got := counterMap(r2.Snapshot())
	if got[`CLIPHUB_errors_total{class="ffmpeg_failed",stage="render"}`] != 2 {
		t.Errorf("counter did not accumulate across reopen: %v", got)
	}
}

func TestWritePrometheusFormat(t *testing.T) {
	r, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := r.RecordError(Event{Stage: StageParse, Class: "corrupt", Message: "x"}); err != nil {
		t.Fatalf("RecordError: %v", err)
	}
	var b strings.Builder
	WritePrometheus(&b, r.Snapshot())
	out := b.String()
	for _, want := range []string{
		"# HELP CLIPHUB_errors_total",
		"# TYPE CLIPHUB_errors_total counter",
		`CLIPHUB_errors_total{class="corrupt",stage="parse"} 1`,
		"# TYPE CLIPHUB_stage_runs_total counter",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prometheus output missing %q\n---\n%s", want, out)
		}
	}
}

func TestRecordSpanWritesPrivacyBoundedJournal(t *testing.T) {
	r, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := r.RecordSpan(Span{Stage: StageWorker, Name: "render:variant", Result: "ok", DurationMS: 1250}); err != nil {
		t.Fatalf("RecordSpan: %v", err)
	}
	b, err := os.ReadFile(r.SpansPath())
	if err != nil {
		t.Fatalf("read spans: %v", err)
	}
	var span Span
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(b))), &span); err != nil {
		t.Fatalf("unmarshal span: %v", err)
	}
	if span.Stage != StageWorker || span.Name != "render:variant" || span.Result != "ok" || span.DurationMS != 1250 || span.Time.IsZero() {
		t.Fatalf("span = %+v", span)
	}
	if strings.Contains(string(b), "message") || strings.Contains(string(b), "demo") {
		t.Fatalf("span journal crossed privacy boundary: %s", b)
	}
}

func TestRotateFileAtSizeKeepsFourBoundedPreviousJournals(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spans.jsonl")
	for _, value := range []string{"first", "second", "third", "fourth", "fifth"} {
		if err := os.WriteFile(path, []byte(value+"-journal"), 0o600); err != nil {
			t.Fatalf("seed spans: %v", err)
		}
		if err := rotateFileAtSize(path, 4); err != nil {
			t.Fatalf("rotateFileAtSize: %v", err)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("current journal still exists: %v", err)
	}
	for generation, want := range map[int]string{1: "fifth-journal", 2: "fourth-journal", 3: "third-journal", 4: "second-journal"} {
		contents, err := os.ReadFile(fmt.Sprintf("%s.%d", path, generation))
		if err != nil {
			t.Fatalf("read generation %d: %v", generation, err)
		}
		if string(contents) != want {
			t.Fatalf("generation %d = %q, want %q", generation, contents, want)
		}
	}
}

func TestMetricsPromFileWritten(t *testing.T) {
	r, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := r.RecordSuccess(StageCompose); err != nil {
		t.Fatalf("RecordSuccess: %v", err)
	}
	b, err := os.ReadFile(r.MetricsPromPath())
	if err != nil {
		t.Fatalf("read prom file: %v", err)
	}
	if !strings.Contains(string(b), `CLIPHUB_stage_runs_total{result="ok",stage="compose"} 1`) {
		t.Errorf("prom file missing expected series:\n%s", b)
	}
}

func readJournal(t *testing.T, path string) []Event {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	defer f.Close()
	var events []Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("unmarshal journal line %q: %v", line, err)
		}
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan journal: %v", err)
	}
	return events
}

func counterMap(metrics []Metric) map[string]int64 {
	m := map[string]int64{}
	for _, metric := range metrics {
		m[seriesKey(metric.Name, metric.Labels)] = metric.Value
	}
	return m
}
