package recording

import (
	"fmt"
	"reflect"

	"github.com/google/uuid"
)

// FullDemoCaptureRun preserves each original launch and its verified evidence
// when the existing job-level result accumulates clips across capture runs.
// The aggregate never attributes an earlier segment to a later attestation.
type FullDemoCaptureRun struct {
	Revision         string                   `json:"revision"`
	Plan             RecordingPlan            `json:"plan"`
	Evidence         *FullDemoCaptureEvidence `json:"evidence"`
	InputFingerprint string                   `json:"input_fingerprint"`
	CaptureMode      CaptureMode              `json:"capture_mode"`
	CaptureVerified  bool                     `json:"capture_verified"`
}

func (r RecordingResult) CaptureRuns() []FullDemoCaptureRun {
	if len(r.FullDemoRuns) > 0 {
		return append([]FullDemoCaptureRun(nil), r.FullDemoRuns...)
	}
	if r.Plan.FullDemo == nil {
		return nil
	}
	return []FullDemoCaptureRun{{Revision: r.CaptureRevision, Plan: r.Plan, Evidence: r.FullDemoEvidence, InputFingerprint: r.CaptureInputFingerprint, CaptureMode: r.CaptureMode, CaptureVerified: r.CaptureVerified}}
}

func (r RecordingResult) validateFullDemoRuns() error {
	if len(r.FullDemoRuns) == 0 {
		if len(r.Plan.FullDemoSources) != 0 {
			return fmt.Errorf("full demo capture origins require original run evidence")
		}
		return r.FullDemoEvidence.Validate(r.Plan)
	}
	if len(r.FullDemoRuns) > 1000 || r.FullDemoEvidence == nil {
		return fmt.Errorf("invalid Full Demo capture run count")
	}
	runs := map[string]FullDemoCaptureRun{}
	for _, run := range r.FullDemoRuns {
		if _, err := uuid.Parse(run.Revision); err != nil {
			return fmt.Errorf("invalid Full Demo capture origin revision")
		}
		if _, duplicate := runs[run.Revision]; duplicate {
			return fmt.Errorf("duplicate Full Demo capture origin")
		}
		if len(run.Plan.FullDemoSources) != 0 {
			return fmt.Errorf("a capture origin must be one original launch")
		}
		if run.Plan.DemoSHA256 != r.Plan.DemoSHA256 || run.Plan.TargetSteamID64 != r.Plan.TargetSteamID64 || run.Plan.Tickrate != r.Plan.Tickrate || run.Plan.Stream != r.Plan.Stream {
			return fmt.Errorf("full demo capture origin belongs to another source or capture profile")
		}
		original := RecordingResult{Plan: run.Plan, FullDemoEvidence: run.Evidence, CaptureInputFingerprint: run.InputFingerprint, CaptureMode: run.CaptureMode, CaptureVerified: run.CaptureVerified}
		if err := ValidateRunResult(original); err != nil {
			return fmt.Errorf("validate Full Demo capture origin: %w", err)
		}
		runs[run.Revision] = run
	}
	if len(r.FullDemoEvidence.CertifiedEnds) != len(r.Plan.Segments) {
		return fmt.Errorf("full demo aggregate has an incomplete coverage summary")
	}
	for _, segment := range r.Plan.Segments {
		var origin string
		for _, artifact := range r.Artifacts {
			if isUsableSegmentClip(artifact) && artifact.SegmentID == segment.ID {
				origin = artifact.CaptureRevision
				break
			}
		}
		run, ok := runs[origin]
		if !ok {
			return fmt.Errorf("full demo clip %s lacks its original attestation", segment.ID)
		}
		matched := false
		for _, original := range run.Plan.Segments {
			if reflect.DeepEqual(original, segment) {
				matched = true
				break
			}
		}
		if !matched || run.Evidence.CertifiedEnds[segment.ID] != r.FullDemoEvidence.CertifiedEnds[segment.ID] {
			return fmt.Errorf("full demo aggregate coverage differs from its original run: %s", segment.ID)
		}
	}
	return nil
}
