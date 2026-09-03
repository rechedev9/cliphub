package recording

import (
	"errors"
	"fmt"
	"strings"
)

// ErrSegmentNotRecorded reports a segment selection that names a segment the
// recording result never captured. FilterResultSegments wraps it so callers
// can tell a stale capture apart from a malformed selection.
var ErrSegmentNotRecorded = errors.New("segment not recorded")

// SegmentIDs returns the unique non-empty segment IDs from the recording plan
// in result order.
func SegmentIDs(result RecordingResult) []string {
	return uniqueSegmentIDs(result.Plan.Segments)
}

// EditorialSegmentIDs returns the unique non-empty segment IDs in the order the
// editor compiles them: Plan.EditorialSegmentIDs when the plan carries one,
// otherwise capture order (see RecordingPlan.SegmentsInEditorialOrder).
func EditorialSegmentIDs(result RecordingResult) []string {
	return uniqueSegmentIDs(result.Plan.SegmentsInEditorialOrder())
}

func uniqueSegmentIDs(segments []RecordingSegment) []string {
	seen := map[string]bool{}
	var ids []string
	for _, segment := range segments {
		if segment.ID == "" || seen[segment.ID] {
			continue
		}
		seen[segment.ID] = true
		ids = append(ids, segment.ID)
	}
	return ids
}

// FilterResultSegments returns a copy of result that names exactly the
// segments in ids, so a render for a partial reel selection never sees the
// other segments the job has recorded. Plan.Segments keeps capture (tick)
// order, which RecordingPlan.Validate requires; ids becomes
// Plan.EditorialSegmentIDs, so the editor compiles the selection in the order
// the caller asked for. Artifacts keeps the selected segment clips plus every
// artifact that is not tied to a segment.
//
// Empty ids returns result unchanged: no selection means every recorded
// segment. A duplicate id or an id that result never recorded is an error that
// names the offending ids; the caller must not fall back to a wider selection.
func FilterResultSegments(result RecordingResult, ids []string) (RecordingResult, error) {
	if len(ids) == 0 {
		return result, nil
	}
	recorded := make(map[string]bool, len(result.Plan.Segments))
	for _, segment := range result.Plan.Segments {
		recorded[segment.ID] = true
	}
	selected := make(map[string]bool, len(ids))
	var missing, duplicate []string
	for _, id := range ids {
		switch {
		case selected[id]:
			duplicate = append(duplicate, id)
		case !recorded[id]:
			missing = append(missing, id)
		}
		selected[id] = true
	}
	if len(duplicate) > 0 {
		return RecordingResult{}, fmt.Errorf("segment selection repeats segment(s) %s", strings.Join(duplicate, ", "))
	}
	if len(missing) > 0 {
		return RecordingResult{}, fmt.Errorf("recording result has no segment(s) %s: %w", strings.Join(missing, ", "), ErrSegmentNotRecorded)
	}

	out := result
	out.Plan.Segments = make([]RecordingSegment, 0, len(ids))
	for _, segment := range result.Plan.Segments {
		if selected[segment.ID] {
			out.Plan.Segments = append(out.Plan.Segments, segment)
		}
	}
	out.Plan.EditorialSegmentIDs = append([]string(nil), ids...)
	out.Artifacts = make([]RecordingArtifact, 0, len(result.Artifacts))
	for _, artifact := range result.Artifacts {
		if artifact.SegmentID == "" || selected[artifact.SegmentID] {
			out.Artifacts = append(out.Artifacts, artifact)
		}
	}
	return out, nil
}
