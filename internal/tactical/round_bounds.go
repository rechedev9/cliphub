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

// settleFreeze fixes a round's freeze band against the server's freeze time.
// A missing freeze end is estimated from it; a freeze longer than it by more
// than slack (the halftime break, FACEIT's wait before going live) loses its
// head, with the frames and events in it.
func (r *roundAcc) settleFreeze(freezeTicks, slackTicks int) freezeSettle {
	if freezeTicks <= 0 {
		return freezeKept
	}
	if r.tickFreezeEnd <= r.tickStart {
		r.tickFreezeEnd = min(r.tickStart+freezeTicks, r.tickEnd)
		return freezeEstimated
	}
	if r.tickFreezeEnd-r.tickStart <= freezeTicks+slackTicks {
		return freezeKept
	}
	r.tickStart = r.tickFreezeEnd - freezeTicks
	r.frames = slices.DeleteFunc(r.frames, func(f tacticalplan.Frame) bool { return f.Tick < r.tickStart })
	r.events = slices.DeleteFunc(r.events, func(e tacticalplan.Event) bool { return e.Tick < r.tickStart })
	return freezeTrimmed
}
