package tactical

import (
	"slices"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"

	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// maxFlightSeconds bounds how long a grenade can fly before it detonates; an
// older throw of the same kind is one whose detonation the demo never showed.
const maxFlightSeconds = 20

// pendingThrow is a grenade in flight and the projectile entity its detonation
// event names.
type pendingThrow struct {
	entityID int
	steamID  uint64
	kind     tacticalplan.EventKind
	throw    tacticalplan.Throw
}

// detonation is a grenade going off, as its event reports it.
type detonation struct {
	entityID int
	steamID  uint64
	kind     tacticalplan.EventKind
	tick     int
}

// throwLog holds the round's grenades in flight until they detonate.
type throwLog struct {
	pending []pendingThrow
	last    detonation
	lastHit tacticalplan.Throw
}

func (l *throwLog) reset() {
	l.pending = l.pending[:0]
	l.last = detonation{}
}

func (l *throwLog) add(t pendingThrow) { l.pending = append(l.pending, t) }

// claim removes and returns the throw of a detonation: the one with its
// projectile entity, or else the oldest of its kind by its thrower thrown at or
// after minTick. A molotov's fire is a new entity, so it always falls back. A
// detonation the demo reports twice gets the same throw both times.
func (l *throwLog) claim(d detonation, minTick int) (tacticalplan.Throw, bool) {
	if d.steamID != 0 && d == l.last {
		return l.lastHit, true
	}
	at := -1
	if d.entityID != 0 {
		at = slices.IndexFunc(l.pending, func(p pendingThrow) bool { return p.entityID == d.entityID && p.kind == d.kind })
	}
	if at < 0 && d.steamID != 0 {
		at = slices.IndexFunc(l.pending, func(p pendingThrow) bool {
			return p.steamID == d.steamID && p.kind == d.kind && p.throw.Tick >= minTick
		})
	}
	if at < 0 {
		return tacticalplan.Throw{}, false
	}
	t := l.pending[at].throw
	l.pending = slices.Delete(l.pending, at, at+1)
	l.last, l.lastHit = d, t
	return t, true
}

// grenadeKind maps a thrown grenade to its event kind; ok is false for anything
// that is not a grenade.
func grenadeKind(eq *common.Equipment) (tacticalplan.EventKind, bool) {
	if eq == nil {
		return "", false
	}
	switch eq.Type {
	case common.EqSmoke:
		return tacticalplan.EventSmoke, true
	case common.EqFlash:
		return tacticalplan.EventFlash, true
	case common.EqHE:
		return tacticalplan.EventHE, true
	case common.EqMolotov, common.EqIncendiary:
		return tacticalplan.EventMolotov, true
	case common.EqDecoy:
		return tacticalplan.EventDecoy, true
	default:
		return "", false
	}
}

func (s *scanner) onThrow(e events.GrenadeProjectileThrow) {
	proj := e.Projectile
	if s.current == nil || proj == nil {
		return
	}
	kind, ok := grenadeKind(proj.WeaponInstance)
	thrower := proj.Thrower
	if thrower == nil {
		thrower = proj.Owner
	}
	if !ok || thrower == nil {
		return
	}
	t := tacticalplan.Throw{Tick: s.tick(), Pos: parser.PlayerPosition(thrower)}
	if yaw, pitch, ok := parser.ViewAngles(thrower); ok {
		t.Yaw, t.Pitch = yaw, pitch
	}
	entityID := 0
	if proj.Entity != nil {
		entityID = proj.Entity.ID()
	}
	s.throws.add(pendingThrow{entityID: entityID, steamID: thrower.SteamID64, kind: kind, throw: t})
}

// claimThrow returns where a detonating grenade was thrown from, or nil when
// the demo never showed its throw.
func (s *scanner) claimThrow(entityID int, thrower *common.Player, kind tacticalplan.EventKind) *tacticalplan.Throw {
	d := detonation{entityID: entityID, kind: kind, tick: s.tick()}
	if thrower != nil {
		d.steamID = thrower.SteamID64
	}
	t, ok := s.throws.claim(d, d.tick-int(maxFlightSeconds*s.tickrateOrDefault()))
	if !ok {
		return nil
	}
	return &t
}
