package editor

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

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
	format := subtitles.FormatOf(outputDimensions(short))
	for _, part := range short.Parts {
		if isVictimReplayPart(part) || isSlowMotion(part) {
			continue
		}
		captureStart, _ := captureTicks(part)
		from := part.TimelineStartSeconds
		cues = append(cues, windowCues(track, cueWindow{startTick: captureStart, rate: float64(rate), base: from, pov: cmp.Or(part.Player, short.Player),
			lo: max(from, floor), hi: min(from+partSyncDuration(part, rate), ceiling), format: format})...)
	}
	return cues
}

// cueWindow maps demo ticks onto a stretch of video: tick startTick plays at base seconds, and
// only what falls between lo and hi shows.
type cueWindow struct {
	startTick    int
	rate         float64
	base, lo, hi float64
	pov          string
	format       subtitles.Format
}

func windowCues(track *subtitles.Track, w cueWindow) []subtitles.Cue {
	var cues []subtitles.Cue
	at := func(tick int) float64 { return w.base + float64(tick-w.startTick)/w.rate }
	for _, p := range track.Phrases {
		words := make([]subtitles.Word, 0, len(p.Words))
		for _, word := range p.Words {
			words = append(words, subtitles.Word{Text: word.Text, StartSeconds: at(word.StartTick), EndSeconds: at(word.EndTick)})
		}
		pov := w.pov != "" && (p.Speaker == w.pov || p.SteamID64 == w.pov)
		for _, b := range subtitles.Split(subtitles.SplitInput{Text: subtitles.SentenceCase(p.Text), Words: words, StartSeconds: at(p.StartTick), EndSeconds: at(p.EndTick),
			LeadRunes: utf8.RuneCountInString(p.Speaker) + 2, Format: w.format}) {
			start, end := max(b.StartSeconds, w.lo), min(b.EndSeconds, w.hi)
			if end-start >= 0.05 {
				cues = append(cues, subtitles.Cue{ID: p.ID, Speaker: p.Speaker, Slot: p.Slot, POV: pov, Lines: b.Lines, StartSeconds: start, EndSeconds: end})
			}
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
	width, height := outputDimensions(short)
	cues := windowCues(track, cueWindow{startTick: item.SourceStartTick, rate: float64(rate), base: -float64(item.SourceOffsetFrames) / 60, hi: duration, pov: short.Player,
		format: subtitles.FormatOf(width, height)})
	if len(cues) == 0 {
		return "", nil
	}
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
