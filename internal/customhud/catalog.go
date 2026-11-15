// Package customhud composes broadcast HUDs from recorded demo facts. Its
// renderer is independent of wall-clock playback, the game and network GSI.
package customhud

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/rechedev9/cliphub/internal/hudthemes"
)

const Version = "broadcast-hud-v9"
const TelemetryVersion = "broadcast-hud-v6"
const CaptureProfile = "broadcast-clean-v2"
const LegacyCaptureProfile = "broadcast-clean"

func IsCaptureProfile(profile string) bool {
	return profile == CaptureProfile || profile == LegacyCaptureProfile
}

//go:embed themes.json
var catalogJSON []byte

type Theme struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	CT           string  `json:"ct"`
	T            string  `json:"t"`
	Background   string  `json:"background"`
	Surface      string  `json:"surface"`
	Text         string  `json:"text"`
	Muted        string  `json:"muted"`
	Shape        string  `json:"shape"`
	Layout       string  `json:"layout"`
	ScoreWidth   int     `json:"score_width"`
	FocusX       int     `json:"focus_x"`
	FocusY       int     `json:"focus_y"`
	FocusWidth   int     `json:"focus_width"`
	LoadoutX     int     `json:"loadout_x"`
	LoadoutY     int     `json:"loadout_y"`
	LoadoutWidth int     `json:"loadout_width"`
	Accent       string  `json:"accent"`
	Opacity      float64 `json:"opacity"`
	// Radius is the corner radius of a round shape; 0 keeps the design's own.
	Radius int `json:"radius,omitempty"`
}

// Options are the composition switches of overlays.hud_options.
type Options struct {
	CompactHeader  bool `json:"compact_header,omitempty"`
	HideLoadout    bool `json:"hide_loadout,omitempty"`
	RoundWinBanner bool `json:"round_win_banner,omitempty"`
}

// Teams names the teams by the side each one started the match on.
type Teams struct {
	CTStart string `json:"ct_start,omitempty"`
	TStart  string `json:"t_start,omitempty"`
}

// WithStyle applies a user theme to its catalog base. The layout stays the base's.
func (t Theme) WithStyle(s hudthemes.Style) Theme {
	t.Name, t.CT, t.T, t.Background = s.Name, s.CT, s.T, s.Background
	if s.Radius != nil {
		t.Shape, t.Radius = "square", 0
		if *s.Radius > 0 {
			t.Shape, t.Radius = "round", *s.Radius
		}
	}
	return t
}

var catalog = func() []Theme {
	var themes []Theme
	if err := json.Unmarshal(catalogJSON, &themes); err != nil {
		panic(err) // Embedded, source-controlled catalog; not an external input.
	}
	return themes
}()

func Themes() []Theme { return append([]Theme(nil), catalog...) }

func Lookup(id string) (Theme, bool) {
	for _, theme := range catalog {
		if theme.ID == id {
			return theme, true
		}
	}
	return Theme{}, false
}

func ValidateTheme(id string) error {
	if _, ok := Lookup(id); !ok {
		return fmt.Errorf("unknown custom HUD %q", id)
	}
	return nil
}
