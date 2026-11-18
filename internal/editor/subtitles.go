package editor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/subtitles"
)

// subtitleCues place the transcript on the edited timeline where the team comms play: each gameplay
// part shows the phrases said during its capture; victim replays and slow motion carry no comms.
func subtitleCues(short ShortEdit) []subtitles.Cue {
	track, rate := short.Overlays.Subtitles, short.Tickrate
	if rate <= 0 {
		rate = short.VoiceTickrate
	}
	if track == nil || rate <= 0 {
		return nil
	}
	floor, ceiling := 0.0, short.DurationSeconds
	if short.Intro {
		floor = transitionEnd(short, winProbHookSeconds)
	}
	if end, ok := openingCardWindow(short); ok && openingPlayerCard(short) != "" {
		floor = max(floor, end)
	}
	if cardStart, _, ok := statsCardWindow(short); ok {
		ceiling = cardStart
	}
	var cues []subtitles.Cue
	for _, part := range short.Parts {
		if isVictimReplayPart(part) || isSlowMotion(part) {
			continue
		}
		captureStart, _ := captureTicks(part)
		from := part.TimelineStartSeconds
		cues = append(cues, windowCues(track, cueWindow{startTick: captureStart, rate: float64(rate), base: from,
			lo: max(from, floor), hi: min(from+partSyncDuration(part, rate), ceiling)})...)
	}
	return cues
}

// cueWindow maps demo ticks onto a stretch of video: tick startTick plays at base seconds, and
// only what falls between lo and hi shows.
type cueWindow struct {
	startTick    int
	rate         float64
	base, lo, hi float64
}

func windowCues(track *subtitles.Track, w cueWindow) []subtitles.Cue {
	var cues []subtitles.Cue
	for _, p := range track.Phrases {
		start := max(w.base+float64(p.StartTick-w.startTick)/w.rate, w.lo)
		end := min(w.base+float64(p.EndTick-w.startTick)/w.rate, w.hi)
		if end-start >= 0.05 {
			cues = append(cues, subtitles.Cue{ID: p.ID, Speaker: p.Speaker, Text: p.Text, StartSeconds: start, EndSeconds: end})
		}
	}
	return cues
}

// writeSubtitles writes the short's subtitle script beside its output, on the edited timeline.
func writeSubtitles(short *ShortEdit) error {
	short.SubtitlesASS, short.SubtitlesFontsDir = "", ""
	if short.FullDemo != nil {
		return nil
	}
	cues := subtitleCues(*short)
	if len(cues) == 0 {
		return nil
	}
	width, height := outputDimensions(*short)
	duration := short.DurationSeconds
	if duration <= 0 {
		duration = cues[len(cues)-1].EndSeconds + subtitles.MaxSeconds
	}
	script, err := subtitles.ASS(subtitles.Script{Width: width, Height: height, DurationSeconds: duration, Cues: cues, Hidden: short.Overlays.Subtitles.Hidden})
	if err != nil {
		return fmt.Errorf("subtitles: %w", err)
	}
	path := short.Output + ".subtitles.ass"
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		return err
	}
	fonts, err := mediafont.MaterializeHUD()
	if err != nil {
		return fmt.Errorf("subtitle fonts: %w", err)
	}
	short.SubtitlesASS, short.SubtitlesFontsDir = path, fonts
	return nil
}

// fullDemoSubtitlesFilter draws the phrases said during one round of a Full Demo, on that item's clock.
func fullDemoSubtitlesFilter(short ShortEdit, item recapplan.TimelineItem, output string) (string, error) {
	if short.fullDemo == nil || short.fullDemo.execution.Subtitles == nil || item.Role != "round" {
		return "", nil
	}
	rate := short.fullDemo.recording.Plan.Tickrate
	if rate <= 0 {
		return "", fmt.Errorf("full demo subtitles need the capture tick rate")
	}
	track := short.fullDemo.execution.Subtitles
	duration := float64(item.EndFrame-item.StartFrame) / 60
	cues := windowCues(track, cueWindow{startTick: item.SourceStartTick, rate: float64(rate), base: -float64(item.SourceOffsetFrames) / 60, hi: duration})
	if len(cues) == 0 {
		return "", nil
	}
	width, height := outputDimensions(short)
	script, err := subtitles.ASS(subtitles.Script{Width: width, Height: height, DurationSeconds: duration, Cues: cues, Hidden: track.Hidden})
	if err != nil {
		return "", err
	}
	path := output + ".subtitles.ass"
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		return "", err
	}
	fonts, err := mediafont.MaterializeHUD()
	if err != nil {
		return "", err
	}
	return customhud.ASSFilter(path, fonts, false), nil
}

func appendSubtitlesFilter(filters []string, short ShortEdit) []string {
	if short.SubtitlesASS == "" {
		return filters
	}
	return append(filters, customhud.ASSFilter(short.SubtitlesASS, short.SubtitlesFontsDir, false))
}
