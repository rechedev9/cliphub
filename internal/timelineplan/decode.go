package timelineplan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

func Decode(body []byte) (Document, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var doc Document
	if err := decoder.Decode(&doc); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return Document{}, fmt.Errorf("decode timeline: the plan must be a JSON object, found %s", typeErr.Value)
		}
		return Document{}, fmt.Errorf("decode timeline: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return Document{}, err
	}
	return Normalize(doc), nil
}

func Normalize(doc Document) Document {
	if doc.SchemaVersion == "" {
		doc.SchemaVersion = SchemaVersion
	}
	if doc.Canvas.Width == 0 && doc.Canvas.Height == 0 {
		doc.Canvas = Canvas{Width: CanvasPortraitWidth, Height: CanvasPortraitHeight, FPS: DefaultFPS}
	}
	if doc.Canvas.FPS == 0 {
		doc.Canvas.FPS = DefaultFPS
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = time.Now().UTC()
	}
	if len(doc.Tracks) > 0 {
		doc.Tracks = append([]Track(nil), doc.Tracks...)
	}
	for i := range doc.Tracks {
		doc.Tracks[i].ID = strings.TrimSpace(doc.Tracks[i].ID)
		doc.Tracks[i].Kind = TrackKind(strings.TrimSpace(string(doc.Tracks[i].Kind)))
		if len(doc.Tracks[i].Items) > 0 {
			doc.Tracks[i].Items = append([]Item(nil), doc.Tracks[i].Items...)
		}
		for j := range doc.Tracks[i].Items {
			doc.Tracks[i].Items[j] = normalizeItem(doc.Tracks[i].Items[j])
		}
	}
	if len(doc.Overlays) > 0 {
		doc.Overlays = append([]TextOverlay(nil), doc.Overlays...)
	}
	for i := range doc.Overlays {
		doc.Overlays[i].ID = strings.TrimSpace(doc.Overlays[i].ID)
		doc.Overlays[i].Text = strings.TrimSpace(doc.Overlays[i].Text)
	}
	if len(doc.Transitions) > 0 {
		doc.Transitions = append([]Transition(nil), doc.Transitions...)
	}
	for i := range doc.Transitions {
		doc.Transitions[i].ID = strings.TrimSpace(doc.Transitions[i].ID)
		doc.Transitions[i].AfterItem = strings.TrimSpace(doc.Transitions[i].AfterItem)
		doc.Transitions[i].Kind = TransitionKind(strings.TrimSpace(string(doc.Transitions[i].Kind)))
		if doc.Transitions[i].Kind == TransitionCut {
			doc.Transitions[i].Duration = 0
		}
	}
	doc.Music.Key = strings.TrimSpace(doc.Music.Key)
	if doc.Music.Key == "" {
		doc.Music.Volume = 0
	} else if doc.Music.Volume == 0 {
		doc.Music.Volume = 0.25
	}
	return applyTransitions(doc)
}

func normalizeItem(item Item) Item {
	item.ID = strings.TrimSpace(item.ID)
	item.AssetID = strings.ToLower(strings.TrimSpace(item.AssetID))
	item.Filter = Filter(strings.TrimSpace(string(item.Filter)))
	if item.Speed == 1 {
		item.Speed = 0
	}
	return item
}

func applyTransitions(doc Document) Document {
	byID := map[string]Transition{}
	for _, tr := range doc.Transitions {
		if tr.Kind == TransitionCrossfade && tr.Duration > 0 {
			byID[tr.AfterItem] = tr
		}
	}
	if len(byID) == 0 {
		return doc
	}
	for i := range doc.Tracks {
		items := doc.Tracks[i].Items
		for j := range items {
			tr, ok := byID[items[j].ID]
			if !ok || j+1 >= len(items) {
				continue
			}
			if items[j].FadeOut == 0 {
				items[j].FadeOut = tr.Duration
			}
			if items[j+1].FadeIn == 0 {
				items[j+1].FadeIn = tr.Duration
			}
		}
	}
	return doc
}
