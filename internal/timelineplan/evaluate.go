package timelineplan

// Layer is one visible video item at a sample time. Preview and render share
// this evaluation so a WASM compositor and FFmpeg see the same stack.
type Layer struct {
	ItemID     string    `json:"item_id"`
	TrackID    string    `json:"track_id"`
	AssetID    string    `json:"asset_id"`
	SourceTime float64   `json:"source_time"`
	Transform  Transform `json:"transform"`
	Opacity    float64   `json:"opacity"`
	Filter     Filter    `json:"filter,omitempty"`
}

// TextSample is a text overlay visible at a sample time.
type TextSample struct {
	ID        string  `json:"id"`
	Text      string  `json:"text"`
	PositionY float64 `json:"position_y"`
	FontSize  int     `json:"font_size"`
}

// Sample is the compositor state at one output timestamp.
type Sample struct {
	Time     float64      `json:"time"`
	Duration float64      `json:"duration"`
	Layers   []Layer      `json:"layers"`
	Texts    []TextSample `json:"texts,omitempty"`
}

func Evaluate(doc Document, t float64) Sample {
	doc = Normalize(doc)
	out := Sample{Time: t, Duration: doc.DurationSeconds()}
	if t < 0 || t > out.Duration {
		return out
	}
	for _, track := range doc.Tracks {
		if track.Kind != KindVideo {
			continue
		}
		for _, item := range track.Items {
			if t < item.TimelineStart || t >= item.TimelineEnd() {
				continue
			}
			local := t - item.TimelineStart
			opacity := item.EffectiveOpacity() * fadeOpacity(local, item.OutputDuration(), item.FadeIn, item.FadeOut)
			if opacity <= 0 {
				continue
			}
			tf := item.ResolvedTransform()
			tf.Opacity = opacity
			out.Layers = append(out.Layers, Layer{
				ItemID:     item.ID,
				TrackID:    track.ID,
				AssetID:    item.AssetID,
				SourceTime: item.SourceIn + local*item.EffectiveSpeed(),
				Transform:  tf,
				Opacity:    opacity,
				Filter:     item.Filter,
			})
		}
	}
	for _, overlay := range doc.Overlays {
		end := out.Duration
		if overlay.EndSeconds != nil {
			end = *overlay.EndSeconds
		}
		if t < overlay.StartSeconds || t >= end {
			continue
		}
		size := overlay.FontSize
		if size == 0 {
			size = defaultFontSize
		}
		out.Texts = append(out.Texts, TextSample{
			ID:        overlay.ID,
			Text:      overlay.Text,
			PositionY: overlay.PositionY,
			FontSize:  size,
		})
	}
	return out
}

func fadeOpacity(local, duration, fadeIn, fadeOut float64) float64 {
	opacity := 1.0
	if fadeIn > 0 && local < fadeIn {
		opacity = local / fadeIn
	}
	if fadeOut > 0 && local > duration-fadeOut {
		tail := (duration - local) / fadeOut
		if tail < opacity {
			opacity = tail
		}
	}
	if opacity < 0 {
		return 0
	}
	if opacity > 1 {
		return 1
	}
	return opacity
}
