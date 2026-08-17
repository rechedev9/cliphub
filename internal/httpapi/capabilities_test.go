package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
)

func TestGetCapabilitiesReportsPerToolStatus(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "recorder.exe")
	// LookPath / ExecutableFile require the execute bit on Unix; Windows ignores mode.
	if err := os.WriteFile(present, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	caps := Capabilities{
		RecordEnabled: true,
		RecordTools: []CaptureTool{
			{Name: "ZV_RECORDER_PATH", Path: present},                       // configured + accessible
			{Name: "ZV_HLAE_PATH", Path: filepath.Join(dir, "missing.exe")}, // configured, not accessible
			{Name: "ZV_CS2_PATH", Path: ""},                                 // unset
		},
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithCapabilities(caps))

	rw := httptest.NewRecorder()
	h.GetCapabilities(rw, httptest.NewRequest(http.MethodGet, "/api/capabilities", nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rw.Code)
	}

	var got struct {
		Record struct {
			Enabled bool          `json:"enabled"`
			Tools   []CaptureTool `json:"tools"`
		} `json:"record"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Record.Enabled {
		t.Error("record.enabled = false, want true")
	}
	var raw struct {
		Stream map[string]json.RawMessage `json:"stream"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw capabilities: %v", err)
	}
	for _, removed := range []string{"groq_enabled", "whisper_enabled", "xai_enabled"} {
		if _, ok := raw.Stream[removed]; ok {
			t.Errorf("stream capabilities still report removed field %q", removed)
		}
	}
	want := map[string][2]bool{ // [configured, accessible]
		"ZV_RECORDER_PATH": {true, true},
		"ZV_HLAE_PATH":     {true, false},
		"ZV_CS2_PATH":      {false, false},
	}
	for _, tool := range got.Record.Tools {
		w, ok := want[tool.Name]
		if !ok {
			t.Errorf("unexpected tool %q", tool.Name)
			continue
		}
		if got := [2]bool{tool.Configured, tool.Accessible}; got != w {
			t.Errorf("%s: got configured/accessible %v, want %v", tool.Name, got, w)
		}
	}
}

func TestGetCapabilitiesDoesNotReportADirectoryAsAnAccessibleTool(t *testing.T) {
	dir := t.TempDir()
	caps := Capabilities{
		RecordTools: []CaptureTool{{Name: "ZV_RECORDER_PATH", Path: dir}},
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithCapabilities(caps))
	rw := httptest.NewRecorder()
	h.GetCapabilities(rw, httptest.NewRequest(http.MethodGet, "/api/capabilities", nil))

	var got struct {
		Record struct {
			Tools []CaptureTool `json:"tools"`
		} `json:"record"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Record.Tools) != 1 || !got.Record.Tools[0].Configured || got.Record.Tools[0].Accessible {
		t.Fatalf("directory tool status = %#v, want configured but inaccessible", got.Record.Tools)
	}
}

func TestGetCapabilitiesResolvesPATHBasename(t *testing.T) {
	dir := t.TempDir()
	executableName := "recorder"
	if runtime.GOOS == "windows" {
		executableName += ".exe"
		t.Setenv("PATHEXT", ".EXE")
	}
	executable := filepath.Join(dir, executableName)
	if err := os.WriteFile(executable, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	h := NewHandlers(
		newFakeRepo(),
		newFakeStorage(),
		&fakeQueue{},
		WithCapabilities(Capabilities{
			RecordTools: []CaptureTool{{Name: "ZV_RECORDER_PATH", Path: "recorder"}},
		}),
	)

	rw := httptest.NewRecorder()
	h.GetCapabilities(rw, httptest.NewRequest(http.MethodGet, "/api/capabilities", nil))

	var got struct {
		Record struct {
			Tools []CaptureTool `json:"tools"`
		} `json:"record"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Record.Tools) != 1 {
		t.Fatalf("tools = %#v, want one", got.Record.Tools)
	}
	tool := got.Record.Tools[0]
	resolvedInfo, resolvedErr := os.Stat(tool.Path)
	executableInfo, executableErr := os.Stat(executable)
	if !tool.Configured ||
		!tool.Accessible ||
		resolvedErr != nil ||
		executableErr != nil ||
		!os.SameFile(resolvedInfo, executableInfo) {
		t.Fatalf("PATH tool = %#v, want configured accessible path %q", tool, executable)
	}
}

func TestGetCapabilitiesReportsReadAuthentication(t *testing.T) {
	h := NewHandlers(
		newFakeRepo(),
		newFakeStorage(),
		&fakeQueue{},
		WithMutationToken("local-secret"),
		WithRequireReadAuth(true),
	)
	rw := httptest.NewRecorder()
	h.GetCapabilities(rw, httptest.NewRequest(http.MethodGet, "/api/capabilities", nil))

	var got struct {
		Auth struct {
			ReadRequiresToken bool `json:"read_requires_token"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Auth.ReadRequiresToken {
		t.Error("auth.read_requires_token = false, want true")
	}
}

func TestStartRecordingGatesOnCaptureReadiness(t *testing.T) {
	parsedJob := func() (*fakeRepo, uuid.UUID) {
		repo := newFakeRepo()
		id := uuid.New()
		repo.jobs[id] = job.Job{ID: id, Status: job.StatusParsed, KillPlan: &killplan.Plan{}}
		return repo, id
	}

	t.Run("409 with no orphaned task when capture is unconfigured", func(t *testing.T) {
		repo, id := parsedJob()
		q := &fakeQueue{}
		h := NewHandlers(repo, newFakeStorage(), q) // no WithCapabilities -> RecordEnabled false

		rw := httptest.NewRecorder()
		Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+id.String()+"/record", nil))

		if rw.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", rw.Code)
		}
		if len(q.enqueued) != 0 {
			t.Errorf("enqueued %d tasks, want 0 (no record task should be orphaned)", len(q.enqueued))
		}
	})

	t.Run("202 when capture is configured", func(t *testing.T) {
		repo, id := parsedJob()
		q := &fakeQueue{}
		h := NewHandlers(repo, newFakeStorage(), q, WithCapabilities(Capabilities{RecordEnabled: true}))

		rw := httptest.NewRecorder()
		Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+id.String()+"/record", nil))

		if rw.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202", rw.Code)
		}
		if len(q.enqueued) != 1 {
			t.Errorf("enqueued %d tasks, want 1", len(q.enqueued))
		}
	})

	// A duplicate enqueue (the reconcile loop re-POSTs record until the worker
	// dequeues the unique task) must be a 202, not a 500 - otherwise the web
	// client would flip a reel that is already recording to failed.
	t.Run("202 on duplicate enqueue, not 500", func(t *testing.T) {
		repo, id := parsedJob()
		q := &fakeQueue{err: asynq.ErrDuplicateTask}
		h := NewHandlers(repo, newFakeStorage(), q, WithCapabilities(Capabilities{RecordEnabled: true}))

		rw := httptest.NewRecorder()
		Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+id.String()+"/record", nil))

		if rw.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202 (duplicate enqueue is success)", rw.Code)
		}
	})
}
