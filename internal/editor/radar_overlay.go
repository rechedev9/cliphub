package editor

import (
	"fmt"
	"image"
	_ "image/png" // the affiliate plate is a PNG
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/radaroverlay"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/subtitles"
)

const (
	radarFadeSeconds = 0.15
	// radarJoinSeconds joins two plays' plates across a cut shorter than this.
	radarJoinSeconds = 0.25
)

// RadarOverlay is ClipHub's radar of a Short: the worker's track, the map art of each level, the
// chosen corner, size and opacity, and each segment's capture start tick.
type RadarOverlay struct {
	TrackPath      string         `json:"track_path"`
	ImagePath      string         `json:"image_path,omitempty"`
	LowerImagePath string         `json:"lower_image_path,omitempty"`
	Position       string         `json:"position,omitempty"`
	Size           string         `json:"size,omitempty"`
	Opacity        float64        `json:"opacity,omitempty"`
	Starts         map[string]int `json:"starts"`
}

type radarOverlayInput struct {
	cfg  Config
	plan recording.RecordingPlan
}

// prepareRadarOverlay checks the radar flags and the worker's track. The radar draws only over a capture
// without the CS2 HUD, whose own radar would show twice; the capture itself is never changed for it.
func prepareRadarOverlay(in radarOverlayInput) (*RadarOverlay, []string, error) {
	cfg, plan := in.cfg, in.plan
	if !cfg.Radar {
		return nil, nil, nil
	}
	if err := radaroverlay.ValidatePosition(cfg.RadarPosition); err != nil {
		return nil, nil, err
	}
	if err := radaroverlay.ValidateSize(cfg.RadarSize); err != nil {
		return nil, nil, err
	}
	if err := radaroverlay.ValidateOpacity(cfg.RadarOpacity); err != nil {
		return nil, nil, err
	}
	if mode := plan.Stream.HUDMode; mode != recording.HUDModeClean && mode != recording.HUDModeDeathnotices {
		return nil, []string{"radar left out: the capture shows the CS2 HUD and its own radar"}, nil
	}
	if strings.TrimSpace(cfg.RadarTrackPath) == "" {
		return nil, []string{"radar left out: no radar track"}, nil
	}
	track, err := radaroverlay.Load(cfg.RadarTrackPath)
	if err != nil {
		return nil, nil, err
	}
	if track.TickRate != plan.Tickrate || track.DemoSHA256 != plan.DemoSHA256 {
		return nil, nil, fmt.Errorf("radar track is for another demo or tick rate")
	}
	inTrack := map[string]bool{}
	for _, s := range track.Segments {
		inTrack[s.ID] = true
	}
	out := &RadarOverlay{
		TrackPath: cfg.RadarTrackPath, ImagePath: cfg.RadarImagePath, LowerImagePath: cfg.RadarLowerImagePath,
		Position: cfg.RadarPosition, Size: cfg.RadarSize, Opacity: cfg.RadarOpacity, Starts: map[string]int{},
	}
	for _, segment := range plan.Segments {
		if inTrack[segment.ID] {
			out.Starts[segment.ID] = recording.EffectiveRecordStartTick(segment, plan.Tickrate)
		}
	}
	if len(out.Starts) == 0 {
		return nil, []string{"radar left out: the radar track does not cover these plays"}, nil
	}
	return out, nil, nil
}

// radarSpans place the gameplay parts on the edited timeline, after the hook title or the intro
// and before the closing stats card and the outro; victim replays show no radar.
func radarSpans(short ShortEdit, track radaroverlay.Track) []radaroverlay.Span {
	r := short.Overlays.Radar
	bySegment := map[string]radaroverlay.Segment{}
	for _, s := range track.Segments {
		bySegment[s.ID] = s
	}
	floor, ceiling := 0.0, short.DurationSeconds
	if short.HookText || short.Intro {
		floor = transitionEnd(short, winProbHookSeconds)
	}
	if short.Outro {
		ceiling = transitionStart(short, 1.6)
	}
	if cardStart, _, ok := statsCardWindow(short); ok {
		ceiling = min(ceiling, cardStart)
	}
	var spans []radaroverlay.Span
	for _, window := range winProbWindows(short) {
		seg, ok := bySegment[window.segmentID]
		start, started := r.Starts[window.segmentID]
		if !ok || !started {
			continue
		}
		from, until := max(window.offset, floor), min(window.until, ceiling)
		if until-from < 0.05 {
			continue
		}
		spans = append(spans, radaroverlay.Span{
			Segment: seg, StartTick: start, Offset: window.offset, From: from, Until: until, Level: seg.POVLevel(track.Calibration),
		})
	}
	return spans
}

// writeRadarOverlay bakes the radar plates and writes the moving layer's script beside the output.
// Each plate is an image effect; the script is drawn on top of it, after the other image overlays.
func writeRadarOverlay(short *ShortEdit) error {
	short.RadarASS, short.RadarFontsDir = "", ""
	r := short.Overlays.Radar
	if r == nil || short.FullDemo != nil {
		return nil
	}
	track, err := radaroverlay.Load(r.TrackPath)
	if err != nil {
		return err
	}
	spans := radarSpans(*short, track)
	if len(spans) == 0 {
		return nil
	}
	width, height := outputDimensions(*short)
	box := radarBox(*short)
	opacity := radaroverlay.EffectiveOpacity(r.Opacity)
	script, err := radaroverlay.Script(radaroverlay.Layout{
		Width: width, Height: height, TickRate: track.TickRate, Calibration: track.Calibration, Box: box, Opacity: opacity,
	}, spans)
	if err != nil || script == "" {
		return err
	}
	plates := map[string]string{}
	for _, group := range radarPlateGroups(spans) {
		path, ok := plates[group.level]
		if !ok {
			path = fmt.Sprintf("%s.radar-%s.png", short.Output, group.level)
			if err := radaroverlay.WritePlate(path, radarArt(*r, group.level), box.W, opacity); err != nil {
				return fmt.Errorf("radar plate: %w", err)
			}
			plates[group.level] = path
		}
		short.Effects = append(short.Effects, Effect{
			Type: EffectImage, Path: path, X: strconv.Itoa(box.X), Y: strconv.Itoa(box.Y), Width: box.W, Height: box.H,
			StartSeconds: group.from, EndSeconds: group.until, FadeInSeconds: radarFadeSeconds, FadeOutSeconds: radarFadeSeconds,
			Source: "radar",
		})
	}
	path := short.Output + ".radar.ass"
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		return err
	}
	fonts, err := mediafont.MaterializeHUD()
	if err != nil {
		return fmt.Errorf("radar fonts: %w", err)
	}
	short.RadarASS, short.RadarFontsDir = path, fonts
	return nil
}

// radarArt is the bundled map picture of a level; a lower level without its own picture uses the upper one.
func radarArt(r RadarOverlay, level string) []byte {
	path := r.ImagePath
	if level == radarmap.LevelLower && r.LowerImagePath != "" {
		path = r.LowerImagePath
	}
	if path == "" {
		return nil
	}
	// #nosec G304 -- the render worker wrote the map art for this job.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return raw
}

type radarPlateGroup struct {
	level       string
	from, until float64
}

// radarPlateGroups joins consecutive plays on one level so a quick cut does not blink the plate.
func radarPlateGroups(spans []radaroverlay.Span) []radarPlateGroup {
	var out []radarPlateGroup
	for _, s := range spans {
		if n := len(out); n > 0 && out[n-1].level == s.Level && s.From-out[n-1].until < radarJoinSeconds {
			out[n-1].until = max(out[n-1].until, s.Until)
			continue
		}
		out = append(out, radarPlateGroup{level: s.Level, from: s.From, until: s.Until})
	}
	return out
}

func radarBox(short ShortEdit) radaroverlay.Rect {
	width, height := outputDimensions(short)
	avoid := radarAvoidRects(short, height)
	if short.SubtitlesASS != "" && height > width {
		block := subtitles.PlacementOf(width, height).Box
		avoid = append(avoid, radaroverlay.Rect{X: block.Left, Y: block.Top, W: block.Right - block.Left, H: block.Bottom - block.Top})
	}
	r := short.Overlays.Radar
	return radaroverlay.Place(radaroverlay.Placement{Position: r.Position, Size: r.Size, Width: width, Height: height, Avoid: avoid})
}

// radarAvoidRects are the brand watermark, the sponsor banner and the affiliate plate, which keep their place.
func radarAvoidRects(short ShortEdit, frameH int) []radaroverlay.Rect {
	var out []radaroverlay.Rect
	for _, e := range short.Effects {
		if e.Type != EffectImage || e.Width <= 0 {
			continue
		}
		if e.Source == "brand-watermark" || e.Source == "brand-banner" {
			x, errX := strconv.Atoi(e.X)
			y, errY := strconv.Atoi(e.Y)
			if errX == nil && errY == nil {
				out = append(out, radaroverlay.Rect{X: x, Y: y, W: e.Width, H: e.Height})
			}
			continue
		}
		if e.Path == short.KeyDropImagePath && short.KeyDropImagePath != "" {
			if rect, ok := keyDropRect(short, e, frameH); ok {
				out = append(out, rect)
			}
		}
	}
	return out
}

// keyDropRect is where the affiliate plate lands: centred, at KeyDropPositionY, scaled to its effect width.
func keyDropRect(short ShortEdit, e Effect, frameH int) (radaroverlay.Rect, bool) {
	// #nosec G304 -- the plate was composited for this render.
	f, err := os.Open(e.Path)
	if err != nil {
		return radaroverlay.Rect{}, false
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width <= 0 {
		return radaroverlay.Rect{}, false
	}
	frameW, _ := outputDimensions(short)
	h := cfg.Height * e.Width / cfg.Width
	positionY := 0.86
	if short.KeyDropPositionY != nil {
		positionY = *short.KeyDropPositionY
	}
	return radaroverlay.Rect{X: (frameW - e.Width) / 2, Y: int(float64(frameH)*positionY) - h/2, W: e.Width, H: h}, true
}

// radarClauses draws the radar's moving layer over the composed frame, plates included.
func radarClauses(short ShortEdit, current string) ([]string, string) {
	if short.RadarASS == "" {
		return nil, current
	}
	return []string{fmt.Sprintf("[%s]%s[vradar]", current, customhud.ASSFilter(short.RadarASS, short.RadarFontsDir, false))}, "vradar"
}
