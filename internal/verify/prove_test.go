package verify

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestProveDryRunDoesNotHTTPOrTouchJobsDB(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	userData, probe := windowsStudioProbe(t, true, true, true, true, true)
	db := filepath.Join(userData, filepath.FromSlash(StudioJobsDBRel))
	if err := os.MkdirAll(filepath.Dir(db), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(db, []byte("SQLite format 3"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := fileSum(db)
	report, err := Prove(ProveOptions{
		Root:     root,
		Feature:  "demo-completa",
		JobID:    "11111111-1111-1111-1111-111111111111",
		DryRun:   true,
		UserData: userData,
		GOOS:     "windows",
		GOARCH:   "amd64",
		Probe:    probe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Executed || !report.DryRun {
		t.Fatalf("dry-run prove = %#v", report)
	}
	if probe.healthN != 0 || probe.getN != 0 {
		t.Fatalf("dry-run issued HTTP health=%d get=%d", probe.healthN, probe.getN)
	}
	if fileSum(db) != before {
		t.Fatal("dry-run mutated jobs.db")
	}
}

func TestProveInspectsLiveJobStatus(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	userData, probe := windowsStudioProbe(t, true, true, true, true, true)
	probe.jsonStatus = http.StatusOK
	probe.jsonBody = []byte(`{"status":"recording","progress":{"done":1,"total":4,"percent":25}}`)
	report, err := Prove(ProveOptions{
		Root:     root,
		Feature:  "shorts-9x16-wait",
		JobID:    "11111111-1111-1111-1111-111111111111",
		UserData: userData,
		GOOS:     "windows",
		GOARCH:   "amd64",
		Probe:    probe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Closed || report.Job == nil || report.Job.Progress == nil {
		t.Fatalf("prove = %#v", report)
	}
	if report.Job.Status != "recording" || report.Job.Progress.Percent != 25 {
		t.Fatalf("job = %#v", report.Job)
	}
	if report.Feature.UserPath == "pass" {
		t.Fatal("live inspect must not claim user_path pass")
	}
	if report.Detail == "" || containsFullDemoPass(report.Detail) {
		t.Fatalf("detail = %q", report.Detail)
	}
}

func TestProveLiveStudioNeedsJobID(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	userData, probe := windowsStudioProbe(t, true, true, true, true, true)
	report, err := Prove(ProveOptions{
		Root:     root,
		Feature:  "demo-completa",
		UserData: userData,
		GOOS:     "windows",
		GOARCH:   "amd64",
		Probe:    probe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !report.Closed || report.Gap == nil || report.Gap.ID != JobIDRequiredGapID {
		t.Fatalf("prove = %#v, want job-id required", report)
	}
	if probe.getN != 0 {
		t.Fatalf("prove without job-id issued GET: %d", probe.getN)
	}
}

func TestParseStudioPorts(t *testing.T) {
	t.Parallel()
	ports, err := parseStudioPorts([]byte(`{"orchestrator":41001,"web":42002,"keep":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if ports.Orchestrator != 41001 || ports.Web != 42002 {
		t.Fatalf("ports = %#v", ports)
	}
	if _, err := parseStudioPorts([]byte(`{"orchestrator":0,"web":42002}`)); err == nil {
		t.Fatal("expected invalid ports error")
	}
}

func TestJobStatusJSONContract(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(JobStatusReport{
		OK:     true,
		Status: "recording",
		Progress: &JobProgress{
			Done:    1,
			Total:   4,
			Percent: 25,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	progress, ok := decoded["progress"].(map[string]any)
	if !ok || progress["percent"] != float64(25) {
		t.Fatalf("progress = %#v", decoded["progress"])
	}
}

func containsFullDemoPass(detail string) bool {
	return detail == "Full Demo Pass"
}
