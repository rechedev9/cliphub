package moments

import (
	"math"
	"slices"

	"github.com/rechedev9/cliphub/internal/killplan"
)

// Reason codes from kill flags, the round result and kill speed; each one is also a filter kind.
const (
	ReasonClutchWon     = "clutch_won"
	ReasonNoScope       = "noscope"
	ReasonThroughSmoke  = "through_smoke"
	ReasonAttackerBlind = "attacker_blind"
	ReasonFastDouble    = "fast_double"
	ReasonFastTriple    = "fast_triple"
	ReasonFastQuad      = "fast_quad"
	ReasonFastAce       = "fast_ace"
)

// Reason codes for the kind of play: the round's opening kill, a flick, a
// post-plant CT kill in a round won by defuse, and a round won on an eco.
const (
	ReasonEntry  = "entry"
	ReasonFlick  = "flick"
	ReasonRetake = "retake"
	ReasonEcoWin = "eco_win"
)

// fastWindows are the named fast multi-kills, biggest first: N kills within the seconds.
var fastWindows = []struct {
	reason  string
	kills   int
	seconds float64
}{
	{ReasonFastAce, 5, 45},
	{ReasonFastQuad, 4, 15},
	{ReasonFastTriple, 3, 10},
	{ReasonFastDouble, 2, 1.5},
}

// fastMultiKill is the biggest named fast multi-kill in the segment, or "".
func fastMultiKill(segment killplan.Segment, tickrate int) string {
	if tickrate <= 0 {
		return ""
	}
	ticks := make([]int, 0, len(segment.Kills))
	for _, kill := range segment.Kills {
		ticks = append(ticks, kill.Tick)
	}
	slices.Sort(ticks)
	for _, w := range fastWindows {
		limit := int(math.Round(w.seconds * float64(tickrate)))
		for i := 0; i+w.kills <= len(ticks); i++ {
			if ticks[i+w.kills-1]-ticks[i] <= limit {
				return w.reason
			}
		}
	}
	return ""
}

// segmentClutchWon is the result of the segment's biggest clutch, or nil when
// it has no clutch or the round winner is unknown.
func segmentClutchWon(segment killplan.Segment) *bool {
	vs, result := 0, ""
	for _, kill := range segment.Kills {
		if kill.ClutchVs > vs && kill.ClutchResult != "" {
			vs, result = kill.ClutchVs, kill.ClutchResult
		}
	}
	if result == "" {
		return nil
	}
	won := result == killplan.ClutchWon
	return &won
}

// segmentWinSwing adds the win-chance swing of every kill in the segment.
func segmentWinSwing(segment killplan.Segment) float64 {
	total := 0.0
	for _, kill := range segment.Kills {
		total += kill.WinProbSwing
	}
	return math.Round(total*1000) / 1000
}

// insertReason adds a reason to a sorted list, keeping it sorted and unique.
func insertReason(reasons []string, reason string) []string {
	i, found := slices.BinarySearch(reasons, reason)
	if found {
		return reasons
	}
	return slices.Insert(reasons, i, reason)
}
