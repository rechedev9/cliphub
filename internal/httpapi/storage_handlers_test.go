package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/storage"
)

func TestDeleteJobCapturesFreesOnlyFullDemoClipsOfARenderedJob(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	prefix := "jobs/" + j.ID.String() + "/"
	files := map[string]string{
		"recording/revisions/a1/segments/round-001.mp4":                    "round one",
		"recording/revisions/a0/segments/round-001.mp4":                    "old",
		"recording/revisions/a1/recording-result.json":                     "{}",
		"recording/segments/seg-001.mp4":                                   "short",
		"renders/gameplay-pov-60/revisions/r1/videos/demo-compilation.mp4": "final video",
	}
	for key, body := range files {
		if err := store.Put(prefix+key, bytes.NewReader([]byte(body))); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandlers(repo, store, &fakeQueue{})
	r := chi.NewRouter()
	r.Get("/api/storage", h.GetStorage)
	r.Delete("/api/jobs/{id}/captures", h.DeleteJobCaptures)

	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, "/api/jobs/"+j.ID.String()+"/captures", nil))
	if rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), `"code":"no_full_demo_render"`) {
		t.Fatalf("delete before the render: status = %d, want 409 no_full_demo_render; body=%s", rw.Code, rw.Body.String())
	}

	writeReadyFullDemoState(t, h, j.ID)

	usage := readStorageUsage(t, r)
	if len(usage.Jobs) != 1 || !usage.Jobs[0].Freeable || usage.Jobs[0].CaptureBytes != int64(len("round one")+len("old")) || usage.Jobs[0].RenderBytes < int64(len("final video")) {
		t.Fatalf("usage before delete = %+v", usage)
	}

	rw = httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, "/api/jobs/"+j.ID.String()+"/captures", nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var freed struct {
		FreedBytes int64 `json:"freed_bytes"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &freed); err != nil || freed.FreedBytes != int64(len("round one")+len("old")) {
		t.Fatalf("freed = %+v, err = %v", freed, err)
	}
	for key := range files {
		exists, err := store.Exists(prefix + key)
		if err != nil {
			t.Fatal(err)
		}
		if want := key != "recording/revisions/a1/segments/round-001.mp4" && key != "recording/revisions/a0/segments/round-001.mp4"; exists != want {
			t.Errorf("%s exists = %v, want %v", key, exists, want)
		}
	}
	if usage := readStorageUsage(t, r); usage.Jobs[0].CaptureBytes != 0 {
		t.Fatalf("usage after delete = %+v", usage)
	}
}

func TestDeleteJobCapturesKeepsACaptureNewerThanTheReadyRender(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, &fakeQueue{})
	state := writeReadyFullDemoState(t, h, j.ID)
	clip := "jobs/" + j.ID.String() + "/recording/revisions/a2/segments/round-001.mp4"
	if err := store.Put(clip, bytes.NewReader([]byte("recaptured"))); err != nil {
		t.Fatal(err)
	}
	clipPath, err := store.ResolvePath(clip)
	if err != nil {
		t.Fatal(err)
	}
	later := state.UpdatedAt.Add(time.Minute)
	if err := os.Chtimes(clipPath, later, later); err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Get("/api/storage", h.GetStorage)
	r.Delete("/api/jobs/{id}/captures", h.DeleteJobCaptures)

	if usage := readStorageUsage(t, r); len(usage.Jobs) != 1 || usage.Jobs[0].Freeable {
		t.Fatalf("usage = %+v, want one job that is not freeable", usage)
	}
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, "/api/jobs/"+j.ID.String()+"/captures", nil))
	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	if exists, err := store.Exists(clip); err != nil || !exists {
		t.Fatalf("recaptured clip exists = %v, %v; want true", exists, err)
	}
}

func TestDeleteJobCapturesRejectsACaptureInFlight(t *testing.T) {
	repo := newFakeRepo()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecording, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	r := chi.NewRouter()
	r.Delete("/api/jobs/{id}/captures", h.DeleteJobCaptures)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, "/api/jobs/"+j.ID.String()+"/captures", nil))
	if rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), `"code":"job_busy"`) {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
}

type storageUsageResponse struct {
	Jobs       []jobStorageUsage `json:"jobs"`
	TotalBytes int64             `json:"total_bytes"`
}

func readStorageUsage(t *testing.T, r http.Handler) storageUsageResponse {
	t.Helper()
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/storage", nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("storage status = %d; body=%s", rw.Code, rw.Body.String())
	}
	var usage storageUsageResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &usage); err != nil {
		t.Fatal(err)
	}
	return usage
}

func writeReadyFullDemoState(t *testing.T, h *Handlers, id uuid.UUID) renderplan.RenderVariantState {
	t.Helper()
	loadout, err := renderplan.LoadoutForVariant(editor.PresetGameplayPOV60)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{JobID: id, Loadout: loadout, Status: renderplan.RenderVariantStatusReady, RevisionID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.writeRenderVariantState(state); err != nil {
		t.Fatal(err)
	}
	return state
}
