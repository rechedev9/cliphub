package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/httpapi"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tasks"
	"github.com/rechedev9/cliphub/internal/timelineplan"
	"github.com/rechedev9/cliphub/internal/workers"
)

func TestEditorRenderE2E(t *testing.T) {
	t.Parallel()
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not found on PATH")
	}
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not found on PATH")
	}
	filters, err := exec.Command(ffmpegPath, "-hide_banner", "-filters").Output()
	if err != nil || !bytes.Contains(filters, []byte("overlay")) {
		t.Skip("ffmpeg cannot overlay")
	}

	dataDir := t.TempDir()
	assets := store.NewMemoryEditorAssetRepository()
	projects := store.NewMemoryEditorProjectRepository()
	jobs := store.NewMemoryJobRepository()
	files, err := storage.NewLocal(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	worker := workers.NewTimelineRenderWorker(projects, files, workers.TimelineRenderWorkerConfig{
		WorkDir:     filepath.Join(dataDir, "work"),
		FFmpegPath:  ffmpegPath,
		Timeout:     "2m",
		AssetLookup: assets,
	})
	queue := newInlineQueue(map[string]taskHandler{tasks.TypeRenderTimeline: worker.HandleRenderTimeline}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	queue.Start(ctx)
	t.Cleanup(cancel)

	h := httpapi.NewHandlers(jobs, files, queue,
		httpapi.WithEditorRepositories(assets, projects),
		httpapi.WithStreamProber(streamclips.FFprobeProber{Path: ffprobePath}),
	)

	srv := httptest.NewServer(httpapi.Routes(h))
	t.Cleanup(srv.Close)
	client := srv.Client()

	red := filepath.Join(dataDir, "red.mp4")
	blue := filepath.Join(dataDir, "blue.mp4")
	writeEditorColorClip(t, ffmpegPath, red, "red", 3)
	writeEditorColorClip(t, ffmpegPath, blue, "blue", 3)
	assetA := uploadEditorAsset(t, client, srv.URL, red)
	assetB := uploadEditorAsset(t, client, srv.URL, blue)

	projectID := createEditorProject(t, client, srv.URL)
	end := 2.0
	plan := timelineplan.Document{
		SchemaVersion: timelineplan.SchemaVersion,
		Canvas:        timelineplan.Canvas{Width: 1080, Height: 1920, FPS: 60},
		Tracks: []timelineplan.Track{
			{ID: "v1", Kind: timelineplan.KindVideo, Items: []timelineplan.Item{{
				ID: "base", AssetID: assetA.String(), SourceIn: 0.2, SourceOut: 2.2,
			}}},
			{ID: "v2", Kind: timelineplan.KindVideo, Items: []timelineplan.Item{{
				ID: "pip", AssetID: assetB.String(), TimelineStart: 0.4, SourceIn: 0, SourceOut: 1,
				Transform: &timelineplan.Transform{X: 0.62, Y: 0.06, Width: 0.32, Height: 0.22},
			}}},
		},
		Overlays: []timelineplan.TextOverlay{{
			ID: "title", Text: "ACE", PositionY: 0.12, StartSeconds: 0, EndSeconds: &end,
		}},
	}
	putEditorPlan(t, client, srv.URL, projectID, plan)

	start, err := http.NewRequest(http.MethodPost, srv.URL+"/api/editor/projects/"+projectID.String()+"/render", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(start)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("start render status = %d", resp.StatusCode)
	}

	deadline := time.Now().Add(90 * time.Second)
	var state timelineplan.RenderState
	for time.Now().Before(deadline) {
		got, err := http.NewRequest(http.MethodGet, srv.URL+"/api/editor/projects/"+projectID.String()+"/render", nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := client.Do(got)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if err := json.Unmarshal(body, &state); err != nil {
			t.Fatal(err)
		}
		if state.Status == timelineplan.StatusRendered {
			break
		}
		if state.Status == timelineplan.StatusFailed {
			t.Fatalf("render failed: %s", state.Error)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if state.Status != timelineplan.StatusRendered {
		t.Fatalf("status = %s error=%s", state.Status, state.Error)
	}

	video, err := client.Get(srv.URL + "/api/editor/projects/" + projectID.String() + "/render/video")
	if err != nil {
		t.Fatal(err)
	}
	defer video.Body.Close()
	if video.StatusCode != http.StatusOK {
		t.Fatalf("video status = %d", video.StatusCode)
	}
	out := filepath.Join(dataDir, "downloaded.mp4")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(f, video.Body); err != nil {
		t.Fatal(err)
	}
	f.Close()
	probe := exec.Command(ffprobePath, "-v", "error", "-show_entries", "stream=codec_name,width,height", "-of", "csv=p=0", out)
	raw, err := probe.Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	if !bytes.Contains(raw, []byte("h264")) || !bytes.Contains(raw, []byte("1080")) {
		t.Fatalf("unexpected probe: %s", raw)
	}
}

func uploadEditorAsset(t *testing.T, client *http.Client, base, path string) uuid.UUID {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("video", filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(fw, src); err != nil {
		t.Fatal(err)
	}
	src.Close()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/editor/assets", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload status = %d body=%s", resp.StatusCode, body)
	}
	var asset struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatal(err)
	}
	return asset.ID
}

func createEditorProject(t *testing.T, client *http.Client, base string) uuid.UUID {
	t.Helper()
	resp, err := client.Post(base+"/api/editor/projects", "application/json", bytes.NewReader([]byte(`{"title":"e2e"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create project status = %d", resp.StatusCode)
	}
	var body struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body.ID
}

func putEditorPlan(t *testing.T, client *http.Client, base string, id uuid.UUID, plan timelineplan.Document) {
	t.Helper()
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPut, base+"/api/editor/projects/"+id.String()+"/plan", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("put plan status = %d body=%s", resp.StatusCode, body)
	}
}

func writeEditorColorClip(t *testing.T, ffmpegPath, path, color string, seconds int) {
	t.Helper()
	cmd := exec.Command(ffmpegPath,
		"-hide_banner", "-y",
		"-f", "lavfi", "-i", "color=c="+color+":s=1280x720:d=3:r=30",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac",
		path,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("write clip: %v: %s", err, out)
	}
}
