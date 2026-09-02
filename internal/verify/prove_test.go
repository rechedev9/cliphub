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
	tests := []struct {
		name string
		raw  string
		want StudioPorts
		bad  bool
	}{
		{name: "single port", raw: `{"web":42002,"keep":true}`, want: StudioPorts{Web: 42002}},
		{name: "legacy two ports", raw: `{"orchestrator":41001,"web":42002}`, want: StudioPorts{Orchestrator: 41001, Web: 42002}},
		{name: "missing web", raw: `{"orchestrator":41001}`, bad: true},
		{name: "invalid legacy orchestrator", raw: `{"orchestrator":65536,"web":42002}`, bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStudioPorts([]byte(tt.raw))
			if tt.bad {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
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

func TestProveDryRunDriveReadsPortsWithoutHTTP(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	userData, probe := windowsStudioProbe(t, true, true, true, true, true)
	report, err := Prove(ProveOptions{
		Root:     root,
		Feature:  "inicio",
		DryRun:   true,
		UserData: userData,
		GOOS:     "windows",
		GOARCH:   "amd64",
		Probe:    probe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if probe.healthN != 0 || probe.getN != 0 {
		t.Fatalf("drive lookup issued HTTP health=%d get=%d", probe.healthN, probe.getN)
	}
	if report.Drive == nil || report.Drive.Route != "/onboarding" {
		t.Fatalf("drive = %#v", report.Drive)
	}
	if report.Drive.OpenURL != "http://127.0.0.1:42002/onboarding" {
		t.Fatalf("open_url = %q", report.Drive.OpenURL)
	}
}

func TestProveCheapLiveInspectsAPI(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	userData, probe := windowsStudioProbe(t, true, true, true, true, true)
	probe.jsonStatus = http.StatusOK
	probe.jsonBody = []byte(`{"jobs":[]}`)
	report, err := Prove(ProveOptions{
		Root:     root,
		Feature:  "partidas",
		UserData: userData,
		GOOS:     "windows",
		GOARCH:   "amd64",
		Probe:    probe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Closed || report.Live == nil || !report.Live.OK {
		t.Fatalf("prove = %#v", report)
	}
	if report.Feature.UserPath != "inspected" {
		t.Fatalf("user_path = %q, want inspected", report.Feature.UserPath)
	}
	if report.Feature.UserPath == "pass" || containsFullDemoPass(report.Detail) {
		t.Fatalf("cheap live inspect leaked a Pass: %q", report.Detail)
	}
	if probe.getN == 0 {
		t.Fatal("expected live GET")
	}
	if report.Live.ItemCount == nil || *report.Live.ItemCount != 0 {
		t.Fatalf("item_count = %v", report.Live.ItemCount)
	}
	if report.Live.URL != "http://127.0.0.1:42002/api/demos/jobs" {
		t.Fatalf("live.url = %q, want Studio web proxy not orchestrator", report.Live.URL)
	}
}

func TestProveCheapStudioDownSkipsLiveGET(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	userData, probe := windowsStudioProbe(t, false, false, true, true, false)
	report, err := Prove(ProveOptions{
		Root:     root,
		Feature:  "partidas",
		UserData: userData,
		GOOS:     "windows",
		GOARCH:   "amd64",
		Probe:    probe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Closed || report.Live != nil {
		t.Fatalf("prove = %#v", report)
	}
	if probe.getN != 0 {
		t.Fatalf("studio-down cheap prove issued GET: %d", probe.getN)
	}
	if report.Feature.UserPath != "unproven" {
		t.Fatalf("user_path = %q", report.Feature.UserPath)
	}
}

func TestProveCheapLiveGETFailureIsNotCaptureClose(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	userData, probe := windowsStudioProbe(t, true, true, true, true, true)
	probe.jsonStatus = http.StatusInternalServerError
	probe.jsonBody = []byte(`{"error":"internal error"}`)
	report, err := Prove(ProveOptions{
		Root:     root,
		Feature:  "partidas",
		UserData: userData,
		GOOS:     "windows",
		GOARCH:   "amd64",
		Probe:    probe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Closed || report.Gap != nil {
		t.Fatalf("prove = %#v, want failed inspect without capture close", report)
	}
	if report.Live == nil || report.Live.OK {
		t.Fatalf("live = %#v", report.Live)
	}
	if report.Feature.UserPath == "pass" || containsFullDemoPass(report.Detail) {
		t.Fatalf("detail = %q", report.Detail)
	}
}

func containsFullDemoPass(detail string) bool {
	return detail == "Full Demo Pass"
}
