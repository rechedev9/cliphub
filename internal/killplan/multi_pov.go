package killplan

import (
	"fmt"
	"slices"
)

// SegmentPOV is the player a segment is filmed from: its own POV in a
// multi-POV plan, else the plan target.
func (p Plan) SegmentPOV(s Segment) Target {
	if s.POV != nil {
		return *s.POV
	}
	return p.Target
}

// IsMultiPOV reports whether any segment is filmed from a player other than
// the plan target.
func (p Plan) IsMultiPOV() bool {
	return slices.ContainsFunc(p.Segments, func(s Segment) bool {
		return s.POV != nil && s.POV.SteamID64 != p.Target.SteamID64
	})
}

// POVClash reports whether two segments film different players within half a
// second of each other: one demo pass cannot film both or switch in time.
func (p Plan) POVClash(a, b Segment) bool {
	if p.SegmentPOV(a).SteamID64 == p.SegmentPOV(b).SteamID64 {
		return false
	}
	gap := max(1, p.Demo.Tickrate/2)
	return a.TickStart < b.TickEnd+gap && b.TickStart < a.TickEnd+gap
}

// MergePOVs joins the kill plans of several players of one demo. The first
// plan is the anchor: its demo, target and rules head the result. Segments
// keep their player in POV, run in tick order and are renumbered seg-NNN.
func MergePOVs(plans []Plan) (Plan, error) {
	if len(plans) < 2 {
		return Plan{}, fmt.Errorf("a multi-POV plan needs at least two players")
	}
	out := plans[0]
	out.Segments = []Segment{}
	out.Stats = Stats{}
	for i, plan := range plans {
		if plan.Demo.SHA256 != out.Demo.SHA256 {
			return Plan{}, fmt.Errorf("plan %d comes from another demo", i)
		}
		if slices.ContainsFunc(plans[:i], func(prev Plan) bool { return prev.Target.SteamID64 == plan.Target.SteamID64 }) {
			return Plan{}, fmt.Errorf("player %s appears twice", plan.Target.SteamID64)
		}
		for _, s := range plan.Segments {
			pov := plan.Target
			s.POV = &pov
			out.Segments = append(out.Segments, s)
		}
		out.Stats = addStats(out.Stats, plan.Stats)
	}
	// Stable on equal ticks keeps the players' pick order.
	slices.SortStableFunc(out.Segments, func(a, b Segment) int {
		if a.TickStart != b.TickStart {
			return a.TickStart - b.TickStart
		}
		return a.TickEnd - b.TickEnd
	})
	for i := range out.Segments {
		out.Segments[i].ID = FormatSegmentID(i + 1)
	}
	out.Stats.SegmentsCreated = len(out.Segments)
	return out, nil
}

func addStats(a, b Stats) Stats {
	return Stats{
		TotalKillsTarget:     a.TotalKillsTarget + b.TotalKillsTarget,
		KillsAfterFilters:    a.KillsAfterFilters + b.KillsAfterFilters,
		TotalUtilityTarget:   a.TotalUtilityTarget + b.TotalUtilityTarget,
		UtilityAfterFilters:  a.UtilityAfterFilters + b.UtilityAfterFilters,
		TotalSmokesTarget:    a.TotalSmokesTarget + b.TotalSmokesTarget,
		SmokesAfterFilters:   a.SmokesAfterFilters + b.SmokesAfterFilters,
		SegmentsCreated:      a.SegmentsCreated + b.SegmentsCreated,
		DurationSecondsTotal: a.DurationSecondsTotal + b.DurationSecondsTotal,
	}
}
