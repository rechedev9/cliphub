package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestCaptureRetentionSettingFreesTheOlderLongVideos(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(repo, store, &fakeQueue{})
	older := seedRenderedLongVideo(t, h, repo, time.Now().Add(time.Hour))
	newer := seedRenderedLongVideo(t, h, repo, time.Now().Add(2*time.Hour))
	r := chi.NewRouter()
	r.Get("/api/storage", h.GetStorage)
	r.Put("/api/storage/capture-retention", h.PutCaptureRetention)

	if usage := readStorageUsage(t, r); usage.CaptureRetention.Keep != 3 || len(usage.Jobs) != 2 || !usage.Jobs[0].Freeable || !usage.Jobs[1].Freeable {
		t.Fatalf("usage = %+v, want the default of 3 and both long videos keeping their captures", usage)
	}
	rw := putCaptureRetention(r, `{"keep":1}`)
	var put struct {
		Keep       int   `json:"keep"`
		FreedBytes int64 `json:"freed_bytes"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &put); rw.Code != http.StatusOK || err != nil || put.Keep != 1 || put.FreedBytes != int64(len("round")) {
		t.Fatalf("PUT keep 1 = %d %s", rw.Code, rw.Body.String())
	}
	if exists, _ := store.Exists(older); exists {
		t.Fatal("the older long video kept its captures past the new limit")
	}
	if exists, _ := store.Exists(newer); !exists {
		t.Fatal("the newest long video lost its captures")
	}
	if usage := readStorageUsage(t, r); usage.CaptureRetention.Keep != 1 {
		t.Fatalf("usage after PUT = %+v", usage)
	}
	if rw := putCaptureRetention(r, `{"keep":99}`); rw.Code != http.StatusBadRequest {
		t.Fatalf("keep 99 = %d, want 400", rw.Code)
	}
}

func TestCaptureRetentionYieldsToTheLabSwitch(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(repo, store, &fakeQueue{}, WithKeepFullDemoCaptures(true))
	clip := seedRenderedLongVideo(t, h, repo, time.Now().Add(time.Hour))
	r := chi.NewRouter()
	r.Get("/api/storage", h.GetStorage)
	r.Put("/api/storage/capture-retention", h.PutCaptureRetention)
	if rw := putCaptureRetention(r, `{"keep":0}`); rw.Code != http.StatusOK {
		t.Fatalf("PUT keep 0 = %d %s", rw.Code, rw.Body.String())
	}
	if exists, _ := store.Exists(clip); !exists {
		t.Fatal("ZV_KEEP_FULL_DEMO_CAPTURES must keep every capture")
	}
	if usage := readStorageUsage(t, r); !usage.CaptureRetention.KeepsAll {
		t.Fatalf("usage = %+v, want keeps_all", usage)
	}
}

func putCaptureRetention(r http.Handler, body string) *httptest.ResponseRecorder {
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodPut, "/api/storage/capture-retention", strings.NewReader(body)))
	return rw
}

// seedRenderedLongVideo stores a job whose long video became ready at renderedAt, with one round clip.
func seedRenderedLongVideo(t *testing.T, h *Handlers, repo *fakeRepo, renderedAt time.Time) string {
	t.Helper()
	j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	clip := "jobs/" + j.ID.String() + "/recording/revisions/" + uuid.NewString() + "/segments/round-001.mp4"
	if err := h.storage.Put(clip, bytes.NewReader([]byte("round"))); err != nil {
		t.Fatal(err)
	}
	loadout, err := renderplan.LoadoutForVariant(editor.PresetGameplayPOV60)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{JobID: j.ID, Loadout: loadout, Status: renderplan.RenderVariantStatusReady, RevisionID: uuid.New(), Now: renderedAt, FullDemo: approvedFullDemo(t)})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.writeRenderVariantState(state); err != nil {
		t.Fatal(err)
	}
	if err := h.storage.Put(state.ArtifactPrefix+"/videos/demo-compilation.mp4", bytes.NewReader([]byte("final video"))); err != nil {
		t.Fatal(err)
	}
	return clip
}
