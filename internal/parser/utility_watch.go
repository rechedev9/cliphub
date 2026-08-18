package parser

import (
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

// utilityWatch is the shared grenade observer used by utility-mode parsing
// and by recap, so throw origin, click, action, and pop/landing stay one
// vocabulary instead of drifting per segment mode.
type utilityWatch struct {
	targetID   uint64
	target     string
	pending    []*RawUtilityThrow
	byEntityID map[int]*RawUtilityThrow
	byUniqueID map[int64]*RawUtilityThrow
	maxTick    *int
	onIdentity func(name, team string)
}

func newUtilityWatch(targetID uint64, target string, maxTick *int, onIdentity func(string, string)) *utilityWatch {
	return &utilityWatch{
		targetID:   targetID,
		target:     target,
		byEntityID: map[int]*RawUtilityThrow{},
		byUniqueID: map[int64]*RawUtilityThrow{},
		maxTick:    maxTick,
		onIdentity: onIdentity,
	}
}

func (w *utilityWatch) noteTick(tick int) {
	if w.maxTick != nil && tick > *w.maxTick {
		*w.maxTick = tick
	}
}

func (w *utilityWatch) identity(p *common.Player) {
	if w.onIdentity == nil || p == nil {
		return
	}
	w.onIdentity(p.Name, teamLabel(p.Team))
}

func (w *utilityWatch) reset() {
	w.pending = nil
	w.byEntityID = map[int]*RawUtilityThrow{}
	w.byUniqueID = map[int64]*RawUtilityThrow{}
}

func (w *utilityWatch) startThrow(p *common.Player, typ string, round, tick int, source string) *RawUtilityThrow {
	u := &RawUtilityThrow{
		Type:       typ,
		Round:      round,
		ThrowTick:  tick,
		Thrower:    playerIdentity(p),
		ThrowPos:   playerPosition(p),
		ThrowPlace: safeLastPlaceName(p),
	}
	applyThrowState(u, p, tick, source)
	w.pending = append(w.pending, u)
	return u
}

func (w *utilityWatch) bind(p demoinfocs.Parser) {
	p.RegisterEventHandler(func(events.MatchStart) {
		w.reset()
	})

	p.RegisterEventHandler(func(e events.WeaponFire) {
		gs := p.GameState()
		tick := gs.IngameTick()
		w.noteTick(tick)
		if e.Shooter == nil || e.Shooter.SteamID64 != w.targetID || !isTrackedUtilityEquipment(e.Weapon) {
			return
		}
		w.identity(e.Shooter)
		w.startThrow(e.Shooter, utilityTypeFromEquipment(e.Weapon), gs.TotalRoundsPlayed()+1, tick, "weapon_fire")
	})

	p.RegisterEventHandler(func(e events.GrenadeProjectileThrow) {
		gs := p.GameState()
		tick := gs.IngameTick()
		w.noteTick(tick)
		projectile := e.Projectile
		if !isTrackedUtilityProjectile(projectile) || projectile.Thrower == nil || projectile.Thrower.SteamID64 != w.targetID {
			return
		}
		thrower := projectile.Thrower
		w.identity(thrower)
		typ := utilityTypeFromEquipment(projectile.WeaponInstance)
		u := findRecentPendingUtility(w.pending, thrower, w.target, typ, tick, int(p.TickRate()))
		if u == nil {
			u = w.startThrow(thrower, typ, gs.TotalRoundsPlayed()+1, tick, "projectile_throw")
		}
		applyThrowState(u, thrower, tick, "projectile_throw")
		u.LandingPos = projectilePosition(projectile)
		u.LandingSource = "projectile_spawn"
		w.byUniqueID[projectile.UniqueID()] = u
		if entityID := projectileEntityID(projectile); entityID != 0 {
			w.byEntityID[entityID] = u
		}
	})

	p.RegisterEventHandler(func(e events.FlashExplode) {
		w.handlePop(p, e.GrenadeEvent, FlashbangType, "flash_explode", 4)
	})
	p.RegisterEventHandler(func(e events.HeExplode) {
		w.handlePop(p, e.GrenadeEvent, HeGrenadeType, "he_explode", 4)
	})
	p.RegisterEventHandler(func(e events.DecoyStart) {
		w.handlePop(p, e.GrenadeEvent, DecoyType, "decoy_start", 8)
	})
	p.RegisterEventHandler(func(e events.DecoyExpired) {
		gs := p.GameState()
		tick := gs.IngameTick()
		w.noteTick(tick)
		if u := findPendingUtility(w.byEntityID, w.pending, e.GrenadeEntityID, e.Thrower, w.targetID, w.target, DecoyType, tick, 20*int(p.TickRate())); u != nil {
			u.ExpireTick = tick
		}
	})
	p.RegisterEventHandler(func(e events.SmokeStart) {
		w.handlePop(p, e.GrenadeEvent, SmokeGrenadeType, "smoke_start", 12)
	})
	p.RegisterEventHandler(func(e events.SmokeExpired) {
		gs := p.GameState()
		tick := gs.IngameTick()
		w.noteTick(tick)
		if e.GrenadeType != common.EqSmoke {
			return
		}
		if u := findPendingUtility(w.byEntityID, w.pending, e.GrenadeEntityID, e.Thrower, w.targetID, w.target, SmokeGrenadeType, tick, 35*int(p.TickRate())); u != nil {
			u.ExpireTick = tick
		}
	})

	p.RegisterEventHandler(func(e events.InfernoStart) {
		gs := p.GameState()
		tick := gs.IngameTick()
		w.noteTick(tick)
		if e.Inferno == nil || e.Inferno.Thrower() == nil || e.Inferno.Thrower().SteamID64 != w.targetID {
			return
		}
		thrower := e.Inferno.Thrower()
		w.identity(thrower)
		u := findRecentPendingFireUtility(w.pending, thrower, w.target, tick, int(p.TickRate()))
		if u == nil {
			u = w.startThrow(thrower, MolotovType, gs.TotalRoundsPlayed()+1, tick, "inferno_start")
		}
		applyThrowState(u, thrower, tick, "inferno_start")
		u.PopTick = tick
		if pos, ok := infernoCenter(e.Inferno); ok {
			u.LandingPos = pos
			u.LandingSource = "inferno_center"
		}
	})
	p.RegisterEventHandler(func(e events.InfernoExpired) {
		gs := p.GameState()
		tick := gs.IngameTick()
		w.noteTick(tick)
		if e.Inferno == nil || e.Inferno.Thrower() == nil || e.Inferno.Thrower().SteamID64 != w.targetID {
			return
		}
		if u := findRecentPendingFireUtility(w.pending, e.Inferno.Thrower(), w.target, tick, 12*int(p.TickRate())); u != nil {
			u.ExpireTick = tick
		}
	})

	p.RegisterEventHandler(func(e events.GrenadeProjectileDestroy) {
		gs := p.GameState()
		tick := gs.IngameTick()
		w.noteTick(tick)
		projectile := e.Projectile
		if !isTrackedUtilityProjectile(projectile) {
			return
		}
		u := w.byUniqueID[projectile.UniqueID()]
		if u == nil {
			u = w.byEntityID[projectileEntityID(projectile)]
		}
		if u == nil || u.Thrower.SteamID64 != w.target {
			return
		}
		if u.LandingSource != "" && u.LandingSource != "projectile_spawn" {
			return
		}
		if last, ok := lastTrajectoryPosition(projectile); ok {
			u.LandingPos = last
			u.LandingSource = "projectile_destroy_trajectory"
		} else {
			u.LandingPos = projectilePosition(projectile)
			u.LandingSource = "projectile_destroy_position"
		}
		if u.PopTick == 0 {
			u.PopTick = tick
		}
	})
}

func (w *utilityWatch) handlePop(p demoinfocs.Parser, e events.GrenadeEvent, typ, source string, gapSeconds int) {
	gs := p.GameState()
	tick := gs.IngameTick()
	w.noteTick(tick)
	if !isTrackedUtilityType(typ) {
		return
	}
	u := findPendingUtility(w.byEntityID, w.pending, e.GrenadeEntityID, e.Thrower, w.targetID, w.target, typ, tick, gapSeconds*int(p.TickRate()))
	if u == nil {
		if e.Thrower == nil || e.Thrower.SteamID64 != w.targetID {
			return
		}
		w.identity(e.Thrower)
		u = w.startThrow(e.Thrower, typ, gs.TotalRoundsPlayed()+1, tick, source)
		if e.GrenadeEntityID != 0 {
			w.byEntityID[e.GrenadeEntityID] = u
		}
	}
	applyThrowState(u, e.Thrower, tick, source)
	u.PopTick = tick
	u.LandingPos = vectorPosition(e.Position)
	u.LandingSource = source
}

func (w *utilityWatch) flush() []*RawUtilityThrow {
	return w.pending
}
