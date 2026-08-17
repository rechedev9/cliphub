package timelineplan

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	idPattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	assetIDPattern  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	musicKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

func DefaultDocument() Document {
	return Document{
		SchemaVersion: SchemaVersion,
		Canvas:        Canvas{Width: CanvasPortraitWidth, Height: CanvasPortraitHeight, FPS: DefaultFPS},
		Tracks:        []Track{{ID: "v1", Kind: KindVideo}},
	}
}

func (s Status) Validate() error {
	switch s {
	case StatusDraft, StatusRendering, StatusRendered, StatusFailed:
		return nil
	default:
		return fmt.Errorf("unknown editor project status %q", s)
	}
}

func (c Canvas) Validate() error {
	switch {
	case c.Width == CanvasPortraitWidth && c.Height == CanvasPortraitHeight:
	case c.Width == CanvasLandscapeWidth && c.Height == CanvasLandscapeHeight:
	default:
		return fmt.Errorf("canvas must be %dx%d or %dx%d", CanvasPortraitWidth, CanvasPortraitHeight, CanvasLandscapeWidth, CanvasLandscapeHeight)
	}
	if c.FPS != DefaultFPS {
		return fmt.Errorf("canvas fps must be %d", DefaultFPS)
	}
	return nil
}

func (d Document) Validate() error {
	if d.SchemaVersion != "" && d.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version must be %q", SchemaVersion)
	}
	if err := d.Canvas.Validate(); err != nil {
		return err
	}
	if len(d.Tracks) == 0 {
		return fmt.Errorf("timeline needs at least one track")
	}
	if len(d.Tracks) > maxTracks {
		return fmt.Errorf("timeline has at most %d tracks", maxTracks)
	}
	seenTracks := map[string]bool{}
	seenItems := map[string]Item{}
	hasVideo := false
	for _, track := range d.Tracks {
		if err := track.Validate(); err != nil {
			return err
		}
		if seenTracks[track.ID] {
			return fmt.Errorf("duplicate track id %q", track.ID)
		}
		seenTracks[track.ID] = true
		if track.Kind == KindVideo {
			hasVideo = true
		}
		for _, item := range track.Items {
			if _, exists := seenItems[item.ID]; exists {
				return fmt.Errorf("duplicate item id %q", item.ID)
			}
			seenItems[item.ID] = item
		}
	}
	if !hasVideo {
		return fmt.Errorf("timeline needs at least one video track")
	}
	if len(d.Overlays) > maxOverlays {
		return fmt.Errorf("timeline has at most %d text overlays", maxOverlays)
	}
	seenOverlays := map[string]bool{}
	duration := d.DurationSeconds()
	for _, overlay := range d.Overlays {
		if err := overlay.validate(duration); err != nil {
			return err
		}
		if seenOverlays[overlay.ID] {
			return fmt.Errorf("duplicate overlay id %q", overlay.ID)
		}
		seenOverlays[overlay.ID] = true
	}
	seenTransitions := map[string]bool{}
	for _, tr := range d.Transitions {
		if err := tr.validate(seenItems); err != nil {
			return err
		}
		if seenTransitions[tr.ID] {
			return fmt.Errorf("duplicate transition id %q", tr.ID)
		}
		seenTransitions[tr.ID] = true
	}
	if d.Music.Key != "" && !musicKeyPattern.MatchString(d.Music.Key) {
		return fmt.Errorf("invalid music key %q", d.Music.Key)
	}
	if d.Music.Volume < 0 || d.Music.Volume > 1 {
		return fmt.Errorf("music volume must be between 0 and 1")
	}
	return nil
}

func (d Document) ValidateForRender() error {
	if err := d.Validate(); err != nil {
		return err
	}
	items := 0
	for _, track := range d.Tracks {
		items += len(track.Items)
	}
	if items == 0 {
		return fmt.Errorf("timeline has no items")
	}
	return nil
}

func (t Track) Validate() error {
	if !idPattern.MatchString(t.ID) {
		return fmt.Errorf("invalid track id %q", t.ID)
	}
	if t.Kind != KindVideo && t.Kind != KindAudio {
		return fmt.Errorf("track %s kind must be video or audio", t.ID)
	}
	if len(t.Items) > maxItemsPerTrack {
		return fmt.Errorf("track %s has at most %d items", t.ID, maxItemsPerTrack)
	}
	for _, item := range t.Items {
		if err := item.Validate(t.ID); err != nil {
			return err
		}
	}
	return nil
}

func (it Item) Validate(trackID string) error {
	if !idPattern.MatchString(it.ID) {
		return fmt.Errorf("invalid item id %q", it.ID)
	}
	if !assetIDPattern.MatchString(it.AssetID) {
		return fmt.Errorf("item %s asset_id must be a uuid", it.ID)
	}
	if !finiteNonNeg(it.TimelineStart) {
		return fmt.Errorf("item %s timeline_start must be finite and >= 0", it.ID)
	}
	if !finiteNonNeg(it.SourceIn) {
		return fmt.Errorf("item %s source_in must be finite and >= 0", it.ID)
	}
	if math.IsNaN(it.SourceOut) || math.IsInf(it.SourceOut, 0) || it.SourceOut <= it.SourceIn {
		return fmt.Errorf("item %s source_out must be greater than source_in", it.ID)
	}
	if it.Speed != 0 && (math.IsNaN(it.Speed) || it.Speed < minSpeed || it.Speed > maxSpeed) {
		return fmt.Errorf("item %s speed must be between 0.25 and 3", it.ID)
	}
	if it.Volume != nil && (math.IsNaN(*it.Volume) || *it.Volume < 0 || *it.Volume > maxVolume) {
		return fmt.Errorf("item %s volume must be between 0 and 2", it.ID)
	}
	if math.IsNaN(it.FadeIn) || it.FadeIn < 0 || it.FadeIn > maxFadeSeconds {
		return fmt.Errorf("item %s fade_in must be between 0 and 5", it.ID)
	}
	if math.IsNaN(it.FadeOut) || it.FadeOut < 0 || it.FadeOut > maxFadeSeconds {
		return fmt.Errorf("item %s fade_out must be between 0 and 5", it.ID)
	}
	if it.FadeIn+it.FadeOut > it.OutputDuration() {
		return fmt.Errorf("item %s fades must fit within the item output duration", it.ID)
	}
	if it.Filter != FilterNone && it.Filter != FilterGrade {
		return fmt.Errorf("item %s filter must be empty or grade", it.ID)
	}
	if it.Transform != nil {
		if err := it.Transform.Validate(it.ID); err != nil {
			return err
		}
	}
	if trackID == "" {
		return fmt.Errorf("item %s is missing its track", it.ID)
	}
	return nil
}

func (tr Transform) Validate(itemID string) error {
	if !finiteUnit(tr.X) || !finiteUnit(tr.Y) || !finitePositive(tr.Width) || !finitePositive(tr.Height) {
		return fmt.Errorf("item %s transform must use finite normalized coordinates", itemID)
	}
	if tr.X+tr.Width > 1 || tr.Y+tr.Height > 1 {
		return fmt.Errorf("item %s transform must stay within the canvas", itemID)
	}
	if tr.Opacity != 0 && (math.IsNaN(tr.Opacity) || tr.Opacity < 0 || tr.Opacity > 1) {
		return fmt.Errorf("item %s opacity must be between 0 and 1", itemID)
	}
	return nil
}

func (o TextOverlay) validate(timelineDuration float64) error {
	if !idPattern.MatchString(o.ID) {
		return fmt.Errorf("invalid overlay id %q", o.ID)
	}
	text := strings.TrimSpace(o.Text)
	if text == "" {
		return fmt.Errorf("overlay %s text is required", o.ID)
	}
	if utf8.RuneCountInString(text) > maxTextRunes {
		return fmt.Errorf("overlay %s text must be at most %d characters", o.ID, maxTextRunes)
	}
	for _, r := range text {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("overlay %s text must not contain control characters", o.ID)
		}
	}
	if math.IsNaN(o.PositionY) || math.IsInf(o.PositionY, 0) || o.PositionY < minVerticalCenterY || o.PositionY > maxVerticalCenterY {
		return fmt.Errorf("overlay %s position_y must be between 0.025 and 0.975", o.ID)
	}
	if o.FontSize != 0 && (o.FontSize < minFontSize || o.FontSize > maxFontSize) {
		return fmt.Errorf("overlay %s font_size must be between 24 and 120", o.ID)
	}
	if !finiteNonNeg(o.StartSeconds) {
		return fmt.Errorf("overlay %s start_seconds must be finite and >= 0", o.ID)
	}
	end := timelineDuration
	if o.EndSeconds != nil {
		end = *o.EndSeconds
		if math.IsNaN(end) || math.IsInf(end, 0) || end <= o.StartSeconds {
			return fmt.Errorf("overlay %s end_seconds must be greater than start_seconds", o.ID)
		}
	}
	if timelineDuration > 0 && o.StartSeconds >= timelineDuration {
		return fmt.Errorf("overlay %s start_seconds must be inside the timeline", o.ID)
	}
	if timelineDuration > 0 && end > timelineDuration+0.001 {
		return fmt.Errorf("overlay %s end_seconds exceeds timeline duration", o.ID)
	}
	return nil
}

func (tr Transition) validate(items map[string]Item) error {
	if !idPattern.MatchString(tr.ID) {
		return fmt.Errorf("invalid transition id %q", tr.ID)
	}
	if tr.Kind != TransitionCut && tr.Kind != TransitionCrossfade {
		return fmt.Errorf("transition %s kind must be cut or crossfade", tr.ID)
	}
	if _, ok := items[tr.AfterItem]; !ok {
		return fmt.Errorf("transition %s after_item %q is unknown", tr.ID, tr.AfterItem)
	}
	if tr.Kind == TransitionCrossfade {
		if math.IsNaN(tr.Duration) || tr.Duration <= 0 || tr.Duration > maxFadeSeconds {
			return fmt.Errorf("transition %s duration must be in (0, 5]", tr.ID)
		}
	}
	return nil
}

func (it Item) EffectiveSpeed() float64 {
	if it.Speed == 0 {
		return 1
	}
	return it.Speed
}

func (it Item) OutputDuration() float64 {
	return (it.SourceOut - it.SourceIn) / it.EffectiveSpeed()
}

func (it Item) TimelineEnd() float64 {
	return it.TimelineStart + it.OutputDuration()
}

func (it Item) EffectiveOpacity() float64 {
	if it.Transform == nil || it.Transform.Opacity == 0 {
		return 1
	}
	return it.Transform.Opacity
}

func (it Item) ResolvedTransform() Transform {
	if it.Transform != nil {
		out := *it.Transform
		if out.Opacity == 0 {
			out.Opacity = 1
		}
		return out
	}
	return Transform{X: 0, Y: 0, Width: 1, Height: 1, Opacity: 1}
}

func (d Document) DurationSeconds() float64 {
	var end float64
	for _, track := range d.Tracks {
		for _, item := range track.Items {
			if itemEnd := item.TimelineEnd(); itemEnd > end {
				end = itemEnd
			}
		}
	}
	return end
}

func finiteNonNeg(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0
}

func finitePositive(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0
}

func finiteUnit(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1
}
