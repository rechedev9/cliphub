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
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func putProgressSnapshot(t *testing.T, store storage.Storage, key string, stage, unit, label string, done, total int64) {
	t.Helper()
	snap, err := jobprogress.NewSnapshot(stage, unit, label, done, total, time.Now().UTC())
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

func TestCaptureLabelsUsesInFlightKindNotRecapPlan(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	if err := recapplan.Store(store, id, killplan.Plan{Segments: []killplan.Segment{{ID: "seg-001"}}}); err != nil {
		t.Fatal(err)
	}
	unit, label := captureLabels(store, id)
	if unit != jobprogress.UnitSegments || label != "segmentos" {
		t.Fatalf("shorts labels = %s/%s, want segments/segmentos", unit, label)
	}
	if err := writeCaptureKind(store, id, true); err != nil {
		t.Fatal(err)
	}
	unit, label = captureLabels(store, id)
	if unit != jobprogress.UnitRounds || label != "rondas" {
		t.Fatalf("recap labels = %s/%s, want rounds/rondas", unit, label)
	}
}

func TestGetJobStatusOmitsLeftoverRenderProgressWhileComposing(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusComposing}
	putProgressSnapshot(t, store, artifacts.ProgressKey(id), jobprogress.StageRender, jobprogress.UnitClips, "clips", 8, 8)

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
		t.Fatalf("leftover render progress leaked onto compose: %+v", got.Progress)
	}
}

func TestGetAnticheatOmitsFinishedProgressAfterRestart(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := scannedJob()
	repo.jobs[j.ID] = j
	putAnticheat(t, store, j.ID, anticheat.NewRunningDocument(j.ID.String(), time.Now()))
	old, err := jobprogress.NewSnapshot(jobprogress.StageAnticheat, jobprogress.UnitTicks, "ticks", 4000, 4000, time.Now().Add(-time.Hour).UTC())
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := old.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(artifacts.AnticheatProgressKey(j.ID), bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}

	h := NewHandlers(repo, store, &fakeQueue{})
	rw := httptest.NewRecorder()
	anticheatRouter(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/anticheat", nil))
	var got anticheatResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Progress != nil {
		t.Fatalf("stale 100%% anticheat progress leaked onto a restart: %+v", got.Progress)
	}
}

func TestGetStreamJobOmitsAcquireProgressAfterReady(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	streamRepo := newFakeStreamRepo()
	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusReady, SourcePath: streamclips.SourceKey(id)}
	putProgressSnapshot(t, store, streamclips.ProgressKey(id), jobprogress.StageAcquire, jobprogress.UnitBytes, "bytes", 1000, 1000)

	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(streamRepo))
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+id.String(), nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rw.Code, rw.Body.String())
	}
	var got streamJobResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Progress != nil {
		t.Fatalf("acquire leftover leaked onto ready stream job: %+v", got.Progress)
	}
}

func writeRecordingProgress(t *testing.T, store storage.Storage, id uuid.UUID, ids []string, done int) {
	t.Helper()
	completed := append([]string{}, ids[:done]...)
	progress, err := recording.NewCaptureProgress(uuid.New(), ids, completed, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(progress)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(artifacts.CaptureProgressKey(id), bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
}

func getJobStatusProgress(t *testing.T, repo *fakeRepo, store storage.Storage, id uuid.UUID) *captureProgressView {
	t.Helper()
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
	return got.Progress
}

func TestGetJobStatusRecordingLabelsShortsDespiteStoredRecapPlan(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusRecording}
	if err := recapplan.Store(store, id, killplan.Plan{Segments: []killplan.Segment{{ID: "seg-001"}, {ID: "seg-002"}}}); err != nil {
		t.Fatal(err)
	}
	if err := writeCaptureKind(store, id, false); err != nil {
		t.Fatal(err)
	}
	writeRecordingProgress(t, store, id, []string{"seg-001", "seg-002", "seg-003", "seg-004"}, 1)

	got := getJobStatusProgress(t, repo, store, id)
	if got == nil || got.Label != "segmentos" || got.Unit != jobprogress.UnitSegments {
		t.Fatalf("shorts recording progress = %+v, want segmentos", got)
	}
	if got.Done != 1 || got.Total != 4 {
		t.Fatalf("shorts recording count = %+v, want 1/4", got)
	}
}

func TestGetJobStatusRecordingLabelsFullDemoRounds(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusRecording}
	if err := recapplan.Store(store, id, killplan.Plan{Segments: []killplan.Segment{{ID: "seg-001"}, {ID: "seg-002"}}}); err != nil {
		t.Fatal(err)
	}
	if err := writeCaptureKind(store, id, true); err != nil {
		t.Fatal(err)
	}
	writeRecordingProgress(t, store, id, []string{"seg-001", "seg-002", "seg-003"}, 1)

	got := getJobStatusProgress(t, repo, store, id)
	if got == nil || got.Label != "rondas" || got.Unit != jobprogress.UnitRounds {
		t.Fatalf("full demo recording progress = %+v, want rondas", got)
	}
}

func TestGetStreamRenderHandoffReplacesAcquireBytesWithClips(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	streamRepo := newFakeStreamRepo()
	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusAcquiring, SourcePath: streamclips.SourceKey(id)}
	putProgressSnapshot(t, store, streamclips.ProgressKey(id), jobprogress.StageAcquire, jobprogress.UnitBytes, "bytes", 500, 1000)

	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(streamRepo))
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+id.String(), nil))
	var acquiring streamJobResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &acquiring); err != nil {
		t.Fatal(err)
	}
	if acquiring.Progress == nil || acquiring.Progress.Stage != jobprogress.StageAcquire || acquiring.Progress.Unit != jobprogress.UnitBytes {
		t.Fatalf("acquire poll = %+v, want bytes", acquiring.Progress)
	}

	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusRendering, SourcePath: streamclips.SourceKey(id)}
	putProgressSnapshot(t, store, streamclips.ProgressKey(id), jobprogress.StageRender, jobprogress.UnitClips, "clips", 0, 4)
	state, err := streamclips.NewRenderState(id, streamclips.VariantStreamerVerticalStack, streamclips.StatusRendering, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.writeStreamRenderState(state); err != nil {
		t.Fatal(err)
	}
	rw = httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+id.String()+"/renders/"+streamclips.VariantStreamerVerticalStack, nil))
	var rendering streamRenderResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &rendering); err != nil {
		t.Fatal(err)
	}
	if rendering.Progress == nil || rendering.Progress.Stage != jobprogress.StageRender || rendering.Progress.Done != 0 || rendering.Progress.Total != 4 {
		t.Fatalf("render handoff = %+v, want 0/4 clips", rendering.Progress)
	}
}

func TestGetStreamRenderOmitsAcquireProgress(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	streamRepo := newFakeStreamRepo()
	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusRendering, SourcePath: streamclips.SourceKey(id)}
	putProgressSnapshot(t, store, streamclips.ProgressKey(id), jobprogress.StageAcquire, jobprogress.UnitBytes, "bytes", 1000, 1000)
	state, err := streamclips.NewRenderState(id, streamclips.VariantStreamerVerticalStack, streamclips.StatusRendering, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(streamRepo))
	if err := h.writeStreamRenderState(state); err != nil {
		t.Fatal(err)
	}

	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+id.String()+"/renders/"+streamclips.VariantStreamerVerticalStack, nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rw.Code, rw.Body.String())
	}
	var got streamRenderResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Progress != nil {
		t.Fatalf("acquire leftover leaked onto stream render: %+v", got.Progress)
	}
}
