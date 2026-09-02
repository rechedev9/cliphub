package demooverlay

import "strings"

// CardTheme carries intro card chrome colors for programmatic rendering.
type CardTheme struct {
	Name       string
	Accent     string // primary neon, ffmpeg 0xRRGGBB
	AccentSoft string // glow / secondary stroke
}

func NormalizeTheme(theme string) string {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case ThemeNeonViolet:
		return ThemeNeonViolet
	case ThemeFaceitOrange, "":
		return ThemeFaceitOrange
	default:
		return ThemeFaceitOrange
	}
}

func ResolveTheme(doc Document) CardTheme {
	theme := NormalizeTheme(doc.Theme)
	if theme == "" && NormalizeSource(doc.Source) == SourceFACEIT {
		theme = ThemeFaceitOrange
	}
	spec, ok := defaultFaceitLayout.Themes[theme]
	if !ok {
		theme = ThemeFaceitOrange
		spec = defaultFaceitLayout.Themes[theme]
	}
	return CardTheme{Name: theme, Accent: spec.Accent, AccentSoft: spec.AccentSoft}
}

// UsesProgrammaticIntroChrome selects generated panel chrome instead of bundled
// intro JPG plates. FACEIT overlays always use live programmatic chrome so an
// old screenshot plate can never replace current roster data.
func UsesProgrammaticIntroChrome(doc Document) bool {
	return NormalizeSource(doc.Source) == SourceFACEIT || NormalizeTheme(doc.Theme) == ThemeNeonViolet
}
