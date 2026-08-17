package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/tickcut/internal/killplan"
	"github.com/rechedev9/tickcut/internal/recording"
)

func testCaptureAttestation() string {
	return strings.Repeat("unit-test-", 4)
}

func TestCS2LaunchCommandLineUsesWindowedMode(t *testing.T) {
	plan := recording.RecordingPlan{
		DemoPath: `C:\demos\match.dem`,
		Stream: recording.StreamConfig{
			Width:  1920,
			Height: 1080,
		},
	}

	got := cs2LaunchCommandLine(plan, `C:\runs\recording.js`)

	cases := []struct {
		name string
		ok   bool
	}{
		{name: "windowed flag", ok: strings.Contains(got, "-windowed")},
		{name: "windowed before resolution", ok: strings.Index(got, "-windowed") < strings.Index(got, "-w 1920")},
		{name: "playdemo path", ok: strings.Contains(got, `+playdemo "C:\demos\match.dem"`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.ok {
				t.Fatalf("cs2LaunchCommandLine() = %q", got)
			}
		})
	}
}

func TestWriteResultAndReportEmitsMachineReadableDryRunSummary(t *testing.T) {
	outDir := t.TempDir()
	result := recording.RecordingResult{
		Plan:   recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "seg-001"}, {ID: "seg-004"}}},
		Script: filepath.Join(outDir, "recording.js"),
	}
	var output bytes.Buffer
	if err := writeResultAndReport(outDir, result, true, "json", &output); err != nil {
		t.Fatal(err)
	}
	var got recordingSummary
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.OK || !got.DryRun || got.Executed || got.SegmentCount != 2 {
		t.Fatalf("summary = %#v", got)
	}
	if _, err := os.Stat(got.ResultPath); err != nil {
		t.Fatalf("recording result: %v", err)
	}
}

func TestWriteResultOmitsAbsentPerformanceAndRoundTripsPresentPerformance(t *testing.T) {
	outDir := t.TempDir()
	result := recording.RecordingResult{Plan: recording.RecordingPlan{}, Script: "recording.js"}
	if err := writeResult(outDir, result); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(outDir, "recording-result.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"performance"`) {
		t.Fatalf("legacy result unexpectedly contains performance: %s", body)
	}

	result.Performance = &recording.RecordingPerformance{
		Version: 1,
		Runs: []recording.RecordingRunPerformance{{
			CaptureSegmentIDs:   []string{"seg-001"},
			Stream:              recording.StreamConfig{FPS: 60, Width: 1920, Height: 1080},
			BeforeResultWriteMS: 123,
		}},
	}
	if err := writeResult(outDir, result); err != nil {
		t.Fatal(err)
	}
	var decoded recording.RecordingResult
	body, err = os.ReadFile(filepath.Join(outDir, "recording-result.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Performance == nil || decoded.Performance.Runs[0].BeforeResultWriteMS != 123 ||
		decoded.Performance.Runs[0].Stream.FPS != 60 {
		t.Fatalf("performance round trip = %#v", decoded.Performance)
	}
}

func TestPerformanceTraceParsesRecorderMarkers(t *testing.T) {
	run := recording.RecordingRunPerformance{}
	trace := newPerformanceTrace(time.Now(), &run)
	monitor := &cs2ConsoleLogMonitor{trace: trace}
	monitor.consumePerformanceMarkers("[zackvideo] armed at tick 100\n[zackvideo] seek 1 -> 200 attempt 1 ")
	monitor.consumePerformanceMarkers("(at 120)\n[zackvideo] seek-landed -> 200 (at 198)\nnoise\n[zackvideo] record-start-seg:001: mirv_streams record start\n")
	monitor.consumePerformanceMarkers("[zackvideo] record-end-seg:001: mirv_streams record end\n")

	if got, want := len(run.Events), 5; got != want {
		t.Fatalf("event count = %d, want %d: %#v", got, want, run.Events)
	}
	if run.Events[0].Kind != "demo_armed_observed" || run.Events[0].ObservedTick != 100 {
		t.Fatalf("armed event = %#v", run.Events[0])
	}
	if run.Events[1].Kind != "seek_requested_observed" || run.Events[1].TargetTick != 200 || run.Events[1].ObservedTick != 120 {
		t.Fatalf("seek event = %#v", run.Events[1])
	}
	if run.Events[2].Kind != "seek_landed_observed" || run.Events[2].ObservedTick != 198 {
		t.Fatalf("seek landed event = %#v", run.Events[2])
	}
	if run.Events[3].SegmentID != "seg:001" || run.Events[4].Kind != "record_end_requested_observed" {
		t.Fatalf("record events = %#v", run.Events[3:])
	}
}

func TestCaptureSegmentIDsAndSegmentSummary(t *testing.T) {
	plan := recording.RecordingPlan{
		Stream: recording.StreamConfig{FPS: 60, Width: 1920, Height: 1080},
		Segments: []recording.RecordingSegment{
			{ID: "early", TickStart: 100},
			{ID: "late", TickStart: 300},
		},
	}
	if got := strings.Join(captureSegmentIDs(plan), ","); got != "early,late" {
		t.Fatalf("capture segment ids = %q", got)
	}
	run := recording.RecordingRunPerformance{
		CaptureSegmentIDs: []string{"early"},
		Events: []recording.RecordingPerformanceEvent{
			{Kind: "record_start_requested_observed", SegmentID: "early", ElapsedMS: 1_000},
			{Kind: "record_end_requested_observed", SegmentID: "early", ElapsedMS: 3_000},
		},
	}
	got := summarizeSegmentPerformance(run, []recording.RecordingArtifact{
		{SegmentID: "early", Type: "video", FrameCount: 10},
		{SegmentID: "early", Type: "video", Role: "segment", FrameCount: 120, DurationSeconds: 2},
	})
	if len(got) != 1 || got[0].RequestedActiveMS != 2_000 || got[0].VideoFrameCount != 120 ||
		got[0].VideoDurationSeconds != 2 || got[0].ObservedFramesPerSecond != 60 {
		t.Fatalf("segment summary = %#v", got)
	}
}

func TestValidateRecordingOutputDirectoryRejectsSourceInsideNamespace(t *testing.T) {
	outDir := t.TempDir()
	killPlanPath := filepath.Join(outDir, "recording-result.json")
	demoPath := filepath.Join(t.TempDir(), "source.dem")
	if err := validateRecordingOutputDirectory(outDir, killPlanPath, demoPath); err == nil {
		t.Fatal("validateRecordingOutputDirectory error = nil, want source/output namespace conflict")
	}
}

func TestValidateKillPlanDemoBindsSchemaAndSHA256(t *testing.T) {
	demoPath := filepath.Join(t.TempDir(), "match.dem")
	contents := []byte("deterministic demo fixture")
	if err := os.WriteFile(demoPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(contents)
	plan := killplan.NewPlan()
	plan.Demo.SHA256 = fmt.Sprintf("%x", sum)

	if err := validateKillPlanDemo(plan, demoPath); err != nil {
		t.Fatalf("validateKillPlanDemo error = %v", err)
	}
	plan.Demo.SHA256 = strings.Repeat("0", 64)
	if err := validateKillPlanDemo(plan, demoPath); err == nil {
		t.Fatal("validateKillPlanDemo mismatch error = nil")
	}
	plan.Demo.SHA256 = fmt.Sprintf("%x", sum)
	plan.SchemaVersion = "999"
	if err := validateKillPlanDemo(plan, demoPath); err == nil {
		t.Fatal("validateKillPlanDemo future schema error = nil")
	}
}

func TestValidateFreshOutputNamespaceRejectsStaleCaptureArtifacts(t *testing.T) {
	for _, name := range []string{"take0001.mp4", filepath.Join("segments", "seg-001.mp4"), "unexpected.txt"} {
		t.Run(name, func(t *testing.T) {
			outDir := t.TempDir()
			path := filepath.Join(outDir, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := validateFreshOutputNamespace(outDir); err == nil {
				t.Fatal("validateFreshOutputNamespace error = nil")
			}
		})
	}

	outDir := t.TempDir()
	for _, name := range []string{"recording.js", "recording-result.json"} {
		if err := os.WriteFile(filepath.Join(outDir, name), []byte("dry-run metadata"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateFreshOutputNamespace(outDir); err != nil {
		t.Fatalf("validateFreshOutputNamespace dry-run metadata error = %v", err)
	}
}

func TestEnsureDefaultAvatarCreatesValidPNGAndPreservesExistingFile(t *testing.T) {
	root := t.TempDir()
	cs2Exe := filepath.Join(root, "game", "bin", "win64", "cs2.exe")
	avatarPath := filepath.Join(root, "game", "csgo", "avatars", "default.png")

	if err := ensureDefaultAvatar(cs2Exe); err != nil {
		t.Fatalf("ensureDefaultAvatar() error = %v", err)
	}
	file, err := os.Open(avatarPath)
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(file)
	closeErr := file.Close()
	if err != nil {
		t.Fatalf("decode generated avatar: %v", err)
	}
	if closeErr != nil {
		t.Fatalf("close generated avatar: %v", closeErr)
	}
	if config.Width != 32 || config.Height != 32 {
		t.Fatalf("generated avatar dimensions = %dx%d, want 32x32", config.Width, config.Height)
	}

	existing := []byte("existing avatar")
	if err := os.WriteFile(avatarPath, existing, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureDefaultAvatar(cs2Exe); err != nil {
		t.Fatalf("ensureDefaultAvatar() with existing file error = %v", err)
	}
	got, err := os.ReadFile(avatarPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(existing) {
		t.Fatalf("existing avatar = %q, want preserved %q", got, existing)
	}
}

func TestPatchWindowedVideoSettings(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		want        string
		wantChanged bool
	}{
		{
			name: "forces fullscreen and borderless off",
			content: "\t\"setting.fullscreen\"\t\t\"1\"\n" +
				"\t\"setting.nowindowborder\"\t\t\"1\"\n" +
				"\t\"setting.defaultres\"\t\t\"1920\"\n",
			want: "\t\"setting.fullscreen\"\t\t\"0\"\n" +
				"\t\"setting.nowindowborder\"\t\t\"0\"\n" +
				"\t\"setting.defaultres\"\t\t\"1920\"\n",
			wantChanged: true,
		},
		{
			name: "already windowed is untouched",
			content: "\t\"setting.fullscreen\"\t\t\"0\"\n" +
				"\t\"setting.nowindowborder\"\t\t\"0\"\n",
			want: "\t\"setting.fullscreen\"\t\t\"0\"\n" +
				"\t\"setting.nowindowborder\"\t\t\"0\"\n",
			wantChanged: false,
		},
		{
			name:        "absent settings stay absent",
			content:     "\t\"setting.defaultres\"\t\t\"1920\"\n",
			want:        "\t\"setting.defaultres\"\t\t\"1920\"\n",
			wantChanged: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := patchWindowedVideoSettings(tt.content)
			if got != tt.want {
				t.Errorf("content = %q, want %q", got, tt.want)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
		})
	}
}

func TestForceWindowedVideoConfigPatchesAndRestores(t *testing.T) {
	steam := t.TempDir()
	cfgDir := filepath.Join(steam, "userdata", "50084006", "730", "local", "cfg")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "cs2_video.txt")
	original := "\t\"setting.fullscreen\"\t\t\"1\"\n\t\"setting.nowindowborder\"\t\t\"1\"\n"
	if err := os.WriteFile(cfgPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	cs2 := filepath.Join(steam, "steamapps", "common", "Counter-Strike Global Offensive", "game", "bin", "win64", "cs2.exe")

	restore := forceWindowedVideoConfig(cs2)
	patched, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(patched), `"setting.fullscreen"		"0"`) || !strings.Contains(string(patched), `"setting.nowindowborder"		"0"`) {
		t.Fatalf("patched config = %q, want fullscreen and borderless forced off", patched)
	}

	restore()
	restored, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != original {
		t.Fatalf("restored config = %q, want original %q", restored, original)
	}
}

func TestIsHookErrorWindowTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{"afxhooksource2 dialog", "Error - AfxHookSource2", true},
		{"afxhooksource dialog", "Error - AfxHookSource", true},
		{"afxhookgold dialog", "Error - AfxHookGold", true},
		{"game window", "Counter-Strike 2", false},
		{"empty", "", false},
		{"na placeholder", "N/A", false},
		{"errors plural prefix", "Errors - Afx", false},
		{"lowercase is case sensitive", "error - afxhooksource2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHookErrorWindowTitle(tt.title); got != tt.want {
				t.Errorf("isHookErrorWindowTitle(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

func TestParseTasklistVerboseCSV(t *testing.T) {
	tests := []struct {
		name        string
		out         string
		image       string
		wantRunning bool
		wantTitle   string
	}{
		{
			name:        "running with normal title",
			out:         `"cs2.exe","12345","Console","1","2,345,678 K","Running","DESKTOP\user","0:01:23","Counter-Strike 2"` + "\n",
			image:       "cs2.exe",
			wantRunning: true,
			wantTitle:   "Counter-Strike 2",
		},
		{
			name:        "running with hook-crash dialog title",
			out:         `"cs2.exe","12345","Console","1","2,345,678 K","Running","DESKTOP\user","0:01:23","Error - AfxHookSource2"` + "\n",
			image:       "cs2.exe",
			wantRunning: true,
			wantTitle:   "Error - AfxHookSource2",
		},
		{
			name:        "no matching tasks line",
			out:         "INFO: No tasks are running which match the specified criteria.\n",
			image:       "cs2.exe",
			wantRunning: false,
			wantTitle:   "",
		},
		{
			name:        "empty output",
			out:         "",
			image:       "cs2.exe",
			wantRunning: false,
			wantTitle:   "",
		},
		{
			name:        "case-insensitive image match",
			out:         `"CS2.EXE","12345","Console","1","2,345,678 K","Running","DESKTOP\user","0:01:23","Counter-Strike 2"` + "\n",
			image:       "cs2.exe",
			wantRunning: true,
			wantTitle:   "Counter-Strike 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRunning, gotTitle := parseTasklistVerboseCSV(tt.out, tt.image)
			if gotRunning != tt.wantRunning {
				t.Errorf("running = %v, want %v", gotRunning, tt.wantRunning)
			}
			if gotTitle != tt.wantTitle {
				t.Errorf("title = %q, want %q", gotTitle, tt.wantTitle)
			}
		})
	}
}

func TestWaitForWindowsProcessRunAndExitStopsCS2OnHookError(t *testing.T) {
	var stopped string
	status := func(image string) (bool, string, error) {
		return true, "Error - AfxHookSource2", nil
	}
	terminate := func(image string) error {
		stopped = image
		return nil
	}

	err := waitForWindowsProcessRunAndExitWith(
		context.Background(),
		"cs2.exe",
		time.Second,
		time.Millisecond,
		time.Hour,
		nil,
		status,
		terminate,
	)
	var hookErr *hookIncompatibleError
	if !errors.As(err, &hookErr) {
		t.Fatalf("error = %v, want hookIncompatibleError", err)
	}
	if stopped != "cs2.exe" {
		t.Fatalf("terminated image = %q, want cs2.exe", stopped)
	}
}

func TestWaitForWindowsProcessRunAndExitReportsCleanupFailure(t *testing.T) {
	wantCleanupErr := errors.New("access denied")
	status := func(image string) (bool, string, error) {
		return true, "Error - AfxHookSource2", nil
	}
	terminate := func(image string) error {
		return wantCleanupErr
	}

	err := waitForWindowsProcessRunAndExitWith(
		context.Background(),
		"cs2.exe",
		time.Second,
		time.Millisecond,
		time.Hour,
		nil,
		status,
		terminate,
	)
	var hookErr *hookIncompatibleError
	if !errors.As(err, &hookErr) {
		t.Fatalf("error = %v, want hookIncompatibleError", err)
	}
	for _, want := range []string{"stop cs2.exe", "access denied"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q", err, want)
		}
	}
}

func TestWaitForWindowsProcessRunAndExitStopsRunningProcessOnStatusFailure(t *testing.T) {
	wantErr := errors.New("demo playback failed")
	var stopped string
	status := func(image string) (bool, string, error) {
		return true, "Counter-Strike 2", wantErr
	}
	terminate := func(image string) error {
		stopped = image
		return nil
	}

	err := waitForWindowsProcessRunAndExitWith(
		context.Background(),
		"cs2.exe",
		time.Second,
		time.Millisecond,
		time.Hour,
		nil,
		status,
		terminate,
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if stopped != "cs2.exe" {
		t.Fatalf("terminated image = %q, want cs2.exe", stopped)
	}
}

func TestWaitForWindowsProcessRunAndExitCancellationDoesNotStopUnobservedProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stopped string
	err := waitForWindowsProcessRunAndExitWith(
		ctx,
		"cs2.exe",
		time.Hour,
		time.Hour,
		time.Hour,
		nil,
		func(string) (bool, string, error) { return false, "", nil },
		func(image string) error {
			stopped = image
			return nil
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if stopped != "" {
		t.Fatalf("terminated image = %q, want no unobserved process termination", stopped)
	}
}

func TestWaitForWindowsProcessRunAndExitCancellationDoesNotClaimProcessFoundByFinalLookup(t *testing.T) {
	statusErr := errors.New("tasklist failed after finding cs2.exe")
	for _, tt := range []struct {
		name      string
		statusErr error
	}{
		{name: "successful lookup"},
		{name: "failed lookup", statusErr: statusErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			var stopped string

			err := waitForWindowsProcessRunAndExitWith(
				ctx,
				"cs2.exe",
				time.Hour,
				time.Hour,
				time.Hour,
				nil,
				func(string) (bool, string, error) {
					return true, "Counter-Strike 2", tt.statusErr
				},
				func(image string) error {
					stopped = image
					return nil
				},
			)

			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
			if tt.statusErr != nil && !errors.Is(err, tt.statusErr) {
				t.Fatalf("error = %v, want status error %v", err, tt.statusErr)
			}
			if stopped != "" {
				t.Fatalf("terminated image = %q, want no process termination without prior ownership evidence", stopped)
			}
		})
	}
}

func TestWaitForWindowsProcessRunAndExitCancellationStopsObservedProcessWhenStatusFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	observed := make(chan struct{})
	statusCalls := 0
	statusErr := errors.New("tasklist unavailable")
	var stopped string
	status := func(string) (bool, string, error) {
		statusCalls++
		if statusCalls == 1 {
			close(observed)
			return true, "Counter-Strike 2", nil
		}
		return false, "", statusErr
	}
	go func() {
		<-observed
		cancel()
	}()

	err := waitForWindowsProcessRunAndExitWith(
		ctx,
		"cs2.exe",
		time.Hour,
		time.Millisecond,
		time.Hour,
		nil,
		status,
		func(image string) error {
			stopped = image
			return nil
		},
	)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, statusErr) {
		t.Fatalf("error = %v, want cancellation and status error", err)
	}
	if stopped != "cs2.exe" {
		t.Fatalf("terminated image = %q, want cs2.exe", stopped)
	}
}

func TestLauncherFailurePreservesCauseWithoutAssumingCS2Ownership(t *testing.T) {
	waitErr := errors.New("launcher exit 1")
	err := launcherFailure(waitErr)
	if !errors.Is(err, waitErr) {
		t.Fatalf("error = %v, want launcher cause preserved", err)
	}
	if !strings.Contains(err.Error(), "HLAE launcher failed") {
		t.Fatalf("error = %q, want launcher failure context", err)
	}
}

func TestWaitForWindowsProcessRunAndExitStopsOwnedProcessOnDemoParseFailure(t *testing.T) {
	var stopped string
	status := func(image string) (bool, string, error) {
		return false, "", &demoParseError{path: `C:\game\csgo\console.log`}
	}
	terminate := func(image string) error {
		stopped = image
		return nil
	}

	err := waitForWindowsProcessRunAndExitWith(
		context.Background(),
		"cs2.exe",
		time.Second,
		time.Millisecond,
		time.Hour,
		nil,
		status,
		terminate,
	)
	var parseErr *demoParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error = %v, want demoParseError", err)
	}
	if stopped != "cs2.exe" {
		t.Fatalf("terminated image = %q, want cs2.exe", stopped)
	}
}

func TestWaitForWindowsProcessRunAndExitChecksDemoParseFailureAtFirstDeadline(t *testing.T) {
	var stopped string
	status := func(image string) (bool, string, error) {
		return false, "", &demoParseError{path: `C:\game\csgo\console.log`}
	}
	terminate := func(image string) error {
		stopped = image
		return nil
	}

	err := waitForWindowsProcessRunAndExitWith(
		context.Background(),
		"cs2.exe",
		time.Millisecond,
		time.Hour,
		time.Hour,
		nil,
		status,
		terminate,
	)
	var parseErr *demoParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error = %v, want demoParseError", err)
	}
	if stopped != "cs2.exe" {
		t.Fatalf("terminated image = %q, want cs2.exe", stopped)
	}
}

func TestWaitForWindowsProcessRunAndExitDoesNotStopUnobservedProcessOnGenericStatusFailure(t *testing.T) {
	wantErr := errors.New("tasklist unavailable")
	var stopped string
	err := waitForWindowsProcessRunAndExitWith(
		context.Background(),
		"cs2.exe",
		time.Second,
		time.Millisecond,
		time.Hour,
		nil,
		func(string) (bool, string, error) { return false, "", wantErr },
		func(image string) error {
			stopped = image
			return nil
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if stopped != "" {
		t.Fatalf("terminated image = %q, want no unowned process termination", stopped)
	}
}

func TestWaitForWindowsProcessRunAndExitClosesCS2AfterVerifiedCapture(t *testing.T) {
	// Once the capture-verified attestation marker proves this launch owns
	// cs2.exe and the frames are captured, the close must not depend on the
	// in-engine soft-quit succeeding: a process that outlives the grace window
	// is force-closed deterministically, while a clean in-engine quit within the
	// window needs no force-close.
	const (
		firstWait = time.Hour             // process is already running; first-appearance deadline never fires
		poll      = 20 * time.Millisecond // poll >> hung grace so only the grace branch is ready when it fires
		hungGrace = 5 * time.Millisecond  // shorter than one poll: fires cleanly between ticks
	)
	verifiedNow := func() func() bool { return func() bool { return true } }
	runningForever := func() func(string) (bool, string, error) {
		return func(string) (bool, string, error) { return true, "Counter-Strike 2", nil }
	}

	tests := []struct {
		name          string
		closeGrace    time.Duration
		newVerified   func() func() bool
		newStatus     func() func(string) (bool, string, error)
		terminateErr  error
		wantTerminate bool
		wantErr       string // "" means the wait returns nil
	}{
		{
			name:          "hung capture is force-closed after the grace window",
			closeGrace:    hungGrace,
			newVerified:   verifiedNow,
			newStatus:     runningForever,
			wantTerminate: true,
		},
		{
			name:          "force-close failure is surfaced with capture-ownership context",
			closeGrace:    hungGrace,
			newVerified:   verifiedNow,
			newStatus:     runningForever,
			terminateErr:  errors.New("access denied"),
			wantTerminate: true,
			wantErr:       "force-close cs2.exe after verified capture",
		},
		{
			name:        "clean in-engine quit within grace needs no force-close",
			closeGrace:  time.Hour, // never fires: the process exits within a few polls
			newVerified: verifiedNow,
			newStatus: func() func(string) (bool, string, error) {
				calls := 0
				return func(string) (bool, string, error) {
					calls++
					if calls >= 3 {
						return false, "", nil
					}
					return true, "Counter-Strike 2", nil
				}
			},
			wantTerminate: false,
		},
		{
			name:       "grace arms only after verification is observed",
			closeGrace: hungGrace,
			newVerified: func() func() bool {
				calls := 0
				return func() bool {
					calls++
					return calls > 3
				}
			},
			newStatus:     runningForever,
			wantTerminate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var terminated string
			err := waitForWindowsProcessRunAndExitWith(
				context.Background(),
				"cs2.exe",
				firstWait,
				poll,
				tt.closeGrace,
				tt.newVerified(),
				tt.newStatus(),
				func(image string) error {
					terminated = image
					return tt.terminateErr
				},
			)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("error = %v, want nil", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
			if gotTerminate := terminated == "cs2.exe"; gotTerminate != tt.wantTerminate {
				t.Fatalf("force-closed = %v, want %v", gotTerminate, tt.wantTerminate)
			}
		})
	}
}

func TestWaitForWindowsProcessRunAndExitDoesNotForceCloseUnverifiedProcess(t *testing.T) {
	// The grace gate must arm only after the capture-verified marker. Without
	// verification a still-running cs2.exe must never be treated as a completed
	// capture and force-closed: the wait runs until cancellation instead. A
	// grace so short it would fire at once if it ever armed makes the gating
	// explicit — the wait must still return context.Canceled, not the nil of a
	// clean grace-driven close.
	ctx, cancel := context.WithCancel(context.Background())
	var terminated string
	calls := 0
	status := func(string) (bool, string, error) {
		calls++
		if calls >= 5 {
			cancel()
		}
		return true, "Counter-Strike 2", nil
	}

	err := waitForWindowsProcessRunAndExitWith(
		ctx,
		"cs2.exe",
		time.Hour,
		time.Millisecond,
		time.Nanosecond, // would fire immediately if the gate ignored verification
		func() bool { return false },
		status,
		func(image string) error {
			terminated = image
			return nil
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled (grace must not complete an unverified wait)", err)
	}
	// The observed process is still stopped, but via the cancellation path, not
	// the grace gate — which is exactly why the error is context.Canceled.
	if terminated != "cs2.exe" {
		t.Fatalf("terminated image = %q, want cs2.exe via the cancellation path", terminated)
	}
}

func TestDemoParseErrorMessage(t *testing.T) {
	path := `C:\game\csgo\console.log`
	err := &demoParseError{path: path}
	for _, want := range []string{
		demoParseFailureMarker,
		"demo incompatible with current cs2 build",
		"older game version",
		strconv.Quote(path),
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Error() = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestCS2ConsoleLogMonitorReturnsDemoParseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "console.log")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	monitor := newCS2ConsoleLogMonitor(path, testCaptureAttestation())
	appendConsoleLog(t, path, "disconnect: "+demoParseFailureMarker+"\n")

	err := monitor.failure()
	var parseErr *demoParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("failure() error = %v, want *demoParseError", err)
	}
}

func TestCS2ConsoleLogMonitorAttestsCompletedPOVVerification(t *testing.T) {
	path := filepath.Join(t.TempDir(), "console.log")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	monitor := newCS2ConsoleLogMonitor(path, testCaptureAttestation())
	appendConsoleLog(t, path, "echo "+recording.CaptureVerifiedAttestation(testCaptureAttestation())+"\n")
	if err := monitor.failure(); err != nil {
		t.Fatalf("failure() error = %v", err)
	}
	if err := monitor.requireCaptureVerified(); err != nil {
		t.Fatalf("requireCaptureVerified() error = %v", err)
	}
}

func TestCS2ConsoleLogMonitorClassifiesUnplayableStart(t *testing.T) {
	cases := []struct {
		name       string
		console    string
		verified   bool
		wantUnplay bool
	}{
		{
			name:       "armed zero no seek breakpad",
			console:    "[zackvideo] armed at tick 0\nResetBreakpadAppId: Universe is 1\n",
			wantUnplay: true,
		},
		{
			name:    "seek landed is not unplayable",
			console: "[zackvideo] armed at tick 0\n[zackvideo] seek-landed -> 2863 (at 2863)\nResetBreakpadAppId: Universe is 1\n",
		},
		{
			name:    "timeout without breakpad stays generic",
			console: "[zackvideo] armed at tick 0\n",
		},
		{
			name:     "verified wins even with breakpad",
			console:  "[zackvideo] armed at tick 0\nResetBreakpadAppId: Universe is 1\n",
			verified: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "console.log")
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			monitor := newCS2ConsoleLogMonitor(path, testCaptureAttestation())
			body := tc.console
			if tc.verified {
				body += recording.CaptureVerifiedAttestation(testCaptureAttestation()) + "\n"
			}
			appendConsoleLog(t, path, body)
			if err := monitor.failure(); err != nil {
				t.Fatalf("failure() = %v", err)
			}
			err := monitor.requireCaptureVerified()
			if tc.verified {
				if err != nil {
					t.Fatalf("requireCaptureVerified() = %v, want nil", err)
				}
				return
			}
			var startErr *unplayableStartError
			if tc.wantUnplay {
				if !errors.As(err, &startErr) {
					t.Fatalf("requireCaptureVerified() = %v, want *unplayableStartError", err)
				}
				if !strings.Contains(err.Error(), "unplayable_start:") {
					t.Fatalf("Error() = %q, want prefix unplayable_start:", err.Error())
				}
				return
			}
			if errors.As(err, &startErr) {
				t.Fatalf("requireCaptureVerified() = %v, want generic capture error", err)
			}
			var verifyErr *captureVerificationError
			if !errors.As(err, &verifyErr) {
				t.Fatalf("requireCaptureVerified() = %v, want *captureVerificationError", err)
			}
		})
	}
}

func TestCS2ConsoleLogMonitorRejectsMarkerFromAnotherRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "console.log")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	monitor := newCS2ConsoleLogMonitor(path, testCaptureAttestation())
	appendConsoleLog(t, path, recording.CaptureVerifiedAttestation(strings.Repeat("other-run-", 4))+"\n")
	if err := monitor.failure(); err != nil {
		t.Fatalf("unrelated marker caused failure: %v", err)
	}
	if err := monitor.requireCaptureVerified(); err == nil {
		t.Fatal("marker from another run attested this capture")
	}
}

func TestNewCaptureAttestationTokenIsRandomAndStrong(t *testing.T) {
	first, err := newCaptureAttestationToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newCaptureAttestationToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 || len(second) != 64 {
		t.Fatalf("token lengths = %d, %d, want 64 hex characters", len(first), len(second))
	}
	if first == second {
		t.Fatal("two capture attestation tokens were identical")
	}
}

func TestCS2ConsoleLogMonitorRejectsFailedOrMissingPOVVerification(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content string
		poll    bool
	}{
		{name: "runtime failure marker", content: recording.CaptureFailedMarker, poll: true},
		{name: "missing completion marker"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "console.log")
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			monitor := newCS2ConsoleLogMonitor(path, testCaptureAttestation())
			if tt.content != "" {
				appendConsoleLog(t, path, recording.CaptureFailedAttestation(testCaptureAttestation())+"\n")
			}
			var err error
			if tt.poll {
				err = monitor.failure()
			} else {
				err = monitor.requireCaptureVerified()
			}
			var verificationErr *captureVerificationError
			if !errors.As(err, &verificationErr) {
				t.Fatalf("error = %v, want *captureVerificationError", err)
			}
		})
	}
}

func TestPrepareCS2ConsoleLogClearsHistoricalOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "console.log")
	if err := os.WriteFile(path, []byte("old "+demoParseFailureMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepareCS2ConsoleLog(path); err != nil {
		t.Fatalf("prepareCS2ConsoleLog error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) != 0 {
		t.Fatalf("console log = %q, want empty", content)
	}
}

func TestCS2ConsoleLogMonitorDetectsNewSplitDemoParseFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "console.log")
	if err := os.WriteFile(path, []byte("old "+demoParseFailureMarker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	monitor := newCS2ConsoleLogMonitor(path, testCaptureAttestation())
	if err := monitor.failure(); err != nil {
		t.Fatalf("failure() detected historical log content: %v", err)
	}

	split := len(demoParseFailureMarker) / 2
	appendConsoleLog(t, path, "new "+demoParseFailureMarker[:split])
	if err := monitor.failure(); err != nil {
		t.Fatalf("failure() detected an incomplete marker: %v", err)
	}
	appendConsoleLog(t, path, demoParseFailureMarker[split:]+"\n")
	err := monitor.failure()
	if err == nil {
		t.Fatal("failure() error = nil, want demo parse failure")
	}
	for _, want := range []string{demoParseFailureMarker, strconv.Quote(path)} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("failure() error = %q, want %q", err, want)
		}
	}
}

func TestCS2ConsoleLogMonitorHandlesStartupTruncation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "console.log")
	oldContent := strings.Repeat("old output\n", 100) + demoParseFailureMarker
	if err := os.WriteFile(path, []byte(oldContent), 0o600); err != nil {
		t.Fatal(err)
	}
	monitor := newCS2ConsoleLogMonitor(path, testCaptureAttestation())
	if err := os.WriteFile(path, []byte("startup\n"+demoParseFailureMarker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := monitor.failure(); err == nil {
		t.Fatal("failure() error = nil after log truncation, want demo parse failure")
	}
}

func appendConsoleLog(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCaptureResultIncludesConsoleLog(t *testing.T) {
	cs2 := filepath.FromSlash("C:/Steam/game/bin/win64/cs2.exe")
	killPlan := killplan.NewPlan()
	killPlan.Demo.SHA256 = strings.Repeat("a", 64)
	killPlan.Demo.Tickrate = 64
	killPlan.Demo.DurationTicks = 1000
	killPlan.Target.SteamID64 = "76561197960265729"
	killPlan.Segments = []killplan.Segment{{
		ID:        "seg-001",
		TickStart: 64,
		TickEnd:   128,
	}}
	plan, err := recording.NewPlanFromKillPlan(
		killPlan,
		"match.dem",
		"recording-output",
		recording.DefaultStreamConfig(),
	)
	if err != nil {
		t.Fatal(err)
	}
	result := recording.RecordingResult{
		Plan:            plan,
		CaptureMode:     recording.CaptureModeReal,
		CaptureVerified: true,
	}
	result.CaptureInputFingerprint, err = recording.CaptureInputFingerprint(result.Plan)
	if err != nil {
		t.Fatal(err)
	}
	err = validateCaptureResult(result, cs2)
	if err == nil {
		t.Fatal("validateCaptureResult() error = nil, want missing clips error")
	}
	for _, want := range []string{"recording result has no segment clips", strconv.Quote(filepath.FromSlash("C:/Steam/game/csgo/console.log"))} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("validateCaptureResult() error = %q, want %q", err, want)
		}
	}
}

func TestSteamRootFromCS2Path(t *testing.T) {
	cs2 := filepath.FromSlash("D:/SteamLibrary/steamapps/common/Counter-Strike Global Offensive/game/bin/win64/cs2.exe")
	if got, want := steamRootFromCS2Path(cs2), filepath.FromSlash("D:/SteamLibrary"); got != want {
		t.Errorf("steamRootFromCS2Path = %q, want %q", got, want)
	}
	if got := steamRootFromCS2Path(filepath.FromSlash("C:/tools/cs2.exe")); got != "" {
		t.Errorf("steamRootFromCS2Path outside steamapps = %q, want empty", got)
	}
}

// fakeWindowTitlePollerClock lets tests advance a windowTitlePoller's notion
// of "now" deterministically instead of sleeping, so the 5s throttling
// cadence is verifiable in milliseconds.
type fakeWindowTitlePollerClock struct {
	current time.Time
}

func (c *fakeWindowTitlePollerClock) now() time.Time { return c.current }

func (c *fakeWindowTitlePollerClock) advance(d time.Duration) { c.current = c.current.Add(d) }

func TestWindowTitlePollerThrottlesExpensiveTitleLookup(t *testing.T) {
	clock := &fakeWindowTitlePollerClock{current: time.Unix(0, 0)}
	var titleCalls, cheapCalls int
	poller := newWindowTitlePoller(
		5*time.Second,
		clock.now,
		func(string) (bool, string, error) {
			titleCalls++
			return true, "cs2", nil
		},
		func(string) (bool, error) {
			cheapCalls++
			return true, nil
		},
	)

	steps := []struct {
		name           string
		advance        time.Duration
		wantTitleCalls int
		wantCheapCalls int
	}{
		{name: "first call always does the slow lookup", advance: 0, wantTitleCalls: 1, wantCheapCalls: 0},
		{name: "next tick well inside the interval stays cheap", advance: 500 * time.Millisecond, wantTitleCalls: 1, wantCheapCalls: 1},
		{name: "still inside the interval stays cheap", advance: 4 * time.Second, wantTitleCalls: 1, wantCheapCalls: 2},
		{name: "crossing the interval re-does the slow lookup", advance: 600 * time.Millisecond, wantTitleCalls: 2, wantCheapCalls: 2},
		{name: "interval resets after the slow lookup", advance: 500 * time.Millisecond, wantTitleCalls: 2, wantCheapCalls: 3},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			clock.advance(step.advance)
			if _, _, err := poller.status("cs2.exe"); err != nil {
				t.Fatalf("status() error = %v", err)
			}
			if titleCalls != step.wantTitleCalls {
				t.Errorf("titleCalls = %d, want %d", titleCalls, step.wantTitleCalls)
			}
			if cheapCalls != step.wantCheapCalls {
				t.Errorf("cheapCalls = %d, want %d", cheapCalls, step.wantCheapCalls)
			}
		})
	}
}

func TestWindowTitlePollerDetectsHookErrorWithinTitleInterval(t *testing.T) {
	clock := &fakeWindowTitlePollerClock{current: time.Unix(0, 0)}
	const titleInterval = 5 * time.Second
	poller := newWindowTitlePoller(
		titleInterval,
		clock.now,
		func(string) (bool, string, error) { return true, "Error - AfxHookSource2", nil },
		func(string) (bool, error) { return true, nil },
	)

	// Simulate the recorder's 500ms poll ticks: the cheap ticks in between
	// report no title, but the next slow tick at or after titleInterval must
	// still surface the hook-error dialog title well inside the accepted
	// <10s detection budget.
	elapsed := time.Duration(0)
	var lastTitle string
	for elapsed <= 2*titleInterval {
		_, title, err := poller.status("cs2.exe")
		if err != nil {
			t.Fatalf("status() error = %v", err)
		}
		if title != "" {
			lastTitle = title
			break
		}
		clock.advance(500 * time.Millisecond)
		elapsed += 500 * time.Millisecond
	}
	if !isHookErrorWindowTitle(lastTitle) {
		t.Fatalf("hook error title not detected within %s, got %q at elapsed=%s", 2*titleInterval, lastTitle, elapsed)
	}
	if elapsed > titleInterval {
		t.Fatalf("hook error detected after %s, want within titleInterval %s", elapsed, titleInterval)
	}
}

func TestWindowTitlePollerPropagatesTitleLookupError(t *testing.T) {
	wantErr := errors.New("tasklist unavailable")
	poller := newWindowTitlePoller(
		5*time.Second,
		func() time.Time { return time.Unix(0, 0) },
		func(string) (bool, string, error) { return false, "", wantErr },
		func(string) (bool, error) {
			t.Fatal("cheap status should not run when the slow lookup fails")
			return false, nil
		},
	)
	if _, _, err := poller.status("cs2.exe"); !errors.Is(err, wantErr) {
		t.Fatalf("status() error = %v, want %v", err, wantErr)
	}
}

func TestWindowTitlePollerDefaultsToRealStatusFunctions(t *testing.T) {
	poller := newWindowTitlePoller(5*time.Second, nil, nil, nil)
	if poller.now == nil {
		t.Fatal("now defaults to a non-nil clock")
	}
	if poller.titleStatus == nil || poller.cheapStatus == nil {
		t.Fatal("titleStatus and cheapStatus default to non-nil implementations")
	}
}
