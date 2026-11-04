package parser

import (
	"fmt"
	"slices"

	"github.com/rechedev9/cliphub/internal/killplan"
)

// DeathLeadSeconds is how much play a death clip shows before the death.
const DeathLeadSeconds = 8

// DeathTailSeconds keeps the hit and the first frames of the death cam.
const DeathTailSeconds = 1

// DeathSegmentID names the n-th death clip; it never collides with "seg-NNN".
func DeathSegmentID(n int) string {
	return fmt.Sprintf("death-%03d", n)
}

// SegmentDeaths returns one clip per live-round death of the target, from
// DeathLeadSeconds before the death (never before the round goes live) to
// DeathTailSeconds after it. Clips carry no kills: kills would make the
// recorder start late and the editor add kill effects.
func SegmentDeaths(deaths []TargetDeath, liveStarts []RoundLiveStart, roundStarts []RoundStart, tickrate int) []killplan.Segment {
	if tickrate <= 0 || len(deaths) == 0 {
		return nil
	}
	ordered := slices.Clone(deaths)
	slices.SortStableFunc(ordered, func(a, b TargetDeath) int { return a.Tick - b.Tick })
	liveByRound := indexRoundLiveStarts(liveStarts)
	startByRound := indexRoundStarts(roundStarts)
	out := make([]killplan.Segment, 0, len(ordered))
	seenRound := map[int]bool{}
	for _, death := range ordered {
		// A player dies once per live round; a second event is a demo artifact.
		if death.Round <= 0 || death.Tick <= 0 || seenRound[death.Round] {
			continue
		}
		seenRound[death.Round] = true
		start := death.Tick - DeathLeadSeconds*tickrate
		if live, ok := liveByRound[death.Round]; ok {
			start = max(start, live)
		} else if roundStart, ok := startByRound[death.Round]; ok {
			start = max(start, roundStart)
		}
		start = max(start, 1)
		end := death.Tick + DeathTailSeconds*tickrate
		for round, next := range startByRound {
			if round > death.Round && next > death.Tick && next <= end {
				end = next - 1
			}
		}
		if end <= start {
			continue
		}
		out = append(out, killplan.Segment{
			ID:          DeathSegmentID(len(out) + 1),
			Round:       death.Round,
			TickStart:   start,
			TickEnd:     end,
			LiveEndTick: death.Tick,
			Death:       &killplan.Kill{Tick: death.Tick, Weapon: death.Weapon, Headshot: death.Headshot, Killer: death.Killer, Victim: death.Victim},
		})
	}
	return out
}
