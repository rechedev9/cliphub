package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vodfetch"
)

func TestGetStreamJobReportsDownloadProgressWhileAcquiring(t *testing.T) {
	for _, status := range []streamclips.Status{streamclips.StatusAcquiring, streamclips.StatusReady} {
		t.Run(string(status), func(t *testing.T) {
			streamRepo := newFakeStreamRepo()
			store, err := storage.NewLocal(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			j := streamclips.Job{ID: uuid.New(), Status: status}
			streamRepo.jobs[j.ID] = j
			progress := streamclips.NewAcquireProgress()
			pct := 42.5
			progress.Set(j.ID, vodfetch.Progress{DownloadedBytes: 425, TotalBytes: 1000, Percent: &pct})
			h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(streamRepo), WithStreamAcquireProgress(progress))

			rw := httptest.NewRecorder()
			Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+j.ID.String(), nil))
			if rw.Code != http.StatusOK {
				t.Fatalf("status = %d; body=%s", rw.Code, rw.Body.String())
			}
			var body struct {
				DownloadProgress *vodfetch.Progress `json:"download_progress"`
			}
			if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if status != streamclips.StatusAcquiring {
				if body.DownloadProgress != nil {
					t.Fatalf("download_progress = %+v on a %s job", body.DownloadProgress, status)
				}
				return
			}
			if body.DownloadProgress == nil || body.DownloadProgress.Percent == nil || *body.DownloadProgress.Percent != pct {
				t.Fatalf("download_progress = %+v", body.DownloadProgress)
			}
		})
	}
}
