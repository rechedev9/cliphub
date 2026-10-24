// Package prosettings is the versioned catalog of pro players' CS2 video and
// viewmodel settings, copied by hand from prosettings.net; nothing is fetched at runtime.
package prosettings

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode"
)

//go:embed players.json
var catalogJSON []byte

// CaptureHeight keeps the Full Demo delivery height: a pro resolution sets
// the aspect ratio (and so the field of view), never a lower pixel count.
const CaptureHeight = 1080

const (
	ScalingStretched = "stretched"
	ScalingBlackBars = "black-bars"
	ScalingNative    = "native"
)

type Viewmodel struct {
	FOV     float64 `json:"fov"`
	OffsetX float64 `json:"offset_x"`
	OffsetY float64 `json:"offset_y"`
	OffsetZ float64 `json:"offset_z"`
}

type Player struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Team      string    `json:"team"`
	Aliases   []string  `json:"aliases,omitempty"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Scaling   string    `json:"scaling"`
	Viewmodel Viewmodel `json:"viewmodel"`
	Source    string    `json:"source"`
	Updated   string    `json:"updated"`
}

var catalog = mustLoad(catalogJSON)

func mustLoad(raw []byte) []Player {
	players, err := load(raw)
	if err != nil {
		panic(err)
	}
	return players
}

func load(raw []byte) ([]Player, error) {
	var players []Player
	if err := json.Unmarshal(raw, &players); err != nil {
		return nil, fmt.Errorf("decode pro settings catalog: %w", err)
	}
	seen := map[string]bool{}
	for _, p := range players {
		if err := p.Validate(); err != nil {
			return nil, fmt.Errorf("pro settings %q: %w", p.ID, err)
		}
		if seen[p.ID] {
			return nil, fmt.Errorf("duplicate pro settings id %q", p.ID)
		}
		seen[p.ID] = true
	}
	return players, nil
}

func Catalog() []Player {
	out := make([]Player, len(catalog))
	for i, p := range catalog {
		out[i] = clonePlayer(p)
	}
	return out
}

func Find(id string) (Player, bool) {
	for _, p := range catalog {
		if p.ID == id {
			return clonePlayer(p), true
		}
	}
	return Player{}, false
}

// Detect accepts the nickname as the whole name or, after a team tag, its last
// token; a token elsewhere ("im_noob") is not a match.
func Detect(nameInDemo string) (Player, bool) {
	tokens := strings.FieldsFunc(strings.ToLower(nameInDemo), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(tokens) == 0 {
		return Player{}, false
	}
	whole, last := strings.Join(tokens, ""), tokens[len(tokens)-1]
	for _, p := range catalog {
		for _, alias := range append([]string{p.ID}, p.Aliases...) {
			if alias == whole || alias == last {
				return clonePlayer(p), true
			}
		}
	}
	return Player{}, false
}

func clonePlayer(p Player) Player {
	p.Aliases = append([]string(nil), p.Aliases...)
	return p
}

func (p Player) CaptureSize() (int, int) {
	return CaptureWidth(p.Width, p.Height), CaptureHeight
}

// CaptureWidth keeps a resolution's aspect ratio at the delivery height,
// rounded to the even widths H.264 needs: 1280x960 records at 1440x1080.
func CaptureWidth(width, height int) int {
	return int(math.Round(float64(CaptureHeight)*float64(width)/float64(height)/2)) * 2
}

func (p Player) Validate() error {
	if p.ID == "" || strings.ToLower(p.ID) != p.ID || strings.TrimSpace(p.Name) == "" || p.Source == "" || len(p.Updated) != len("2006-01-02") {
		return fmt.Errorf("id, name, source and updated date are required")
	}
	for _, alias := range p.Aliases {
		if alias == "" || strings.ToLower(alias) != alias {
			return fmt.Errorf("aliases must be lowercase tokens")
		}
	}
	if p.Width < 640 || p.Height < 480 || p.Width > 3840 || p.Height > 2160 || p.Width < p.Height {
		return fmt.Errorf("resolution %dx%d is outside the supported range", p.Width, p.Height)
	}
	if width, _ := p.CaptureSize(); width > 1920 {
		return fmt.Errorf("resolution %dx%d is wider than 16:9", p.Width, p.Height)
	}
	if !ValidScaling(p.Scaling) {
		return fmt.Errorf("unknown scaling %q", p.Scaling)
	}
	return p.Viewmodel.Validate()
}

func ValidScaling(scaling string) bool {
	return scaling == ScalingStretched || scaling == ScalingBlackBars || scaling == ScalingNative
}

// Validate applies CS2's own cvar limits so a capture never fails on clamping.
func (v Viewmodel) Validate() error {
	if v.FOV < 54 || v.FOV > 68 || math.Abs(v.OffsetX) > 2.5 || math.Abs(v.OffsetY) > 2 || math.Abs(v.OffsetZ) > 2 {
		return fmt.Errorf("viewmodel outside CS2 limits")
	}
	return nil
}
