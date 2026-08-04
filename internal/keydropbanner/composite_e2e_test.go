package keydropbanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rechedev9/tickcut/internal/mediafont"
)

func TestCompositeWithCodeWritesCustomLabel(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	font, err := mediafont.Materialize()
	if err != nil {
		t.Fatalf("font: %v", err)
	}
	out := filepath.Join(t.TempDir(), "plate.png")
	if err := CompositeWithCode("ffmpeg", StyleClassic, "OTROXYZ", font, out); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(out)
	if err != nil || info.Size() < 1000 {
		t.Fatalf("bad out: %v size=%v", err, info)
	}
	// copy
	b, _ := os.ReadFile(out)
	_ = os.WriteFile("/tmp/kd-demo-composite.png", b, 0o644)
}
