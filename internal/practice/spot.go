// Package practice opens CS2 on a local practice server standing on saved
// grenade lineups or positions. It writes one fixed cfg file next to the
// user's configs and launches plain CS2 through Steam, never HLAE.
package practice

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
)

// MaxSpots bounds one practice session: nine keypad keys plus cycling is
// already more than a player walks through in one sitting.
const MaxSpots = 20

const (
	maxLabelRunes = 48
	maxWorldCoord = 32768
)

var mapPattern = regexp.MustCompile(`^de_[a-z0-9_]{1,32}$`)

// ErrInvalid marks a request the caller must fix; the message says what.
var ErrInvalid = errors.New("invalid practice request")

// grenadeWeapons maps the analysis grenade kinds to their give names.
var grenadeWeapons = map[string]string{
	"smoke":      "weapon_smokegrenade",
	"flash":      "weapon_flashbang",
	"he":         "weapon_hegrenade",
	"molotov":    "weapon_molotov",
	"incendiary": "weapon_incgrenade",
	"decoy":      "weapon_decoy",
}

// SpotInput is one spot as the API receives it.
type SpotInput struct {
	Label   string    `json:"label"`
	Pos     []float64 `json:"pos"`
	Ang     []float64 `json:"ang"`
	Grenade string    `json:"grenade,omitempty"`
}

// Input is the launch request body.
type Input struct {
	Map   string      `json:"map"`
	Spots []SpotInput `json:"spots"`
}

// Spot is one validated spot whose label is safe inside a console command.
type Spot struct {
	Label   string
	Pos     [3]float64
	Ang     [2]float64
	Grenade string
}

// Plan is a validated practice session.
type Plan struct {
	Map   string
	Spots []Spot
}

// Validate checks every field strictly and sanitizes the labels.
func Validate(in Input) (Plan, error) {
	if !mapPattern.MatchString(in.Map) {
		return Plan{}, invalid("map must match de_<name>")
	}
	if len(in.Spots) == 0 {
		return Plan{}, invalid("at least one spot is required")
	}
	if len(in.Spots) > MaxSpots {
		return Plan{}, invalid(fmt.Sprintf("at most %d spots", MaxSpots))
	}
	plan := Plan{Map: in.Map, Spots: make([]Spot, 0, len(in.Spots))}
	for i, raw := range in.Spots {
		spot, err := validateSpot(raw, i+1)
		if err != nil {
			return Plan{}, err
		}
		plan.Spots = append(plan.Spots, spot)
	}
	return plan, nil
}

func validateSpot(raw SpotInput, n int) (Spot, error) {
	if len(raw.Pos) != 3 {
		return Spot{}, invalid(fmt.Sprintf("spot %d: pos needs x, y, z", n))
	}
	if len(raw.Ang) != 2 {
		return Spot{}, invalid(fmt.Sprintf("spot %d: ang needs pitch, yaw", n))
	}
	spot := Spot{Label: SanitizeLabel(raw.Label)}
	if spot.Label == "" {
		spot.Label = fmt.Sprintf("Spot %d", n)
	}
	for i, v := range raw.Pos {
		if !finiteWithin(v, maxWorldCoord) {
			return Spot{}, invalid(fmt.Sprintf("spot %d: pos is out of the map", n))
		}
		spot.Pos[i] = v
	}
	if !finiteWithin(raw.Ang[0], 90) || !finiteWithin(raw.Ang[1], 360) {
		return Spot{}, invalid(fmt.Sprintf("spot %d: pitch must be within 90 and yaw within 360", n))
	}
	spot.Ang = [2]float64{raw.Ang[0], raw.Ang[1]}
	if raw.Grenade != "" {
		if _, ok := grenadeWeapons[raw.Grenade]; !ok {
			return Spot{}, invalid(fmt.Sprintf("spot %d: unknown grenade %q", n, raw.Grenade))
		}
		spot.Grenade = raw.Grenade
	}
	return spot, nil
}

func finiteWithin(v, limit float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= limit
}

// SanitizeLabel keeps letters, digits, spaces and punctuation the CS2 console
// reads literally. Quotes, semicolons and slashes would end or comment out
// the alias that echoes the label, so they are dropped.
func SanitizeLabel(label string) string {
	var b strings.Builder
	runes := 0
	lastSpace := true
	for _, r := range label {
		if runes == maxLabelRunes {
			break
		}
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("-_.,:()!?#+&", r):
			b.WriteRune(r)
			lastSpace = false
		case unicode.IsSpace(r):
			if lastSpace {
				continue
			}
			b.WriteByte(' ')
			lastSpace = true
		default:
			continue
		}
		runes++
	}
	return strings.TrimSpace(b.String())
}

func invalid(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalid, msg)
}
