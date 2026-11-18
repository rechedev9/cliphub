package main

import (
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// introDesigns maps plan_full_demo intro_design to the plan's overlays.theme.
var introDesigns = map[string]string{"neon": recapplan.OverlayThemeNeon, "hud": recapplan.OverlayThemeHUD}

func applyIntroDesign(options *recapplan.Options, choice string) error {
	if choice == "" {
		return nil
	}
	theme, ok := introDesigns[strings.ToLower(strings.TrimSpace(choice))]
	if !ok {
		return fmt.Errorf("intro_design must be neon or hud")
	}
	options.Overlays.Theme = theme
	return nil
}

func introDesignOf(o recapplan.OverlayOptions) string {
	if o.Theme == recapplan.OverlayThemeHUD {
		return "hud"
	}
	return "neon"
}

func describeIntroDesign(o recapplan.OverlayOptions) string {
	switch {
	case o.Theme != recapplan.OverlayThemeHUD:
		return ""
	case o.HUDTheme == "":
		return " Intro y marcador final a juego con el HUD, en tonos neutros."
	}
	return " Intro y marcador final a juego con el HUD."
}
