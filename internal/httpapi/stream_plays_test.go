package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

func TestStreamPlayScanFindsTheHighlightedKill(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	// A black 1080p video with a CS2-like red notice border from 1s to 3.5s.
	source := filepath.Join(t.TempDir(), "source.mp4")
	cmd := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=black:s=1920x1080:r=30:d=5",
		"-vf", "drawbox=x=1500:y=110:w=402:h=34:color=red:t=3:enable='between(t,1,3.5)'",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make test video: %v: %s", err, out)
	}

	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	file, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := store.Put(streamclips.SourceKey(id), file); err != nil {
		t.Fatal(err)
	}
	repo := newFakeStreamRepo()
	if err := repo.Create(context.Background(), &streamclips.Job{
		ID: id, Status: streamclips.StatusReady, SourcePath: streamclips.SourceKey(id),
		Probe: streamclips.SourceProbe{DurationSeconds: 5},
	}); err != nil {
		t.Fatal(err)
	}
	router := Routes(NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(repo), WithStreamPlayScanner(ffmpeg)))

	if scan := requestPlays(t, router, http.MethodGet, id); scan.Status != playScanIdle {
		t.Fatalf("status before scan = %q, want idle", scan.Status)
	}
	requestPlays(t, router, http.MethodPost, id)
	deadline := time.Now().Add(60 * time.Second)
	scan := requestPlays(t, router, http.MethodGet, id)
	for scan.Status == playScanRunning && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		scan = requestPlays(t, router, http.MethodGet, id)
	}
	if scan.Status != playScanDone || len(scan.Plays) != 1 || scan.Plays[0].Kills() != 1 {
		t.Fatalf("scan = %+v, want one play with one kill", scan)
	}
	if got := scan.Plays[0].KillSeconds[0]; got < 0.9 || got > 1.3 {
		t.Fatalf("kill at %.2fs, want about 1s", got)
	}
}

func TestStreamPlayScanWaitsForTheSource(t *testing.T) {
	repo := newFakeStreamRepo()
	id := uuid.New()
	if err := repo.Create(context.Background(), &streamclips.Job{ID: id, Status: streamclips.StatusAcquiring}); err != nil {
		t.Fatal(err)
	}
	router := Routes(NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithStreamRepository(repo), WithStreamPlayScanner("ffmpeg")))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/plays", nil))
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", response.Code, response.Body.String())
	}
}

func requestPlays(t *testing.T, router http.Handler, method string, id uuid.UUID) streamPlayScan {
	t.Helper()
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(method, "/api/stream-jobs/"+id.String()+"/plays", nil))
	if response.Code != http.StatusOK && response.Code != http.StatusAccepted {
		t.Fatalf("%s plays status = %d; body=%s", method, response.Code, response.Body.String())
	}
	var scan streamPlayScan
	if err := json.Unmarshal(response.Body.Bytes(), &scan); err != nil {
		t.Fatal(err)
	}
	return scan
}
