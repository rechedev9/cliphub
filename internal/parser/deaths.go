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

// DeathSegmentsInput is what SegmentDeaths cuts the death clips from.
type DeathSegmentsInput struct {
	Deaths      []TargetDeath
	LiveStarts  []RoundLiveStart
	RoundStarts []RoundStart
	Tickrate    int
}

// SegmentDeaths returns one clip per live-round death, never before the round goes live.
// Clips carry no kills, which would delay the recorder start and add kill effects.
func SegmentDeaths(in DeathSegmentsInput) []killplan.Segment {
	tickrate := in.Tickrate
	if tickrate <= 0 || len(in.Deaths) == 0 {
		return nil
	}
	ordered := slices.Clone(in.Deaths)
	slices.SortStableFunc(ordered, func(a, b TargetDeath) int { return a.Tick - b.Tick })
	liveByRound := indexRoundLiveStarts(in.LiveStarts)
	startByRound := indexRoundStarts(in.RoundStarts)
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
