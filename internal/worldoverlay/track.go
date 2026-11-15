// Package worldoverlay draws player labels and grenade trails over first-person gameplay. The
// editor projects the demo's own 3D positions through the POV camera; the CS2 capture is unchanged.
package worldoverlay

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Version names the track layout; a track of another version is refused.
const Version = "world-overlay-v1"

// MaxBytes bounds a track file so a malformed one cannot exhaust memory.
const MaxBytes = 256 << 20

// Grenade kinds of a trail.
const (
	KindSmoke = "smoke"
	KindFlash = "flash"
	KindHE    = "he"
	KindFire  = "fire"
	KindDecoy = "decoy"
)

// Vec is a world position in CS2 units.
type Vec [3]float64

// Track holds the sampled demo state of every captured segment of one render.
type Track struct {
	Version    string    `json:"version"`
	DemoSHA256 string    `json:"demo_sha256"`
	TickRate   int       `json:"tick_rate"`
	Segments   []Segment `json:"segments"`
}

// Window is one capture window to sample, seen from its POV player.
type Window struct {
	ID           string
	POVSteamID64 string
	TickStart    int
	TickEnd      int
}

// Segment is the state of one capture window, one frame per demo tick.
type Segment struct {
	ID        string `json:"id"`
	POV       string `json:"pov_steamid64"`
	TickStart int    `json:"tick_start"`
	TickEnd   int    `json:"tick_end"`
	// Names lists the players a frame refers to by slot.
	Names    []string  `json:"names"`
	Frames   []Frame   `json:"frames"`
	Grenades []Grenade `json:"grenades,omitempty"`
}

// Frame is one tick. Camera is nil while the POV is dead or its eye position is unknown.
type Frame struct {
	Tick    int      `json:"tick"`
	Camera  *Camera  `json:"camera,omitempty"`
	Players []Player `json:"players,omitempty"`
}

// Camera is the POV's eye: CS2 yaw and pitch in degrees, pitch positive looking down.
type Camera struct {
	Eye   Vec     `json:"eye"`
	Yaw   float64 `json:"yaw"`
	Pitch float64 `json:"pitch"`
	// FOV is the pawn's m_iFOV; 0 is the default 90.
	FOV    int    `json:"fov,omitempty"`
	Scoped bool   `json:"scoped,omitempty"`
	Side   string `json:"side"`
}

// Player is one living player other than the POV.
type Player struct {
	Slot   int    `json:"slot"`
	Side   string `json:"side"`
	Health int    `json:"hp"`
	Eye    Vec    `json:"eye"`
	// Spotted is CS2's own m_bSpottedByMask bit for the POV: the game saw this player from the POV.
	Spotted bool `json:"spotted,omitempty"`
	// Hit is true on a tick the POV damaged this player (player_hurt).
	Hit bool `json:"hit,omitempty"`
}

// Grenade is one projectile's flight, one point per tick it was seen.
type Grenade struct {
	Kind   string  `json:"kind"`
	Side   string  `json:"side"`
	Points []Point `json:"points"`
}

// Point is a grenade position at a tick.
type Point struct {
	Tick int `json:"tick"`
	Pos  Vec `json:"pos"`
}

// Load reads and checks a track written by the render worker.
func Load(path string) (Track, error) {
	f, err := os.Open(path)
	if err != nil {
		return Track{}, fmt.Errorf("open world overlay track: %w", err)
	}
	defer f.Close()
	var track Track
	if err := json.NewDecoder(io.LimitReader(f, MaxBytes)).Decode(&track); err != nil {
		return Track{}, fmt.Errorf("decode world overlay track: %w", err)
	}
	return track, track.Validate()
}

// Validate refuses a track the overlay could misread: another version, no tick rate, frames out of order.
func (t Track) Validate() error {
	if t.Version != Version {
		return fmt.Errorf("world overlay track version %q, want %q", t.Version, Version)
	}
	if t.TickRate < 1 || t.TickRate > 1024 {
		return fmt.Errorf("world overlay track tick rate %d out of range", t.TickRate)
	}
	seen := map[string]bool{}
	for _, s := range t.Segments {
		if s.ID == "" || seen[s.ID] {
			return fmt.Errorf("world overlay segment id %q is empty or repeated", s.ID)
		}
		seen[s.ID] = true
		if err := s.validate(); err != nil {
			return fmt.Errorf("world overlay segment %s: %w", s.ID, err)
		}
	}
	return nil
}

func (s Segment) validate() error {
	for i, f := range s.Frames {
		if i > 0 && f.Tick <= s.Frames[i-1].Tick {
			return fmt.Errorf("frame ticks are not increasing at %d", f.Tick)
		}
		for _, p := range f.Players {
			if p.Slot < 0 || p.Slot >= len(s.Names) {
				return fmt.Errorf("player slot %d has no name", p.Slot)
			}
		}
	}
	for _, g := range s.Grenades {
		switch g.Kind {
		case KindSmoke, KindFlash, KindHE, KindFire, KindDecoy:
		default:
			return fmt.Errorf("unknown grenade kind %q", g.Kind)
		}
		for i := 1; i < len(g.Points); i++ {
			if g.Points[i].Tick <= g.Points[i-1].Tick {
				return fmt.Errorf("grenade point ticks are not increasing at %d", g.Points[i].Tick)
			}
		}
	}
	return nil
}
