// Package prolineups turns pro match analyses into a lineup library: every
// grenade throw is read with its technique, outcome and context, throws of the
// same spot, aim, technique and landing are grouped into lineups, and each
// lineup and combo gets the usage numbers Studio shows.
package prolineups

import (
	"time"

	"github.com/rechedev9/cliphub/internal/placename"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// techniqueWindowSeconds is how far before the throw the movement sample is read.
const techniqueWindowSeconds = 0.25

// MatchMeta is what the source knows about a match beyond the demo itself.
type MatchMeta struct {
	Source   string    `json:"source"`
	MatchID  string    `json:"match_id"`
	MapKey   string    `json:"map_key"`
	Event    string    `json:"event"`
	PlayedAt time.Time `json:"played_at"`
	// TeamNames maps a document team key to the source's team name, when known.
	TeamNames map[string]string `json:"-"`
}

// Throw is one grenade a player threw in a pro match.
type Throw struct {
	Map        string                 `json:"map"`
	Kind       tacticalplan.EventKind `json:"kind"`
	Side       tacticalplan.Side      `json:"side"`
	Match      string                 `json:"match"`
	MapKey     string                 `json:"map_key"`
	Event      string                 `json:"event"`
	PlayedAt   time.Time              `json:"played_at"`
	Round      int                    `json:"round"`
	Team       string                 `json:"team"`
	SteamID64  string                 `json:"steamid64"`
	Player     string                 `json:"player"`
	Origin     [3]float64             `json:"origin"`
	Yaw        float64                `json:"yaw"`
	Pitch      float64                `json:"pitch"`
	Movement   string                 `json:"movement"`
	Jump       bool                   `json:"jump"`
	Landing    [3]float64             `json:"landing"`
	ThrowPlace string                 `json:"throw_place"`
	LandPlace  string                 `json:"land_place"`
	ThrowTick  int                    `json:"throw_tick"`
	Clock      float64                `json:"clock"`
	AirTime    float64                `json:"air_time"`
	RoundWon   bool                   `json:"round_won"`
	PlantSite  tacticalplan.Site      `json:"plant_site,omitempty"`
	EnemyHits  int                    `json:"enemy_hits"`
}

func isLineupKind(k tacticalplan.EventKind) bool {
	switch k {
	case tacticalplan.EventSmoke, tacticalplan.EventFlash, tacticalplan.EventHE, tacticalplan.EventMolotov:
		return true
	}
	return false
}

// ExtractThrows reads every smoke, flash, HE and molotov whose throw the demo
// showed. blob is the analysis position stream, used for the technique.
func ExtractThrows(doc tacticalplan.Document, blob []byte, meta MatchMeta) []Throw {
	rate := doc.Demo.Tickrate
	if rate <= 0 {
		return nil
	}
	players := map[uint8]tacticalplan.Player{}
	for _, p := range doc.Players {
		players[p.Slot] = p
	}
	teams := map[string]string{}
	for _, t := range doc.Teams {
		teams[t.Key] = t.Name
	}
	for k, v := range meta.TeamNames {
		teams[k] = v
	}
	out := []Throw{}
	for _, r := range doc.Rounds {
		for _, ev := range r.Events {
			if !isLineupKind(ev.Kind) || ev.Throw == nil || ev.ActorSlot == nil || ev.Side == "" {
				continue
			}
			p, ok := players[*ev.ActorSlot]
			if !ok {
				continue
			}
			t := Throw{
				Map: doc.Demo.Map, Kind: ev.Kind, Side: ev.Side, Match: meta.MatchID, MapKey: meta.MapKey, Event: meta.Event, PlayedAt: meta.PlayedAt,
				Round: r.Number, Team: teams[p.TeamKey], SteamID64: p.SteamID64, Player: p.Name,
				Origin: ev.Throw.Pos, Yaw: tacticalplan.SetangYaw(ev.Throw.Yaw), Pitch: tacticalplan.SetangPitch(ev.Throw.Pitch), Landing: ev.Pos,
				ThrowPlace: placename.Label(tacticalplan.NearestCallout(doc.Geometry, ev.Throw.Pos)), LandPlace: placename.Label(tacticalplan.NearestCallout(doc.Geometry, ev.Pos)),
				ThrowTick: ev.Throw.Tick, Clock: float64(ev.Throw.Tick-r.TickFreezeEnd) / rate, AirTime: float64(ev.Tick-ev.Throw.Tick) / rate,
				RoundWon: r.Winner == ev.Side,
			}
			if r.Bomb != nil && r.Bomb.PlantTick > 0 {
				t.PlantSite = r.Bomb.Site
			}
			if ev.Effect != nil {
				for _, b := range ev.Effect.Blinded {
					if b.Enemy {
						t.EnemyHits++
					}
				}
				if ev.Effect.EnemyDamage > 0 && t.EnemyHits == 0 {
					t.EnemyHits = 1
				}
			}
			if tech, ok := technique(blob, doc, r.Number, *ev.ActorSlot, ev.Throw.Tick); ok {
				t.Movement, t.Jump = tech.Movement, tech.Jump
			}
			out = append(out, t)
		}
	}
	return out
}

func technique(blob []byte, doc tacticalplan.Document, round int, slot uint8, tick int) (tacticalplan.Technique, bool) {
	if len(blob) == 0 {
		return tacticalplan.Technique{}, false
	}
	rate := doc.Demo.Tickrate
	at, okAt, errAt := tacticalplan.FrameAt(blob, doc, round, tick)
	before, okBefore, errBefore := tacticalplan.FrameAt(blob, doc, round, tick-int(techniqueWindowSeconds*rate))
	if errAt != nil || errBefore != nil || !okAt || !okBefore || at.Tick <= before.Tick {
		return tacticalplan.Technique{}, false
	}
	sAt, ok1 := sampleOf(at, slot)
	sBefore, ok2 := sampleOf(before, slot)
	if !ok1 || !ok2 {
		return tacticalplan.Technique{}, false
	}
	return tacticalplan.EstimateTechnique(sBefore, sAt, float64(at.Tick-before.Tick)/rate), true
}

func sampleOf(f tacticalplan.Frame, slot uint8) (tacticalplan.Sample, bool) {
	for _, s := range f.Samples {
		if s.Slot == slot {
			return s, true
		}
	}
	return tacticalplan.Sample{}, false
}
