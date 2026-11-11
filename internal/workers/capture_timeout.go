package workers

import (
	"time"

	"github.com/rechedev9/cliphub/internal/killplan"
)

// A slow PC needs about 1.4 min per Full Demo round, so twice the captured demo
// time plus CS2's load and seeks bounds a hung capture without cutting a long one.
const (
	captureTimeoutFactor = 2
	captureTimeoutSlack  = 10 * time.Minute
)

// captureTimeout is the recorder's deadline for a plan: the configured one, raised
// when the plan captures more demo time than it fits, such as a Full Demo of 29 rounds.
func captureTimeout(configured string, plan *killplan.Plan) string {
	base, err := time.ParseDuration(configured)
	if err != nil || plan == nil || plan.Demo.Tickrate <= 0 {
		return configured
	}
	ticks := 0
	for _, s := range plan.Segments {
		if s.TickEnd > s.TickStart {
			ticks += s.TickEnd - s.TickStart
		}
	}
	captured := time.Duration(ticks) * time.Second / time.Duration(plan.Demo.Tickrate)
	need := captureTimeoutFactor*captured + captureTimeoutSlack
	if need <= base {
		return configured
	}
	return need.Round(time.Second).String()
}
