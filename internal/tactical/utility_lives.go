package tactical

import (
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"

	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// utilityLives finds a round's smokes, decoys, fires and dropped bomb again
// when they end, so each event carries the tick it really ended on.
type utilityLives struct {
	byEntity  map[int]int
	byInferno map[int64]int
	// drop is the index plus one of the bomb on the ground, 0 when carried.
	drop int
}

func (l *utilityLives) startEntity(entityID, index int) {
	if index < 0 || entityID == 0 {
		return
	}
	if l.byEntity == nil {
		l.byEntity = map[int]int{}
	}
	l.byEntity[entityID] = index
}

func (l *utilityLives) startInferno(id int64, index int) {
	if index < 0 {
		return
	}
	if l.byInferno == nil {
		l.byInferno = map[int64]int{}
	}
	l.byInferno[id] = index
}

func (s *scanner) registerLifetimes() {
	p := s.p
	p.RegisterEventHandler(func(e events.SmokeExpired) { s.endEntity(e.GrenadeEntityID) })
	p.RegisterEventHandler(func(e events.DecoyExpired) { s.endEntity(e.GrenadeEntityID) })
	p.RegisterEventHandler(func(e events.InfernoExpired) {
		if s.current == nil || e.Inferno == nil {
			return
		}
		if at, ok := s.current.lives.byInferno[e.Inferno.UniqueID()]; ok {
			s.endEvent(at)
		}
	})
	p.RegisterEventHandler(func(e events.GrenadeProjectileDestroy) {
		if e.Projectile != nil && e.Projectile.Entity != nil {
			s.throws.land(e.Projectile.Entity.ID(), s.tick())
		}
	})
	p.RegisterEventHandler(func(e events.BombDropped) { s.onBombDropped(e) })
	p.RegisterEventHandler(func(events.BombPickup) { s.endDrop() })
}

func (s *scanner) endEntity(entityID int) {
	if s.current == nil {
		return
	}
	if at, ok := s.current.lives.byEntity[entityID]; ok {
		s.endEvent(at)
	}
}

func (s *scanner) endEvent(at int) {
	ev := &s.current.events[at]
	if ev.EndTick == 0 {
		ev.EndTick = s.tick()
	}
}

func (s *scanner) onBombDropped(e events.BombDropped) {
	if s.current == nil {
		return
	}
	s.endDrop()
	pos := parser.PlayerPosition(e.Player)
	if bomb := s.p.GameState().Bomb(); bomb != nil {
		if at := bomb.LastOnGroundPosition; at.X != 0 || at.Y != 0 || at.Z != 0 {
			pos = [3]float64{at.X, at.Y, at.Z}
		}
	}
	at := s.addEvent(tacticalplan.Event{
		Kind:  tacticalplan.EventDropBomb,
		Tick:  s.tick(),
		Side:  tacticalplan.SideT,
		Pos:   pos,
		Place: placeOf(e.Player),
	}, e.Player)
	s.current.lives.drop = at + 1
}

// endDrop closes the bomb on the ground: picked up or planted. A pass between
// teammates in freeze time is no event at all.
func (s *scanner) endDrop() {
	r := s.current
	if r == nil || r.lives.drop == 0 {
		return
	}
	at := r.lives.drop - 1
	r.lives.drop = 0
	if s.p.GameState().IsFreezetimePeriod() && at == len(r.events)-1 {
		r.events = r.events[:at]
		return
	}
	s.endEvent(at)
}

// followDrop moves the dropped bomb's event to where the bomb came to rest:
// a thrown bomb flies on after the drop.
func (s *scanner) followDrop() {
	r := s.current
	if r == nil || r.lives.drop == 0 {
		return
	}
	bomb := s.p.GameState().Bomb()
	if bomb == nil || bomb.Carrier != nil {
		return
	}
	if at := bomb.LastOnGroundPosition; at.X != 0 || at.Y != 0 || at.Z != 0 {
		r.events[r.lives.drop-1].Pos = [3]float64{at.X, at.Y, at.Z}
	}
}

// watchFires ends a fire when its last flame stops burning. The inferno
// entity itself lingers for about 20 s, so its expiry says nothing of a
// molotov a smoke put out.
func (s *scanner) watchFires() {
	r := s.current
	if r == nil || len(r.lives.byInferno) == 0 {
		return
	}
	for _, inf := range s.p.GameState().Infernos() {
		at, ok := r.lives.byInferno[inf.UniqueID()]
		if !ok || r.events[at].EndTick != 0 {
			continue
		}
		fires := inf.Fires()
		if len(fires.List()) > 0 && len(fires.Active().List()) == 0 {
			s.endEvent(at)
		}
	}
}

// freezeTicks reads mp_freezetime in ticks; 0 when the rules do not say.
func (s *scanner) freezeTicks() int {
	rules := s.p.GameState().Rules()
	if rules == nil {
		return 0
	}
	freeze, err := rules.FreezeTime()
	if err != nil || freeze <= 0 {
		return 0
	}
	return int(freeze.Seconds() * s.tickrateOrDefault())
}
