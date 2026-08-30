package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/anticheat"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/jobprogress"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func putProgressSnapshot(t *testing.T, store storage.Storage, key string, stage, unit, label string, done, total int64) {
	t.Helper()
	snap, err := jobprogress.NewSnapshot(stage, unit, label, done, total, time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := snap.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(key, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
}

func TestGetJobStatusReportsParseProgress(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusParsing}
	putProgressSnapshot(t, store, artifacts.ProgressKey(id), jobprogress.StageParse, jobprogress.UnitTicks, "ticks", 64000, 172772)

	h := NewHandlers(repo, store, &fakeQueue{})
	router := chi.NewRouter()
	router.Get("/api/jobs/{id}", h.GetJob)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"?view=status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var got jobStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Progress == nil || got.Progress.Done != 64000 || got.Progress.Total != 172772 || got.Progress.Percent != 37 {
		t.Fatalf("progress = %+v, want 64000/172772 37%%", got.Progress)
	}
	if got.Progress.Unit != "ticks" || got.Progress.Label != "ticks" || got.Progress.Stage != "parse" {
		t.Fatalf("progress labels = %+v", got.Progress)
	}
}

func TestGetJobStatusOmitsStaleParseProgressAfterParsed(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusParsed}
	putProgressSnapshot(t, store, artifacts.ProgressKey(id), jobprogress.StageParse, jobprogress.UnitTicks, "ticks", 172772, 172772)

	h := NewHandlers(repo, store, &fakeQueue{})
	router := chi.NewRouter()
	router.Get("/api/jobs/{id}", h.GetJob)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"?view=status", nil))
	var got jobStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Progress != nil {
		t.Fatalf("stale parse progress leaked onto parsed job: %+v", got.Progress)
	}
}

func TestGetAnticheatIncludesSideLaneProgress(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := scannedJob()
	repo.jobs[j.ID] = j
	putAnticheat(t, store, j.ID, anticheat.NewRunningDocument(j.ID.String(), time.Now()))
	putProgressSnapshot(t, store, artifacts.AnticheatProgressKey(j.ID), jobprogress.StageAnticheat, jobprogress.UnitTicks, "ticks", 1000, 4000)

	h := NewHandlers(repo, store, &fakeQueue{})
	rw := httptest.NewRecorder()
	anticheatRouter(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/anticheat", nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rw.Code, rw.Body.String())
	}
	var got anticheatResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != anticheat.StatusRunning {
		t.Fatalf("status = %s, want running", got.Status)
	}
	if got.Progress == nil || got.Progress.Done != 1000 || got.Progress.Total != 4000 || got.Progress.Percent != 25 {
		t.Fatalf("progress = %+v, want 1000/4000 25%%", got.Progress)
	}
}

func TestGetJobStatusReportsComposeProgress(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusComposing}
	putProgressSnapshot(t, store, artifacts.ProgressKey(id), jobprogress.StageCompose, jobprogress.UnitClips, "clips", 0, 8)

	h := NewHandlers(repo, store, &fakeQueue{})
	router := chi.NewRouter()
	router.Get("/api/jobs/{id}", h.GetJob)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"?view=status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var got jobStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Progress == nil || got.Progress.Done != 0 || got.Progress.Total != 8 || got.Progress.Percent != 0 {
		t.Fatalf("progress = %+v, want 0/8 0%%", got.Progress)
	}
	if got.Progress.Unit != "clips" || got.Progress.Stage != "compose" {
		t.Fatalf("progress labels = %+v", got.Progress)
	}
}

func TestGetTacticalStatusIncludesSideLaneProgress(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := tacticalTestJob(repo)
	if err := store.Put(artifacts.TacticalStatusKey(id), bytes.NewReader([]byte(
		`{"state":"running","generated_at":"2026-08-30T00:00:00Z","schema_version":"`+tacticalplan.SchemaVersion+`","sample_hz":8}`+"\n",
	))); err != nil {
		t.Fatal(err)
	}
	putProgressSnapshot(t, store, artifacts.TacticalProgressKey(id), jobprogress.StageTactical, jobprogress.UnitTicks, "ticks", 8000, 20000)

	h := NewHandlers(repo, store, &fakeQueue{})
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"/tactical/status", nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rw.Code, rw.Body.String())
	}
	var got tacticalStatusResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.State != artifacts.TacticalStateRunning {
		t.Fatalf("state = %s, want running", got.State)
	}
	if got.Progress == nil || got.Progress.Done != 8000 || got.Progress.Total != 20000 {
		t.Fatalf("progress = %+v, want 8000/20000", got.Progress)
	}
}
