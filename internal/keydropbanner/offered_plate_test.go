package keydropbanner

import (
	"bytes"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/mediafont"
)

// Studio offers Jcorko as a KeyDrop chip. A missing plate used to fail the
// whole stream-to-short job with "plate is missing".
func TestStudioOfferedJcorkoPlateMaterializes(t *testing.T) {
	t.Parallel()
	style, ok := Lookup(StyleJcorko)
	if !ok {
		t.Fatal("jcorko is offered in Studio but missing from the catalog")
	}
	if len(style.Data) == 0 {
		t.Fatalf("keydrop banner style %q plate is missing", style.ID)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(style.Data))
	if err != nil {
		t.Fatalf("jcorko plate is not a PNG: %v", err)
	}
	if cfg.Width < 800 || cfg.Height < 300 {
		t.Fatalf("jcorko plate size %dx%d is too small for a lower-third", cfg.Width, cfg.Height)
	}
	dir := t.TempDir()
	path, err := materializeAt(dir, style)
	if err != nil {
		t.Fatalf("Materialize jcorko: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() < 1000 {
		t.Fatalf("materialized jcorko plate: %v size=%d", err, info.Size())
	}
}

func TestCompositeOfferedJcorkoStyleWithCode(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	filterList, err := exec.Command("ffmpeg", "-hide_banner", "-filters").Output()
	if err != nil {
		t.Skipf("ffmpeg -filters failed: %v", err)
	}
	if !bytes.Contains(filterList, []byte("drawtext")) {
		t.Skip("ffmpeg build has no drawtext filter")
	}
	font, err := mediafont.Materialize()
	if err != nil {
		t.Fatalf("font: %v", err)
	}
	out := filepath.Join(t.TempDir(), "jcorko-huaso.png")
	if err := CompositeWithCode("ffmpeg", StyleJcorko, "HUASO", font, out); err != nil {
		t.Fatalf("composite jcorko HUASO: %v", err)
	}
	info, err := os.Stat(out)
	if err != nil || info.Size() < 1000 {
		t.Fatalf("composited jcorko plate: %v size=%d", err, info.Size())
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("/tmp/kd-jcorko-huaso.png", raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
