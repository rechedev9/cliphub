package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/reconcile"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
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
		{name: "ready parent with a queued variant is refused", status: streamclips.StatusReady, renderState: streamclips.StatusQueued, wantCode: http.StatusConflict},
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

// A rendering project reads its asset media in place, so the asset must
// survive until that render settles; afterwards the same delete goes through.
func TestDeleteEditorAssetRefusesWhileRenderingProjectUsesIt(t *testing.T) {
	assets := newFakeEditorAssets()
	projects := newFakeEditorProjects()
	store := newFakeStorage()
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithEditorRepositories(assets, projects))

	asset := &mediaassets.Asset{SHA256: "abc", FileName: "clip.mp4", Origin: mediaassets.OriginUpload}
	if err := assets.Create(context.Background(), asset); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(mediaassets.MediaKey(asset.ID), bytes.NewReader([]byte("mp4"))); err != nil {
		t.Fatal(err)
	}
	project := &timelineplan.Project{Title: "busy", Status: timelineplan.StatusDraft}
	if err := projects.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	plan := timelineplan.DefaultDocument()
	plan.Tracks[0].Items = []timelineplan.Item{{ID: "clip-1", AssetID: asset.ID.String(), SourceIn: 0, SourceOut: 1}}
	if err := projects.SetPlan(context.Background(), project.ID, plan); err != nil {
		t.Fatal(err)
	}
	if err := projects.UpdateStatus(context.Background(), project.ID, timelineplan.StatusRendering, ""); err != nil {
		t.Fatal(err)
	}

	del := func() *httptest.ResponseRecorder {
		rw := httptest.NewRecorder()
		Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, "/api/editor/assets/"+asset.ID.String(), nil))
		return rw
	}
	if rw := del(); rw.Code != http.StatusConflict {
		t.Fatalf("delete while rendering = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	if exists, _ := store.Exists(mediaassets.MediaKey(asset.ID)); !exists {
		t.Fatal("media removed by a refused delete")
	}
	if err := projects.UpdateStatus(context.Background(), project.ID, timelineplan.StatusRendered, ""); err != nil {
		t.Fatal(err)
	}
	if rw := del(); rw.Code != http.StatusNoContent {
		t.Fatalf("delete after render settled = %d, want 204; body=%s", rw.Code, rw.Body.String())
	}
	if _, err := assets.Get(context.Background(), asset.ID); err == nil {
		t.Fatal("asset row survived delete")
	}
}

func TestDeleteStreamVideo(t *testing.T) {
	ctx := context.Background()
	streamRepo := store.NewMemoryStreamJobRepository()
	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	j := &streamclips.Job{Status: streamclips.StatusRendered, SourcePath: "stream-jobs/x/source.mp4", SourceSHA256: "sha"}
	if err := streamRepo.Create(ctx, j); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), files, &fakeQueue{}, WithStreamRepository(streamRepo), WithStreamJobLocks(streamclips.NewJobLocks()))
	variant := streamclips.DefaultVariant().Name
	revision := uuid.New()
	put := func(key string) {
		if err := files.Put(key, bytes.NewReader([]byte("x"))); err != nil {
			t.Fatal(err)
		}
	}
	var videos []streamclips.VideoEntry
	var delivery []streamclips.DeliveryEntry
	for _, clip := range []string{"keep", "drop"} {
		videoKey, _ := streamclips.RenderRevisionVideoKey(j.ID, variant, revision, clip)
		copyKey, _ := streamclips.RenderRevisionDeliveryKey(j.ID, variant, revision, clip+".mp4")
		put(videoKey)
		put(copyKey)
		videos = append(videos, streamclips.VideoEntry{ClipID: clip, Key: videoKey})
		delivery = append(delivery, streamclips.DeliveryEntry{Name: clip + ".mp4", Kind: "video", Key: copyKey})
	}
	state, err := streamclips.NewRenderState(j.ID, variant, streamclips.StatusRendered, nil, "", videos)
	if err != nil {
		t.Fatal(err)
	}
	state.ArtifactDir, _ = streamclips.RenderRevisionPrefix(j.ID, variant, revision)
	state.ResultKey, _ = streamclips.RenderRevisionResultKey(j.ID, variant, revision)
	state.GalleryKey, _ = streamclips.RenderRevisionGalleryKey(j.ID, variant, revision)
	state.Delivery = delivery
	put(state.GalleryKey)
	result, err := streamclips.NewRenderResult(j.ID, variant, videos, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.putStreamJSON(state.ResultKey, result); err != nil {
		t.Fatal(err)
	}
	if err := h.writeStreamRenderState(state); err != nil {
		t.Fatal(err)
	}
	deleteClip := func(clip string) int {
		rw := httptest.NewRecorder()
		Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, "/api/stream-jobs/"+j.ID.String()+"/renders/"+variant+"/videos/"+clip, nil))
		return rw.Code
	}
	exists := func(key string) bool {
		ok, err := files.Exists(key)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}

	// Twice: the second delete finds nothing left and still succeeds.
	for range 2 {
		if code := deleteClip("drop"); code != http.StatusNoContent {
			t.Fatalf("delete drop = %d", code)
		}
	}
	if exists(videos[1].Key) || exists(delivery[1].Key) || !exists(videos[0].Key) || !exists(delivery[0].Key) {
		t.Fatal("delete removed the wrong files")
	}
	// A restart must still find the remaining clip's render whole.
	swept, err := reconcile.InterruptedWork(ctx, store.NewMemoryJobRepository(), streamRepo, store.NewMemoryEditorProjectRepository(), files, nil)
	if err != nil || swept.StreamRenderStates != 0 {
		t.Fatalf("restart reconciliation = %+v, %v", swept, err)
	}
	if got := getStreamListOutput(t, h, j.ID); got.ClipID != "keep" {
		t.Fatalf("listed after delete = %+v", got)
	}

	rendering := state
	rendering.Status = streamclips.StatusRendering
	rendering.Published = true
	rendering.Videos, rendering.Delivery = videos[:1], delivery[:1]
	if err := h.writeStreamRenderState(rendering); err != nil {
		t.Fatal(err)
	}
	if code := deleteClip("keep"); code != http.StatusConflict {
		t.Fatalf("delete while rendering = %d, want 409", code)
	}
	rendering.Status = streamclips.StatusRendered
	if err := h.writeStreamRenderState(rendering); err != nil {
		t.Fatal(err)
	}

	// The last clip takes the whole render, and the stream is ready to export again.
	if code := deleteClip("keep"); code != http.StatusNoContent {
		t.Fatalf("delete last clip = %d", code)
	}
	if _, found, err := h.readStreamRenderState(j.ID, variant); err != nil || found || exists(videos[0].Key) {
		t.Fatalf("render left after last delete: found=%v err=%v", found, err)
	}
	if after, _ := streamRepo.Get(ctx, j.ID); after.Status != streamclips.StatusReady {
		t.Fatalf("stream status after last delete = %s, want ready", after.Status)
	}
}
