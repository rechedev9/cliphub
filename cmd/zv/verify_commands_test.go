package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/verify"
)

func TestRunVerifyDoctorJSONSchemaAndGap(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"zv", "verify", "doctor", "--format", "json"}, &stdout, &stderr, nil, nil)
	var report verify.DoctorReport
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		t.Fatalf("unmarshal doctor: %v\n%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if report.SchemaVersion != verify.SchemaVersion {
		t.Fatalf("schema_version = %d, want %d", report.SchemaVersion, verify.SchemaVersion)
	}
	if !report.Skill.OK {
		t.Fatalf("skill = %#v", report.Skill)
	}
	if len(report.Features) != len(verify.Features()) {
		t.Fatalf("features = %d, want %d", len(report.Features), len(verify.Features()))
	}
	named := false
	for _, gap := range report.Gaps {
		if gap.ID == verify.ClosedCaptureGapID && strings.Contains(gap.Message, "Cloud Linux") {
			named = true
		}
	}
	if runtime.GOOS != "windows" {
		if code != exitInvalidArgs {
			t.Fatalf("linux doctor code = %d, want %d; stderr=%s", code, exitInvalidArgs, stderr.String())
		}
		if report.OK || !report.Closed || !named {
			t.Fatalf("linux doctor must fail closed with named gap: %#v", report)
		}
	}
	if report.Host.CaptureRecertification == verify.CaptureUnavailable && (report.OK || !named) {
		t.Fatalf("unavailable capture must name the gap and not be ok: %#v", report)
	}
}

func TestRunVerifyFeaturesJSON(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"zv", "verify", "features", "--format", "json"}, &stdout, &stderr, nil, nil)
	if code != exitSuccess {
		t.Fatalf("code = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var report verify.FeatureMapReport
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK || !report.IndexPresent {
		t.Fatalf("feature map = %#v", report)
	}
}

func TestRunVerifyHTTPAbsent(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"zv", "verify", "http", "--url", "http://127.0.0.1:1", "--format", "json"}, &stdout, &stderr, nil, nil)
	if code != exitSuccess {
		t.Fatalf("absent orchestrator should be honest, not a crash: code=%d stderr=%s", code, stderr.String())
	}
	var report verify.HTTPReport
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		t.Fatal(err)
	}
	if report.Status != verify.HTTPStatusAbsent {
		t.Fatalf("status = %q, want absent", report.Status)
	}
}

func TestRunVerifyGatesDryRun(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"zv", "verify", "gates", "--run", "--dry-run", "--format", "json"}, &stdout, &stderr, nil, nil)
	if code != exitSuccess {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	var report verify.GateReport
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		t.Fatal(err)
	}
	if report.Playwright || report.HLAE || !report.DryRun {
		t.Fatalf("gates = %#v", report)
	}
}

func TestRunVerifyGatesRunWithoutDryRunFails(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"zv", "verify", "gates", "--run"}, &stdout, &stderr, nil, nil)
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(stderr.String(), "second CI runner") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunVerifyProveCaptureGap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows host may have capture tools; linux fail-closed is the CI contract")
	}
	var stdout, stderr strings.Builder
	code := Run([]string{"zv", "verify", "prove", "--feature", "full-demo-16x9-wait", "--format", "json"}, &stdout, &stderr, nil, nil)
	if code != exitInvalidArgs {
		t.Fatalf("code = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var report verify.ProveReport
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		t.Fatal(err)
	}
	if report.OK || !report.Closed || report.Gap == nil || report.Gap.ID != verify.ClosedCaptureGapID {
		t.Fatalf("prove = %#v, want named closed gap", report)
	}
}

func TestRunVerifyProveDryRun(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"zv", "verify", "prove", "--feature", "inicio", "--dry-run", "--format", "json"}, &stdout, &stderr, nil, nil)
	if code != exitSuccess {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"dry_run": true`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRunVerifyProveDryRunDoesNotTouchJobsDB(t *testing.T) {
	userData := t.TempDir()
	if err := os.MkdirAll(filepath.Join(userData, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(userData, "data", "jobs.db")
	if err := os.WriteFile(db, []byte("SQLite format 3"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userData, "ports.json"), []byte(`{"orchestrator":41001,"web":42002}`), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code := Run([]string{
		"zv", "verify", "prove",
		"--feature", "demo-completa",
		"--job-id", "11111111-1111-1111-1111-111111111111",
		"--dry-run",
		"--user-data", userData,
		"--format", "json",
	}, &stdout, &stderr, nil, nil)
	if code != exitSuccess {
		t.Fatalf("code = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	after, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("dry-run mutated jobs.db")
	}
}

func TestRunVerifyDoctorHelpListsFlagsAndGaps(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"zv", "verify", "doctor", "--help"}, &stdout, &stderr, nil, nil)
	if code != exitSuccess || stderr.String() != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	text := strings.ToLower(stdout.String())
	for _, want := range []string{
		"--format", "--dry-run", "--user-data",
		"hlae_cs2_windows_studio", "studio_ports_missing", "studio_jobs_db_missing",
		"studio_orchestrator_down", "hlae_not_detected", "cs2_not_running",
		"never fake", "fail-closed",
	} {
		if !strings.Contains(text, strings.ToLower(want)) {
			t.Fatalf("doctor help missing %q\n%s", want, stdout.String())
		}
	}
}

func TestValidateVerifyCommand(t *testing.T) {
	tests := []struct {
		command []string
		want    string
	}{
		{command: []string{"verify", "doctor", "--format", "json"}},
		{command: []string{"verify", "doctor", "--dry-run", "--format", "json"}},
		{command: []string{"verify", "features", "--format", "json"}},
		{command: []string{"verify", "http", "--url", "http://127.0.0.1:8080", "--format", "json"}},
		{command: []string{"verify", "gates", "--run", "--dry-run", "--format", "json"}},
		{command: []string{"verify", "prove", "--feature", "inicio", "--dry-run", "--format", "json"}},
		{command: []string{"verify"}, want: `uses non-standard zv command "verify"; expected "verify doctor", "verify features", "verify http", "verify gates", or "verify prove"`},
		{command: []string{"verify", "prove"}, want: `"verify prove" requires --feature`},
	}
	for _, tt := range tests {
		got := validateSkillCommand(tt.command)
		if got != tt.want {
			t.Fatalf("validateSkillCommand(%v) = %q, want %q", tt.command, got, tt.want)
		}
	}
}

func TestRunVerifyHelp(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"zv", "verify", "--help"}, &stdout, &stderr, nil, nil)
	if code != exitSuccess || stderr.String() != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if stdout.String() != verifyUsage {
		t.Fatalf("help = %q, want verifyUsage", stdout.String())
	}
}
