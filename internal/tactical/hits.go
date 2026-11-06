package tactical

import (
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"

	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// hitMergeSeconds sums a spray or a burning fire into one hit per pair: the
// replay draws one line for it instead of a flicker of lines.
const hitMergeSeconds = 0.25

// recordHit keeps damage dealt to an enemy, keyed by provisional slots until
// buildRound remaps them.
func (s *scanner) recordHit(e events.PlayerHurt) {
	r := s.current
	if r == nil || e.Player == nil || e.Attacker == nil || e.Player == e.Attacker || e.HealthDamageTaken <= 0 {
		return
	}
	if e.Attacker.Team == e.Player.Team {
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
	r.hits = mergeHit(r.hits, tacticalplan.Hit{
		Tick:       s.tick(),
		ActorSlot:  attacker,
		TargetSlot: victim,
		Damage:     e.HealthDamageTaken,
		Weapon:     parser.WeaponName(e.Weapon),
		Headshot:   e.HitGroup == events.HitGroupHead,
	}, int(hitMergeSeconds*s.tickrateOrDefault()))
}

// mergeHit appends hit, or adds it to the latest hit of the same pair and
// weapon when that one started at most window ticks earlier.
func mergeHit(hits []tacticalplan.Hit, hit tacticalplan.Hit, window int) []tacticalplan.Hit {
	for i := len(hits) - 1; i >= 0; i-- {
		prev := &hits[i]
		if hit.Tick-prev.Tick > window {
			break
		}
		if prev.ActorSlot != hit.ActorSlot || prev.TargetSlot != hit.TargetSlot || prev.Weapon != hit.Weapon {
			continue
		}
		prev.Damage += hit.Damage
		prev.Headshot = prev.Headshot || hit.Headshot
		return hits
	}
	return append(hits, hit)
}

func remapHits(hits []tacticalplan.Hit, remap map[uint8]uint8) []tacticalplan.Hit {
	if len(hits) == 0 {
		return nil
	}
	out := make([]tacticalplan.Hit, len(hits))
	for i, hit := range hits {
		hit.ActorSlot = remap[hit.ActorSlot]
		hit.TargetSlot = remap[hit.TargetSlot]
		out[i] = hit
	}
	return out
}
