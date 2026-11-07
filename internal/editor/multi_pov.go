package editor

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rechedev9/cliphub/internal/recording"
)

// playerName names the Short's player: the target, or the filmed players of a
// multi-POV plan in capture order ("ropz, b1t y 2 más").
func playerName(plan recording.RecordingPlan) string {
	if plan.IsMultiPOV() {
		var names []string
		for _, s := range plan.Segments {
			name := plan.ObservedName(s)
			if name == "" {
				name = plan.ObservedSteamID(s)
			}
			if !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
		if len(names) > 2 {
			return fmt.Sprintf("%s, %s y %d más", names[0], names[1], len(names)-2)
		}
		return strings.Join(names, " y ")
	}
	if plan.TargetNameInDemo != "" {
		return plan.TargetNameInDemo
	}
	if plan.TargetSteamID64 != "" {
		return plan.TargetSteamID64
	}
	return "target player"
}

// segmentPlayer is the player one segment's own Short is about.
func segmentPlayer(plan recording.RecordingPlan, s recording.RecordingSegment, fallback string) string {
	if !plan.IsMultiPOV() {
		return fallback
	}
	if name := plan.ObservedName(s); name != "" {
		return name
	}
	return fallback
}

// partPlayer labels a compiled part with its POV, only in multi-POV plans.
func partPlayer(plan recording.RecordingPlan, s recording.RecordingSegment) string {
	if !plan.IsMultiPOV() {
		return ""
	}
	return segmentPlayer(plan, s, plan.ObservedSteamID(s))
}

// MultiPOV reports whether the Short's parts film more than one player.
func (s ShortEdit) MultiPOV() bool {
	first := ""
	for _, part := range s.Parts {
		switch {
		case part.Player == "":
		case first == "":
			first = part.Player
		case part.Player != first:
			return true
		}
	}
	return false
}

// generatedPlayerTagEffects names the player at the start of each part of a
// multi-POV Short, so viewers know whose play they are watching.
func generatedPlayerTagEffects(short ShortEdit) []Effect {
	if !short.MultiPOV() {
		return nil
	}
	y, size := "h*0.22", 46
	if short.OutputFormat == OutputFormatLandscape16x9 {
		y, size = "h*0.80", 40
	}
	var effects []Effect
	for _, part := range short.Parts {
		if part.Player == "" {
			continue
		}
		start := clampSeconds(part.TimelineStartSeconds+0.1, 0, short.DurationSeconds)
		end := clampSeconds(part.TimelineStartSeconds+min(2.2, part.DurationSeconds), 0, short.DurationSeconds)
		if end <= start {
			continue
		}
		effects = append(effects, Effect{
			Type: EffectText, Value: strings.ToUpper(part.Player),
			X: "(w-text_w)/2", Y: y, Size: size,
			FontColor: "white@0.96", BoxColor: "none", ShadowColor: "black@0.85", ShadowX: 3, ShadowY: 3,
			Bold: true, BorderWidth: 2, BorderColor: "black@0.9",
			StartSeconds: start, EndSeconds: end, FadeInSeconds: 0.12, FadeOutSeconds: 0.3,
			Source: "edit-request",
		})
	}
	return effects
}
