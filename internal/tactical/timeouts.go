package tactical

import (
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	st "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const technicalTimeoutProp = "m_pGameRules.m_bTechnicalTimeOut"

// timeoutProps are the game rules flags that stay true while a side's timeout runs.
var timeoutProps = []struct {
	name string
	side tacticalplan.Side
}{
	{"m_pGameRules.m_bCTTimeOutActive", tacticalplan.SideCT},
	{"m_pGameRules.m_bTerroristTimeOutActive", tacticalplan.SideT},
}

// timeoutLog follows the timeout that is running. The technical flag flips on
// the same tick as the side's, in either order.
type timeoutLog struct {
	running   *tacticalplan.Timeout
	technical bool
}

func (l *timeoutLog) setTechnical(on bool) {
	l.technical = on
	if on && l.running != nil {
		l.running.Technical = true
	}
}

func (l *timeoutLog) start(side tacticalplan.Side, tick int) {
	l.running = &tacticalplan.Timeout{Side: side, Tick: tick, Technical: l.technical}
}

// stop ends the side's running timeout at tick; false when none was running.
func (l *timeoutLog) stop(side tacticalplan.Side, tick int) (tacticalplan.Timeout, bool) {
	t := l.running
	if t == nil || t.Side != side {
		return tacticalplan.Timeout{}, false
	}
	l.running = nil
	t.EndTick = tick
	return *t, true
}

func (s *scanner) registerTimeouts() {
	s.p.RegisterEventHandler(func(events.DataTablesParsed) {
		class := s.p.ServerClasses().FindByName("CCSGameRulesProxy")
		if class == nil {
			return
		}
		class.OnEntityCreated(func(rules st.Entity) {
			if prop := rules.Property(technicalTimeoutProp); prop != nil {
				prop.OnUpdate(func(v st.PropertyValue) { s.timeouts.setTechnical(v.BoolVal()) })
			}
			for _, flag := range timeoutProps {
				prop := rules.Property(flag.name)
				if prop == nil {
					continue
				}
				prop.OnUpdate(func(v st.PropertyValue) { s.onTimeoutFlag(flag.side, v.BoolVal()) })
			}
		})
	})
}

func (s *scanner) onTimeoutFlag(side tacticalplan.Side, on bool) {
	if on {
		s.timeouts.start(side, s.tick())
		return
	}
	t, ok := s.timeouts.stop(side, s.tick())
	if ok && s.current != nil {
		s.current.addTimeout(t)
	}
}

// addTimeout keeps the part of a timeout inside the round, which is the freeze
// time it ends in.
func (r *roundAcc) addTimeout(t tacticalplan.Timeout) {
	t.Tick = max(t.Tick, r.tickStart)
	if t.EndTick > t.Tick {
		r.timeouts = append(r.timeouts, t)
	}
}
