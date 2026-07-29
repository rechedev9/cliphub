package recording

import (
	"strings"
	"testing"
)

func verifiedTestResult(t *testing.T, result RecordingResult) RecordingResult {
	t.Helper()
	plan := testPlan()
	plan.Segments = append([]RecordingSegment(nil), plan.Segments[:1]...)
	if len(result.Plan.Segments) > 0 {
		plan.Segments = append([]RecordingSegment(nil), result.Plan.Segments...)
	}
	result.Plan = plan
	result.CaptureMode = CaptureModeReal
	result.CaptureVerified = true
	if err := result.Plan.Validate(); err != nil {
		t.Fatalf("verified test plan is invalid: %v", err)
	}
	var err error
	result.CaptureInputFingerprint, err = CaptureInputFingerprint(result.Plan)
	if err != nil {
		t.Fatalf("fingerprint verified test plan: %v", err)
	}
	return result
}

func TestValidateRunResultAcceptsSuccessfulResult(t *testing.T) {
	err := ValidateRunResult(verifiedTestResult(t, RecordingResult{}))
	if err != nil {
		t.Fatalf("ValidateRunResult error = %v", err)
	}
}

func TestValidateRunResultAcceptsVerifiedLegacyV1Result(t *testing.T) {
	result := verifiedTestResult(t, RecordingResult{})
	result.Plan.CaptureContract = LegacyCaptureContractVersion
	result.Plan.KillPlanSchemaVersion = ""
	result.Plan.DemoSHA256 = ""
	result.Plan.DemoDurationTicks = 0
	result.Plan.EditorialSegmentIDs = nil
	result.CaptureMode = ""
	result.CaptureInputFingerprint = ""

	if err := ValidateRunResult(result); err != nil {
		t.Fatalf("ValidateRunResult verified legacy V1 error = %v", err)
	}
	if err := ValidateUploadResult(result); err == nil {
		t.Fatal("ValidateUploadResult accepted legacy result without artifact evidence")
	}
	result.Artifacts = []RecordingArtifact{{
		SegmentID: "seg-001",
		Role:      "segment",
		Type:      "video",
		Path:      "segments/seg-001.mp4",
		SizeBytes: 1,
	}}
	if err := ValidateUploadResult(result); err != nil {
		t.Fatalf("ValidateUploadResult verified legacy V1 error = %v", err)
	}
}

func TestValidateRunResultRejectsLegacyV1WithNewContractFields(t *testing.T) {
	base := verifiedTestResult(t, RecordingResult{})
	base.Plan.CaptureContract = LegacyCaptureContractVersion
	base.Plan.KillPlanSchemaVersion = ""
	base.Plan.DemoSHA256 = ""
	base.Plan.DemoDurationTicks = 0
	base.Plan.EditorialSegmentIDs = nil
	base.CaptureMode = ""
	base.CaptureInputFingerprint = ""

	for _, tt := range []struct {
		name   string
		mutate func(*RecordingResult)
	}{
		{name: "capture mode", mutate: func(result *RecordingResult) { result.CaptureMode = CaptureModeFake }},
		{name: "fingerprint", mutate: func(result *RecordingResult) { result.CaptureInputFingerprint = strings.Repeat("a", 64) }},
		{name: "demo hash", mutate: func(result *RecordingResult) { result.Plan.DemoSHA256 = strings.Repeat("a", 64) }},
		{name: "unsafe segment", mutate: func(result *RecordingResult) { result.Plan.Segments[0].ID = "../clip" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := base
			result.Plan.Segments = append([]RecordingSegment(nil), base.Plan.Segments...)
			tt.mutate(&result)
			if err := ValidateRunResult(result); err == nil {
				t.Fatal("ValidateRunResult error = nil, want malformed legacy contract rejected")
			}
		})
	}
}

func TestValidateRunResultRejectsFailedResult(t *testing.T) {
	err := ValidateRunResult(RecordingResult{Error: "recorder failed"})
	if err == nil {
		t.Fatal("ValidateRunResult error = nil, want recording result error")
	}
	if !strings.Contains(err.Error(), "recording result error: recorder failed") {
		t.Fatalf("error = %q, want recording result error", err.Error())
	}
}

func TestValidateRunResultRejectsPendingPublication(t *testing.T) {
	result := verifiedTestResult(t, RecordingResult{})
	result.PublicationPending = true

	err := ValidateRunResult(result)
	if err == nil || !strings.Contains(err.Error(), "publication is pending") {
		t.Fatalf("ValidateRunResult error = %v, want pending publication rejection", err)
	}
}

func TestValidateRunResultRequiresCompletedPOVVerification(t *testing.T) {
	plan := testPlan()
	plan.Segments = append([]RecordingSegment(nil), plan.Segments[:1]...)
	result := RecordingResult{
		Plan:        plan,
		CaptureMode: CaptureModeReal,
	}
	err := ValidateRunResult(result)
	if err == nil || !strings.Contains(err.Error(), "lacks completed POV verification") {
		t.Fatalf("ValidateRunResult error = %v, want missing POV verification", err)
	}

	result.CaptureVerified = true
	result.CaptureInputFingerprint, _ = CaptureInputFingerprint(result.Plan)
	if err := ValidateRunResult(result); err != nil {
		t.Fatalf("ValidateRunResult verified result error = %v", err)
	}
}

func TestValidateRunResultRejectsMalformedPlanWithMatchingFingerprint(t *testing.T) {
	result := verifiedTestResult(t, RecordingResult{})
	result.Plan.Segments[0].TickEnd = result.Plan.Segments[0].TickStart
	var err error
	result.CaptureInputFingerprint, err = CaptureInputFingerprint(result.Plan)
	if err != nil {
		t.Fatal(err)
	}

	err = ValidateRunResult(result)
	if err == nil || !strings.Contains(err.Error(), "recording result plan:") ||
		!strings.Contains(err.Error(), "tick_end must be greater than tick_start") {
		t.Fatalf("ValidateRunResult error = %v, want malformed plan rejection before fingerprint acceptance", err)
	}
}

func TestValidateUploadResultAcceptsSegmentClip(t *testing.T) {
	err := ValidateUploadResult(verifiedTestResult(t, RecordingResult{
		Artifacts: []RecordingArtifact{{
			SegmentID: "seg-001",
			Type:      "video",
			Role:      "segment",
			Path:      "seg-001.mp4",
			SizeBytes: 1,
		}},
	}))
	if err != nil {
		t.Fatalf("ValidateUploadResult error = %v", err)
	}
}

func TestValidateUploadResultAcceptsAllPlannedSegmentClips(t *testing.T) {
	err := ValidateUploadResult(verifiedTestResult(t, RecordingResult{
		Plan: RecordingPlan{Segments: []RecordingSegment{
			{ID: "seg-001", TickStart: 1000, TickEnd: 1200},
			{ID: "seg-002", TickStart: 2000, TickEnd: 2200},
		}},
		Artifacts: []RecordingArtifact{
			{SegmentID: "seg-001", Type: "video", Role: "segment", Path: "seg-001.mp4", SizeBytes: 1},
			{SegmentID: "seg-002", Type: "video", Role: "segment", Path: "seg-002.mp4", SizeBytes: 1},
		},
	}))
	if err != nil {
		t.Fatalf("ValidateUploadResult error = %v", err)
	}
}

func TestValidateUploadResultRejectsMissingPlannedSegmentClips(t *testing.T) {
	err := ValidateUploadResult(verifiedTestResult(t, RecordingResult{
		Plan: RecordingPlan{Segments: []RecordingSegment{
			{ID: "seg-001", TickStart: 1000, TickEnd: 1200},
			{ID: "seg-002", TickStart: 2000, TickEnd: 2200},
			{ID: "seg-003", TickStart: 3000, TickEnd: 3200},
		}},
		Artifacts: []RecordingArtifact{
			{SegmentID: "seg-001", Type: "video", Role: "segment", Path: "seg-001.mp4", SizeBytes: 1},
		},
	}))
	if err == nil {
		t.Fatal("ValidateUploadResult error = nil, want missing planned segments")
	}
	if want := "recording result missing segment clips: seg-002, seg-003"; !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestValidateUploadResultRejectsFailedSegmentArtifacts(t *testing.T) {
	for _, tt := range []struct {
		name     string
		artifact RecordingArtifact
	}{
		{
			name:     "mux error",
			artifact: RecordingArtifact{SegmentID: "seg-001", Type: "video", Role: "segment", Path: "seg-001.mp4", ProbeError: "ffmpeg mux failed"},
		},
		{
			name:     "empty clip",
			artifact: RecordingArtifact{SegmentID: "seg-001", Type: "video", Role: "segment", Path: "seg-001.mp4"},
		},
		{
			name:     "missing path",
			artifact: RecordingArtifact{SegmentID: "seg-001", Type: "video", Role: "segment", SizeBytes: 1},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUploadResult(verifiedTestResult(t, RecordingResult{
				Plan: RecordingPlan{Segments: []RecordingSegment{{
					ID:        "seg-001",
					TickStart: 1000,
					TickEnd:   1200,
				}}},
				Artifacts: []RecordingArtifact{tt.artifact},
			}))
			if err == nil {
				t.Fatal("ValidateUploadResult error = nil, want invalid segment clip")
			}
		})
	}
}

func TestValidateUploadResultRejectsSuccessfulResultWithoutSegmentClips(t *testing.T) {
	err := ValidateUploadResult(verifiedTestResult(t, RecordingResult{
		Artifacts: []RecordingArtifact{{
			SegmentID: "seg-001",
			Type:      "audio",
			Role:      "raw",
		}},
	}))
	if err == nil {
		t.Fatal("ValidateUploadResult error = nil, want missing segment clips")
	}
	if !strings.Contains(err.Error(), "recording result has no segment clips") {
		t.Fatalf("error = %q, want no segment clips", err.Error())
	}
}

func TestValidateUploadResultAcceptsFailedResult(t *testing.T) {
	err := ValidateUploadResult(RecordingResult{Error: "recorder failed"})
	if err != nil {
		t.Fatalf("ValidateUploadResult error = %v", err)
	}
}
