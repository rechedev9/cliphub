package timelinerender

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/timelineplan"
)

func TestRenderMultitrackE2E(t *testing.T) {
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
	if err != nil || !bytes.Contains(filters, []byte("drawtext")) {
		t.Skip("ffmpeg build cannot drawtext")
	}
	fontPath, err := mediafont.Materialize()
	if err != nil {
		t.Skipf("font: %v", err)
	}

	dir := t.TempDir()
	red := filepath.Join(dir, "red.mp4")
	blue := filepath.Join(dir, "blue.mp4")
	writeColorClip(t, ffmpegPath, red, "red", 3)
	writeColorClip(t, ffmpegPath, blue, "blue", 3)

	assetA := "11111111-1111-1111-1111-111111111111"
	assetB := "22222222-2222-2222-2222-222222222222"
	end := 2.0
	doc := timelineplan.Document{
		SchemaVersion: timelineplan.SchemaVersion,
		Canvas:        timelineplan.Canvas{Width: 1080, Height: 1920, FPS: 60},
		Tracks: []timelineplan.Track{
			{
				ID:   "v1",
				Kind: timelineplan.KindVideo,
				Items: []timelineplan.Item{{
					ID: "base", AssetID: assetA,
					SourceIn: 0.2, SourceOut: 2.2,
				}},
			},
			{
				ID:   "v2",
				Kind: timelineplan.KindVideo,
				Items: []timelineplan.Item{{
					ID: "pip", AssetID: assetB,
					TimelineStart: 0.4, SourceIn: 0, SourceOut: 1,
					Transform: &timelineplan.Transform{X: 0.62, Y: 0.06, Width: 0.32, Height: 0.22},
				}},
			},
		},
		Overlays: []timelineplan.TextOverlay{{
			ID: "title", Text: "ACE", PositionY: 0.12, StartSeconds: 0, EndSeconds: &end,
		}},
	}
	textPaths, err := WriteOverlayTexts(filepath.Join(dir, "texts"), doc.Overlays)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "final.mp4")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := Render(ctx, ffmpegPath, Inputs{
		Assets: map[string]AssetInput{
			assetA: {Path: red, HasAudio: true},
			assetB: {Path: blue, HasAudio: true},
		},
		OutputPath:       out,
		FontPath:         fontPath,
		TextOverlayPaths: textPaths,
	}, doc)
	if err != nil {
		t.Fatal(err)
	}
	if result.Width != 1080 || result.Height != 1920 {
		t.Fatalf("size = %dx%d", result.Width, result.Height)
	}
	if result.Performance.RenderMS <= 0 || result.Performance.OutputBytes <= 0 || result.Performance.MediaDurationSeconds != doc.DurationSeconds() {
		t.Fatalf("performance = %#v", result.Performance)
	}
	info, err := os.Stat(out)
	if err != nil || info.Size() == 0 {
		t.Fatalf("output missing: %v", err)
	}
	if _, err := os.Stat(result.CoverPath); err != nil {
		t.Fatalf("cover missing: %v", err)
	}

	probe := exec.Command(ffprobePath, "-v", "error", "-show_entries", "stream=codec_name,width,height", "-of", "csv=p=0", out)
	raw, err := probe.Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	if !bytes.Contains(raw, []byte("h264")) || !bytes.Contains(raw, []byte("1080")) {
		t.Fatalf("unexpected probe: %s", raw)
	}
}

func writeColorClip(t *testing.T, ffmpegPath, path, color string, seconds int) {
	t.Helper()
	args := []string{
		"-hide_banner", "-y",
		"-f", "lavfi", "-i", "color=c=" + color + ":s=1280x720:d=" + strconv.Itoa(seconds) + ":r=30",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=" + strconv.Itoa(seconds),
		"-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac",
		path,
	}
	cmd := exec.Command(ffmpegPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("write %s: %v: %s", path, err, out)
	}
}
