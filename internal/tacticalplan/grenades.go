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
	// LandPlace and ThrowPlace are the callouts of the landing and of the origin.
	LandPlace  string `json:"land_place"`
	ThrowPlace string `json:"throw_place"`
	// Spot is the key of the repeated throw this grenade belongs to (see assignSpots).
	Spot  string       `json:"spot"`
	After GrenadeAfter `json:"after"`
}

// GrenadeRound is one round the filter kept, seen from the side of the throwers.
type GrenadeRound struct {
	MatchID     uuid.UUID `json:"match_id"`
	Round       int       `json:"round"`
	Tick        int       `json:"tick"`
	Side        Side      `json:"side"`
	Buy         BuyType   `json:"buy"`
	OpponentBuy BuyType   `json:"opponent_buy"`
	Won         bool      `json:"won"`
	// Category is the round's playbook category and Site where it went.
	Category string `json:"category"`
	Site     Site   `json:"site"`
	// PlantSeconds is when the bomb went down after freeze end; nil without a plant.
	PlantSeconds *float64 `json:"plant_seconds"`
}

// TeamGrenades is every grenade of the team in the rounds the filter kept.
type TeamGrenades struct {
	Grenades []TeamGrenade `json:"grenades"`
	// Rounds is the team's rounds the filter kept, with or without grenades;
	// RoundList is those rounds, the denominator of every share of the grenades.
	Rounds    int            `json:"rounds"`
	RoundList []GrenadeRound `json:"round_list"`
}

// GrenadeQuery is what a grenade read narrows beyond the scout filter.
type GrenadeQuery struct {
	// Kinds keeps only these kinds; empty keeps them all.
	Kinds []EventKind
	// Rivals reads the grenades the team's rivals threw in the same rounds.
	Rivals bool
}

// CollectTeamGrenades lists the team's grenades of the kinds asked for, newest
// match first and in game order inside it, with what the team did after each
// one and the rounds they came from. Tick is when the grenade went off and
// Seconds that tick after freeze end.
func CollectTeamGrenades(matches []FramedMatch, f ScoutFilter, q GrenadeQuery) TeamGrenades {
	out := TeamGrenades{Grenades: []TeamGrenade{}, RoundList: []GrenadeRound{}}
	for _, m := range selectFramed(matches, f) {
		src := openFrames(m)
		doc := m.Document
		for _, r := range f.rounds(m.ScoutMatch) {
			side := doc.TeamSide(m.TeamKey, r)
			if q.Rivals {
				side = side.Opponent()
			}
			team := teamSlots(r, side)
			track := teamTrack(doc, r, team, src.round(r.Number))
			category, _ := readPlaybookTrack(doc, r, side, track).category()
			out.RoundList = append(out.RoundList, grenadeRound(m.ScoutMatch, r, side, category))
			for _, ev := range r.Events {
				if !isGrenadeEvent(ev.Kind) || ev.ActorSlot == nil || !team[*ev.ActorSlot] {
					continue
				}
				if len(q.Kinds) > 0 && !slices.Contains(q.Kinds, ev.Kind) {
					continue
				}
				g := teamGrenade(m.ScoutMatch, r, side, ev)
				g.After = grenadeAfter(afterRead{doc: doc, round: r, team: team, track: track}, ev.Tick)
				out.Grenades = append(out.Grenades, g)
			}
		}
	}
	out.Rounds = len(out.RoundList)
	assignSpots(out.Grenades)
	return out
}

func grenadeRound(m ScoutMatch, r Round, side Side, category string) GrenadeRound {
	g := GrenadeRound{
		MatchID: m.MatchID, Round: r.Number, Tick: evidence(m, r, 0).Tick, Side: side,
		Buy: r.Economy.Buy(side), OpponentBuy: r.Economy.Buy(side.Opponent()), Won: r.Winner == side,
		Category: category, Site: r.Class.Site,
	}
	if r.Bomb != nil && r.Bomb.PlantTick > 0 {
		if secs, ok := secondsSince(r, r.Bomb.PlantTick, m.Document.Demo.Tickrate); ok {
			secs = math.Round(secs*100) / 100
			g.PlantSeconds = &secs
		}
	}
	return g
}

func teamGrenade(m ScoutMatch, r Round, side Side, ev Event) TeamGrenade {
	g := TeamGrenade{
		MatchID: m.MatchID, Round: r.Number, Tick: ev.Tick, Kind: ev.Kind, Side: side,
		Landing: ev.Pos, Buy: r.Economy.Buy(side), RoundWon: r.Winner == side,
		LandPlace: NearestCallout(m.Document.Geometry, ev.Pos),
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
		g.ThrowPlace = NearestCallout(m.Document.Geometry, t.Pos)
	}
	return g
}
