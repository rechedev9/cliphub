package recapplan

// The intro and final scoreboard designs a new plan can use, stored in overlays.theme.
const (
	OverlayThemeNeon = "neon-violet"
	// OverlayThemeHUD takes the colours of the plan's HUD, or a neutral palette over the native CS2 HUD.
	OverlayThemeHUD = "hud"
)

func canonicalOverlayTheme(theme string) string {
	if theme == OverlayThemeHUD {
		return OverlayThemeHUD
	}
	return OverlayThemeNeon
}
