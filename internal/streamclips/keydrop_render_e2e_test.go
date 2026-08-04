package streamclips

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/tickcut/internal/keydropbanner"
	"github.com/rechedev9/tickcut/internal/mediafont"
)

func TestKeyDropRenderBurnsCustomCode(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp4")
	out := filepath.Join(dir, "out.mp4")
	frame := filepath.Join(dir, "frame.png")

	// 2s black 1080x1920 source (fullframe layout output size).
	if outb, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=gray:s=1080x1920:d=2",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", src,
	).CombinedOutput(); err != nil {
		t.Fatalf("make source: %v: %s", err, outb)
	}

	font, err := mediafont.Materialize()
	if err != nil {
		t.Fatalf("font: %v", err)
	}
	// Mirror the stream worker: burn the plan code into a plate PNG first.
	plate := filepath.Join(dir, "keydrop-banner.png")
	if err := keydropbanner.CompositeWithCode("ffmpeg", "classic", "NUEVO99", font, plate); err != nil {
		t.Fatalf("composite plate: %v", err)
	}

	start, end := 0.0, 2.0
	plan := DefaultEditPlan()
	plan.Variant = VariantStreamerFullframeNoCam
	plan.KeyDropBanner = KeyDropBannerPlan{
		Style: "classic", Code: "NUEVO99", StartSeconds: &start, EndSeconds: &end,
	}
	clip := ClipRange{ID: "c1", StartSeconds: 0, EndSeconds: 2}
	args, err := BuildFFmpegArgs(FFmpegInputs{
		SourcePath: src, OutputPath: out,
		KeyDropImagePath: plate, SourceHasAudio: false,
	}, plan, clip)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "keydrop-banner.png") {
		t.Fatalf("ffmpeg args missing pre-composited plate: %s", joined)
	}
	if outb, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("render: %v: %s", err, outb)
	}
	if outb, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-ss", "0.5", "-i", out, "-frames:v", "1", frame,
	).CombinedOutput(); err != nil {
		t.Fatalf("extract frame: %v: %s", err, outb)
	}
	// Copy for manual inspection when the test fails.
	_ = os.WriteFile(filepath.Join(dir, "ok"), []byte(frame+"\n"), 0o600)
	info, err := os.Stat(out)
	if err != nil || info.Size() < 1000 {
		t.Fatalf("output missing or tiny: %v size=%d", err, info.Size())
	}
	// Bring frame into a stable path under /tmp for agent review.
	dst := "/tmp/kd-e2e-frame.png"
	in, _ := os.ReadFile(frame)
	_ = os.WriteFile(dst, in, 0o644)
	t.Logf("frame written to %s", dst)
}
