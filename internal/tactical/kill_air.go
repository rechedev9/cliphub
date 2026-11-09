package tactical

import (
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// registerKillAir reads player_death's attackerinair, which events.Kill does
// not carry. The generic event follows the Kill it belongs to on the same tick.
func (s *scanner) registerKillAir() {
	s.p.RegisterEventHandler(func(e events.GenericGameEvent) {
		if e.Name != "player_death" || s.current == nil {
			return
		}
		if key, ok := e.Data["attackerinair"]; ok && key.GetValBool() {
			markAirKill(s.current.events, s.tick())
		}
	})
}

// markAirKill marks the last kill when it happened at tick.
func markAirKill(events []tacticalplan.Event, tick int) {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Kind != tacticalplan.EventKill {
			continue
		}
		if events[i].Tick == tick {
			events[i].AttackerInAir = true
		}
		return
	}
}
