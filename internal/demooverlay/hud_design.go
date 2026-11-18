package demooverlay

import (
	"math"
	"strconv"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// Palette is what the HUD design takes from the plan's HUD: six-digit hex
// colours and the corner radius in px.
type Palette struct {
	CT         string `json:"ct"`
	T          string `json:"t"`
	Background string `json:"background"`
	Text       string `json:"text"`
	Muted      string `json:"muted"`
	Radius     int    `json:"radius"`
}

// nativePalette is the fixed neutral look over the original CS2 HUD, which has no colours of its own.
var nativePalette = Palette{CT: "8FA6C4", T: "CDB78A", Background: "17181B", Text: "F1F2F4", Muted: "9EA2A9", Radius: 6}

// DesignPalette is the palette the HUD design draws with, including a user
// theme; nil for every other design.
func DesignPalette(o recapplan.OverlayOptions) *Palette {
	if NormalizeTheme(o.Theme) != ThemeHUD {
		return nil
	}
	theme, ok := customhud.Lookup(o.HUDTheme)
	if !ok {
		palette := nativePalette
		return &palette
	}
	if o.HUDCustom != nil {
		theme = theme.WithStyle(*o.HUDCustom)
	}
	radius := 0
	if theme.Shape == "round" {
		// A round design without its own radius draws 8 px plates (customhud shapePath).
		radius = 8
		if theme.Radius > 0 {
			radius = theme.Radius
		}
	}
	text, muted := legibleText(theme)
	return &Palette{CT: theme.CT, T: theme.T, Background: theme.Background, Text: text, Muted: muted, Radius: radius}
}

// legibleText keeps the design's text colours unless an own background makes
// them fall below WCAG AA; then it takes a neutral light or dark pair.
func legibleText(t customhud.Theme) (text, muted string) {
	background := luminance(t.Background)
	if contrast(luminance(t.Text), background) >= 4.5 && contrast(luminance(t.Muted), background) >= 3 {
		return t.Text, t.Muted
	}
	// Above this luminance dark text contrasts more than white.
	if background > 0.18 {
		return "15181D", "4A505A"
	}
	return "F4F5F7", "A9AEB7"
}

// luminance is the WCAG relative luminance of a six-digit hex colour.
func luminance(hex string) float64 {
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0
	}
	channel := func(c uint64) float64 {
		v := float64(c) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(value>>16) + 0.7152*channel(value>>8&0xFF) + 0.0722*channel(value&0xFF)
}

func contrast(a, b float64) float64 {
	return (max(a, b) + 0.05) / (min(a, b) + 0.05)
}
