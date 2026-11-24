package editor

import (
	"fmt"

	"github.com/rechedev9/cliphub/internal/recording"
)

// CS2 side colours: white on the CT blue, black on the T gold.
const (
	winProbCTBox = "0x5D79AE@0.92"
	winProbTBox  = "0xDE9B35@0.94"
	// winProbHookSeconds keeps the overlay clear of the opening title or intro card.
	winProbHookSeconds = 1.8
)

// WinProbCue is one post-plant win chance, in seconds from the segment's capture start.
type WinProbCue struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	CTPct        int     `json:"ct_pct"`
}

// winProbCues moves each segment's kill plan spans onto the clock of its clip, like killCues.
func winProbCues(plan recording.RecordingPlan) map[string][]WinProbCue {
	if plan.Tickrate <= 0 {
		return nil
	}
	out := map[string][]WinProbCue{}
	for _, segment := range plan.Segments {
		start := recording.EffectiveRecordStartTick(segment, plan.Tickrate)
		for _, span := range segment.WinProb {
			if span.TickEnd <= start || span.CTPct < 1 || span.CTPct > 99 {
				continue
			}
			out[segment.ID] = append(out[segment.ID], WinProbCue{
				StartSeconds: float64(max(span.TickStart, start)-start) / float64(plan.Tickrate),
				EndSeconds:   float64(span.TickEnd-start) / float64(plan.Tickrate),
				CTPct:        span.CTPct,
			})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type winProbWindow struct {
	segmentID     string
	offset, until float64
}

// winProbWindows are the Short's gameplay parts; a victim replay is another player's view.
func winProbWindows(short ShortEdit) []winProbWindow {
	if len(short.Parts) == 0 {
		return []winProbWindow{{segmentID: short.SegmentID, until: short.DurationSeconds}}
	}
	var out []winProbWindow
	for _, part := range short.Parts {
		if !isVictimReplayPart(part) {
			out = append(out, winProbWindow{segmentID: part.SegmentID, offset: part.TimelineStartSeconds, until: part.TimelineStartSeconds + part.DurationSeconds})
		}
	}
	return out
}

// generatedWinProbEffects draws the win chance label over a CT and a T plate, under the kill feed lane.
func generatedWinProbEffects(short ShortEdit) []Effect {
	if len(short.Overlays.WinProbability) == 0 || short.FullDemo != nil {
		return nil
	}
	width, height := outputDimensions(short)
	labelSize, plateSize := 30, 46
	if width > height {
		labelSize, plateSize = 24, 36
	}
	floor, ceiling := 0.0, short.DurationSeconds
	if short.HookText || short.Intro {
		floor = transitionEnd(short, winProbHookSeconds)
	}
	if cardStart, _, ok := statsCardWindow(short); ok {
		ceiling = cardStart
	}
	var effects []Effect
	phrases := overlayTexts(short.Overlays.Language)
	for _, window := range winProbWindows(short) {
		for i, cue := range short.Overlays.WinProbability[window.segmentID] {
			start := max(window.offset+cue.StartSeconds, window.offset, floor)
			end := min(window.offset+cue.EndSeconds, window.until, ceiling)
			if end-start < 0.05 {
				continue
			}
			// Only the first value of a run fades in; a new value replaces the last in place.
			fade := 0.0
			if i == 0 || cue.StartSeconds > short.Overlays.WinProbability[window.segmentID][i-1].EndSeconds {
				fade = 0.15
			}
			plate := func(value, x, fontColor, box string) Effect {
				return Effect{
					Type: EffectText, Value: value, X: x, Y: fmt.Sprintf("h*0.24+%d", labelSize+14), Size: plateSize,
					FontColor: fontColor, BoxColor: box, BoxBorder: 14, Bold: true,
					StartSeconds: start, EndSeconds: end, FadeInSeconds: fade, Source: "win-probability",
				}
			}
			effects = append(effects,
				Effect{
					Type: EffectText, Value: phrases.winProbability, X: "(w-text_w)/2", Y: "h*0.24", Size: labelSize,
					FontColor: "white@0.96", BoxColor: "none", ShadowColor: "black@0.85", ShadowX: 2, ShadowY: 2, Bold: true,
					StartSeconds: start, EndSeconds: end, FadeInSeconds: fade, Source: "win-probability",
				},
				plate(fmt.Sprintf(phrases.ctPlate, cue.CTPct), "w/2-24-text_w", "white", winProbCTBox),
				plate(fmt.Sprintf(phrases.tPlate, 100-cue.CTPct), "w/2+24", "black", winProbTBox),
			)
		}
	}
	return effects
}
