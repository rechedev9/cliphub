package keydropbanner

import (
	"bytes"
	"image/png"
	"os"
	"testing"
)

// Studio offers every KeyDrop style as a chip. A missing plate used to fail
// the whole stream-to-short job with "plate is missing" (Jcorko once, then
// Tigerr, whose PNG never landed with its catalog entry).
func TestStudioOfferedKeyDropPlatesMaterialize(t *testing.T) {
	t.Parallel()
	for _, id := range []string{StyleOperator, StyleClassic, StyleTigerr, StyleJcorko} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			style, ok := Lookup(FamilyKeyDrop, id)
			if !ok {
				t.Fatalf("%s is offered in Studio but missing from the catalog", id)
			}
			if len(style.Data) == 0 {
				t.Fatalf("keydrop banner style %q plate is missing", style.ID)
			}
			cfg, err := png.DecodeConfig(bytes.NewReader(style.Data))
			if err != nil {
				t.Fatalf("%s plate is not a PNG: %v", id, err)
			}
			if cfg.Width < 800 || cfg.Height < 300 {
				t.Fatalf("%s plate size %dx%d is too small for a lower-third", id, cfg.Width, cfg.Height)
			}
			dir := t.TempDir()
			path, err := materializeAt(dir, style)
			if err != nil {
				t.Fatalf("Materialize %s: %v", id, err)
			}
			info, err := os.Stat(path)
			if err != nil || info.Size() < 1000 {
				t.Fatalf("materialized %s plate: %v size=%d", id, err, info.Size())
			}
		})
	}
}
