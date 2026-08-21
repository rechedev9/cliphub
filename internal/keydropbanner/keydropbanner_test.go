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
		{name: "tigerr custom code", style: "tigerr", code: "tigerr"},
		{name: "jcorko custom code", style: "jcorko", code: "jcorko"},
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
	if got := DisplayLabelFor(StyleJcorko, "otro"); got != "CODIGO: OTRO" {
		t.Fatalf("DisplayLabelFor jcorko = %q", got)
	}
	if got := DisplayLabelFor(StyleTigerr, "tiger"); got != "CODE: TIGER" {
		t.Fatalf("DisplayLabelFor tigerr = %q", got)
	}
}

func TestMaterializeWritesCachedPlates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, style := range Styles() {
		if len(style.Data) == 0 {
			t.Logf("skip materialize %s: plate not on disk", style.ID)
			continue
		}
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
		again, err := materializeAt(dir, style)
		if err != nil || again != path {
			t.Fatalf("rematerialize = %q %v, want %q", again, err, path)
		}
	}
}

func TestLookupAndStyles(t *testing.T) {
	t.Parallel()
	if len(Styles()) != 4 {
		t.Fatalf("Styles len = %d, want 4", len(Styles()))
	}
	if _, ok := Lookup(StyleTigerr); !ok {
		t.Fatal("Lookup tigerr failed")
	}
	if _, ok := Lookup(StyleJcorko); !ok {
		t.Fatal("Lookup jcorko failed")
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
	if right := style.CoverX + style.CoverW; right > 0.86 {
		t.Fatalf("CoverX+CoverW = %v, want ≤ 0.86 so knife art stays clear", right)
	}
}

func TestTigerrAndJcorkoCoverStayInsideBar(t *testing.T) {
	t.Parallel()
	for _, id := range []string{StyleTigerr, StyleJcorko} {
		style, ok := Lookup(id)
		if !ok {
			t.Fatalf("%s style missing", id)
		}
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			if style.CoverX < 0.12 || style.CoverX > 0.28 {
				t.Fatalf("CoverX = %v, want in [0.12, 0.28]", style.CoverX)
			}
			if style.CoverW < 0.45 || style.CoverW > 0.70 {
				t.Fatalf("CoverW = %v, want in [0.45, 0.70]", style.CoverW)
			}
			if right := style.CoverX + style.CoverW; right > 0.88 {
				t.Fatalf("CoverX+CoverW = %v, want ≤ 0.88 so side art stays clear", right)
			}
		})
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
