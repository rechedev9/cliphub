package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/timelineplan"
)

func TestDeleteStreamJob(t *testing.T) {
	cases := []struct {
		name        string
		status      streamclips.Status
		renderState streamclips.Status
		wantCode    int
	}{
		{name: "ready deletes", status: streamclips.StatusReady, wantCode: http.StatusNoContent},
		{name: "rendered deletes", status: streamclips.StatusRendered, wantCode: http.StatusNoContent},
		{name: "failed deletes", status: streamclips.StatusFailed, wantCode: http.StatusNoContent},
		{name: "acquiring is refused", status: streamclips.StatusAcquiring, wantCode: http.StatusConflict},
		{name: "rendering is refused", status: streamclips.StatusRendering, wantCode: http.StatusConflict},
		{name: "ready parent with a rendering variant is refused", status: streamclips.StatusReady, renderState: streamclips.StatusRendering, wantCode: http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			streamRepo := newFakeStreamRepo()
			store, err := storage.NewLocal(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			j := streamclips.Job{ID: uuid.New(), Status: tc.status, SourcePath: "stream-jobs/x/source.mp4", SourceSHA256: "sha"}
			streamRepo.jobs[j.ID] = j
			sourceKey := streamclips.SourceKey(j.ID)
			if err := store.Put(sourceKey, bytes.NewReader([]byte("mp4"))); err != nil {
				t.Fatal(err)
			}
			h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(streamRepo), WithStreamJobLocks(streamclips.NewJobLocks()))
			if tc.renderState != "" {
				variant := streamclips.DefaultVariant().Name
				state, err := streamclips.NewRenderState(j.ID, variant, tc.renderState, nil, "", nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := h.writeStreamRenderState(state); err != nil {
					t.Fatal(err)
				}
			}

			rw := httptest.NewRecorder()
			Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, "/api/stream-jobs/"+j.ID.String(), nil))
			if rw.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, tc.wantCode, rw.Body.String())
			}
			_, rowPresent := streamRepo.jobs[j.ID]
			sourcePresent, err := store.Exists(sourceKey)
			if err != nil {
				t.Fatal(err)
			}
			wantPresent := tc.wantCode != http.StatusNoContent
			if rowPresent != wantPresent || sourcePresent != wantPresent {
				t.Fatalf("after %d: row present=%v source present=%v, want %v", rw.Code, rowPresent, sourcePresent, wantPresent)
			}
			if tc.wantCode == http.StatusNoContent {
				rw = httptest.NewRecorder()
				Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, "/api/stream-jobs/"+j.ID.String(), nil))
				if rw.Code != http.StatusNotFound {
					t.Fatalf("repeat delete status = %d, want 404", rw.Code)
				}
			}
		})
	}
}

func TestDeleteEditorProjectAndAsset(t *testing.T) {
	assets := newFakeEditorAssets()
	projects := newFakeEditorProjects()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithEditorRepositories(assets, projects))

	asset := &mediaassets.Asset{SHA256: "abc", FileName: "clip.mp4", Origin: mediaassets.OriginUpload}
	if err := assets.Create(context.Background(), asset); err != nil {
		t.Fatal(err)
	}
	asset.MediaKey = mediaassets.MediaKey(asset.ID)
	if err := store.Put(asset.MediaKey, bytes.NewReader([]byte("mp4"))); err != nil {
		t.Fatal(err)
	}
	rendering := &timelineplan.Project{Title: "busy", Status: timelineplan.StatusRendering}
	draft := &timelineplan.Project{Title: "draft", Status: timelineplan.StatusDraft}
	for _, p := range []*timelineplan.Project{rendering, draft} {
		if err := projects.Create(context.Background(), p); err != nil {
			t.Fatal(err)
		}
		if err := store.Put(timelineplan.PlanKey(p.ID), bytes.NewReader([]byte("{}"))); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name     string
		path     string
		wantCode int
		gone     string
	}{
		{name: "rendering project is refused", path: "/api/editor/projects/" + rendering.ID.String(), wantCode: http.StatusConflict},
		{name: "draft project deletes with its tree", path: "/api/editor/projects/" + draft.ID.String(), wantCode: http.StatusNoContent, gone: timelineplan.PlanKey(draft.ID)},
		{name: "asset deletes with its media", path: "/api/editor/assets/" + asset.ID.String(), wantCode: http.StatusNoContent, gone: asset.MediaKey},
		{name: "unknown project is 404", path: "/api/editor/projects/" + uuid.New().String(), wantCode: http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, tc.path, nil))
			if rw.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, tc.wantCode, rw.Body.String())
			}
			if tc.gone != "" {
				exists, err := store.Exists(tc.gone)
				if err != nil {
					t.Fatal(err)
				}
				if exists {
					t.Fatalf("artifact %s still present after delete", tc.gone)
				}
			}
		})
	}
	if _, err := projects.Get(context.Background(), rendering.ID); err != nil {
		t.Fatalf("rendering project must survive a refused delete: %v", err)
	}
	if _, err := projects.Get(context.Background(), draft.ID); err == nil {
		t.Fatal("draft project row survived delete")
	}
	if _, err := assets.Get(context.Background(), asset.ID); err == nil {
		t.Fatal("asset row survived delete")
	}
}
