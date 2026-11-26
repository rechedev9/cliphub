package workers

import (
	"fmt"
	"slices"
	"sort"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func mergeRecordingResults(prev, next recording.RecordingResult, fullPlan *killplan.Plan) (recording.RecordingResult, error) {
	merged := next
	merged.Plan.Segments = append([]recording.RecordingSegment(nil), next.Plan.Segments...)
	if next.Plan.FullDemo != nil {
		keepRecordedRounds(merged.Plan.Segments, prev.Plan.Segments)
	}
	merged.Artifacts = append([]recording.RecordingArtifact(nil), next.Artifacts...)
	merged.Performance = mergeRecordingPerformance(prev.Performance, next.Performance)
	haveSegment := make(map[string]bool, len(next.Plan.Segments))
	for _, s := range next.Plan.Segments {
		haveSegment[s.ID] = true
	}
	carriedReplay := map[string]bool{}
	for _, s := range prev.Plan.Segments {
		if !haveSegment[s.ID] && !next.Plan.ClashesWith(s) {
			merged.Plan.Segments = append(merged.Plan.Segments, s)
			haveSegment[s.ID] = true
			if s.VictimReplay != nil {
				carriedReplay[recording.VictimReplayID(s.ID)] = true
			}
			if s.SecondCamera != nil {
				carriedReplay[s.SecondCamera.ID] = true
			}
		}
	}
	merged.Plan.EditorialSegmentIDs = make([]string, 0, len(merged.Plan.Segments))
	for _, segment := range fullPlan.Segments {
		if haveSegment[segment.ID] {
			merged.Plan.EditorialSegmentIDs = append(merged.Plan.EditorialSegmentIDs, segment.ID)
		}
	}
	sort.SliceStable(merged.Plan.Segments, func(a, b int) bool {
		if merged.Plan.Segments[a].TickStart != merged.Plan.Segments[b].TickStart {
			return merged.Plan.Segments[a].TickStart < merged.Plan.Segments[b].TickStart
		}
		return merged.Plan.Segments[a].ID < merged.Plan.Segments[b].ID
	})
	haveClip := make(map[string]bool, len(next.Artifacts))
	for _, a := range next.Artifacts {
		if isSegmentClip(a) {
			haveClip[a.SegmentID] = true
		}
	}
	for _, a := range prev.Artifacts {
		if isSegmentClip(a) && !haveClip[a.SegmentID] && (haveSegment[a.SegmentID] || carriedReplay[a.SegmentID]) {
			merged.Artifacts = append(merged.Artifacts, a)
		}
	}
	for i, s := range merged.Plan.Segments {
		merged.Plan.Stream.VictimReplay = merged.Plan.Stream.VictimReplay || s.VictimReplay != nil
		if !s.ReplayOnly {
			continue
		}
		if !slices.ContainsFunc(merged.Artifacts, func(a recording.RecordingArtifact) bool { return isSegmentClip(a) && a.SegmentID == s.ID }) {
			return recording.RecordingResult{}, fmt.Errorf("merge recording plans: replay-only segment %s has no stored main clip", s.ID)
		}
		merged.Plan.Segments[i].ReplayOnly = false
	}
	if next.Plan.FullDemo != nil {
		var err error
		merged, err = mergeFullDemoCaptureRuns(prev, next, merged)
		if err != nil {
			return recording.RecordingResult{}, err
		}
	}
	if err := merged.Plan.Validate(); err != nil {
		return recording.RecordingResult{}, fmt.Errorf("merge recording plans: %w", err)
	}
	fingerprint, err := recording.CaptureInputFingerprint(merged.Plan)
	if err != nil {
		return recording.RecordingResult{}, fmt.Errorf("fingerprint merged recording plan: %w", err)
	}
	merged.CaptureInputFingerprint = fingerprint
	return merged, nil
}

func keepRecordedRounds(segments, recorded []recording.RecordingSegment) {
	for i, s := range segments {
		if !s.ReplayOnly {
			continue
		}
		j := slices.IndexFunc(recorded, func(r recording.RecordingSegment) bool { return r.ID == s.ID })
		if j < 0 {
			continue
		}
		round := recorded[j]
		round.SecondCamera, round.ReplayOnly = s.SecondCamera, true
		segments[i] = round
	}
}
