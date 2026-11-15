package parser

import (
	"math"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
	st "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// postPlantWatch records the post-plant win chance frame by frame. The bomb
// clock is read from the planted bomb's own deadline, never an assumed timer.
type postPlantWatch struct {
	bomb       st.Entity
	serverTick uint32
	planted    bool
	plantTick  int
	open       bool
	spans      []killplan.WinProbSpan
}

func (w *postPlantWatch) bind(p demoinfocs.Parser, mapName func() string) {
	p.RegisterNetMessageHandler(func(tick *msg.CNETMsg_Tick) { w.serverTick = tick.GetTick() })
	p.RegisterEventHandler(func(events.DataTablesParsed) {
		class := p.ServerClasses().FindByName("CPlantedC4")
		if class == nil {
			return
		}
		class.OnEntityCreated(func(entity st.Entity) {
			w.bomb = entity
			entity.OnDestroy(func() {
				if w.bomb == entity {
					w.bomb = nil
				}
			})
		})
	})
	p.RegisterEventHandler(func(events.BombDefused) { w.stop(p.GameState().IngameTick()) })
	p.RegisterEventHandler(func(events.BombExplode) { w.stop(p.GameState().IngameTick()) })
	p.RegisterEventHandler(func(events.BombPlanted) {
		w.planted, w.plantTick = true, p.GameState().IngameTick()
	})
	p.RegisterEventHandler(func(events.FrameDone) {
		if !w.planted || p.GameState().Participants() == nil {
			return
		}
		tick, rate := p.GameState().IngameTick(), p.TickRate()
		pct, ok := 0, false
		if left, known := w.bombLeft(rate); known && tick >= w.plantTick {
			state := tacticalplan.WinState{Map: mapName(), Planted: true, BombSeconds: float64(tick-w.plantTick) / rate}
			if addAlive(&state, p.GameState().Participants().Playing()) {
				pct, ok = tacticalplan.PostPlantWinPctCT(state, left)
			}
		}
		w.sample(tick, pct, ok)
	})
}

// bombLeft is the seconds until the planted bomb blows, from m_flC4Blow.
func (w *postPlantWatch) bombLeft(rate float64) (float64, bool) {
	if w.bomb == nil || w.serverTick == 0 || rate <= 0 {
		return 0, false
	}
	blow, ok := w.bomb.PropertyValue("m_flC4Blow")
	if !ok || blow.Any == nil {
		return 0, false
	}
	left := float64(blow.Float()) - float64(w.serverTick)/rate
	if math.IsNaN(left) || math.IsInf(left, 0) || left > 120 {
		return 0, false
	}
	return left, true
}

// addAlive counts the living players by side with their freeze-end equipment;
// a living player without that value makes the state unknown.
func addAlive(s *tacticalplan.WinState, players []*common.Player) bool {
	for _, p := range players {
		if p == nil || !p.IsAlive() || sideLabel(p.Team) == "" {
			continue
		}
		equip := p.EquipmentValueFreezeTimeEnd()
		if equip <= 0 {
			return false
		}
		addWinPlayer(s, alivePlayer{SteamID64: p.SteamID64, Team: p.Team, Equip: equip})
	}
	return true
}

// sample extends the open span or starts a new one; spans of one plant abut.
func (w *postPlantWatch) sample(tick, pct int, ok bool) {
	last := len(w.spans) - 1
	switch {
	case !ok:
		if w.open {
			w.spans[last].TickEnd = tick
		}
		w.open = false
	case w.open && w.spans[last].CTPct == pct:
		w.spans[last].TickEnd = tick
	default:
		if w.open {
			w.spans[last].TickEnd = tick
		}
		w.spans = append(w.spans, killplan.WinProbSpan{TickStart: tick, TickEnd: tick, CTPct: pct})
		w.open = true
	}
}

// stop ends the plant; collectKills calls it from its match and round handlers.
func (w *postPlantWatch) stop(tick int) {
	if w == nil {
		return
	}
	if w.planted {
		w.sample(tick, 0, false)
	}
	w.planted = false
}

// withWinProb gives each segment the spans inside its window.
func withWinProb(segs []killplan.Segment, spans []killplan.WinProbSpan) {
	for i := range segs {
		s := &segs[i]
		s.WinProb = nil
		for _, span := range spans {
			span.TickStart, span.TickEnd = max(span.TickStart, s.TickStart), min(span.TickEnd, s.TickEnd)
			if span.TickEnd > span.TickStart {
				s.WinProb = append(s.WinProb, span)
			}
		}
	}
}
