package overlayassets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/storage"
)

func TestScreenshotImportPreservesPixelsAndImmutableContent(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pixels := image.NewNRGBA(image.Rect(0, 0, 436, 513))
	pixels.Set(12, 34, color.NRGBA{R: 35, G: 146, B: 247, A: 123})
	var input bytes.Buffer
	if err := png.Encode(&input, pixels); err != nil {
		t.Fatal(err)
	}
	a, err := Store(context.Background(), store, bytes.NewReader(input.Bytes()), "equipo 1.png")
	if err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(input.Bytes())
	if a.SHA256 != hex.EncodeToString(wantHash[:]) || a.Width != 436 || a.Height != 513 || a.ContentType != "image/png" {
		t.Fatalf("asset changed: %+v", a)
	}
	loaded, err := Load(store, a.ID)
	if err != nil || loaded != a {
		t.Fatalf("load: %+v %v", loaded, err)
	}
	r, err := store.Open(MediaKey(a.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	decoded, err := png.Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.At(12, 34) != pixels.At(12, 34) {
		t.Fatal("screenshot pixel changed")
	}
	b, err := Store(context.Background(), store, bytes.NewReader(input.Bytes()), "equipo 2.png")
	if err != nil || b.ID == a.ID || b.SHA256 != a.SHA256 {
		t.Fatalf("replacement identity: %+v %v", b, err)
	}
	if err := store.Put(MediaKey(a.ID), bytes.NewReader([]byte("changed"))); err != nil {
		t.Fatal(err)
	}
	if err := mediaassets.VerifyContent(context.Background(), store, MediaKey(a.ID), a.SHA256, MaxBytes); err == nil {
		t.Fatal("accepted replaced screenshot")
	}
}

func TestScreenshotImportRejectsCorruptUnsupportedAndOversizedInputs(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("not an image"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), bytes.Repeat([]byte{1}, MaxBytes+1)} {
		if _, _, err := Decode(data); err == nil {
			t.Fatal("accepted invalid screenshot")
		}
	}
	var oversized bytes.Buffer
	if err := png.Encode(&oversized, image.NewGray(image.Rect(0, 0, 8193, 1))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Decode(oversized.Bytes()); err == nil {
		t.Fatal("accepted excessive dimensions")
	}
	var valid bytes.Buffer
	if err := png.Encode(&valid, image.NewGray(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Decode(valid.Bytes()[:len(valid.Bytes())/2]); err == nil {
		t.Fatal("accepted truncated image with a valid header")
	}
}
