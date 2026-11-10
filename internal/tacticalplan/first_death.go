package tacticalplan

import (
	"fmt"
	"math"
	"sort"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

// Movement is how fast a player moved at a sample, from the distance to their previous sample.
type Movement string

// Movement values; the speeds are in world units per second on the ground plane.
const (
	MovementStatic Movement = "static"
	MovementSlow   Movement = "slow"
	MovementFast   Movement = "fast"
)

const (
	// StaticBelowSpeed is the speed under which a player holds still.
	StaticBelowSpeed = 20.0
	// FastAboveSpeed is the speed over which a player runs; between the two they walk.
	FastAboveSpeed = 140.0
	// FirstDeathZoneLimit caps the zones listed per player.
	FirstDeathZoneLimit = 5
	// FirstDeathSampleLimit caps the positions a zone sends to draw.
	FirstDeathSampleLimit = 400
	// maxSampleGapSeconds caps the time one sample stands for, so a gap in the blob is not counted as time held.
	maxSampleGapSeconds = 1.0
)

// ParseMovement reads the movement of a query; empty is static, the view's default.
func ParseMovement(raw string) (Movement, error) {
	switch m := Movement(raw); m {
	case "":
		return MovementStatic, nil
	case MovementStatic, MovementSlow, MovementFast:
		return m, nil
	default:
		return "", fmt.Errorf("movement must be static, slow or fast, not %q", raw)
	}
}

func movementOf(speed float64) Movement {
	switch {
	case speed < StaticBelowSpeed:
		return MovementStatic
	case speed > FastAboveSpeed:
		return MovementFast
	default:
		return MovementSlow
	}
}

// FirstDeathOptions selects the rounds (the filter's side is required), the
// window on the round clock and the movement the samples must show.
type FirstDeathOptions struct {
	Filter   ScoutFilter
	Window   PrepWindow
	Movement Movement
	// Roles is the inferred role per SteamID; a missing one reads "unknown".
	Roles map[string]string
}

// FirstDeathZone is a callout where a player spent time in the window before
// the round's first death. Seconds is per round the player played.
type FirstDeathZone struct {
	Callout string       `json:"callout"`
	Seconds float64      `json:"seconds"`
	Rounds  int          `json:"rounds"`
	Refs    []Evidence   `json:"refs"`
	Samples [][3]float64 `json:"samples"`
	Floor   string       `json:"floor,omitempty"`
}

// FirstDeathPlayer is one player of the team and their zones, most time first.
type FirstDeathPlayer struct {
	SteamID64 string           `json:"steamid64"`
	Name      string           `json:"name"`
	Role      string           `json:"role"`
	Rounds    int              `json:"rounds"`
	Zones     []FirstDeathZone `json:"zones"`
}

// FirstDeathThresholds are the rules the view applied.
type FirstDeathThresholds struct {
	StaticBelowSpeed float64 `json:"static_below_speed"`
	FastAboveSpeed   float64 `json:"fast_above_speed"`
	ZoneLimit        int     `json:"zone_limit"`
	SampleLimit      int     `json:"sample_limit"`
}

// FirstDeathPositions is where each player of a team stands on one side until
// the round's first death, in one window of the round and at one movement.
type FirstDeathPositions struct {
	Map        string               `json:"map"`
	Side       Side                 `json:"side"`
	Window     PrepWindow           `json:"window"`
	Movement   Movement             `json:"movement"`
	Thresholds FirstDeathThresholds `json:"thresholds"`
	Matches    int                  `json:"matches"`
	RoundCount int                  `json:"round_count"`
	// Framed counts the rounds with positions, which the players are read from.
	Framed  int                `json:"framed"`
	Players []FirstDeathPlayer `json:"players"`
}

type firstDeathZoneAcc struct {
	seconds   float64
	samples   int
	sum       [3]float64
	positions [][3]float64
	refs      []Evidence
}

type firstDeathPlayerAcc struct {
	name   string
	rounds int
	zones  map[string]*firstDeathZoneAcc
}

type firstDeathRun struct {
	opts    FirstDeathOptions
	players map[string]*firstDeathPlayerAcc
}

// BuildFirstDeathPositions reads the team's rounds on the filter's side.
func BuildFirstDeathPositions(matches []FramedMatch, opts FirstDeathOptions) FirstDeathPositions {
	f := opts.Filter
	out := FirstDeathPositions{
		Side: f.Side, Window: opts.Window, Movement: opts.Movement, Players: []FirstDeathPlayer{},
		Thresholds: FirstDeathThresholds{
			StaticBelowSpeed: StaticBelowSpeed, FastAboveSpeed: FastAboveSpeed,
			ZoneLimit: FirstDeathZoneLimit, SampleLimit: FirstDeathSampleLimit,
		},
	}
	if len(f.Maps) == 1 {
		out.Map = f.Maps[0]
	}
	run := firstDeathRun{opts: opts, players: map[string]*firstDeathPlayerAcc{}}
	selected := selectFramed(matches, f)
	out.Matches = len(selected)
	for _, m := range selected {
		src := openFrames(m)
		for _, r := range f.rounds(m.ScoutMatch) {
			if m.Document.TeamSide(m.TeamKey, r) != f.Side {
				continue
			}
			out.RoundCount++
			frames := src.round(r.Number)
			if len(frames) == 0 {
				continue
			}
			out.Framed++
			run.addRound(m, r, frames)
		}
	}
	out.Players = run.rows(out.Map)
	return out
}

type lastSample struct {
	tick int
	at   [3]float64
}

// addRound counts the round for each player and adds the window's samples before the first death
// that match the movement; a sample stands for the time since the player's previous one (1/8 s at 8 Hz).
func (run *firstDeathRun) addRound(m FramedMatch, r Round, frames []Frame) {
	doc := m.Document
	team := teamSlots(r, run.opts.Filter.Side)
	for slot := range team {
		id, name := playerIdentity(doc, slot)
		run.player(id, name).rounds++
	}
	cut := r.Class.OpeningTick
	if cut <= 0 {
		cut = r.Class.FirstContactTick
	}
	prev := map[uint8]lastSample{}
	seen := map[string]bool{}
	for _, fr := range teamTrack(doc, r, team, frames) {
		if cut > 0 && fr.tick >= cut {
			break
		}
		for slot, at := range fr.pos {
			last, had := prev[slot]
			prev[slot] = lastSample{tick: fr.tick, at: at}
			if !had || !run.opts.Window.contains(fr.seconds) || doc.Demo.Tickrate <= 0 {
				continue
			}
			dt := float64(fr.tick-last.tick) / doc.Demo.Tickrate
			if dt <= 0 || movementOf(math.Hypot(at[0]-last.at[0], at[1]-last.at[1])/dt) != run.opts.Movement {
				continue
			}
			id, _ := playerIdentity(doc, slot)
			callout := fr.callouts[slot]
			zone := run.players[id].zone(callout)
			zone.seconds += min(dt, maxSampleGapSeconds)
			zone.samples++
			zone.positions = append(zone.positions, [3]float64{math.Round(at[0]), math.Round(at[1]), math.Round(at[2])})
			for k := range at {
				zone.sum[k] += at[k]
			}
			if key := id + "|" + callout; !seen[key] {
				seen[key] = true
				zone.refs = append(zone.refs, evidence(m.ScoutMatch, r, fr.tick))
			}
		}
	}
}

func (run *firstDeathRun) player(id, name string) *firstDeathPlayerAcc {
	acc, ok := run.players[id]
	if !ok {
		acc = &firstDeathPlayerAcc{name: name, zones: map[string]*firstDeathZoneAcc{}}
		run.players[id] = acc
	}
	return acc
}

func (acc *firstDeathPlayerAcc) zone(callout string) *firstDeathZoneAcc {
	z, ok := acc.zones[callout]
	if !ok {
		z = &firstDeathZoneAcc{}
		acc.zones[callout] = z
	}
	return z
}

// thinned keeps at most limit positions, evenly spread over the samples.
func thinned(positions [][3]float64, limit int) [][3]float64 {
	if len(positions) <= limit {
		return positions
	}
	out := make([][3]float64, 0, limit)
	for i := range limit {
		out = append(out, positions[i*len(positions)/limit])
	}
	return out
}

func (run *firstDeathRun) rows(mapName string) []FirstDeathPlayer {
	cal, calibrated := radarmap.Lookup(mapName)
	out := make([]FirstDeathPlayer, 0, len(run.players))
	for id, acc := range run.players {
		role := run.opts.Roles[id]
		if role == "" {
			role = "unknown"
		}
		p := FirstDeathPlayer{SteamID64: id, Name: acc.name, Role: role, Rounds: acc.rounds, Zones: make([]FirstDeathZone, 0, len(acc.zones))}
		for callout, z := range acc.zones {
			zone := FirstDeathZone{
				Callout: callout, Seconds: math.Round(z.seconds/float64(acc.rounds)*100) / 100, Rounds: len(z.refs),
				Refs: z.refs, Samples: thinned(z.positions, FirstDeathSampleLimit),
			}
			if calibrated && cal.Level(z.sum[2]/float64(z.samples)) == radarmap.LevelLower {
				zone.Floor = radarmap.LevelLower
			}
			p.Zones = append(p.Zones, zone)
		}
		sort.Slice(p.Zones, func(i, j int) bool {
			if p.Zones[i].Seconds != p.Zones[j].Seconds {
				return p.Zones[i].Seconds > p.Zones[j].Seconds
			}
			return p.Zones[i].Callout < p.Zones[j].Callout
		})
		if len(p.Zones) > FirstDeathZoneLimit {
			p.Zones = p.Zones[:FirstDeathZoneLimit]
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rounds != out[j].Rounds {
			return out[i].Rounds > out[j].Rounds
		}
		return out[i].SteamID64 < out[j].SteamID64
	})
	return out
}
