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
	switch theme {
	case ThemeNeonViolet:
		return CardTheme{
			Name:       ThemeNeonViolet,
			Accent:     "0xA855F7",
			AccentSoft: "0xC084FC",
		}
	default:
		return CardTheme{
			Name:       ThemeFaceitOrange,
			Accent:     "0xFF5500",
			AccentSoft: "0xFF7733",
		}
	}
}

const (
	introCardFill      = "0x121216@0.88"
	introCardRadius    = 14
	introCardGlowSteps = 4
)

// UsesProgrammaticIntroChrome selects generated panel chrome instead of bundled
// intro JPG plates. Neon-violet is opt-in programmatic-only; faceit-orange
// keeps bundled plates when present.
func UsesProgrammaticIntroChrome(doc Document) bool {
	return NormalizeTheme(doc.Theme) == ThemeNeonViolet
}
