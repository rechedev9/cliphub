package recording

import (
	"fmt"
	"math"
	"strings"
)

// captureDurationToleranceSeconds is the slack allowed between a clip's probed
// duration and the window the plan scheduled. Frame rounding at 60fps is two
// orders of magnitude smaller, so anything beyond this is a real mismatch.
const captureDurationToleranceSeconds = 0.25

// expectedSegmentDurationSeconds is the wall-clock length of the window the
// recorder actually schedules for a segment. Both ends must use the effective
// ticks: record-start applies the camera-settle offset and record-end is pulled
// back inside the demo's EOF safety margin (see EffectiveRecordEndTick), so
// comparing against the raw plan ticks reports a phantom deficit on exactly the
// last segment of a run. Returns 0 when the tickrate is unusable.
func expectedSegmentDurationSeconds(segment RecordingSegment, plan RecordingPlan) float64 {
	if plan.Tickrate <= 0 {
		return 0
	}
	recordStart := EffectiveRecordStartTick(segment, plan.Tickrate)
	recordEnd := EffectiveRecordEndTick(segment, plan)
	return float64(recordEnd-recordStart) / float64(plan.Tickrate)
}

// ValidateCaptureCoverage fails a capture whose recorded video is shorter than
// the window the plan scheduled. A clip truncated before its last kill still
// passes the presence and size checks in ValidateRecordingAttempt, so without
// this the reel is forged with that kill silently missing. Only a deficit is
// fatal: a longer-than-planned clip loses no gameplay and stays a warning.
func ValidateCaptureCoverage(plan RecordingPlan, artifacts []RecordingArtifact) error {
	bySegment := map[string][]RecordingArtifact{}
	for _, a := range artifacts {
		if a.SegmentID != "" {
			bySegment[a.SegmentID] = append(bySegment[a.SegmentID], a)
		}
	}
	var truncated []string
	for _, s := range plan.Segments {
		expected := expectedSegmentDurationSeconds(s, plan)
		if expected <= 0 {
			continue
		}
		for _, a := range bySegment[s.ID] {
			if a.Type != "video" || a.DurationSeconds <= 0 {
				continue
			}
			deficit := expected - a.DurationSeconds
			if deficit > captureDurationToleranceSeconds {
				truncated = append(truncated, fmt.Sprintf(
					"segment %s %s video %s is %.3fs short of the scheduled %.3fs",
					s.ID, a.Role, a.Path, deficit, expected))
			}
		}
	}
	if len(truncated) == 0 {
		return nil
	}
	return fmt.Errorf("capture truncated: %s", strings.Join(truncated, "; "))
}

// ValidateArtifacts returns non-fatal warnings for missing or suspicious
// recorder outputs. The caller still owns deciding whether to fail a job.
func ValidateArtifacts(plan RecordingPlan, artifacts []RecordingArtifact) []string {
	var warnings []string
	for _, a := range artifacts {
		if a.ProbeError != "" {
			warnings = append(warnings, fmt.Sprintf("%s: %s", a.Path, a.ProbeError))
		}
	}

	bySegment := map[string][]RecordingArtifact{}
	rawTakes := map[string]bool{}
	for _, a := range artifacts {
		if a.SegmentID != "" {
			bySegment[a.SegmentID] = append(bySegment[a.SegmentID], a)
		}
		if a.Role == "raw" && a.TakeID != "" {
			rawTakes[a.TakeID] = true
		}
	}
	if len(rawTakes) != len(plan.Segments) {
		warnings = append(warnings, fmt.Sprintf("raw take count = %d, want %d", len(rawTakes), len(plan.Segments)))
	}

	for _, s := range plan.Segments {
		items := bySegment[s.ID]
		if !hasArtifact(items, "raw", "video") {
			warnings = append(warnings, fmt.Sprintf("segment %s missing raw video", s.ID))
		}
		if !hasArtifact(items, "raw", "audio") {
			warnings = append(warnings, fmt.Sprintf("segment %s missing raw audio", s.ID))
		}
		if !hasArtifact(items, "segment", "video") {
			warnings = append(warnings, fmt.Sprintf("segment %s missing muxed clip", s.ID))
		}
		expected := expectedSegmentDurationSeconds(s, plan)
		for _, a := range items {
			if a.Type != "video" || a.DurationSeconds <= 0 {
				continue
			}
			if math.Abs(a.DurationSeconds-expected) > captureDurationToleranceSeconds {
				warnings = append(warnings, fmt.Sprintf("segment %s %s video duration %.3fs differs from expected %.3fs", s.ID, a.Role, a.DurationSeconds, expected))
			}
		}
	}
	return warnings
}

func hasArtifact(items []RecordingArtifact, role, typ string) bool {
	for _, a := range items {
		if a.Role == role && a.Type == typ && a.Path != "" && a.ProbeError == "" {
			return true
		}
	}
	return false
}
