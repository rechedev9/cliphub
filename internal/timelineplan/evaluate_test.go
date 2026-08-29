package timelineplan

import (
	"math"
	"testing"
)

func TestEvaluate(t *testing.T) {
	t.Parallel()
	end := 4.0
	doc := Document{
		SchemaVersion: SchemaVersion,
		Canvas:        Canvas{Width: 1080, Height: 1920, FPS: 60},
		Tracks: []Track{
			{
				ID:   "v1",
				Kind: KindVideo,
				Items: []Item{{
					ID:            "base",
					AssetID:       "11111111-1111-1111-1111-111111111111",
					TimelineStart: 0,
					SourceIn:      1,
					SourceOut:     5,
					FadeIn:        0.5,
				}},
			},
			{
				ID:   "v2",
				Kind: KindVideo,
				Items: []Item{{
					ID:            "pip",
					AssetID:       "22222222-2222-2222-2222-222222222222",
					TimelineStart: 1,
					SourceIn:      0,
					SourceOut:     1,
					Speed:         2,
					Transform:     &Transform{X: 0.6, Y: 0.05, Width: 0.35, Height: 0.25, Opacity: 0.8},
				}},
			},
		},
		Overlays: []TextOverlay{{
			ID:           "title",
			Text:         "ACE",
			PositionY:    0.1,
			StartSeconds: 0.2,
			EndSeconds:   &end,
			FontSize:     72,
		}},
	}
	cases := []struct {
		name       string
		t          float64
		wantLayers int
		wantTexts  int
		wantSrc    float64
		wantOp     float64
		wantItem   string
	}{
		{name: "before start", t: -0.1, wantLayers: 0},
		{name: "fade in base", t: 0.25, wantLayers: 1, wantSrc: 1.25, wantOp: 0.5, wantItem: "base", wantTexts: 1},
		{name: "full base plus pip", t: 1.1, wantLayers: 2, wantSrc: 2.1, wantOp: 1, wantItem: "base", wantTexts: 1},
		{name: "after pip", t: 1.6, wantLayers: 1, wantSrc: 2.6, wantOp: 1, wantItem: "base", wantTexts: 1},
		{name: "after end", t: 5, wantLayers: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Evaluate(doc, tc.t)
			if len(got.Layers) != tc.wantLayers {
				t.Fatalf("layers = %d, want %d (%+v)", len(got.Layers), tc.wantLayers, got.Layers)
			}
			if len(got.Texts) != tc.wantTexts {
				t.Fatalf("texts = %d, want %d", len(got.Texts), tc.wantTexts)
			}
			if tc.wantLayers == 0 {
				return
			}
			layer := got.Layers[0]
			if layer.ItemID != tc.wantItem {
				t.Fatalf("first layer = %q, want %q", layer.ItemID, tc.wantItem)
			}
			if math.Abs(layer.SourceTime-tc.wantSrc) > 1e-9 {
				t.Fatalf("source_time = %v, want %v", layer.SourceTime, tc.wantSrc)
			}
			if math.Abs(layer.Opacity-tc.wantOp) > 1e-9 {
				t.Fatalf("opacity = %v, want %v", layer.Opacity, tc.wantOp)
			}
		})
	}
}

func TestFingerprintStableAcrossUpdatedAt(t *testing.T) {
	t.Parallel()
	doc := DefaultDocument()
	doc.Tracks[0].Items = []Item{{
		ID: "a", AssetID: "11111111-1111-1111-1111-111111111111",
		SourceOut: 1,
	}}
	a, err := Fingerprint(doc)
	if err != nil {
		t.Fatal(err)
	}
	doc.UpdatedAt = doc.UpdatedAt.Add(60e9)
	b, err := Fingerprint(doc)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("fingerprint changed with UpdatedAt: %s vs %s", a, b)
	}
	doc.Tracks[0].Items[0].SourceOut = 2
	c, err := Fingerprint(doc)
	if err != nil {
		t.Fatal(err)
	}
	if c == a {
		t.Fatal("fingerprint did not change after a render-affecting edit")
	}
}
