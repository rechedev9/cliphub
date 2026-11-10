// Package botreplay builds "Practicar contra esta ronda": a CS2 practice cfg in
// which bots stand where the other side stood at the key moments of a
// recorded round. Plain CS2 cannot make a bot walk a recorded path or throw a
// grenade on cue without a server plugin, so the plan is moments, not motion.
package botreplay

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/rechedev9/cliphub/internal/practice"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const (
	// MaxMoments bounds the snapshots one keypress cycle walks through.
	MaxMoments = 12
	// MaxLineups is one per keypad digit.
	MaxLineups = 9
	// MaxBots is one team.
	MaxBots = 5
	// MaxNotes bounds the grenades one moment echoes, so its alias stays short.
	MaxNotes = 4
	// mergeSeconds folds moments this close into one.
	mergeSeconds  = 2.0
	defaultRate   = 64.0
	maxSteamIDLen = 20
	// maxNameRunes keeps every echo and alias of the cfg short.
	maxNameRunes = 20
)

// ErrInvalid marks a request the caller must fix; the message says what.
var ErrInvalid = errors.New("invalid bot replay request")

// ErrNoPositions means the round has no sampled positions to place bots from.
var ErrNoPositions = errors.New("round has no saved positions")

// Options is one round and who the user plays.
type Options struct {
	Map    string
	Doc    tacticalplan.Document
	Round  tacticalplan.Round
	Frames []tacticalplan.Frame
	// Side is the side the user plays; the bots are the other side.
	Side tacticalplan.Side
	// ReplaceSteamID is the player of Side the user starts as; empty takes the first.
	ReplaceSteamID string
}

// Pose is a feet position and setang angles.
type Pose struct {
	Pos   [3]float64 `json:"pos"`
	Pitch float64    `json:"pitch"`
	Yaw   float64    `json:"yaw"`
}

// Player is a recorded player with a console-safe name.
type Player struct {
	Slot      uint8  `json:"-"`
	Name      string `json:"name"`
	SteamID64 string `json:"steamid64"`
}

// Bot is one player of the other side. Index is its setpos_player number at
// the default base; the user can shift the base in game.
type Bot struct {
	Player
	Index int  `json:"index"`
	Spawn Pose `json:"-"`
}

// Start is a player of the user's side the user can start as ("otra salida").
type Start struct {
	Player
	Spawn Pose `json:"-"`
}

// BotPose is where a bot stands at a moment; a dead player's bot waits at its spawn.
type BotPose struct {
	Pose
	Alive bool
}

// Moment is one snapshot of the round.
type Moment struct {
	Tick    int       `json:"tick"`
	Seconds float64   `json:"seconds"`
	Label   string    `json:"label"`
	Notes   []string  `json:"notes"`
	Bots    []BotPose `json:"-"`
	// Users has one pose per Start, nil once that player is dead.
	Users []*Pose `json:"-"`
}

// Lineup is a grenade of the other side the user can throw from its spot.
type Lineup struct {
	Seconds float64 `json:"seconds"`
	Kind    string  `json:"kind"`
	Player  string  `json:"player"`
	Key     string  `json:"key"`
	Weapon  string  `json:"-"`
	Pose    Pose    `json:"-"`
}

// Plan is a validated bot replay of one round.
type Plan struct {
	Map      string
	Round    int
	UserSide tacticalplan.Side
	BotSide  tacticalplan.Side
	Bots     []Bot
	Starts   []Start
	Current  int
	Moments  []Moment
	Lineups  []Lineup
}

type grenade struct {
	tick   int
	kind   string
	weapon string
	player string
	throw  *Pose
}

// Build reads the round into a plan. Frames are the round's samples in tick order.
func Build(opts Options) (Plan, error) {
	if !practice.ValidMap(opts.Map) {
		return Plan{}, invalid("map must match de_<name>")
	}
	if opts.Side != tacticalplan.SideT && opts.Side != tacticalplan.SideCT {
		return Plan{}, invalid("side must be T or CT")
	}
	if len(opts.ReplaceSteamID) > maxSteamIDLen {
		return Plan{}, invalid("replace_steamid must be a SteamID64")
	}
	rate := opts.Doc.Demo.Tickrate
	if rate <= 0 {
		rate = defaultRate
	}
	r := opts.Round
	start := frameAt(opts.Frames, r.TickFreezeEnd)
	if start < 0 {
		return Plan{}, ErrNoPositions
	}
	plan := Plan{Map: opts.Map, Round: r.Number, UserSide: opts.Side, BotSide: otherSide(opts.Side)}
	for _, pr := range sortedPlayers(r.Players) {
		p, ok := opts.Doc.PlayerBySlot(pr.Slot)
		if !ok {
			continue
		}
		spawn, ok := firstPose(opts.Frames[start:], pr.Slot)
		if !ok {
			continue
		}
		player := Player{Slot: pr.Slot, Name: displayName(p.Name, int(pr.Slot)), SteamID64: p.SteamID64}
		switch {
		case pr.Side == plan.BotSide && len(plan.Bots) < MaxBots:
			plan.Bots = append(plan.Bots, Bot{Player: player, Index: DefaultBase + len(plan.Bots), Spawn: spawn})
		case pr.Side == plan.UserSide:
			plan.Starts = append(plan.Starts, Start{Player: player, Spawn: spawn})
		}
	}
	if len(plan.Bots) == 0 {
		return Plan{}, invalid(fmt.Sprintf("round %d has no %s player with saved positions", r.Number, plan.BotSide))
	}
	if opts.ReplaceSteamID != "" {
		i := slices.IndexFunc(plan.Starts, func(s Start) bool { return s.SteamID64 == opts.ReplaceSteamID })
		if i < 0 {
			return Plan{}, invalid(fmt.Sprintf("replace_steamid did not play %s in round %d", opts.Side, r.Number))
		}
		plan.Current = i
	}
	end := opts.Frames[len(opts.Frames)-1].Tick
	if r.TickEnd > 0 && r.TickEnd < end {
		end = r.TickEnd
	}
	grenades := roundGrenades(opts.Doc, r, plan.BotSide)
	ticks := momentTicks(momentOptions{round: r, grenades: grenades, end: end, merge: int(mergeSeconds * rate)})
	for _, mt := range ticks {
		plan.Moments = append(plan.Moments, buildMoment(plan, opts.Frames, mt, seconds(mt.tick-r.TickFreezeEnd, rate)))
	}
	attachNotes(plan.Moments, grenades)
	for _, g := range grenades {
		if g.throw == nil || len(plan.Lineups) == MaxLineups {
			continue
		}
		n := len(plan.Lineups) + 1
		plan.Lineups = append(plan.Lineups, Lineup{
			Seconds: seconds(g.tick-r.TickFreezeEnd, rate), Kind: g.kind, Player: g.player,
			Key: practice.SpotKey(n), Weapon: g.weapon, Pose: *g.throw,
		})
	}
	return plan, nil
}

func sortedPlayers(players []tacticalplan.PlayerRound) []tacticalplan.PlayerRound {
	out := slices.Clone(players)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Slot < out[j].Slot })
	return out
}

func otherSide(side tacticalplan.Side) tacticalplan.Side {
	if side == tacticalplan.SideT {
		return tacticalplan.SideCT
	}
	return tacticalplan.SideT
}

func displayName(name string, slot int) string {
	if clean := practice.SanitizeLabel(name); clean != "" {
		if runes := []rune(clean); len(runes) > maxNameRunes {
			clean = strings.TrimSpace(string(runes[:maxNameRunes]))
		}
		return clean
	}
	return fmt.Sprintf("Jugador %d", slot+1)
}

// frameAt is the first frame at or after tick, the last one past the end, or -1.
func frameAt(frames []tacticalplan.Frame, tick int) int {
	if len(frames) == 0 {
		return -1
	}
	for i, f := range frames {
		if f.Tick >= tick {
			return i
		}
	}
	return len(frames) - 1
}

func sampleOf(f tacticalplan.Frame, slot uint8) (tacticalplan.Sample, bool) {
	for _, s := range f.Samples {
		if s.Slot == slot {
			return s, true
		}
	}
	return tacticalplan.Sample{}, false
}

func poseOf(s tacticalplan.Sample) Pose {
	return Pose{Pos: [3]float64{s.X, s.Y, s.Z}, Pitch: tacticalplan.SetangPitch(s.Pitch), Yaw: tacticalplan.SetangYaw(s.Yaw)}
}

func firstPose(frames []tacticalplan.Frame, slot uint8) (Pose, bool) {
	for _, f := range frames {
		if s, ok := sampleOf(f, slot); ok {
			return poseOf(s), true
		}
	}
	return Pose{}, false
}

func roundGrenades(doc tacticalplan.Document, r tacticalplan.Round, side tacticalplan.Side) []grenade {
	var out []grenade
	seen := map[string]bool{}
	for _, ev := range r.Events {
		kind := string(ev.Kind)
		switch ev.Kind {
		case tacticalplan.EventSmoke, tacticalplan.EventFlash, tacticalplan.EventHE, tacticalplan.EventMolotov:
		default:
			continue
		}
		if ev.Side != side || ev.Tick < r.TickFreezeEnd {
			continue
		}
		if ev.Kind == tacticalplan.EventMolotov && side == tacticalplan.SideCT {
			kind = "incendiary"
		}
		g := grenade{tick: ev.Tick, kind: kind, weapon: practice.GrenadeWeapon(kind), player: "?"}
		if ev.Throw != nil {
			g.tick = ev.Throw.Tick
			g.throw = &Pose{Pos: ev.Throw.Pos, Pitch: tacticalplan.SetangPitch(ev.Throw.Pitch), Yaw: tacticalplan.SetangYaw(ev.Throw.Yaw)}
		}
		if ev.ActorSlot != nil {
			if p, ok := doc.PlayerBySlot(*ev.ActorSlot); ok {
				g.player = displayName(p.Name, int(p.Slot))
			}
		}
		// Some demos report one flash twice at the same tick and spot.
		key := fmt.Sprintf("%s|%d|%v", kind, ev.Tick, ev.Pos)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, g)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].tick < out[j].tick })
	return out
}

type momentTick struct {
	tick   int
	labels []string
}

type momentOptions struct {
	round    tacticalplan.Round
	grenades []grenade
	end      int
	merge    int
}

func momentTicks(o momentOptions) []momentTick {
	r := o.round
	cands := []momentTick{}
	for _, g := range o.grenades {
		cands = append(cands, momentTick{tick: g.tick, labels: []string{"Granadas"}})
	}
	if r.Class.FirstContactTick > 0 {
		cands = append(cands, momentTick{tick: r.Class.FirstContactTick, labels: []string{"Primer contacto"}})
	}
	if r.Bomb != nil && r.Bomb.PlantTick > 0 {
		cands = append(cands, momentTick{tick: r.Bomb.PlantTick, labels: []string{"Plantada"}})
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].tick < cands[j].tick })
	out := []momentTick{{tick: r.TickFreezeEnd, labels: []string{"Salida"}}}
	for _, c := range cands {
		if c.tick <= r.TickFreezeEnd || c.tick > o.end {
			continue
		}
		last := &out[len(out)-1]
		if c.tick-last.tick < o.merge {
			for _, l := range c.labels {
				if !slices.Contains(last.labels, l) {
					last.labels = append(last.labels, l)
				}
			}
			continue
		}
		out = append(out, c)
	}
	return thin(out, MaxMoments)
}

// thin keeps the first moment and spreads the rest evenly, always keeping the last.
func thin(moments []momentTick, limit int) []momentTick {
	if len(moments) <= limit {
		return moments
	}
	out := []momentTick{moments[0]}
	rest := moments[1:]
	slots := limit - 1
	for i := 0; i < slots; i++ {
		idx := int(math.Round(float64(i+1) * float64(len(rest)-1) / float64(slots)))
		if idx >= 0 && idx < len(rest) && rest[idx].tick > out[len(out)-1].tick {
			out = append(out, rest[idx])
		}
	}
	return out
}

func buildMoment(plan Plan, frames []tacticalplan.Frame, mt momentTick, secs float64) Moment {
	f := frames[frameAt(frames, mt.tick)]
	m := Moment{Tick: mt.tick, Seconds: secs, Label: strings.Join(mt.labels, ", "), Notes: []string{}}
	for _, b := range plan.Bots {
		s, ok := sampleOf(f, b.Slot)
		if ok && s.Flags.Has(tacticalplan.FlagAlive) {
			m.Bots = append(m.Bots, BotPose{Pose: poseOf(s), Alive: true})
		} else {
			m.Bots = append(m.Bots, BotPose{Pose: b.Spawn})
		}
	}
	for _, st := range plan.Starts {
		s, ok := sampleOf(f, st.Slot)
		if ok && s.Flags.Has(tacticalplan.FlagAlive) {
			p := poseOf(s)
			m.Users = append(m.Users, &p)
		} else {
			m.Users = append(m.Users, nil)
		}
	}
	return m
}

// attachNotes lists each grenade on the latest moment at or before its throw.
func attachNotes(moments []Moment, grenades []grenade) {
	extra := make([]int, len(moments))
	for _, g := range grenades {
		i := len(moments) - 1
		for i > 0 && moments[i].Tick > g.tick {
			i--
		}
		if len(moments[i].Notes) == MaxNotes {
			extra[i]++
			continue
		}
		moments[i].Notes = append(moments[i].Notes, fmt.Sprintf("%s de %s", grenadeLabel(g.kind), g.player))
	}
	for i, n := range extra {
		if n > 0 {
			moments[i].Notes = append(moments[i].Notes, fmt.Sprintf("y %d más", n))
		}
	}
}

func grenadeLabel(kind string) string {
	switch kind {
	case "smoke":
		return "Humo"
	case "flash":
		return "Flash"
	case "he":
		return "HE"
	case "incendiary":
		return "Incendiaria"
	default:
		return "Molotov"
	}
}

func seconds(ticks int, rate float64) float64 {
	return math.Round(float64(ticks)/rate*10) / 10
}

func invalid(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalid, msg)
}
