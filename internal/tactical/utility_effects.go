package tactical

import (
	"math"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// How far from a grenade event a blind or a damage hit may land and still be
// credited to it. Flash and HE effects arrive on the detonation tick, give or
// take the order the parser dispatches them in; a molotov burns for 7 s.
const (
	flashAttachSeconds   = 1.0
	heAttachSeconds      = 0.5
	molotovAttachSeconds = 8.0
)

// utilityHit is one blind or damage hit, keyed by provisional slots like the
// round's events, waiting to be attached to the grenade that caused it.
type utilityHit struct {
	kind       tacticalplan.EventKind
	tick       int
	attacker   uint8
	victim     uint8
	enemy      bool
	seconds    float64
	healthLost int
}

func (s *scanner) onFlashed(e events.PlayerFlashed) {
	r := s.current
	if r == nil || e.Player == nil || e.Attacker == nil || e.Player == e.Attacker || !e.Player.IsAlive() {
		return
	}
	seconds := float64(e.Player.FlashDuration)
	if seconds <= 0 {
		return
	}
	attacker, ok := s.slotFor(e.Attacker)
	if !ok {
		return
	}
	victim, ok := s.slotFor(e.Player)
	if !ok {
		return
	}
	r.utilityHits = append(r.utilityHits, utilityHit{
		kind: tacticalplan.EventFlash, tick: s.tick(), attacker: attacker, victim: victim,
		enemy: e.Attacker.Team != e.Player.Team, seconds: math.Round(seconds*100) / 100,
	})
}

// recordUtilityDamage keeps HE and fire damage, including damage to teammates,
// which the player damage totals leave out.
func (s *scanner) recordUtilityDamage(e events.PlayerHurt) {
	r := s.current
	if r == nil || e.Weapon == nil || e.Player == e.Attacker || e.HealthDamageTaken <= 0 {
		return
	}
	var kind tacticalplan.EventKind
	switch e.Weapon.Type {
	case common.EqHE:
		kind = tacticalplan.EventHE
	case common.EqMolotov, common.EqIncendiary:
		kind = tacticalplan.EventMolotov
	default:
		return
	}
	attacker, ok := s.slotFor(e.Attacker)
	if !ok {
		return
	}
	victim, ok := s.slotFor(e.Player)
	if !ok {
		return
	}
	r.utilityHits = append(r.utilityHits, utilityHit{
		kind: kind, tick: s.tick(), attacker: attacker, victim: victim,
		enemy: e.Attacker.Team != e.Player.Team, healthLost: e.HealthDamageTaken,
	})
}

// attachUtilityEffects credits each hit to the grenade event of the same kind
// and thrower that best explains it: the nearest detonation for a flash or an
// HE, the latest fire started before the hit for a molotov. Ties go to the
// earlier event, so a duplicated detonation never splits its effect.
func attachUtilityEffects(evs []tacticalplan.Event, hits []utilityHit, tickrate float64) {
	for _, hit := range hits {
		idx := grenadeFor(evs, hit, tickrate)
		if idx < 0 {
			continue
		}
		ev := &evs[idx]
		if ev.Effect == nil {
			ev.Effect = &tacticalplan.UtilityEffect{}
		}
		if hit.kind == tacticalplan.EventFlash {
			ev.Effect.Blinded = append(ev.Effect.Blinded, tacticalplan.BlindHit{Slot: hit.victim, Seconds: hit.seconds, Enemy: hit.enemy})
			continue
		}
		if hit.enemy {
			ev.Effect.EnemyDamage += hit.healthLost
		} else {
			ev.Effect.TeamDamage += hit.healthLost
		}
	}
}

func grenadeFor(evs []tacticalplan.Event, hit utilityHit, tickrate float64) int {
	window := 0.0
	switch hit.kind {
	case tacticalplan.EventFlash:
		window = flashAttachSeconds * tickrate
	case tacticalplan.EventHE:
		window = heAttachSeconds * tickrate
	case tacticalplan.EventMolotov:
		window = molotovAttachSeconds * tickrate
	}
	best, bestDist := -1, math.Inf(1)
	for i, ev := range evs {
		if ev.Kind != hit.kind || ev.ActorSlot == nil || *ev.ActorSlot != hit.attacker {
			continue
		}
		dist := float64(hit.tick - ev.Tick)
		if hit.kind == tacticalplan.EventMolotov {
			if dist < 0 {
				continue
			}
		} else {
			dist = math.Abs(dist)
		}
		if dist <= window && dist < bestDist {
			best, bestDist = i, dist
		}
	}
	return best
}

func remapEffect(effect *tacticalplan.UtilityEffect, remap map[uint8]uint8) *tacticalplan.UtilityEffect {
	if effect == nil {
		return nil
	}
	out := *effect
	out.Blinded = make([]tacticalplan.BlindHit, len(effect.Blinded))
	for i, b := range effect.Blinded {
		b.Slot = remap[b.Slot]
		out.Blinded[i] = b
	}
	if len(out.Blinded) == 0 {
		out.Blinded = nil
	}
	return &out
}
