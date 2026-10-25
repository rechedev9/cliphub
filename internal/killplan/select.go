package killplan

import (
	"fmt"
	"strings"
)

// Select returns plan narrowed to the segments ids names, in that order, with
// the filtered stats recomputed. GeneratedAt is left as is; callers stamp it.
func Select(plan Plan, ids []string) (Plan, error) {
	byID := make(map[string]Segment, len(plan.Segments))
	for _, segment := range plan.Segments {
		if strings.TrimSpace(segment.ID) == "" {
			return Plan{}, fmt.Errorf("kill plan contains a segment without an id")
		}
		if _, exists := byID[segment.ID]; exists {
			return Plan{}, fmt.Errorf("kill plan contains duplicate segment id %q", segment.ID)
		}
		byID[segment.ID] = segment
	}
	selected := plan
	selected.Segments = make([]Segment, 0, len(ids))
	var missing []string
	for _, id := range ids {
		segment, ok := byID[id]
		if !ok {
			missing = append(missing, id)
			continue
		}
		selected.Segments = append(selected.Segments, segment)
	}
	if len(missing) > 0 {
		return Plan{}, fmt.Errorf("unknown segment ids: %s", strings.Join(missing, ", "))
	}
	selected.Stats.KillsAfterFilters = 0
	selected.Stats.UtilityAfterFilters = 0
	selected.Stats.SmokesAfterFilters = 0
	selected.Stats.SegmentsCreated = len(selected.Segments)
	selected.Stats.DurationSecondsTotal = 0
	for _, segment := range selected.Segments {
		selected.Stats.KillsAfterFilters += len(segment.Kills)
		selected.Stats.UtilityAfterFilters += len(segment.Utility)
		for _, utility := range segment.Utility {
			if strings.EqualFold(utility.Type, "smoke") || strings.EqualFold(utility.Type, "smokegrenade") {
				selected.Stats.SmokesAfterFilters++
			}
		}
		if segment.TickEnd > segment.TickStart {
			selected.Stats.DurationSecondsTotal += float64(segment.TickEnd-segment.TickStart) / float64(selected.Demo.Tickrate)
		}
	}
	return selected, nil
}
