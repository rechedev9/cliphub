package customhud

import (
	"fmt"
	"sort"
	"unicode"
)

// Timeline records the source clock, not the final edit clock. Every change is
// sampled at a demo FrameDone boundary and identical states are coalesced.
// EndTick attests how far the parser got, even when no visible value changed.
type Timeline struct {
	Version       string     `json:"version"`
	DemoSHA256    string     `json:"demo_sha256"`
	TargetSteamID string     `json:"target_steamid64"`
	Map           string     `json:"map"`
	TickRate      int        `json:"tick_rate"`
	EndTick       int        `json:"end_tick"`
	Snapshots     []Snapshot `json:"snapshots"`
}

type Snapshot struct {
	Tick          int      `json:"tick"`
	Round         int      `json:"round"`
	CTScore       int      `json:"ct_score"`
	TScore        int      `json:"t_score"`
	CTName        string   `json:"ct_name"`
	TName         string   `json:"t_name"`
	Phase         string   `json:"phase"`
	TimeRemaining int      `json:"time_remaining"` // -1 means unobserved, never a guessed timer.
	Players       []Player `json:"players"`
	Defuse        *Defuse  `json:"defuse,omitempty"`
	MVP           *MVP     `json:"mvp,omitempty"`
	// PlantTick is the tick of the round's plant, set only while planted or defusing.
	PlantTick int    `json:"plant_tick,omitempty"`
	Streak    Streak `json:"-"` // Derived by roundTracker at render time.
	// Swapped is true once the team that started CT plays T; SidesKnown says it was derived.
	Swapped    bool   `json:"-"`
	SidesKnown bool   `json:"-"`
	Winner     string `json:"-"` // Side that won the round, only while it is ended.
	// WinCT is the CT post-plant win chance in percent; 0 hides the overlay.
	WinCT int `json:"-"`
}

// Defuse is the defuse in progress. EndTick is 0 when the bomb's own
// completion deadline was not recorded; the length is never assumed.
type Defuse struct {
	Kit       bool `json:"kit"`
	StartTick int  `json:"start_tick"`
	EndTick   int  `json:"end_tick,omitempty"`
}

// MVP is the round's MVP as the game announced or counted it.
type MVP struct {
	SteamID string `json:"steamid64"`
	Reason  string `json:"reason,omitempty"`
}

const (
	MVPEliminations = "eliminations"
	MVPBombDefused  = "bomb_defused"
	MVPBombPlanted  = "bomb_planted"
)

// Streak is the run of consecutive round wins of the team now on Side.
type Streak struct {
	Side   string
	Rounds int
}

type Player struct {
	SteamID  string  `json:"steamid64"`
	Name     string  `json:"name"`
	Side     string  `json:"side"`
	Inactive bool    `json:"inactive,omitempty"` // Retained identity, outside the current roster.
	Known    bool    `json:"known"`              // Missing pawn/controller values are rendered as unavailable.
	Alive    bool    `json:"alive"`
	Health   int     `json:"health"`
	Armor    int     `json:"armor"`
	Money    int     `json:"money"`
	Kills    int     `json:"kills"`
	Deaths   int     `json:"deaths"`
	Assists  int     `json:"assists"`
	Weapon   string  `json:"weapon"`
	Ammo     int     `json:"ammo"` // -1 for equipment that has no magazine.
	Reserve  int     `json:"reserve"`
	Movement *uint64 `json:"movement,omitempty"` // Recorded actions; nil means unavailable, not released keys.
	// Equip is the game's freeze-end equipment value; 0 is unrecorded, never a guess.
	Equip int `json:"equip,omitempty"`
	// RoundKills is derived from the scoreboard at render time, never stored.
	RoundKills int `json:"-"`
}

func (d Timeline) Validate() error {
	if d.Version != TelemetryVersion || len(d.DemoSHA256) != 64 || len(d.TargetSteamID) != 17 || d.TickRate < 1 || d.TickRate > 1024 || d.EndTick < 1 {
		return fmt.Errorf("invalid custom HUD source identity or clock")
	}
	for _, c := range d.DemoSHA256 {
		if !((c >= 'a' && c <= 'f') || (c >= '0' && c <= '9')) {
			return fmt.Errorf("invalid custom HUD demo hash")
		}
	}
	for _, c := range d.TargetSteamID {
		if c < '0' || c > '9' {
			return fmt.Errorf("invalid custom HUD target")
		}
	}
	if len(d.Snapshots) == 0 || len(d.Snapshots) > 2_000_000 {
		return fmt.Errorf("invalid custom HUD snapshot count")
	}
	last := -1
	for _, s := range d.Snapshots {
		if s.Tick <= last || s.Tick > d.EndTick || s.Round < 0 || s.TimeRemaining < -1 || s.TimeRemaining > 86400 || s.CTScore < 0 || s.TScore < 0 || len(s.Players) > maxIdentities {
			return fmt.Errorf("invalid custom HUD state at tick %d", s.Tick)
		}
		last = s.Tick
		if !validRoundFacts(s, d.TickRate) {
			return fmt.Errorf("invalid custom HUD round facts at tick %d", s.Tick)
		}
		seen := map[string]bool{}
		for _, p := range s.Players {
			if seen[p.SteamID] || p.SteamID == "" || (p.Side != "CT" && p.Side != "T") || len(p.Name) > 512 || len(p.Weapon) > 128 || p.Health < 0 || p.Armor < 0 || p.Equip < 0 || p.Equip > 100000 {
				return fmt.Errorf("invalid custom HUD player at tick %d", s.Tick)
			}
			seen[p.SteamID] = true
		}
	}
	return nil
}

func validRoundFacts(s Snapshot, tickRate int) bool {
	if s.PlantTick != 0 && ((s.Phase != "planted" && s.Phase != "defusing") || s.PlantTick < 0 || s.PlantTick > s.Tick || s.Tick-s.PlantTick > 120*tickRate) {
		return false
	}
	if f := s.Defuse; f != nil {
		if s.Phase != "defusing" || f.StartTick < 0 || f.StartTick > s.Tick || (f.EndTick != 0 && (f.EndTick <= f.StartTick || f.EndTick-f.StartTick > 15*tickRate)) {
			return false
		}
	}
	if m := s.MVP; m != nil {
		if s.Phase != "ended" || m.SteamID == "" || len(m.SteamID) > 20 {
			return false
		}
		switch m.Reason {
		case "", MVPEliminations, MVPBombDefused, MVPBombPlanted:
		default:
			return false
		}
	}
	return true
}

func (d Timeline) At(tick int) (Snapshot, bool) {
	i := sort.Search(len(d.Snapshots), func(i int) bool { return d.Snapshots[i].Tick > tick }) - 1
	if i < 0 || tick > d.EndTick {
		return Snapshot{}, false
	}
	return d.Snapshots[i], true
}

func cleanText(s string, limit int) string {
	r := []rune{}
	for _, c := range s {
		if unicode.IsControl(c) || c == '\u2028' || c == '\u2029' {
			continue
		}
		r = append(r, c)
	}
	if len(r) > limit {
		return string(r[:max(0, limit-1)]) + "…"
	}
	return string(r)
}

// Example is explicitly illustrative. It is used by the design picker only;
// delivery always requires telemetry extracted from the approved demo.
const ExampleTarget = "76561199000000001"

func Example() Snapshot {
	s := Snapshot{Round: 3, CTScore: 0, TScore: 2, CTName: "COUNTER-TERRORISTS", TName: "TERRORISTS", Phase: "live", TimeRemaining: 59}
	names := []string{"eskyy", "donk", "po4kagod", "BLR1337", "sh1ro", "KWEZZLINGEN", "JustGreat-", "frozen", "rain", "ropz"}
	for i, name := range names {
		side := "CT"
		if i >= 5 {
			side = "T"
		}
		alive := i < 4 || i == 8
		hp := 100
		if !alive {
			hp = 0
		}
		if i == 2 {
			hp = 74
		}
		weapon := "M4A1-S"
		if side == "T" {
			weapon = "AK-47"
		}
		s.Players = append(s.Players, Player{SteamID: fmt.Sprintf("765611990000000%02d", i), Name: name, Side: side, Known: true, Alive: alive, Health: hp, Armor: 100, Money: 2450 + i*150, Kills: 2 + i%4, Deaths: 1, Assists: i % 2, Weapon: weapon, Ammo: 9, Reserve: 2})
	}
	left := uint64(0x200)
	s.Players[1].Movement = &left
	return s
}
