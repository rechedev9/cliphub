package mapdata

import (
	"bytes"
	"image/png"
	"io/fs"
	"testing"
)

func TestBundledRadarExists(t *testing.T) {
	cases := []struct {
		mapName, layer string
		wantNil        bool
	}{
		{"de_dust2", "upper", false},
		{"de_mirage", "upper", false},
		{"de_inferno", "upper", false},
		{"de_ancient", "upper", false},
		{"de_anubis", "upper", false},
		{"de_cache", "upper", false},
		{"de_overpass", "upper", false},
		{"de_train", "upper", false},
		{"de_nuke", "upper", false},
		{"de_nuke", "lower", false},
		{"de_vertigo", "upper", false},
		{"de_vertigo", "lower", false},
		{"de_mirage", "lower", true},
		{"de_cbble", "upper", true},
	}
	for _, tc := range cases {
		got := Radar(tc.mapName, tc.layer)
		if (got == nil) != tc.wantNil {
			t.Errorf("Radar(%q, %q): nil=%v, want nil=%v", tc.mapName, tc.layer, got == nil, tc.wantNil)
		}
	}
}

func TestBundledShotExists(t *testing.T) {
	cases := []struct {
		mapName string
		wantNil bool
	}{
		{"de_dust2", false},
		{"de_mirage", false},
		{"de_inferno", false},
		{"de_ancient", false},
		{"de_anubis", false},
		{"de_cache", false},
		{"de_overpass", false},
		{"de_train", false},
		{"de_nuke", false},
		{"de_vertigo", false},
		{"de_cbble", true},
	}
	for _, tc := range cases {
		got := Shot(tc.mapName)
		if (got == nil) != tc.wantNil {
			t.Errorf("Shot(%q): nil=%v, want nil=%v", tc.mapName, got == nil, tc.wantNil)
		}
	}
}

// The 2D views draw a radar across the whole calibration square, so a radar
// that is not square would stretch the map under the players.
func TestBundledRadarsAreSquare(t *testing.T) {
	names, err := fs.Glob(radarFiles, "radars/*.png")
	if err != nil || len(names) == 0 {
		t.Fatalf("radars: %v, %d files", err, len(names))
	}
	for _, name := range names {
		data, err := radarFiles.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if cfg.Width != 2048 || cfg.Height != 2048 {
			t.Errorf("%s is %dx%d, want 2048x2048", name, cfg.Width, cfg.Height)
		}
	}
}
