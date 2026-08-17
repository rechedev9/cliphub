package recording

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func TestValidateArtifactsAcceptsCompleteSet(t *testing.T) {
	plan := testPlan()
	artifacts := []RecordingArtifact{
		{SegmentID: "seg-001", TakeID: "take0000", Role: "raw", Type: "video", Path: "take0000/video.mp4", DurationSeconds: 5},
		{SegmentID: "seg-001", TakeID: "take0000", Role: "raw", Type: "audio", Path: "take0000/audio.wav"},
		{SegmentID: "seg-001", TakeID: "take0000", Role: "segment", Type: "video", Path: "segments/seg-001.mp4", DurationSeconds: 5},
		{SegmentID: "seg-002", TakeID: "take0001", Role: "raw", Type: "video", Path: "take0001/video.mp4", DurationSeconds: 8},
		{SegmentID: "seg-002", TakeID: "take0001", Role: "raw", Type: "audio", Path: "take0001/audio.wav"},
		{SegmentID: "seg-002", TakeID: "take0001", Role: "segment", Type: "video", Path: "segments/seg-002.mp4", DurationSeconds: 8},
	}
	if warnings := ValidateArtifacts(plan, artifacts); len(warnings) != 0 {
		t.Fatalf("ValidateArtifacts warnings = %v", warnings)
	}
}

func TestValidateArtifactsReportsMissingOutputs(t *testing.T) {
	plan := testPlan()
	warnings := ValidateArtifacts(plan, []RecordingArtifact{
		{SegmentID: "seg-001", TakeID: "take0000", Role: "raw", Type: "video", Path: "take0000/video.mp4", DurationSeconds: 1},
	})
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{
		"raw take count = 1, want 2",
		"segment seg-001 missing raw audio",
		"segment seg-001 missing muxed clip",
		"segment seg-002 missing raw video",
		"segment seg-001 raw video duration",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("warnings missing %q:\n%s", want, joined)
		}
	}
}

// TestSegmentDurationWindow pins both duration checks to the window the
// recorder actually schedules. The EOF-clamped cases are the regression: their
// record-end is pulled back by EffectiveRecordEndTick, so measuring against the
// raw plan TickEnd reported a phantom deficit on the last segment of every run
// and buried a real truncation in that noise.
func TestSegmentDurationWindow(t *testing.T) {
	tests := []struct {
		name string
		// segment fields; TickEnd beyond the demo's EOF safety margin
		// (softCap = 40000-128 = 39872 for testPlan) triggers the clamp.
		segment RecordingSegment
		// duration probed for both the raw and the muxed video artifact.
		duration float64
		// expected scheduled window, kept explicit so the arithmetic is
		// reviewable rather than recomputed by the test.
		wantExpected     float64
		wantDurationWarn bool
		wantTruncated    bool
	}{
		{
			name:         "no clamp, exact",
			segment:      RecordingSegment{ID: "seg-001", TickStart: 22086, TickEnd: 22406},
			duration:     5.0,
			wantExpected: 5.0,
		},
		{
			name:         "no clamp, camera-settle start offset",
			segment:      RecordingSegment{ID: "seg-001", TickStart: 14029, TickEnd: 14770, Kills: []killplan.Kill{{Tick: 14221}}},
			duration:     9.57,
			wantExpected: 9.578125,
		},
		{
			name:         "clamped by EOF margin",
			segment:      RecordingSegment{ID: "seg-001", TickStart: 39500, TickEnd: 40000},
			duration:     5.8125,
			wantExpected: 5.8125,
		},
		{
			name:             "truncated, no clamp",
			segment:          RecordingSegment{ID: "seg-001", TickStart: 22086, TickEnd: 22406},
			duration:         4.0,
			wantExpected:     5.0,
			wantDurationWarn: true,
			wantTruncated:    true,
		},
		{
			name:             "truncated, clamped by EOF margin",
			segment:          RecordingSegment{ID: "seg-001", TickStart: 39500, TickEnd: 40000},
			duration:         4.8125,
			wantExpected:     5.8125,
			wantDurationWarn: true,
			wantTruncated:    true,
		},
		{
			name:             "longer than scheduled warns but never fails",
			segment:          RecordingSegment{ID: "seg-001", TickStart: 22086, TickEnd: 22406},
			duration:         6.0,
			wantExpected:     5.0,
			wantDurationWarn: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := testPlan()
			plan.Segments = []RecordingSegment{tc.segment}
			if got := expectedSegmentDurationSeconds(tc.segment, plan); got != tc.wantExpected {
				t.Fatalf("expectedSegmentDurationSeconds = %v, want %v", got, tc.wantExpected)
			}
			artifacts := []RecordingArtifact{
				{SegmentID: tc.segment.ID, TakeID: "take0000", Role: "raw", Type: "video", Path: "take0000/video.mp4", DurationSeconds: tc.duration},
				{SegmentID: tc.segment.ID, TakeID: "take0000", Role: "raw", Type: "audio", Path: "take0000/audio.wav"},
				{SegmentID: tc.segment.ID, TakeID: "take0000", Role: "segment", Type: "video", Path: "segments/seg-001.mp4", DurationSeconds: tc.duration},
			}

			durationWarnings := 0
			for _, w := range ValidateArtifacts(plan, artifacts) {
				if !strings.Contains(w, "video duration") {
					t.Fatalf("unexpected warning %q", w)
				}
				durationWarnings++
			}
			// One per video artifact: the raw take and the muxed clip.
			if want := map[bool]int{true: 2, false: 0}[tc.wantDurationWarn]; durationWarnings != want {
				t.Fatalf("duration warnings = %d, want %d", durationWarnings, want)
			}

			err := ValidateCaptureCoverage(plan, artifacts)
			if tc.wantTruncated {
				if err == nil {
					t.Fatalf("ValidateCaptureCoverage = nil, want truncation error")
				}
				if !strings.Contains(err.Error(), "capture truncated") {
					t.Fatalf("ValidateCaptureCoverage error = %v, want capture truncated", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateCaptureCoverage = %v, want nil", err)
			}
		})
	}
}

func TestValidateCaptureCoverageSkipsUnmeasurableClips(t *testing.T) {
	plan := testPlan()
	// A missing probe (DurationSeconds 0) is ValidateArtifacts' business; the
	// fatal check must not turn an unprobed clip into a truncated capture.
	artifacts := []RecordingArtifact{
		{SegmentID: "seg-001", TakeID: "take0000", Role: "segment", Type: "video", Path: "segments/seg-001.mp4"},
		{SegmentID: "seg-002", TakeID: "take0001", Role: "segment", Type: "video", Path: "segments/seg-002.mp4", DurationSeconds: 8},
	}
	if err := ValidateCaptureCoverage(plan, artifacts); err != nil {
		t.Fatalf("ValidateCaptureCoverage = %v, want nil", err)
	}
}
