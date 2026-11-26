package main

import (
	"context"
	"fmt"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type storyOverlaysIn struct {
	HalftimeScoreboard *bool `json:"halftime_scoreboard,omitempty" jsonschema:"show a compact card with both team names and the score for about 4 s when the first round after the regulation side switch starts; skipped when that round is not in the video or the job has no round results; render only; default off"`
	SubscribeButton    *bool `json:"subscribe_button,omitempty" jsonschema:"show a SUBSCRIBE button whose bell swings and dings, in the bottom right corner twice (about 6 s each near 25% and 75% of the match) over the rounds only, with the channel logo and channel name of the brand kit (get_brand_kit) when it has them; a short video shows it once or not at all; render only; default off"`
	subscribeCard      *brandkit.Subscribe
	Hook               *bool `json:"hook,omitempty" jsonschema:"open the video with a teaser of the player's best play (8 to 20 s, labelled MEJOR JUGADA) after the intro clip and before round 1; the play is seen again in its round; skipped with a warning when the video has one round or the player has no kill; render only, reuses the capture; default off"`
}

func applyStoryOverlays(options *recapplan.Options, in storyOverlaysIn) {
	if in.HalftimeScoreboard != nil {
		options.Overlays.Halftime = *in.HalftimeScoreboard
	}
	if in.SubscribeButton != nil {
		options.Overlays.Subscribe = *in.SubscribeButton
		options.Overlays.Brand.Subscribe = in.subscribeCard
	}
	if in.Hook != nil {
		options.Hook = nil
		if *in.Hook {
			options.Hook = &recapplan.HookOptions{Enabled: true}
		}
	}
}

func resolveSubscribeCard(ctx context.Context, c *tuiclient.Client, in *storyOverlaysIn) error {
	if !orDefault(in.SubscribeButton, false) {
		return nil
	}
	kit, err := c.GetBrandKit(ctx)
	if err != nil {
		return toolError(err)
	}
	if card := kit.SubscribeCard(); card.Branded() {
		in.subscribeCard = &card
	}
	return nil
}

func describeHook(doc recapplan.Document) (bool, int, string) {
	requested := doc.Options.Hook != nil && doc.Options.Hook.Enabled
	if doc.Hook == nil {
		return requested, 0, ""
	}
	for _, round := range doc.Rounds {
		if round.ID == doc.Hook.RoundID {
			return requested, round.Number, fmt.Sprintf(" Abre con un gancho de la mejor jugada: %s en la ronda %d.", plural(hookKillsShown(doc, round), "baja", "bajas"), round.Number)
		}
	}
	return requested, 0, ""
}

func hookKillsShown(doc recapplan.Document, round recapplan.Round) int {
	shown := 0
	for _, item := range doc.Timeline {
		if !item.IsHook() {
			continue
		}
		for _, kill := range round.Kills {
			if kill.Tick >= doc.Hook.FirstKillTick && kill.Tick <= doc.Hook.LastKillTick && item.ShowsTick(kill.Tick, doc.Clock.TickRate) {
				shown++
			}
		}
	}
	return shown
}

func describeStoryOverlays(o recapplan.OverlayOptions) string {
	button := "botón de suscripción"
	if o.Brand.Subscribe != nil {
		button += " con tu logo y nombre de canal"
	}
	switch {
	case o.Halftime && o.Subscribe:
		return " Con marcador del descanso y " + button + "."
	case o.Halftime:
		return " Con marcador del descanso."
	case o.Subscribe:
		return " Con " + button + "."
	}
	return ""
}
