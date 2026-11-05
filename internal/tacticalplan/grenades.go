package tacticalplan

import (
	"math"
	"slices"

	"github.com/google/uuid"
)

// GrenadeKinds lists the grenade kinds, in the order a grenade query names them.
var GrenadeKinds = []EventKind{EventSmoke, EventFlash, EventHE, EventMolotov, EventDecoy}

// TeamGrenade is one grenade the scouted team threw. Origin, ThrowTick and View
// are nil for a match analysed before throws were recorded.
type TeamGrenade struct {
	MatchID          uuid.UUID   `json:"match_id"`
	Round            int         `json:"round"`
	Tick             int         `json:"tick"`
	Seconds          float64     `json:"seconds"`
	Kind             EventKind   `json:"kind"`
	Side             Side        `json:"side"`
	ThrowerSteamID64 string      `json:"thrower_steamid64"`
	ThrowerName      string      `json:"thrower_name"`
	Origin           *[3]float64 `json:"origin"`
	ThrowTick        *int        `json:"throw_tick"`
	View             *[2]float64 `json:"view"`
	Landing          [3]float64  `json:"landing"`
	Buy              BuyType     `json:"buy"`
	RoundWon         bool        `json:"round_won"`
}

// TeamGrenades is every grenade of the team in the rounds the filter kept.
type TeamGrenades struct {
	Grenades []TeamGrenade `json:"grenades"`
	// Rounds is the team's rounds the filter kept, with or without grenades.
	Rounds int `json:"rounds"`
}

// CollectTeamGrenades lists the team's grenades of the kinds asked for (all of
// them when kinds is empty), newest match first and in game order inside it.
// Tick is when the grenade went off and Seconds that tick after freeze end.
func CollectTeamGrenades(matches []ScoutMatch, f ScoutFilter, kinds []EventKind) TeamGrenades {
	out := TeamGrenades{Grenades: []TeamGrenade{}}
	for _, m := range f.Select(matches) {
		doc := m.Document
		for _, r := range f.rounds(m) {
			out.Rounds++
			side := doc.TeamSide(m.TeamKey, r)
			team := teamSlots(r, side)
			for _, ev := range r.Events {
				if !isGrenadeEvent(ev.Kind) || ev.ActorSlot == nil || !team[*ev.ActorSlot] {
					continue
				}
				if len(kinds) > 0 && !slices.Contains(kinds, ev.Kind) {
					continue
				}
				out.Grenades = append(out.Grenades, teamGrenade(m, r, side, ev))
			}
		}
	}
	return out
}

func teamGrenade(m ScoutMatch, r Round, side Side, ev Event) TeamGrenade {
	g := TeamGrenade{
		MatchID: m.MatchID, Round: r.Number, Tick: ev.Tick, Kind: ev.Kind, Side: side,
		Landing: ev.Pos, Buy: r.Economy.Buy(side), RoundWon: r.Winner == side,
	}
	if secs, ok := secondsSince(r, ev.Tick, m.Document.Demo.Tickrate); ok {
		g.Seconds = math.Round(secs*100) / 100
	}
	if p, ok := m.Document.PlayerBySlot(*ev.ActorSlot); ok {
		g.ThrowerSteamID64, g.ThrowerName = p.SteamID64, p.Name
	}
	if t := ev.Throw; t != nil {
		origin, tick, view := t.Pos, t.Tick, [2]float64{t.Yaw, t.Pitch}
		g.Origin, g.ThrowTick, g.View = &origin, &tick, &view
	}
	return g
}
