package keydropbanner

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/mediafont"
)

func TestCompositeWithCodeWritesCustomLabel(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	// A minimal ffmpeg build (e.g. one compiled without libfreetype) has no
	// drawtext filter, so the composite plate cannot be rendered. Skip rather
	// than fail, matching the ffmpeg availability guard above.
	filterList, err := exec.Command("ffmpeg", "-hide_banner", "-filters").Output()
	if err != nil {
		t.Skipf("ffmpeg -filters failed: %v, skipping composite e2e", err)
	}
	if !bytes.Contains(filterList, []byte("drawtext")) {
		t.Skip("ffmpeg build has no drawtext filter, skipping composite e2e")
	}
	font, err := mediafont.Materialize()
	if err != nil {
		t.Fatalf("font: %v", err)
	}
	tests := []struct {
		style string
		code  string
		copy  string
	}{
		{style: StyleClassic, code: "OTROXYZ", copy: "/tmp/kd-demo-composite.png"},
		{style: StyleJcorko, code: "HUASO", copy: "/tmp/kd-jcorko-composite.png"},
	}
	for _, tt := range tests {
		t.Run(tt.style+"/"+tt.code, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "plate.png")
			if err := CompositeWithCode("ffmpeg", tt.style, tt.code, font, out); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(out)
			if err != nil || info.Size() < 1000 {
				t.Fatalf("bad out: %v size=%v", err, info)
			}
			b, _ := os.ReadFile(out)
			_ = os.WriteFile(tt.copy, b, 0o644)
		})
	}
}
