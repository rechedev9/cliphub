package recording

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// incidentTailPadResult reproduces the discarded capture: at 64 Hz the
// round-006 window spans 2,040 ticks, which TickFrames rounds half-up from
// 1,912.5 to 1,913 frames, while the HLAE clip holds 1,912 frames.
func incidentTailPadResult(clipFrames int64) RecordingResult {
	return RecordingResult{
		Plan: RecordingPlan{Tickrate: 64, FullDemo: &recapplan.Document{}, Segments: []RecordingSegment{{ID: "round-006", TickStart: 50_000, TickEnd: 52_040}}},
		Artifacts: []RecordingArtifact{{SegmentID: "round-006", Role: "segment", Type: "video", Path: "round-006.mp4", SizeBytes: 1,
			FrameCount: clipFrames, FrameRate: "60/1"}},
	}
}

func TestFullDemoTailPadAcceptsIncidentOneFrameShortfall(t *testing.T) {
	result := incidentTailPadResult(1912)
	pad, err := result.FullDemoRoundTailPad("round-006", 50_000, 52_040)
	if err != nil {
		t.Fatalf("one missing tail frame must be padded, not fail the capture: %v", err)
	}
	want := FullDemoTailPad{SegmentID: "round-006", ClipFrames: 1912, WindowFrames: 1913, PaddedFrames: 1}
	if pad != want {
		t.Fatalf("pad = %+v, want %+v", pad, want)
	}
	// The recorder gate that rejected the incident uses the same check.
	if err := ValidateCaptureCoverage(result.Plan, result.Artifacts); err != nil {
		t.Fatalf("recorder coverage must accept the incident capture: %v", err)
	}
}

func TestFullDemoTailPadAcceptsShortfallAtTolerance(t *testing.T) {
	result := incidentTailPadResult(1913 - FullDemoTailPadToleranceFrames)
	pad, err := result.FullDemoRoundTailPad("round-006", 50_000, 52_040)
	if err != nil {
		t.Fatalf("a shortfall at the tolerance must be padded: %v", err)
	}
	if pad.PaddedFrames != FullDemoTailPadToleranceFrames || pad.ClipFrames != 1911 || pad.WindowFrames != 1913 {
		t.Fatalf("pad = %+v, want %d padded frames", pad, FullDemoTailPadToleranceFrames)
	}
	if err := pad.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestFullDemoTailPadRejectsShortfallBeyondTolerance(t *testing.T) {
	result := incidentTailPadResult(1910)
	pad, err := result.FullDemoRoundTailPad("round-006", 50_000, 52_040)
	want := "full_demo_capture_incomplete: round-006: clip has 1910 frames; approved window needs 1913 (offset=0, length=1913); recapture this round"
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
	if pad != (FullDemoTailPad{}) {
		t.Fatalf("a rejected window must not report a pad: %+v", pad)
	}
	if err := ValidateCaptureCoverage(result.Plan, result.Artifacts); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("recorder coverage error = %v, want %q", err, want)
	}
}

func TestFullDemoTailPadNeedsCapturedFramesInTheWindow(t *testing.T) {
	// A one-frame window whose only frame is missing would clone a frame from
	// before the window; it is not a tail shortfall.
	result := incidentTailPadResult(10)
	if _, err := result.FullDemoRoundTailPad("round-006", 50_011, 50_012); err == nil {
		t.Fatal("a window without a captured frame must fail")
	}
}

func TestFullDemoTailPadsListOnlyShortRounds(t *testing.T) {
	result := incidentTailPadResult(1912)
	result.Plan.Segments = append(result.Plan.Segments, RecordingSegment{ID: "round-007", TickStart: 60_000, TickEnd: 60_640})
	result.Artifacts = append(result.Artifacts, RecordingArtifact{SegmentID: "round-007", Role: "segment", Type: "video", Path: "round-007.mp4", SizeBytes: 1, FrameCount: 600, FrameRate: "60/1"})
	effective := recapplan.Document{Rounds: []recapplan.Round{
		{ID: "round-006", RequestedStartTick: 50_000, EffectiveEndTick: 52_040},
		{ID: "round-007", RequestedStartTick: 60_000, EffectiveEndTick: 60_640},
	}}
	pads, err := result.FullDemoTailPads(effective)
	if err != nil {
		t.Fatal(err)
	}
	if len(pads) != 1 || pads[0].SegmentID != "round-006" || pads[0].PaddedFrames != 1 {
		t.Fatalf("pads = %+v, want only round-006 padded by one frame", pads)
	}
	if err := result.ValidateFullDemoFrames(effective); err != nil {
		t.Fatal(err)
	}
	// A narrower approved tail on the same clip needs no padding at all.
	effective.Rounds[0].EffectiveEndTick = 52_000
	if pads, err := result.FullDemoTailPads(effective); err != nil || len(pads) != 0 {
		t.Fatalf("narrower tail pads = %+v err=%v, want none", pads, err)
	}
}

func TestFullDemoTailPadValidateRejectsUnboundedPads(t *testing.T) {
	for _, pad := range []FullDemoTailPad{
		{SegmentID: "", ClipFrames: 10, WindowFrames: 11, PaddedFrames: 1},
		{SegmentID: "r", ClipFrames: 0, WindowFrames: 1, PaddedFrames: 1},
		{SegmentID: "r", ClipFrames: 10, WindowFrames: 10, PaddedFrames: 0},
		{SegmentID: "r", ClipFrames: 10, WindowFrames: 13, PaddedFrames: 3},
		{SegmentID: "r", ClipFrames: 10, WindowFrames: 12, PaddedFrames: 1},
		{SegmentID: "r", ClipFrames: 10, WindowFrames: 9, PaddedFrames: -1},
	} {
		if err := pad.Validate(); err == nil {
			t.Fatalf("pad %+v must be rejected", pad)
		}
	}
}
