package verify

import (
	"encoding/json"
	"runtime"
	"testing"
)

func TestClassifyHostFailsClosedOffWindows(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		goos       string
		hlae, cs2  bool
		wantRecert string
		wantOK     bool
	}{
		{name: "linux stubs", goos: "linux", hlae: true, cs2: true, wantRecert: CaptureUnavailable},
		{name: "darwin missing", goos: "darwin", wantRecert: CaptureUnavailable},
		{name: "windows missing cs2", goos: "windows", hlae: true, wantRecert: CaptureUnavailable},
		{name: "windows tools", goos: "windows", hlae: true, cs2: true, wantRecert: CaptureToolsPresent, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			host := ClassifyHost(tt.goos, "amd64", tt.hlae, tt.cs2)
			if host.CaptureRecertification != tt.wantRecert {
				t.Fatalf("recert = %q, want %q", host.CaptureRecertification, tt.wantRecert)
			}
			if host.CanRecertifyCapture() != tt.wantOK {
				t.Fatalf("CanRecertifyCapture = %t, want %t", host.CanRecertifyCapture(), tt.wantOK)
			}
			if tt.goos != "windows" && (host.HLAE || host.CS2 || host.WindowsStudio) {
				t.Fatalf("non-windows host leaked studio tools: %#v", host)
			}
		})
	}
}

func TestDoctorSchemaAndNamedGap(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	host := ClassifyHost("linux", "amd64", false, false)
	report := Doctor(root, host, HTTPReport{Status: HTTPStatusAbsent, OK: true, URL: DefaultOrchestratorURL + HealthzPath})
	if report.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %d, want %d", report.SchemaVersion, SchemaVersion)
	}
	if report.OK || !report.Closed {
		t.Fatalf("linux doctor OK=%t Closed=%t, want fail closed", report.OK, report.Closed)
	}
	if !hasGap(report.Gaps, ClosedCaptureGapID, ClosedCaptureGap) {
		t.Fatalf("doctor gaps = %#v, want named %s", report.Gaps, ClosedCaptureGapID)
	}
	if !hasGap(report.Gaps, OverlayWalkGapID, OverlayWalkGap) {
		t.Fatalf("doctor gaps = %#v, want overlay walk named", report.Gaps)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema_version", "ok", "closed", "host", "gaps", "available", "skill", "features", "orchestrator", "gates"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("doctor JSON missing key %q", key)
		}
	}
	if runtime.GOOS != "windows" {
		live := InspectHost()
		if live.CanRecertifyCapture() {
			t.Fatal("this non-windows host must not claim capture recertification")
		}
	}
}

func TestDoctorNeverPassesHLAEFeatures(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	host := ClassifyHost("windows", "amd64", true, true)
	report := Doctor(root, host, HTTPReport{Status: HTTPStatusAbsent, OK: true})
	if !report.Host.CanRecertifyCapture() {
		t.Fatal("expected tools_present on the synthetic windows host")
	}
	for _, feature := range report.Features {
		if !feature.RequiresHLAECS2 {
			continue
		}
		if feature.UserPath == "pass" {
			t.Fatalf("feature %s user_path = pass; doctor must not fake a Pass", feature.ID)
		}
	}
}

func hasGap(gaps []Gap, id, message string) bool {
	for _, gap := range gaps {
		if gap.ID == id && gap.Class == "closed" && gap.Message == message {
			return true
		}
	}
	return false
}
