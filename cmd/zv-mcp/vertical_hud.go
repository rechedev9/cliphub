package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// verticalHUDThemes are the catalog designs the 9:16 HUD can take; the first is the default.
func verticalHUDThemes() []shortOption {
	themes := customhud.Themes()
	out := make([]shortOption, 0, len(themes))
	for i, t := range themes {
		out = append(out, shortOption{ID: t.ID, Label: t.Name, Description: t.Description, Default: i == 0})
	}
	return out
}

// resolveVerticalHUD turns the HUD on with its centered kill feed, as the server does, and checks
// the design. The server also refuses it with a preset that records the CS2 HUD (full-hud-60).
func resolveVerticalHUD(edit *tuiclient.ShortEdit, c shortChoices) error {
	theme := strings.ToLower(strings.TrimSpace(c.VerticalHUDTheme))
	if !orDefault(c.VerticalHUD, false) {
		if theme != "" {
			return errors.New("vertical_hud_theme needs vertical_hud set to true")
		}
		return nil
	}
	switch {
	case edit.Format != "short-9x16":
		return errors.New("vertical_hud needs format short-9x16")
	case edit.NativeHUD:
		return errors.New("vertical_hud replaces the CS2 HUD; leave native_hud off")
	case c.CenteredKillfeed != nil && !*c.CenteredKillfeed:
		return errors.New("vertical_hud includes the centered kill feed; leave centered_killfeed unset or true")
	}
	if theme != "" {
		if _, err := pickOption("vertical_hud_theme", theme, verticalHUDThemes()); err != nil {
			return fmt.Errorf("%w (list_short_options vertical_hud_themes)", err)
		}
	}
	edit.VerticalHUD, edit.VerticalHUDTheme, edit.CenteredKillfeed = true, theme, true
	return nil
}
