package verify

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type fakeProbe struct {
	files      map[string][]byte
	hlaePath   string
	hlaeOK     bool
	cs2Running bool
	cs2Err     error
	health     HTTPReport
	jsonStatus int
	jsonBody   []byte
	jsonErr    error
	healthN    int
	getN       int
}

func (f *fakeProbe) ReadFile(path string) ([]byte, error) {
	if b, ok := f.files[path]; ok {
		return b, nil
	}
	return nil, os.ErrNotExist
}

func (f *fakeProbe) FileExists(path string) bool {
	_, ok := f.files[path]
	return ok
}

func (f *fakeProbe) DetectHLAE() (string, bool) {
	return f.hlaePath, f.hlaeOK
}

func (f *fakeProbe) CS2Running() (bool, error) {
	return f.cs2Running, f.cs2Err
}

func (f *fakeProbe) Healthz(string) HTTPReport {
	f.healthN++
	return f.health
}

func (f *fakeProbe) GetJSON(string) (int, []byte, error) {
	f.getN++
	return f.jsonStatus, f.jsonBody, f.jsonErr
}

func TestClassifyHostFailsClosedOffWindows(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		facts      HostFacts
		wantRecert string
		wantOK     bool
	}{
		{name: "linux stubs", facts: HostFacts{GOOS: "linux", GOARCH: "amd64", StudioUp: true, HLAE: true, CS2Running: true}, wantRecert: CaptureUnavailable},
		{name: "darwin missing", facts: HostFacts{GOOS: "darwin", GOARCH: "amd64"}, wantRecert: CaptureUnavailable},
		{name: "windows missing cs2", facts: HostFacts{GOOS: "windows", GOARCH: "amd64", StudioUp: true, HLAE: true}, wantRecert: CaptureUnavailable},
		{name: "windows tools only", facts: HostFacts{GOOS: "windows", GOARCH: "amd64", HLAE: true, CS2Running: true}, wantRecert: CaptureUnavailable},
		{name: "windows studio live", facts: HostFacts{GOOS: "windows", GOARCH: "amd64", StudioUp: true, HLAE: true, CS2Running: true}, wantRecert: CaptureStudioLive, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			host := ClassifyHost(tt.facts)
			if host.CaptureRecertification != tt.wantRecert {
				t.Fatalf("recert = %q, want %q", host.CaptureRecertification, tt.wantRecert)
			}
			if host.CanRecertifyCapture() != tt.wantOK {
				t.Fatalf("CanRecertifyCapture = %t, want %t", host.CanRecertifyCapture(), tt.wantOK)
			}
			if tt.facts.GOOS != "windows" && (host.HLAE || host.CS2 || host.WindowsStudio) {
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
	report := Doctor(DoctorOptions{Root: root, GOOS: "linux", GOARCH: "amd64", Probe: &fakeProbe{}})
	if report.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %d, want %d", report.SchemaVersion, SchemaVersion)
	}
	if report.OK || !report.Closed {
		t.Fatalf("linux doctor OK=%t Closed=%t, want fail closed", report.OK, report.Closed)
	}
	if !hasGap(report.Gaps, ClosedCaptureGapID, ClosedCaptureGap) {
		t.Fatalf("doctor gaps = %#v, want named %s", report.Gaps, ClosedCaptureGapID)
	}
	if !hasGap(report.Remaining, OverlayWalkGapID, OverlayWalkGap) {
		t.Fatalf("doctor remaining = %#v, want overlay walk named", report.Remaining)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema_version", "ok", "closed", "host", "studio", "hlae", "cs2", "gaps", "available", "skill", "features", "orchestrator", "gates"} {
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

func TestDoctorWindowsStudioDetection(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		ports      bool
		jobsDB     bool
		hlae       bool
		cs2        bool
		healthz    bool
		wantOK     bool
		wantGap    string
		wantRecert string
	}{
		{name: "missing ports.json", wantGap: StudioPortsGapID, wantRecert: CaptureUnavailable},
		{name: "missing jobs.db", ports: true, healthz: true, hlae: true, cs2: true, wantGap: StudioJobsDBGapID, wantRecert: CaptureUnavailable},
		{name: "missing HLAE", ports: true, jobsDB: true, healthz: true, cs2: true, wantGap: HLAEMissingGapID, wantRecert: CaptureUnavailable},
		{name: "CS2 not running", ports: true, jobsDB: true, healthz: true, hlae: true, wantGap: CS2NotRunningGapID, wantRecert: CaptureUnavailable},
		{name: "studio down", ports: true, jobsDB: true, hlae: true, cs2: true, wantGap: StudioDownGapID, wantRecert: CaptureUnavailable},
		{name: "all present", ports: true, jobsDB: true, healthz: true, hlae: true, cs2: true, wantOK: true, wantRecert: CaptureStudioLive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userData, probe := windowsStudioProbe(t, tt.ports, tt.jobsDB, tt.hlae, tt.cs2, tt.healthz)
			report := Doctor(DoctorOptions{
				Root:     root,
				GOOS:     "windows",
				GOARCH:   "amd64",
				UserData: userData,
				Probe:    probe,
			})
			if report.Host.CaptureRecertification != tt.wantRecert {
				t.Fatalf("recert = %q, want %q; gaps=%#v", report.Host.CaptureRecertification, tt.wantRecert, report.Gaps)
			}
			if report.OK != tt.wantOK || report.Closed == tt.wantOK {
				t.Fatalf("OK=%t Closed=%t, want OK=%t; gaps=%#v", report.OK, report.Closed, tt.wantOK, report.Gaps)
			}
			if tt.wantGap != "" && !hasGapID(report.Gaps, tt.wantGap) {
				t.Fatalf("gaps = %#v, want %s", report.Gaps, tt.wantGap)
			}
			if hasGapID(report.Gaps, ClosedCaptureGapID) {
				t.Fatalf("windows doctor must name the specific surface gap, not only %s", ClosedCaptureGapID)
			}
			if tt.wantOK && (report.HLAE.RejectedBare || !report.CS2.Running) {
				t.Fatalf("live doctor leaked a fake CS2/HLAE pass: %#v %#v", report.HLAE, report.CS2)
			}
		})
	}
}

func TestDoctorLinuxNeverPassesEvenWhenSurfaceIsFaked(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	userData, probe := windowsStudioProbe(t, true, true, true, true, true)
	report := Doctor(DoctorOptions{
		Root:     root,
		GOOS:     "linux",
		GOARCH:   "amd64",
		UserData: userData,
		Probe:    probe,
	})
	if report.OK || !report.Closed || report.Host.CanRecertifyCapture() {
		t.Fatalf("linux must not Pass capture recertification: %#v", report)
	}
	if !hasGap(report.Gaps, ClosedCaptureGapID, ClosedCaptureGap) {
		t.Fatalf("gaps = %#v, want %s", report.Gaps, ClosedCaptureGapID)
	}
}

func TestDoctorRejectsBareCHLAE(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	userData, probe := windowsStudioProbe(t, true, true, true, true, true)
	probe.hlaePath = `C:\HLAE\HLAE.exe`
	probe.hlaeOK = true
	report := Doctor(DoctorOptions{
		Root:     root,
		GOOS:     "windows",
		GOARCH:   "amd64",
		UserData: userData,
		Probe:    probe,
	})
	if report.OK || !report.HLAE.RejectedBare || report.HLAE.Detected {
		t.Fatalf("bare C:\\HLAE\\HLAE.exe must not count as detected: %#v", report.HLAE)
	}
	if !hasGapID(report.Gaps, HLAEMissingGapID) {
		t.Fatalf("gaps = %#v, want %s", report.Gaps, HLAEMissingGapID)
	}
}

func TestDoctorNeverPassesHLAEFeatures(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	userData, probe := windowsStudioProbe(t, true, true, true, true, true)
	report := Doctor(DoctorOptions{
		Root:     root,
		GOOS:     "windows",
		GOARCH:   "amd64",
		UserData: userData,
		Probe:    probe,
	})
	if !report.Host.CanRecertifyCapture() {
		t.Fatal("expected studio_live on the synthetic windows host")
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

func TestDoctorDryRunDoesNotHTTP(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	userData, probe := windowsStudioProbe(t, true, true, true, true, true)
	report := Doctor(DoctorOptions{
		Root:     root,
		GOOS:     "windows",
		GOARCH:   "amd64",
		UserData: userData,
		Probe:    probe,
		DryRun:   true,
	})
	if probe.healthN != 0 || probe.getN != 0 {
		t.Fatalf("dry-run issued HTTP health=%d get=%d", probe.healthN, probe.getN)
	}
	if report.OK {
		t.Fatal("dry-run doctor must not Pass")
	}
}

func TestForbiddenHLAEPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want bool
	}{
		{path: `C:\HLAE\HLAE.exe`, want: true},
		{path: `c:/HLAE/HLAE.exe`, want: true},
		{path: `C:\HLAE-2.191.1\HLAE.exe`, want: false},
		{path: `C:\Users\king\AppData\Roaming\cliphub-studio\tools\hlae\2.191.1\HLAE.exe`, want: false},
	}
	for _, tt := range tests {
		if got := isForbiddenHLAE(tt.path); got != tt.want {
			t.Fatalf("isForbiddenHLAE(%q) = %t, want %t", tt.path, got, tt.want)
		}
	}
}

func windowsStudioProbe(t *testing.T, ports, jobsDB, hlae, cs2, healthz bool) (string, *fakeProbe) {
	t.Helper()
	userData := t.TempDir()
	probe := &fakeProbe{files: map[string][]byte{}}
	if ports {
		probe.files[filepath.Join(userData, StudioPortsFile)] = []byte(`{"orchestrator":41001,"web":42002}`)
	}
	if jobsDB {
		probe.files[filepath.Join(userData, filepath.FromSlash(StudioJobsDBRel))] = []byte("SQLite format 3")
	}
	if hlae {
		probe.hlaePath = `C:\HLAE-2.191.1\HLAE.exe`
		probe.hlaeOK = true
	}
	probe.cs2Running = cs2
	if healthz {
		probe.health = HTTPReport{OK: true, Status: HTTPStatusOK, Service: "cliphub", BodyStatus: "ok"}
	} else {
		probe.health = HTTPReport{OK: true, Status: HTTPStatusAbsent, Detail: "connection refused"}
	}
	return userData, probe
}

func hasGap(gaps []Gap, id, message string) bool {
	for _, gap := range gaps {
		if gap.ID == id && gap.Class == "closed" && gap.Message == message {
			return true
		}
	}
	return false
}

func hasGapID(gaps []Gap, id string) bool {
	for _, gap := range gaps {
		if gap.ID == id {
			return true
		}
	}
	return false
}

func fileSum(path string) [32]byte {
	raw, err := os.ReadFile(path)
	if err != nil {
		return [32]byte{}
	}
	return sha256.Sum256(raw)
}
