package recording

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

func expectedCaptureEvidence(t *testing.T, plan RecordingPlan) *FullDemoCaptureEvidence {
	t.Helper()
	expected, err := FullDemoExpectedCaptureCvars(plan)
	if err != nil {
		t.Fatal(err)
	}
	evidence := &FullDemoCaptureEvidence{SchemaVersion: "1.0", Restored: true, FilesRestored: true, CertifiedEnds: map[string]int{"round-001": 100}}
	for name, value := range expected {
		evidence.Before = append(evidence.Before, CvarValue{Name: name, Value: json.RawMessage("1")})
		evidence.Applied = append(evidence.Applied, CvarValue{Name: name, Value: value})
	}
	return evidence
}

// The expected readbacks and the capture contract share their tables, so a
// passing round trip alone would not notice a requirement dropped from both.
// Each wrong readback a capture can produce must still be rejected.
func TestFullDemoCaptureContractRejectsEachWrongReadback(t *testing.T) {
	document := recapplan.Document{Options: recapplan.DefaultOptions()}
	document.Options.Capture.HUDProfile = recapplan.NativeHUDProfile
	plan := RecordingPlan{FullDemo: &document, Segments: []RecordingSegment{{ID: "round-001", TickStart: 0, TickEnd: 100, LiveEndTick: 90}}}
	if err := expectedCaptureEvidence(t, plan).Validate(plan); err != nil {
		t.Fatalf("expected readbacks fail the capture contract: %v", err)
	}
	for _, tc := range []struct {
		cvar, value, reason string
	}{
		{"spec_show_xray", "1", "was not disabled"},
		{"voice_modenable", "true", "was not disabled"},
		{"cl_show_observer_crosshair", "0", "Crosshair source"},
		{"cl_drawhud", "0", "readback differs: cl_drawhud"},
		{"cl_drawhud_force_teamid_overhead", "0", "readback differs: cl_drawhud_force_teamid_overhead"},
		{"crosshair", `"on"`, "Invalid HUD/crosshair readback"},
	} {
		t.Run(tc.cvar+"="+tc.value, func(t *testing.T) {
			evidence := expectedCaptureEvidence(t, plan)
			for i := range evidence.Applied {
				if evidence.Applied[i].Name == tc.cvar {
					evidence.Applied[i].Value = json.RawMessage(tc.value)
				}
			}
			if err := evidence.Validate(plan); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("readback %s=%s: error %v, want %q", tc.cvar, tc.value, err, tc.reason)
			}
		})
	}
}
