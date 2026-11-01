package main

import (
	"bytes"
	"image/jpeg"
	"os"
	"testing"
)

func TestHUDThumbnailIsSmallJPEG(t *testing.T) {
	data, err := os.ReadFile("../../web/public/hud/arena.webp")
	if err != nil {
		t.Skipf("preview not available: %v", err)
	}
	thumb, err := hudThumbnail(data)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("not a JPEG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != thumbWidth || b.Dy() != thumbHeight {
		t.Errorf("size %dx%d", b.Dx(), b.Dy())
	}
	if len(thumb) > 64<<10 {
		t.Errorf("thumbnail is %d bytes, want under 64 KiB", len(thumb))
	}
	if _, err := hudThumbnail([]byte("RIFF....WEBPjunk")); err == nil {
		t.Error("accepted a corrupt WebP")
	}
}
