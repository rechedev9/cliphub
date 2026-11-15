package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/recording"
)

// VerticalHUD is the 9:16 scoreboard of each captured segment on its own clock, in a catalog design.
type VerticalHUD struct {
	Theme    string                      `json:"theme"`
	Segments map[string][]VerticalHUDCue `json:"segments"`
}

// VerticalHUDCue is one scoreboard state, in seconds from the segment's capture start.
type VerticalHUDCue struct {
	StartSeconds float64                 `json:"start_seconds"`
	EndSeconds   float64                 `json:"end_seconds"`
	State        customhud.VerticalState `json:"state"`
}

type verticalHUDInput struct {
	cfg  Config
	plan recording.RecordingPlan
}

// prepareVerticalHUD moves the worker's HUD track onto each segment's clock. It draws only over a
// capture without the CS2 HUD and with CS2's notices outside the 9:16 frame, so nothing shows twice.
func prepareVerticalHUD(in verticalHUDInput) (*VerticalHUD, []string, error) {
	cfg, plan := in.cfg, in.plan
	if !cfg.VerticalHUD {
		return nil, nil, nil
	}
	theme, err := customhud.VerticalTheme(strings.TrimSpace(cfg.VerticalHUDTheme))
	if err != nil {
		return nil, nil, err
	}
	if format, err := normalizeOutputFormat(cfg.OutputFormat); err != nil || format != OutputFormatShort9x16 {
		return nil, []string{"vertical HUD left out: it is only for 9:16 Shorts"}, nil
	}
	mode := plan.Stream.HUDMode
	if mode != recording.HUDModeClean && mode != recording.HUDModeDeathnotices {
		return nil, []string{"vertical HUD left out: the capture shows the CS2 HUD"}, nil
	}
	if plan.Stream.PortraitSafeKillfeed {
		return nil, []string{"vertical HUD left out: the capture already shows CS2's notices inside the 9:16 frame"}, nil
	}
	if strings.TrimSpace(cfg.VerticalHUDTrackPath) == "" {
		return nil, []string{"vertical HUD left out: no HUD track"}, nil
	}
	track, err := customhud.LoadVerticalTrack(cfg.VerticalHUDTrackPath)
	if err != nil {
		return nil, nil, err
	}
	if track.TickRate != plan.Tickrate || track.DemoSHA256 != plan.DemoSHA256 {
		return nil, nil, fmt.Errorf("vertical HUD track is for another demo or tick rate")
	}
	bySegment := map[string]customhud.VerticalSegment{}
	for _, s := range track.Segments {
		bySegment[s.ID] = s
	}
	out := &VerticalHUD{Theme: theme.ID, Segments: map[string][]VerticalHUDCue{}}
	for _, segment := range plan.Segments {
		start := recording.EffectiveRecordStartTick(segment, plan.Tickrate)
		for _, span := range bySegment[segment.ID].Spans {
			if span.TickEnd <= start {
				continue
			}
			out.Segments[segment.ID] = append(out.Segments[segment.ID], VerticalHUDCue{
				StartSeconds: float64(max(span.TickStart, start)-start) / float64(plan.Tickrate),
				EndSeconds:   float64(span.TickEnd-start) / float64(plan.Tickrate),
				State:        span.State,
			})
		}
	}
	if len(out.Segments) == 0 {
		return nil, []string{"vertical HUD left out: the HUD telemetry does not cover these plays"}, nil
	}
	return out, nil, nil
}

// verticalHUDCues place the gameplay parts' states on the edited timeline, after the hook title
// (drawn on the same line) and before the closing stats card.
func verticalHUDCues(short ShortEdit) []customhud.VerticalCue {
	hud := short.Overlays.VerticalHUD
	floor, ceiling := 0.0, short.DurationSeconds
	if short.HookText && !short.Intro {
		floor = transitionEnd(short, winProbHookSeconds)
	}
	if cardStart, _, ok := statsCardWindow(short); ok {
		ceiling = cardStart
	}
	var cues []customhud.VerticalCue
	for _, window := range winProbWindows(short) {
		for _, cue := range hud.Segments[window.segmentID] {
			start := max(window.offset+cue.StartSeconds, window.offset, floor)
			end := min(window.offset+cue.EndSeconds, window.until, ceiling)
			if end-start >= 0.05 {
				cues = append(cues, customhud.VerticalCue{StartSeconds: start, EndSeconds: end, State: cue.State})
			}
		}
	}
	return cues
}

// writeVerticalHUD writes the short's 9:16 HUD script beside its output, on the edited timeline.
func writeVerticalHUD(short *ShortEdit) error {
	short.VerticalHUDASS, short.VerticalHUDFontsDir = "", ""
	hud := short.Overlays.VerticalHUD
	if hud == nil || short.FullDemo != nil || isLandscapeOutput(*short) {
		return nil
	}
	cues := verticalHUDCues(*short)
	if len(cues) == 0 {
		return nil
	}
	theme, err := customhud.VerticalTheme(hud.Theme)
	if err != nil {
		return err
	}
	script, err := customhud.VerticalASS(theme, cues)
	if err != nil {
		return fmt.Errorf("vertical HUD: %w", err)
	}
	path := short.Output + ".vertical-hud.ass"
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		return err
	}
	fonts, err := mediafont.MaterializeHUD()
	if err != nil {
		return fmt.Errorf("vertical HUD fonts: %w", err)
	}
	short.VerticalHUDASS, short.VerticalHUDFontsDir = path, fonts
	return nil
}

// writeShortASSOverlays writes the libass scripts the post-concat chain draws: the centered
// kill feed and the vertical HUD.
func writeShortASSOverlays(short *ShortEdit) error {
	if err := writeCenteredKillfeed(short); err != nil {
		return err
	}
	return writeVerticalHUD(short)
}

// appendShortASSFilters draws the vertical HUD, then the centered kill feed under it.
func appendShortASSFilters(filters []string, short ShortEdit) []string {
	if short.VerticalHUDASS != "" {
		filters = append(filters, customhud.ASSFilter(short.VerticalHUDASS, short.VerticalHUDFontsDir, false))
	}
	return appendCenteredKillfeedFilter(filters, short)
}
