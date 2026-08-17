package timelineplan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEmptyItemsMarshalAsArray(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		doc  Document
	}{
		{name: "default", doc: DefaultDocument()},
		{name: "normalize nil items", doc: Normalize(Document{
			SchemaVersion: SchemaVersion,
			Canvas:        Canvas{Width: CanvasPortraitWidth, Height: CanvasPortraitHeight, FPS: DefaultFPS},
			Tracks:        []Track{{ID: "v1", Kind: KindVideo}},
		})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw, err := json.Marshal(tc.doc)
			if err != nil {
				t.Fatal(err)
			}
			got := string(raw)
			if strings.Contains(got, `"items":null`) {
				t.Fatalf("marshaled null items: %s", got)
			}
			if !strings.Contains(got, `"items":[]`) {
				t.Fatalf("missing empty items array: %s", got)
			}
		})
	}
}

func TestDecodeEmptyItemsStayArray(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
	}{
		{
			name: "null items",
			raw:  `{"schema_version":"1.0","canvas":{"width":1080,"height":1920,"fps":60},"tracks":[{"id":"v1","kind":"video","items":null}]}`,
		},
		{
			name: "empty array",
			raw:  `{"schema_version":"1.0","canvas":{"width":1080,"height":1920,"fps":60},"tracks":[{"id":"v1","kind":"video","items":[]}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc, err := Decode([]byte(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			if doc.Tracks[0].Items == nil {
				t.Fatal("decoded items is nil")
			}
			if len(doc.Tracks[0].Items) != 0 {
				t.Fatalf("decoded items = %#v", doc.Tracks[0].Items)
			}
			out, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			got := string(out)
			if strings.Contains(got, `"items":null`) {
				t.Fatalf("remarshaled null items: %s", got)
			}
			if !strings.Contains(got, `"items":[]`) {
				t.Fatalf("missing empty items array: %s", got)
			}
		})
	}
}
