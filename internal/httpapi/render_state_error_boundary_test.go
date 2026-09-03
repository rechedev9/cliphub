package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

const renderStatePrivateDetail = `C:\private\render-state-marker`

type renderStateOpenFailureStorage struct {
	*fakeStorage
	stateKey string
}

func (s *renderStateOpenFailureStorage) Open(key string) (io.ReadCloser, error) {
	if key == s.stateKey {
		return nil, errors.New("open " + renderStatePrivateDetail + ": access denied")
	}
	return s.fakeStorage.Open(key)
}

func TestRenderStateConsumersHideStateStorageFailures(t *testing.T) {
	endpoints := []struct {
		name   string
		method string
		suffix string
	}{
		{name: "render status", method: http.MethodGet, suffix: ""},
		{name: "publish board", method: http.MethodGet, suffix: "/publish"},
		{name: "quality", method: http.MethodGet, suffix: "/quality"},
		{name: "stream artifact", method: http.MethodGet, suffix: "/videos/seg-001"},
		{name: "delete video", method: http.MethodDelete, suffix: "/videos/seg-001"},
		{name: "publish assistant", method: http.MethodGet, suffix: "/videos/seg-001/publish-assistant"},
	}
	states := []struct {
		name  string
		store func(t *testing.T, base *fakeStorage, stateKey string, id uuid.UUID) storage.Storage
	}{
		{
			name: "open failure",
			store: func(_ *testing.T, base *fakeStorage, stateKey string, _ uuid.UUID) storage.Storage {
				return &renderStateOpenFailureStorage{fakeStorage: base, stateKey: stateKey}
			},
		},
		{
			name: "malformed JSON",
			store: func(t *testing.T, base *fakeStorage, stateKey string, _ uuid.UUID) storage.Storage {
				t.Helper()
				if err := base.Put(stateKey, strings.NewReader(`{"artifact_prefix":"`+renderStatePrivateDetail)); err != nil {
					t.Fatal(err)
				}
				return base
			},
		},
		{
			name: "mismatched identity",
			store: func(t *testing.T, base *fakeStorage, stateKey string, _ uuid.UUID) storage.Storage {
				t.Helper()
				state := renderplan.NewRenderVariantState(renderplan.NewRenderVariantStateOptions{
					JobID:   uuid.New(),
					Variant: editor.PresetViral60Clean,
					Status:  renderplan.RenderVariantStatusReady,
				})
				putRenderStateBoundaryJSON(t, base, stateKey, state)
				return base
			},
		},
		{
			name: "invalid artifact pointer",
			store: func(t *testing.T, base *fakeStorage, stateKey string, id uuid.UUID) storage.Storage {
				t.Helper()
				state := renderplan.NewRenderVariantState(renderplan.NewRenderVariantStateOptions{
					JobID:          id,
					Variant:        editor.PresetViral60Clean,
					Status:         renderplan.RenderVariantStatusReady,
					ArtifactPrefix: renderStatePrivateDetail,
				})
				putRenderStateBoundaryJSON(t, base, stateKey, state)
				return base
			},
		},
	}

	for _, stateCase := range states {
		for _, endpoint := range endpoints {
			t.Run(stateCase.name+"/"+endpoint.name, func(t *testing.T) {
				repo := newFakeRepo()
				id := uuid.New()
				repo.jobs[id] = job.Job{ID: id, Status: job.StatusDone}
				stateKey, err := renderplan.RenderVariantStateKey(id, editor.PresetViral60Clean)
				if err != nil {
					t.Fatal(err)
				}
				base := newFakeStorage()
				store := stateCase.store(t, base, stateKey, id)
				h := NewHandlers(repo, store, &fakeQueue{})

				renderPath := "/api/jobs/" + id.String() + "/renders/" + editor.PresetViral60Clean
				req := httptest.NewRequest(endpoint.method, renderPath+endpoint.suffix, nil)
				rw := httptest.NewRecorder()
				Routes(h).ServeHTTP(rw, req)

				if rw.Code != http.StatusInternalServerError {
					t.Fatalf("status = %d, want 500; body=%s", rw.Code, rw.Body.String())
				}
				if got, want := rw.Body.String(), "{\"code\":\"internal_error\",\"error\":\"internal error\"}\n"; got != want {
					t.Fatalf("body = %q, want %q", got, want)
				}
				if strings.Contains(rw.Body.String(), renderStatePrivateDetail) {
					t.Fatalf("response exposed storage detail: %s", rw.Body.String())
				}
			})
		}
	}
}

func TestRenderStateConsumersRejectInvalidVariantBeforeStorage(t *testing.T) {
	endpoints := []struct {
		name   string
		method string
		suffix string
	}{
		{name: "render status", method: http.MethodGet, suffix: ""},
		{name: "publish board", method: http.MethodGet, suffix: "/publish"},
		{name: "quality", method: http.MethodGet, suffix: "/quality"},
		{name: "stream artifact", method: http.MethodGet, suffix: "/videos/seg-001"},
		{name: "delete video", method: http.MethodDelete, suffix: "/videos/seg-001"},
		{name: "publish assistant", method: http.MethodGet, suffix: "/videos/seg-001/publish-assistant"},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			repo := newFakeRepo()
			id := uuid.New()
			repo.jobs[id] = job.Job{ID: id, Status: job.StatusDone}
			h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

			renderPath := "/api/jobs/" + id.String() + "/renders/not-a-variant"
			req := httptest.NewRequest(endpoint.method, renderPath+endpoint.suffix, nil)
			rw := httptest.NewRecorder()
			Routes(h).ServeHTTP(rw, req)

			if rw.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
			}
		})
	}
}

func TestRenderArtifactConsumersRejectUnsafeNameBeforeReadingCorruptState(t *testing.T) {
	endpoints := []struct {
		name   string
		method string
		suffix string
	}{
		{name: "stream artifact", method: http.MethodGet, suffix: "/videos/seg-001.mp4"},
		{name: "delete video", method: http.MethodDelete, suffix: "/videos/seg-001.mp4"},
		{name: "publish assistant", method: http.MethodGet, suffix: "/videos/seg-001.mp4/publish-assistant"},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			repo := newFakeRepo()
			id := uuid.New()
			repo.jobs[id] = job.Job{ID: id, Status: job.StatusDone}
			stateKey, err := renderplan.RenderVariantStateKey(id, editor.PresetViral60Clean)
			if err != nil {
				t.Fatal(err)
			}
			store := newFakeStorage()
			if err := store.Put(stateKey, strings.NewReader(`{"artifact_prefix":"`+renderStatePrivateDetail)); err != nil {
				t.Fatal(err)
			}
			h := NewHandlers(repo, store, &fakeQueue{})

			renderPath := "/api/jobs/" + id.String() + "/renders/" + editor.PresetViral60Clean
			req := httptest.NewRequest(endpoint.method, renderPath+endpoint.suffix, nil)
			rw := httptest.NewRecorder()
			Routes(h).ServeHTTP(rw, req)

			if rw.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
			}
			if !strings.Contains(rw.Body.String(), "invalid artifact name") {
				t.Fatalf("body = %s, want artifact-name validation error", rw.Body.String())
			}
			if strings.Contains(rw.Body.String(), renderStatePrivateDetail) {
				t.Fatalf("response exposed storage detail: %s", rw.Body.String())
			}
		})
	}
}

func TestWorkbenchRenderStateFailureIsGenericInternalError(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusDone}
	stateKey, err := renderplan.RenderVariantStateKey(id, editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	store := newFakeStorage()
	if err := store.Put(stateKey, strings.NewReader(`{"artifact_prefix":"`+renderStatePrivateDetail)); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(repo, store, &fakeQueue{})

	req := httptest.NewRequest(
		http.MethodGet,
		"/ui/jobs/"+id.String()+"?variant="+editor.PresetViral60Clean,
		nil,
	)
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, req)

	if rw.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rw.Code, rw.Body.String())
	}
	if got, want := rw.Body.String(), "{\"code\":\"internal_error\",\"error\":\"internal error\"}\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestWorkbenchRejectsInvalidRenderVariantBeforeStorage(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusDone}
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	req := httptest.NewRequest(http.MethodGet, "/ui/jobs/"+id.String()+"?variant=not-a-variant", nil)
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
}

func putRenderStateBoundaryJSON(t *testing.T, store *fakeStorage, key string, state renderplan.RenderVariantState) {
	t.Helper()
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(key, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
}
