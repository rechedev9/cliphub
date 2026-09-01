package streamclips

import (
	"bytes"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/keydropbanner"
	"github.com/rechedev9/cliphub/internal/mediafont"
)

func TestKeyDropRenderBurnsCustomCode(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	// A minimal ffmpeg build (e.g. one compiled without libfreetype) has no
	// drawtext filter, so the keydrop plate cannot be composited. Skip rather
	// than fail, matching the ffmpeg availability guard above.
	filterList, err := exec.Command("ffmpeg", "-hide_banner", "-filters").Output()
	if err != nil {
		t.Skipf("ffmpeg -filters failed: %v, skipping keydrop render e2e", err)
	}
	if !bytes.Contains(filterList, []byte("drawtext")) {
		t.Skip("ffmpeg build has no drawtext filter, skipping keydrop render e2e")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp4")

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
	start, end := 0.0, 2.0
	tests := []struct {
		style string
		code  string
		frame string
	}{
		{style: "classic", code: "NUEVO99", frame: "/tmp/kd-e2e-frame.png"},
		{style: "jcorko", code: "HUASO", frame: "/tmp/kd-jcorko-e2e-frame.png"},
	}
	for _, tt := range tests {
		t.Run(tt.style+"/"+tt.code, func(t *testing.T) {
			plate := filepath.Join(dir, "keydrop-"+tt.style+".png")
			clipOut := filepath.Join(dir, tt.style+".mp4")
			clipFrame := filepath.Join(dir, tt.style+"-frame.png")
			if err := keydropbanner.CompositeWithCode("ffmpeg", tt.style, tt.code, font, plate); err != nil {
				t.Fatalf("composite plate: %v", err)
			}

			plan := DefaultEditPlan()
			plan.Variant = VariantStreamerFullframeNoCam
			plan.KeyDropBanner = KeyDropBannerPlan{
				Style: tt.style, Code: tt.code, StartSeconds: &start, EndSeconds: &end,
			}
			clip := ClipRange{ID: "c1", StartSeconds: 0, EndSeconds: 2}
			args, err := BuildFFmpegArgs(FFmpegInputs{
				SourcePath: src, OutputPath: clipOut,
				KeyDropImagePath: plate, SourceHasAudio: false,
			}, plan, clip)
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, filepath.Base(plate)) {
				t.Fatalf("ffmpeg args missing pre-composited plate: %s", joined)
			}
			if outb, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
				t.Fatalf("render: %v: %s", err, outb)
			}
			if outb, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
				"-ss", "0.5", "-i", clipOut, "-frames:v", "1", clipFrame,
			).CombinedOutput(); err != nil {
				t.Fatalf("extract frame: %v: %s", err, outb)
			}
			info, err := os.Stat(clipOut)
			if err != nil || info.Size() < 1000 {
				t.Fatalf("output missing or tiny: %v size=%d", err, info.Size())
			}
			in, err := os.ReadFile(clipFrame)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(tt.frame, in, 0o644); err != nil {
				t.Fatal(err)
			}
			banner := sampleBannerPixel(t, clipFrame)
			if banner.R+banner.G+banner.B > 220 {
				t.Fatalf("banner center looks like empty gray source: %+v", banner)
			}
			t.Logf("frame written to %s banner=%+v", tt.frame, banner)
		})
	}
}

func sampleBannerPixel(t *testing.T, pngPath string) struct{ R, G, B, A uint32 } {
	t.Helper()
	f, err := os.Open(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	bounds := img.Bounds()
	x := (bounds.Min.X + bounds.Max.X) / 2
	// Default KeyDrop position_y is 0.86 of the 1080x1920 fullframe canvas.
	y := bounds.Min.Y + int(0.86*float64(bounds.Dy()))
	if y >= bounds.Max.Y {
		y = bounds.Max.Y - 1
	}
	r, g, b, a := img.At(x, y).RGBA()
	return struct{ R, G, B, A uint32 }{r >> 8, g >> 8, b >> 8, a >> 8}
}
