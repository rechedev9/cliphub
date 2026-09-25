package keydropbanner

import (
	"bytes"
	"image"
	"image/png"
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
	render := func(t *testing.T, style, code string) image.Image {
		t.Helper()
		out := filepath.Join(t.TempDir(), "plate.png")
		if err := CompositeWithCode("ffmpeg", FamilyKeyDrop, style, code, font, out); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			t.Fatalf("decode composited plate: %v", err)
		}
		return img
	}
	tests := []struct {
		style string
		code  string
	}{
		{style: StyleClassic, code: "OTROXYZ"},
		{style: StyleJcorko, code: "HUASO"},
	}
	for _, tt := range tests {
		t.Run(tt.style+"/"+tt.code, func(t *testing.T) {
			got := render(t, tt.style, tt.code)
			other := render(t, tt.style, "ZZ9")
			if got.Bounds() != other.Bounds() {
				t.Fatalf("bounds = %v and %v, want the same plate size", got.Bounds(), other.Bounds())
			}
			if samePixels(got, other) {
				t.Fatalf("plate with code %q is pixel-identical to one with code ZZ9; label was not drawn", tt.code)
			}
		})
	}
}

func samePixels(a, b image.Image) bool {
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false
			}
		}
	}
	return true
}
