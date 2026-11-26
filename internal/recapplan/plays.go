package recapplan

import (
	"slices"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/moments"
)

type roundPlay struct {
	Kills []killplan.Kill
}

func roundPlays(r Round, rate int) []roundPlay {
	kills := slices.Clone(r.Kills)
	slices.SortStableFunc(kills, func(a, b killplan.Kill) int { return a.Tick - b.Tick })
	gap := HookPlayGapSeconds * rate
	var plays []roundPlay
	var play []killplan.Kill
	for _, kill := range kills {
		if kill.Tick < r.RequestedStartTick || kill.Tick > r.LiveEndTick {
			continue
		}
		if len(play) > 0 && kill.Tick-play[len(play)-1].Tick > gap {
			plays = append(plays, roundPlay{Kills: play})
			play = nil
		}
		play = append(play, kill)
	}
	if len(play) > 0 {
		plays = append(plays, roundPlay{Kills: play})
	}
	return plays
}

func (p roundPlay) first() int { return p.Kills[0].Tick }

func (p roundPlay) last() int { return p.Kills[len(p.Kills)-1].Tick }

func (p roundPlay) segment(id string, r Round, rate int) killplan.Segment {
	lead, tail := secondsTicks(HookLeadSeconds, rate), secondsTicks(HookTailSeconds, rate)
	return killplan.Segment{ID: id, Round: r.Number, TickStart: p.first() - lead, TickEnd: p.last() + tail, Kills: p.Kills}
}

func rankSegments(segments []killplan.Segment, rate int) []moments.Moment {
	return moments.Rank(killplan.Plan{Demo: killplan.Demo{Tickrate: rate}, Segments: segments})
}
