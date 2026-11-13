package httpapi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneBundledMapPicturesKeepsOnlyWhatCS2StillServes(t *testing.T) {
	root := t.TempDir()
	build := filepath.Join(root, "gameassets", "v1", "2000943-7714340-1794440084")
	files := map[string]bool{
		"radar/de_nuke_upper.png":   false,
		"radar/de_nuke_lower.png":   false,
		"radar/de_mirage_upper.png": false,
		"shot/de_inferno.png":       false,
		"radar/de_cbble_upper.png":  true,
		"shot/de_cbble.png":         true,
		"icon/ak47.svg":             true,
	}
	for name := range files {
		path := filepath.Join(build, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := PruneBundledMapPictures(root)
	if err != nil || removed != 4 {
		t.Fatalf("PruneBundledMapPictures = %d, %v; want 4 removed", removed, err)
	}
	for name, kept := range files {
		_, err := os.Stat(filepath.Join(build, filepath.FromSlash(name)))
		if (err == nil) != kept {
			t.Errorf("%s kept = %v, want %v", name, err == nil, kept)
		}
	}
}
