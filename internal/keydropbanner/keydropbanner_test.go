package keydropbanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAndNormalize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		style   string
		code    string
		wantErr string
	}{
		{name: "empty is off", style: "", code: ""},
		{name: "operator default code", style: "operator", code: ""},
		{name: "classic custom code", style: "classic", code: "zackcsgo"},
		{name: "unknown style", style: "neon", code: "ABC", wantErr: "unknown keydrop banner style"},
		{name: "bad code chars", style: "operator", code: "hi there", wantErr: "keydrop code"},
		{name: "code too long", style: "operator", code: "ABCDEFGHIJKLMNOPQ", wantErr: "at most"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errStyle := ValidateStyle(tt.style)
			errCode := ValidateCode(tt.code)
			var err error
			switch {
			case errStyle != nil:
				err = errStyle
			case errCode != nil:
				err = errCode
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestEffectiveCodeAndLabel(t *testing.T) {
	t.Parallel()
	if got := EffectiveCode(""); got != DefaultCode {
		t.Fatalf("EffectiveCode empty = %q, want %q", got, DefaultCode)
	}
	if got := EffectiveCode("  foo_bar "); got != "FOO_BAR" {
		t.Fatalf("EffectiveCode = %q, want FOO_BAR", got)
	}
	if got := DisplayLabel("test"); got != "CODE: TEST" {
		t.Fatalf("DisplayLabel = %q", got)
	}
}

func TestMaterializeWritesCachedPlates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, style := range Styles() {
		path, err := materializeAt(dir, style)
		if err != nil {
			t.Fatalf("materialize %s: %v", style.ID, err)
		}
		if filepath.Base(path) != style.FileName {
			t.Fatalf("path base = %q, want %q", filepath.Base(path), style.FileName)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("stat %s: %v size=%d", path, err, info.Size())
		}
		// Second call hits the checksum cache path.
		again, err := materializeAt(dir, style)
		if err != nil || again != path {
			t.Fatalf("rematerialize = %q %v, want %q", again, err, path)
		}
	}
}

func TestLookupAndStyles(t *testing.T) {
	t.Parallel()
	if len(Styles()) != 2 {
		t.Fatalf("Styles len = %d, want 2", len(Styles()))
	}
	if _, ok := Lookup("OPERATOR"); !ok {
		t.Fatal("Lookup OPERATOR failed")
	}
	if _, ok := Lookup("missing"); ok {
		t.Fatal("Lookup missing unexpectedly ok")
	}
}

func TestClassicCoverStaysInsideTextBay(t *testing.T) {
	t.Parallel()
	// The classic plate has a gift logo circle overlapping the left of the
	// brown bar. Cover must not start left of ~0.16 or it paints the logo
	// dark (black incomplete disc in the final MP4).
	style, ok := Lookup(StyleClassic)
	if !ok {
		t.Fatal("classic style missing")
	}
	tests := []struct {
		name string
		got  float64
		min  float64
		max  float64
	}{
		{name: "CoverX", got: style.CoverX, min: 0.16, max: 0.25},
		{name: "CoverY", got: style.CoverY, min: 0.50, max: 0.60},
		{name: "CoverW", got: style.CoverW, min: 0.50, max: 0.70},
		{name: "CoverH", got: style.CoverH, min: 0.15, max: 0.28},
		{name: "TextCenterY", got: style.TextCenterY, min: 0.58, max: 0.72},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got < tt.min || tt.got > tt.max {
				t.Fatalf("%s = %v, want in [%v, %v]", tt.name, tt.got, tt.min, tt.max)
			}
		})
	}
	// Cover right edge must leave the knife art free (~x ≥ 0.86).
	if right := style.CoverX + style.CoverW; right > 0.86 {
		t.Fatalf("CoverX+CoverW = %v, want ≤ 0.86 so knife art stays clear", right)
	}
}

func TestTargetWidth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want int
	}{
		{0, 594},
		{1080, 594},
		{1920, 720},
		{200, 280},
	}
	for _, tt := range tests {
		if got := TargetWidth(tt.in); got != tt.want {
			t.Fatalf("TargetWidth(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
