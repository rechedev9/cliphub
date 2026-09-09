// Package customhud composes broadcast HUDs from recorded demo facts. Its
// renderer is independent of wall-clock playback, the game and network GSI.
package customhud

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

const Version = "broadcast-hud-v3"
const TelemetryVersion = "broadcast-hud-v2"
const CaptureProfile = "broadcast-clean-v2"
const LegacyCaptureProfile = "broadcast-clean"

func IsCaptureProfile(profile string) bool {
	return profile == CaptureProfile || profile == LegacyCaptureProfile
}

//go:embed themes.json
var catalogJSON []byte

type Theme struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	CT          string  `json:"ct"`
	T           string  `json:"t"`
	Background  string  `json:"background"`
	Surface     string  `json:"surface"`
	Text        string  `json:"text"`
	Muted       string  `json:"muted"`
	Shape       string  `json:"shape"`
	Layout      string  `json:"layout"`
	ScoreWidth  int     `json:"score_width"`
	FocusX      int     `json:"focus_x"`
	FocusY      int     `json:"focus_y"`
	FocusWidth  int     `json:"focus_width"`
	Accent      string  `json:"accent"`
	Opacity     float64 `json:"opacity"`
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
