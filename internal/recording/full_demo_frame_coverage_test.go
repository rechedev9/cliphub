package recording

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

func TestFullDemoCaptureRejectsFrameShortageWithinDurationTolerance(t *testing.T) {
	for _, clock := range []struct{ rate, ticks int }{{64, 130}, {100, 203}, {128, 260}} {
		for _, tc := range []struct {
			name      string
			frames    int64
			frameRate string
			wantError bool
		}{
			{"exact", 122, "60/1", false},
			{"extra capture frames", 124, "60/1", false},
			{"one frame short", 121, "60/1", true},
			{"unknown frame count", 0, "60/1", true},
			{"wrong frame rate", 122, "30/1", true},
		} {
			t.Run(fmt.Sprintf("%d/%s", clock.rate, tc.name), func(t *testing.T) {
				plan := testPlan()
				plan.FullDemo = &recapplan.Document{}
				plan.Tickrate = clock.rate
				plan.Segments = []RecordingSegment{{ID: "round-001", TickStart: 1000, TickEnd: 1000 + clock.ticks}}
				artifacts := []RecordingArtifact{
					{SegmentID: "round-001", Role: "raw", Type: "video", FrameCount: 124, FrameRate: "60/1", DurationSeconds: 124.0 / 60},
					{SegmentID: "round-001", Role: "segment", Type: "video", Path: "round-001.mp4", SizeBytes: 1,
						FrameCount: tc.frames, FrameRate: tc.frameRate, DurationSeconds: 122.0 / 60},
				}
				err := ValidateCaptureCoverage(plan, artifacts)
				if (err != nil) != tc.wantError {
					t.Fatalf("rate %d: coverage error=%v, want error=%t", clock.rate, err, tc.wantError)
				}
				if err != nil && !strings.Contains(err.Error(), "round-001") {
					t.Fatalf("missing affected round in error: %v", err)
				}
			})
		}
	}
}

func TestFullDemoFrameCoverageUsesOriginalCaptureStart(t *testing.T) {
	result := RecordingResult{
		Plan:      RecordingPlan{Tickrate: 64, Segments: []RecordingSegment{{ID: "round-001", TickStart: 1000, TickEnd: 1640}}},
		Artifacts: []RecordingArtifact{{SegmentID: "round-001", Role: "segment", Type: "video", Path: "round-001.mp4", SizeBytes: 1, FrameCount: 300, FrameRate: "60/1"}},
	}
	for _, tc := range []struct {
		name       string
		start, end int
		wantError  bool
	}{
		{"shorter approved tail fits", 1000, 1320, false},
		{"start offset also consumes frames", 1064, 1384, true},
		{"one more source tick needs another frame", 1000, 1321, true},
		{"before capture", 999, 1320, true},
		{"after capture", 1000, 1641, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := result.ValidateFullDemoRoundFrames("round-001", tc.start, tc.end)
			if (err != nil) != tc.wantError {
				t.Fatalf("coverage error=%v, want error=%t", err, tc.wantError)
			}
		})
	}
}
