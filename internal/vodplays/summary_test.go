package vodplays

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRenderSummaryJoinsThePlaysInOrderAsLandscape(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes video")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not on PATH")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "vod.mp4")
	if err := runFFmpeg(context.Background(), ffmpeg, []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=30:duration=12",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=12",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-shortest", source,
	}); err != nil {
		t.Fatalf("make source: %v", err)
	}
	output := filepath.Join(dir, "summary.mp4")
	var last float64
	err = RenderSummary(context.Background(), SummaryOptions{
		FFmpeg:   ffmpeg,
		Input:    source,
		Output:   output,
		WorkDir:  dir,
		HasAudio: true,
		// Out of order and with a play too short to show.
		Plays:      []Play{{ID: "b", StartSeconds: 6, EndSeconds: 9.5}, {ID: "a", StartSeconds: 1, EndSeconds: 4}, {ID: "tiny", StartSeconds: 10, EndSeconds: 10.5}},
		OnProgress: func(share float64) { last = share },
	})
	if err != nil {
		t.Fatalf("RenderSummary: %v", err)
	}
	out, err := exec.Command(ffprobe, "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height:format=duration", "-of", "csv=p=0", output).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	lines := strings.Fields(strings.ReplaceAll(string(out), ",", " "))
	if len(lines) < 3 || lines[0] != "1920" || lines[1] != "1080" {
		t.Fatalf("ffprobe = %q, want 1920x1080", out)
	}
	duration, err := strconv.ParseFloat(lines[2], 64)
	if err != nil || duration < 6.3 || duration > 6.8 {
		t.Fatalf("duration = %q, want about 6.5 s (3 + 3.5)", lines[2])
	}
	if last < 0.98 {
		t.Fatalf("last progress = %v, want about 0.99", last)
	}
}

func TestRenderSummaryNeedsAPlay(t *testing.T) {
	err := RenderSummary(context.Background(), SummaryOptions{Plays: []Play{{StartSeconds: 3, EndSeconds: 3.5}}})
	if err == nil {
		t.Fatal("RenderSummary with no showable play succeeded")
	}
}
