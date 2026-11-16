// Package radaroverlay draws ClipHub's own radar in a corner of a Short: the map art of the 2D
// Replay, the players, smokes, fires, flashes and the bomb, read from the demo at render time.
// It is an editor overlay; the CS2 capture is unchanged.
package radaroverlay

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

// Version names the track layout; a track of another version is refused.
const Version = "radar-overlay-v1"

// MaxBytes bounds a track file so a malformed one cannot exhaust memory.
const MaxBytes = 64 << 20

// SamplesPerSecond is how often the players are sampled; the overlay moves them between samples.
const SamplesPerSecond = 16

// Utility kinds, as the 2D Replay names them.
const (
	KindSmoke = "smoke"
	KindFire  = "fire"
	KindFlash = "flash"
	KindHE    = "he"
)

// Bomb states.
const (
	BombCarried = "carried"
	BombDropped = "dropped"
	BombPlanted = "planted"
)

// Track holds the radar state of every captured play of one render.
type Track struct {
	Version     string               `json:"version"`
	DemoSHA256  string               `json:"demo_sha256"`
	TickRate    int                  `json:"tick_rate"`
	Map         string               `json:"map"`
	Calibration radarmap.Calibration `json:"calibration"`
	Segments    []Segment            `json:"segments"`
}

// Window is one capture window to sample, filmed from its POV player.
type Window struct {
	ID           string
	POVSteamID64 string
	TickStart    int
	TickEnd      int
}

// Segment is the radar of one capture window.
type Segment struct {
	ID        string `json:"id"`
	TickStart int    `json:"tick_start"`
	TickEnd   int    `json:"tick_end"`
	// POV is the slot of the filmed player; -1 when the demo never showed it in the window.
	POV     int       `json:"pov"`
	Frames  []Frame   `json:"frames"`
	Deaths  []Death   `json:"deaths,omitempty"`
	Utility []Utility `json:"utility,omitempty"`
}

// Frame is one sample: every living player and the bomb.
type Frame struct {
	Tick    int      `json:"tick"`
	Players []Player `json:"players,omitempty"`
	Bomb    *Bomb    `json:"bomb,omitempty"`
}

// Player is one living player; Yaw is CS2's view yaw in degrees, counter-clockwise from +X.
type Player struct {
	Slot int     `json:"slot"`
	Side string  `json:"side"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Z    float64 `json:"z"`
	Yaw  float64 `json:"yaw"`
}

// Bomb is where the C4 is. Carrier is the slot of the player holding it, for State carried.
type Bomb struct {
	State   string  `json:"state"`
	Carrier int     `json:"carrier"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Z       float64 `json:"z"`
}

// Death is a player killed in the round; Until is the tick the next round cleared it, 0 for never.
type Death struct {
	Slot  int     `json:"slot"`
	Side  string  `json:"side"`
	Tick  int     `json:"tick"`
	Until int     `json:"until,omitempty"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Z     float64 `json:"z"`
}

// Utility is one smoke, fire, flash or HE where it went off. EndTick is when a smoke or fire went
// out; 0 when it was still burning at the end of the window.
type Utility struct {
	Kind    string  `json:"kind"`
	Side    string  `json:"side,omitempty"`
	Tick    int     `json:"tick"`
	EndTick int     `json:"end_tick,omitempty"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Z       float64 `json:"z"`
}

// Load reads and checks a track written by the render worker.
func Load(path string) (Track, error) {
	// #nosec G304 -- path is the track the render worker wrote for this job.
	f, err := os.Open(path)
	if err != nil {
		return Track{}, fmt.Errorf("open radar track: %w", err)
	}
	defer f.Close()
	var track Track
	if err := json.NewDecoder(io.LimitReader(f, MaxBytes)).Decode(&track); err != nil {
		return Track{}, fmt.Errorf("decode radar track: %w", err)
	}
	return track, track.Validate()
}

// Validate refuses a track the overlay could misplace: another version, no calibration, frames out of order.
func (t Track) Validate() error {
	if t.Version != Version {
		return fmt.Errorf("radar track version %q, want %q", t.Version, Version)
	}
	if t.TickRate < 1 || t.TickRate > 1024 {
		return fmt.Errorf("radar track tick rate %d out of range", t.TickRate)
	}
	if !t.Calibration.Valid() {
		return fmt.Errorf("radar track for %q has no radar calibration", t.Map)
	}
	seen := map[string]bool{}
	for _, s := range t.Segments {
		if s.ID == "" || seen[s.ID] {
			return fmt.Errorf("radar segment id %q is empty or repeated", s.ID)
		}
		seen[s.ID] = true
		if err := s.validate(); err != nil {
			return fmt.Errorf("radar segment %s: %w", s.ID, err)
		}
	}
	return nil
}

func (s Segment) validate() error {
	for i, f := range s.Frames {
		if i > 0 && f.Tick <= s.Frames[i-1].Tick {
			return fmt.Errorf("frame ticks are not increasing at %d", f.Tick)
		}
		if b := f.Bomb; b != nil && b.State != BombCarried && b.State != BombDropped && b.State != BombPlanted {
			return fmt.Errorf("unknown bomb state %q", b.State)
		}
	}
	for _, u := range s.Utility {
		switch u.Kind {
		case KindSmoke, KindFire, KindFlash, KindHE:
		default:
			return fmt.Errorf("unknown utility kind %q", u.Kind)
		}
		if u.EndTick != 0 && u.EndTick < u.Tick {
			return fmt.Errorf("%s ends before it starts", u.Kind)
		}
	}
	return nil
}
