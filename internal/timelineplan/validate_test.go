package timelineplan

import (
	"strings"
	"testing"
)

func TestDocumentValidate(t *testing.T) {
	valid := func() Document {
		return Document{
			SchemaVersion: SchemaVersion,
			Canvas:        Canvas{Width: 1080, Height: 1920, FPS: 60},
			Tracks: []Track{{
				ID:   "v1",
				Kind: KindVideo,
				Items: []Item{{
					ID:            "clip-1",
					AssetID:       "11111111-1111-1111-1111-111111111111",
					TimelineStart: 0,
					SourceIn:      0,
					SourceOut:     2,
				}},
			}},
		}
	}
	t.Parallel()
	cases := []struct {
		name    string
		mutate  func(*Document)
		wantErr string
	}{
		{name: "ok", mutate: func(*Document) {}},
		{
			name:    "bad schema",
			mutate:  func(d *Document) { d.SchemaVersion = "9.9" },
			wantErr: `schema_version must be "1.0"`,
		},
		{
			name:    "bad canvas",
			mutate:  func(d *Document) { d.Canvas.Width = 720 },
			wantErr: "canvas must be",
		},
		{
			name:    "no tracks",
			mutate:  func(d *Document) { d.Tracks = nil },
			wantErr: "at least one track",
		},
		{
			name:    "audio only",
			mutate:  func(d *Document) { d.Tracks[0].Kind = KindAudio },
			wantErr: "at least one video track",
		},
		{
			name:    "bad asset id",
			mutate:  func(d *Document) { d.Tracks[0].Items[0].AssetID = "not-a-uuid" },
			wantErr: "asset_id must be a uuid",
		},
		{
			name:    "inverted source",
			mutate:  func(d *Document) { d.Tracks[0].Items[0].SourceOut = 0 },
			wantErr: "source_out must be greater than source_in",
		},
		{
			name:    "speed too fast",
			mutate:  func(d *Document) { d.Tracks[0].Items[0].Speed = 9 },
			wantErr: "speed must be between 0.25 and 3",
		},
		{
			name: "pip out of frame",
			mutate: func(d *Document) {
				d.Tracks[0].Items[0].Transform = &Transform{X: 0.8, Y: 0.8, Width: 0.5, Height: 0.5}
			},
			wantErr: "must stay within the canvas",
		},
		{
			name:    "unknown field already stripped by decode",
			mutate:  func(*Document) {},
			wantErr: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := valid()
			tc.mutate(&doc)
			err := doc.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	_, err := Decode([]byte(`{"schema_version":"1.0","canvas":{"width":1080,"height":1920,"fps":60},"tracks":[],"nope":true}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Decode() = %v, want unknown field", err)
	}
}

func TestValidateForRenderRequiresItems(t *testing.T) {
	t.Parallel()
	doc := DefaultDocument()
	if err := doc.Validate(); err != nil {
		t.Fatalf("draft Validate() = %v", err)
	}
	if err := doc.ValidateForRender(); err == nil || !strings.Contains(err.Error(), "no items") {
		t.Fatalf("ValidateForRender() = %v, want no items", err)
	}
}
