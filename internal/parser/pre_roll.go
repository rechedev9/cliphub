package parser

import (
	"slices"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// WithPreRoll returns plan with each kill play starting seconds before its first kill, never before the
// start of its round (roundStarts may be nil, then never before tick 0); the end of a play does not move.
// Plays of one round and one POV whose windows then overlap merge into the earlier one and keep its id.
// Zero seconds returns plan unchanged.
func WithPreRoll(plan killplan.Plan, seconds int, roundStarts []RoundStart) killplan.Plan {
	groups := preRollGroups(plan, seconds)
	if groups == nil {
		return plan
	}
	starts := indexRoundStarts(roundStarts)
	out := plan
	out.Segments = make([]killplan.Segment, 0, len(groups))
	for _, g := range groups {
		s := g.segment
		if start, ok := starts[s.Round]; ok && s.TickStart < start && len(s.Kills) > 0 && start <= s.Kills[0].Tick {
			s.TickStart = start
		}
		s.TickStart = max(s.TickStart, 0)
		out.Segments = append(out.Segments, s)
	}
	return out
}

// PreRollIDs maps segment ids of plan onto the plays of WithPreRoll(plan, seconds, ...): an id merged into an
// earlier play becomes that play's id. Each play is named once, where the selection first names it.
func PreRollIDs(plan killplan.Plan, seconds int, ids []string) []string {
	groups := preRollGroups(plan, seconds)
	if groups == nil {
		return ids
	}
	owner := make(map[string]string, len(plan.Segments))
	for _, g := range groups {
		for _, member := range g.members {
			owner[member] = g.segment.ID
		}
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if o, ok := owner[id]; ok {
			id = o
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// FactsRoundStarts lists the round starts of a parse's round facts, for WithPreRoll.
func FactsRoundStarts(facts recapplan.Facts) []RoundStart {
	starts := make([]RoundStart, 0, len(facts.Rounds))
	for _, round := range facts.Rounds {
		starts = append(starts, RoundStart{Round: round.Number, Tick: round.StartTick})
	}
	return starts
}

type preRollGroup struct {
	segment killplan.Segment
	members []string
}

// preRollGroups merges on the start before any round clamp, so the grouping (and PreRollIDs) needs no round starts:
// a clamp only moves a start to its round's beginning, which never ends an overlap with an earlier play of that round.
func preRollGroups(plan killplan.Plan, seconds int) []preRollGroup {
	if seconds <= 0 || plan.Demo.Tickrate <= 0 {
		return nil
	}
	lead := seconds * plan.Demo.Tickrate
	groups := make([]preRollGroup, 0, len(plan.Segments))
	for _, s := range plan.Segments {
		if len(s.Kills) > 0 {
			s.TickStart = s.Kills[0].Tick - lead
		}
		if n := len(groups); n > 0 && mergesInto(groups[n-1].segment, s) {
			last := &groups[n-1]
			last.segment.TickEnd = max(last.segment.TickEnd, s.TickEnd)
			last.segment.LiveEndTick = max(last.segment.LiveEndTick, s.LiveEndTick)
			last.segment.Kills = slices.Concat(last.segment.Kills, s.Kills)
			last.segment.Utility = slices.Concat(last.segment.Utility, s.Utility)
			last.segment.WinProb = slices.Concat(last.segment.WinProb, s.WinProb)
			last.members = append(last.members, s.ID)
			continue
		}
		groups = append(groups, preRollGroup{segment: s, members: []string{s.ID}})
	}
	return groups
}

func mergesInto(prev, next killplan.Segment) bool {
	return len(prev.Kills) > 0 && len(next.Kills) > 0 && prev.Round == next.Round && samePOV(prev.POV, next.POV) && next.TickStart <= prev.TickEnd
}

func samePOV(a, b *killplan.Target) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.SteamID64 == b.SteamID64
}
