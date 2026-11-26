package main

import "github.com/rechedev9/cliphub/internal/recapplan"

type storyOverlaysIn struct {
	HalftimeScoreboard *bool `json:"halftime_scoreboard,omitempty" jsonschema:"show a compact card with both team names and the score for about 4 s when the first round after the regulation side switch starts; skipped when that round is not in the video or the job has no round results; render only; default off"`
	SubscribeButton    *bool `json:"subscribe_button,omitempty" jsonschema:"show a small SUSCRÍBETE button with a bell in the bottom right corner twice (about 6 s each near 25% and 75% of the match) over the rounds only; a short video shows it once or not at all; render only; default off"`
}

func applyStoryOverlays(options *recapplan.Options, in storyOverlaysIn) {
	if in.HalftimeScoreboard != nil {
		options.Overlays.Halftime = *in.HalftimeScoreboard
	}
	if in.SubscribeButton != nil {
		options.Overlays.Subscribe = *in.SubscribeButton
	}
}

func describeStoryOverlays(o recapplan.OverlayOptions) string {
	switch {
	case o.Halftime && o.Subscribe:
		return " Con marcador del descanso y botón de suscripción."
	case o.Halftime:
		return " Con marcador del descanso."
	case o.Subscribe:
		return " Con botón de suscripción."
	}
	return ""
}
