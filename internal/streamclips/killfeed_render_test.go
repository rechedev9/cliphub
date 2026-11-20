package streamclips

import (
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestKillFeedRenderMovesTheLiveFeedWithASoftEdge(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes video")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp4")
	// A gray game with a white notice in the top right corner, where CS2 draws the feed.
	if out, err := exec.Command("ffmpeg", "-y", "-v", "error", "-f", "lavfi", "-i", "color=c=0x404040:s=1920x1080:d=2:r=30",
		"-vf", "drawbox=x=1440:y=135:w=460:h=68:color=white:t=fill", "-c:v", "libx264", "-pix_fmt", "yuv420p", src).CombinedOutput(); err != nil {
		t.Fatalf("make source: %v: %s", err, out)
	}
	plan, clip := killFeedPlan(VariantStreamerFullframeNoCam)
	clip.StartSeconds, clip.EndSeconds = 0, 2
	plan.Clips = []ClipRange{clip}
	overlay := NewKillFeedOverlay([]KillFeedMask{{Area: CropRect{X: 0.75, Y: 0.125, Width: 0.24, Height: 0.0625}, To: 2}}, 1920, 1080, 2)
	out := filepath.Join(dir, "out.mp4")
	args, err := BuildFFmpegArgs(FFmpegInputs{SourcePath: src, OutputPath: out, KillFeed: overlay, SourceWidth: 1920, SourceHeight: 1080}, plan, clip)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("render: %v: %s", err, b)
	}
	frame := filepath.Join(dir, "frame.png")
	if b, err := exec.Command("ffmpeg", "-y", "-v", "error", "-ss", "1", "-i", out, "-frames:v", "1", frame).CombinedOutput(); err != nil {
		t.Fatalf("frame: %v: %s", err, b)
	}
	f, err := os.Open(frame)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	layout, _ := VariantByName(plan.Variant)
	kf := newKillFeedGraph(layout, plan, FFmpegInputs{KillFeed: overlay, SourceWidth: 1920, SourceHeight: 1080}, 2)
	luma := func(x, y int) uint32 { r, _, _, _ := img.At(x, y).RGBA(); return r >> 8 }
	centerX, centerY := layout.OutputWidth/2, kf.top+kf.height/2
	// The notice is blended at the cut's opacity, so it is bright but not pure white.
	if c := luma(centerX, centerY); c < 200 || c > 250 {
		t.Fatalf("cut center luma = %d, want the notice at about 92%% opacity", c)
	}
	edge := luma((layout.OutputWidth-kf.width)/2+1, centerY)
	if game := luma(centerX, kf.top+kf.height+40); edge > game+20 || game > 90 {
		t.Fatalf("cut edge luma = %d, game = %d: the edge must fade into the game", edge, game)
	}
}
