package demooverlay

import (
	"bytes"
	"encoding/json"
	"image/png"
	"strings"
	"testing"
)

func TestEmbeddedLayoutManifest(t *testing.T) {
	spec, err := decodeLayoutSpec(bytes.NewReader(faceitLayoutJSON))
	if err != nil {
		t.Fatalf("decode embedded FACEIT layout: %v", err)
	}
	if spec.Intro.Panel.CenterGap < 700 {
		t.Fatalf("intro center gap = %d, want native HUD channel", spec.Intro.Panel.CenterGap)
	}
	if rectsOverlap(spec.Intro.POV.Rect, spec.Intro.Level.Rect) {
		t.Fatal("intro POV and FACEIT level badges overlap")
	}
}

func TestEmbeddedLayoutSchemaIsDraft202012(t *testing.T) {
	var schema struct {
		Dialect string `json:"$schema"`
		ID      string `json:"$id"`
	}
	if err := json.Unmarshal(layoutSchemaJSON, &schema); err != nil {
		t.Fatalf("decode embedded layout schema: %v", err)
	}
	if schema.Dialect != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("schema dialect = %q", schema.Dialect)
	}
	if schema.ID == "" {
		t.Fatal("schema $id is required")
	}
}

func TestDecodeLayoutSpecRejectsUnknownField(t *testing.T) {
	raw := bytes.Replace(faceitLayoutJSON, []byte(`"schema_version"`), []byte(`"unexpected":true,"schema_version"`), 1)
	_, err := decodeLayoutSpec(bytes.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("decode error = %v, want unknown field", err)
	}
}

func TestValidateLayoutSpecRejectsInvalidGeometry(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*layoutSpec)
		wantErr string
	}{
		{
			name: "POV overlaps level",
			mutate: func(spec *layoutSpec) {
				spec.Intro.Level.Rect.X = spec.Intro.POV.Rect.X + spec.Intro.POV.Rect.Width - 1
			},
			wantErr: "POV and level badges overlap",
		},
		{
			name: "declared center gap differs",
			mutate: func(spec *layoutSpec) {
				spec.Intro.Panel.CenterGap++
			},
			wantErr: "intro center gap",
		},
		{
			name: "duplicate outro column",
			mutate: func(spec *layoutSpec) {
				spec.Outro.Columns[1].ID = spec.Outro.Columns[0].ID
			},
			wantErr: "invalid or duplicated",
		},
		{
			name: "outro column outside table",
			mutate: func(spec *layoutSpec) {
				last := len(spec.Outro.Columns) - 1
				spec.Outro.Columns[last].X = FrameWidth
			},
			wantErr: "exceeds or overlaps",
		},
		{
			name: "bitmap requires a file",
			mutate: func(spec *layoutSpec) {
				spec.Assets.IntroChrome.Renderer = "bitmap"
			},
			wantErr: "bitmap file is required",
		},
		{
			name: "asset requires alpha",
			mutate: func(spec *layoutSpec) {
				spec.Assets.OutroChrome.RequiresAlpha = false
			},
			wantErr: "must require alpha",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := cloneLayoutSpec(t)
			tc.mutate(&spec)
			err := validateLayoutSpec(spec)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("validation error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestFaceitOutroGridColumnsFollowsManifestOrder(t *testing.T) {
	want := []string{ColLevel, ColRating, ColMVP}
	got := faceitOutroGridColumns([]string{ColMVP, ColRating, ColLevel})
	if !slicesEqualStrings(got, want) {
		t.Fatalf("columns = %v, want %v", got, want)
	}
}

func TestGeneratedIntroChromePreservesGameplayChannel(t *testing.T) {
	raw, err := renderIntroChromePNG(Document{Source: SourceFACEIT})
	if err != nil {
		t.Fatalf("render intro chrome: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode intro chrome: %v", err)
	}
	_, _, _, centerAlpha := img.At(FrameWidth/2, FrameHeight/2).RGBA()
	if centerAlpha != 0 {
		t.Fatalf("gameplay channel alpha = %d, want transparent", centerAlpha)
	}
	_, _, _, panelAlpha := img.At(defaultFaceitLayout.Intro.Panel.LeftX+20, defaultFaceitLayout.Intro.Panel.Top+20).RGBA()
	if panelAlpha == 0 {
		t.Fatal("intro panel is transparent")
	}
}

func cloneLayoutSpec(t *testing.T) layoutSpec {
	t.Helper()
	raw, err := json.Marshal(defaultFaceitLayout)
	if err != nil {
		t.Fatalf("marshal layout clone: %v", err)
	}
	var spec layoutSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("unmarshal layout clone: %v", err)
	}
	return spec
}

func slicesEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
