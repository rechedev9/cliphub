package tactical

import (
	"slices"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// freezeSettle is what settleFreeze did to a round's freeze time.
type freezeSettle int

const (
	freezeKept freezeSettle = iota
	freezeTrimmed
	freezeEstimated
)

// freezeSlackSeconds is how much longer than mp_freezetime a freeze may run
// before the extra is taken as a halftime break or a pre-live wait.
const freezeSlackSeconds = 3

// timeoutKeepSeconds is how much of a timeout the round keeps: a tactical one
// whole, the last minute of a technical pause that can run for many.
const timeoutKeepSeconds = 60

// freezeLimits are the ticks settleFreeze measures a freeze against.
type freezeLimits struct {
	freeze, slack, timeoutKeep int
}

// settleFreeze fixes a round's freeze band against the server's freeze time.
// A missing freeze end is estimated from it; a freeze longer than it by more
// than slack (the halftime break, FACEIT's wait before going live) loses its
// head, with the frames and events in it. A timeout keeps up to timeoutKeep of
// it; the timeout itself keeps its real start, so its band says how long it ran.
func (r *roundAcc) settleFreeze(limits freezeLimits) freezeSettle {
	freezeTicks := limits.freeze
	if freezeTicks <= 0 {
		return freezeKept
	}
	if r.tickFreezeEnd <= r.tickStart {
		r.tickFreezeEnd = min(r.tickStart+freezeTicks, r.tickEnd)
		return freezeEstimated
	}
	if r.tickFreezeEnd-r.tickStart <= freezeTicks+limits.slack {
		return freezeKept
	}
	start := r.tickFreezeEnd - freezeTicks
	for _, t := range r.timeouts {
		start = min(start, max(t.Tick, t.EndTick-limits.timeoutKeep))
	}
	if start <= r.tickStart {
		return freezeKept
	}
	r.tickStart = start
	r.frames = slices.DeleteFunc(r.frames, func(f tacticalplan.Frame) bool { return f.Tick < r.tickStart })
	r.events = slices.DeleteFunc(r.events, func(e tacticalplan.Event) bool { return e.Tick < r.tickStart })
	return freezeTrimmed
}
