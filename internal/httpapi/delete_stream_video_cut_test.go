package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/reconcile"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

type failingPutStorage struct {
	*storage.Local
	failKey string
}

func (s *failingPutStorage) Put(key string, r io.Reader) error {
	if key == s.failKey {
		return errors.New("disk full")
	}
	return s.Local.Put(key, r)
}

func TestDeleteStreamVideoCutBeforeResultWriteKeepsRemainingClips(t *testing.T) {
	ctx := context.Background()
	streamRepo := store.NewMemoryStreamJobRepository()
	local, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	j := &streamclips.Job{Status: streamclips.StatusRendered, SourcePath: "stream-jobs/x/source.mp4", SourceSHA256: "sha"}
	if err := streamRepo.Create(ctx, j); err != nil {
		t.Fatal(err)
	}
	variant := streamclips.DefaultVariant().Name
	revision := uuid.New()
	resultKey, _ := streamclips.RenderRevisionResultKey(j.ID, variant, revision)
	files := &failingPutStorage{Local: local, failKey: resultKey}
	h := NewHandlers(newFakeRepo(), files, &fakeQueue{}, WithStreamRepository(streamRepo), WithStreamJobLocks(streamclips.NewJobLocks()))
	put := func(key string) {
		if err := local.Put(key, bytes.NewReader([]byte("x"))); err != nil {
			t.Fatal(err)
		}
	}
	var videos []streamclips.VideoEntry
	var delivery []streamclips.DeliveryEntry
	for _, clip := range []string{"first", "drop", "last"} {
		videoKey, _ := streamclips.RenderRevisionVideoKey(j.ID, variant, revision, clip)
		copyKey, _ := streamclips.RenderRevisionDeliveryKey(j.ID, variant, revision, clip+".mp4")
		put(videoKey)
		put(copyKey)
		videos = append(videos, streamclips.VideoEntry{ClipID: clip, Key: videoKey})
		delivery = append(delivery, streamclips.DeliveryEntry{Name: clip + ".mp4", Kind: "video", Key: copyKey})
	}
	state, err := streamclips.NewRenderState(j.ID, variant, streamclips.StatusRendered, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	prefix, _ := streamclips.RenderRevisionPrefix(j.ID, variant, revision)
	galleryKey, _ := streamclips.RenderRevisionGalleryKey(j.ID, variant, revision)
	state.Publish(streamclips.PublishedRevision{
		ResultKey: resultKey, GalleryKey: galleryKey, ArtifactDir: prefix, Videos: videos, Delivery: delivery, At: time.Now(),
	})
	put(galleryKey)
	result, err := streamclips.NewRenderResult(j.ID, variant, videos, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := local.Put(resultKey, bytes.NewReader(mustJSON(t, result))); err != nil {
		t.Fatal(err)
	}
	if err := h.writeStreamRenderState(state); err != nil {
		t.Fatal(err)
	}

	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, "/api/stream-jobs/"+j.ID.String()+"/renders/"+variant+"/videos/drop", nil))
	if rw.Code != http.StatusInternalServerError {
		t.Fatalf("delete with failing result write = %d, want 500; body=%s", rw.Code, rw.Body.String())
	}

	swept, err := reconcile.InterruptedWork(ctx, store.NewMemoryJobRepository(), streamRepo, store.NewMemoryEditorProjectRepository(), local, nil)
	if err != nil || swept.StreamRenderStates != 0 {
		t.Fatalf("restart reconciliation = %+v, %v", swept, err)
	}
	after, found, err := h.readStreamRenderState(j.ID, variant)
	if err != nil || !found || !after.HasPublishedRender() || after.Status != streamclips.StatusRendered {
		t.Fatalf("state after restart = %+v found=%v err=%v", after, found, err)
	}
	var listed []string
	for _, output := range listStreamOutputs(t, h, j.ID) {
		listed = append(listed, output.ClipID)
	}
	if len(listed) != 2 || listed[0] != "first" || listed[1] != "last" {
		t.Fatalf("listed after cut delete = %v, want [first last]", listed)
	}

	files.failKey = ""
	retry := httptest.NewRecorder()
	Routes(h).ServeHTTP(retry, httptest.NewRequest(http.MethodDelete, "/api/stream-jobs/"+j.ID.String()+"/renders/"+variant+"/videos/drop", nil))
	if retry.Code != http.StatusNoContent {
		t.Fatalf("retried delete = %d, want 204; body=%s", retry.Code, retry.Body.String())
	}
	finished, found, err := h.readPublishedStreamResult(after)
	if err != nil || !found || len(finished.Clips) != 2 {
		t.Fatalf("result after retried delete = %+v found=%v err=%v", finished.Clips, found, err)
	}
	for _, key := range []string{videos[1].Key, delivery[1].Key} {
		if exists, err := local.Exists(key); err != nil || exists {
			t.Fatalf("dropped clip file %s exists=%v err=%v after retried delete", key, exists, err)
		}
	}
}

func listStreamOutputs(t *testing.T, h *Handlers, jobID uuid.UUID) []streamRenderedOutput {
	t.Helper()
	rr := httptest.NewRecorder()
	Routes(h).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/stream-jobs?limit=10", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("list response = %d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Jobs []struct {
			ID              uuid.UUID              `json:"id"`
			RenderedOutputs []streamRenderedOutput `json:"rendered_outputs"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, job := range body.Jobs {
		if job.ID == jobID {
			return job.RenderedOutputs
		}
	}
	t.Fatalf("job %s not listed: %s", jobID, rr.Body.String())
	return nil
}
