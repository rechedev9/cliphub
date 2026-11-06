package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

func TestGetStreamRenderReportsProgressWhileRendering(t *testing.T) {
	variant := streamclips.VariantStreamerFullframeNoCam
	for _, status := range []streamclips.Status{streamclips.StatusRendering, streamclips.StatusRendered} {
		t.Run(string(status), func(t *testing.T) {
			streamRepo := newFakeStreamRepo()
			store, err := storage.NewLocal(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			j := streamclips.Job{ID: uuid.New(), Status: status}
			streamRepo.jobs[j.ID] = j
			state, err := streamclips.NewRenderState(j.ID, variant, status, nil, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			key, err := streamclips.RenderStateKey(j.ID, variant)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Put(key, bytes.NewReader(raw)); err != nil {
				t.Fatal(err)
			}
			board := streamclips.NewRenderProgressBoard()
			board.Set(j.ID, streamclips.NewRenderProgress(variant, 1, 4, 0.5))
			h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(streamRepo), WithStreamRenderProgress(board))

			rw := httptest.NewRecorder()
			Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+j.ID.String()+"/renders/"+variant, nil))
			if rw.Code != http.StatusOK {
				t.Fatalf("status = %d; body=%s", rw.Code, rw.Body.String())
			}
			var body struct {
				Status         streamclips.Status          `json:"status"`
				RenderProgress *streamclips.RenderProgress `json:"render_progress"`
			}
			if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Status != status {
				t.Fatalf("status = %q, want %q", body.Status, status)
			}
			if status != streamclips.StatusRendering {
				if body.RenderProgress != nil {
					t.Fatalf("render_progress = %+v on a %s job", body.RenderProgress, status)
				}
				return
			}
			if body.RenderProgress == nil || body.RenderProgress.Percent != 37.5 || body.RenderProgress.Clip != 2 {
				t.Fatalf("render_progress = %+v", body.RenderProgress)
			}
			items := h.streamJobListItems([]streamclips.Job{j})
			if items[0].RenderProgress == nil || items[0].RenderProgress.Percent != 37.5 {
				t.Fatalf("list render_progress = %+v", items[0].RenderProgress)
			}
		})
	}
}
