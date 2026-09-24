package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/rules"
)

// observedRecorderError is the real multi-line recorder failure seen when CS2
// cannot replay a demo recorded on an older build.
const observedRecorderError = `C:\...\zv-recorder.exe failed: exit status 1: 2026/07/19 13:44:07 windowed capture: patched c:\program files (x86)\steam\userdata\50084006\730\local\cfg\cs2_video.txt (fullscreen/borderless off for this run)
error: cs2 demo playback failed with NETWORK_DISCONNECT_MESSAGE_PARSE_ERROR; check console log "..."`

func TestRecordFailureReason(t *testing.T) {
	segment := recording.RecordingArtifact{SegmentID: "seg-001", Role: "segment", Type: "video", Path: "segments/seg-001.mp4"}
	requested16 := make([]string, 16)
	for i := range requested16 {
		requested16[i] = "seg"
	}

	cases := []struct {
		name      string
		err       error
		result    recording.RecordingResult
		requested []string
		want      string
	}{
		{
			name:      "incompatible demo with captured segments",
			err:       errors.New(observedRecorderError),
			result:    recording.RecordingResult{Artifacts: []recording.RecordingArtifact{segment}},
			requested: requested16,
			want:      "demo_incompatible: cs2 cannot replay this demo (it was recorded on an older cs2 build); captured 1/16 segments before the failure",
		},
		{
			name:      "incompatible demo with no captured segments",
			err:       errors.New(observedRecorderError),
			result:    recording.RecordingResult{},
			requested: requested16,
			want:      "demo_incompatible: cs2 cannot replay this demo (it was recorded on an older cs2 build)",
		},
		{
			name:      "unplayable start prefix wins over last error line",
			err:       errors.New("zv-recorder.exe failed: exit status 8: error: unplayable_start: CS2 crashed rewinding playdemo to tick 0; check CS2 console log \"c:\\\\game\\\\csgo\\\\console.log\""),
			requested: []string{"seg-001"},
			want:      "unplayable_start: CS2 crashed rewinding playdemo to tick 0",
		},
		{
			name:      "playback-ended demo is incompatible and not retryable",
			err:       errors.New("zv-recorder.exe failed: exit status 1: error: capture POV verification failed: demo playback ended before every protected segment completed; check CS2 console log \"c:\\\\game\\\\csgo\\\\console.log\""),
			requested: []string{"seg-001"},
			want:      "demo_incompatible: cs2 cannot replay this demo to the end (playback stops before every protected segment completes)",
		},
		{
			name:      "generic failure uses last error line",
			err:       errors.New("zv-recorder.exe failed: exit status 1: some noise\nerror: first problem\nmore noise\nerror: hlae launch failed"),
			requested: []string{"seg-001"},
			want:      "recorder failed: hlae launch failed",
		},
		{
			name:      "no marker and no error line passes through unchanged",
			err:       errors.New("zv-recorder.exe failed: exit status 1: unexpected log noise only"),
			requested: []string{"seg-001"},
			want:      "zv-recorder.exe failed: exit status 1: unexpected log noise only",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := recordFailureReason(tc.err, tc.result, tc.requested); got != tc.want {
				t.Fatalf("recordFailureReason() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewRecordFailureUnwrapsOriginal(t *testing.T) {
	orig := errors.New(observedRecorderError)
	failure := newRecordFailure(orig, recording.RecordingResult{}, []string{"seg-001"})

	if got := failure.Error(); !strings.HasPrefix(got, demoIncompatiblePrefix) {
		t.Fatalf("Error() = %q, want prefix %q", got, demoIncompatiblePrefix)
	}
	if !errors.Is(failure, orig) {
		t.Fatalf("errors.Is(failure, orig) = false, want the original error reachable via Unwrap")
	}
}

func TestRecordWorkerFailsWithConciseIncompatibleReason(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{
		ID:       id,
		Status:   job.StatusParsed,
		DemoPath: "demos/test.dem",
		Rules:    rules.Default(),
		KillPlan: &plan,
	}
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))

	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outDir := argValue(args, "--out")
		scriptPath := filepath.Join(outDir, "recording.js")
		segmentPath := filepath.Join(outDir, "segments", "seg-001.mp4")
		if err := os.MkdirAll(filepath.Dir(segmentPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(scriptPath, []byte("script"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(segmentPath, []byte("clip"), 0o644); err != nil {
			t.Fatal(err)
		}
		result := recordingResultWithSegment(scriptPath, segmentPath)
		if err := writeJSONFile(filepath.Join(outDir, "recording-result.json"), result); err != nil {
			t.Fatal(err)
		}
		return []byte(observedRecorderError), errors.New(observedRecorderError)
	}}
	w := NewRecordWorker(repo, store, RecordWorkerConfig{
		WorkDir:      t.TempDir(),
		RecorderPath: "zv-recorder",
		HLAEPath:     "HLAE.exe",
		CS2Path:      "cs2.exe",
	})
	w.runner = runner

	err := w.HandleRecordDemo(context.Background(), recordTask(t, id))
	if err == nil {
		t.Fatal("HandleRecordDemo error = nil, want failure")
	}

	got := repo.jobs[id]
	if got.Status != job.StatusFailed {
		t.Fatalf("Status = %s, want failed", got.Status)
	}
	if !strings.HasPrefix(got.FailureReason, demoIncompatiblePrefix) {
		t.Fatalf("FailureReason = %q, want prefix %q", got.FailureReason, demoIncompatiblePrefix)
	}
}

func TestRetryableCaptureCrash(t *testing.T) {
	tests := []struct {
		name   string
		runErr error
		result recording.RecordingResult
		want   bool
	}{
		{
			name:   "native CS2 exit is transient",
			runErr: errors.New("recorder failed: capture POV verification failed: " + missingCaptureAttestationMarker),
			want:   true,
		},
		{
			name:   "structured result carries native exit",
			runErr: errors.New("exit status 1"),
			result: recording.RecordingResult{Error: missingCaptureAttestationMarker},
			want:   true,
		},
		{
			name:   "observer drift is deterministic",
			runErr: errors.New("observer target drifted during seg-001"),
		},
		{
			name:   "incompatible demo is deterministic",
			runErr: errors.New(networkDisconnectMarker),
		},
		{
			name:   "unplayable start is deterministic",
			runErr: errors.New(unplayableStartPrefix + " " + missingCaptureAttestationMarker),
		},
		{
			name: "success is not retried",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryableCaptureCrash(tt.runErr, tt.result); got != tt.want {
				t.Fatalf("retryableCaptureCrash() = %v, want %v", got, tt.want)
			}
		})
	}
}

// consoleTailTrace is the cs2.console_tail line zv-recorder logs after a
// capture failure: plain JSON carrying raw CS2 console text.
func consoleTailTrace(t *testing.T, console string) string {
	t.Helper()
	encoded, err := json.Marshal(obs.TraceEntry{
		Time:    time.Date(2026, 9, 24, 8, 0, 3, 0, time.UTC),
		Event:   "cs2.console_tail",
		Level:   "warn",
		Message: fmt.Sprintf("lines=%d\n%s", strings.Count(console, "\n")+1, console),
	})
	if err != nil {
		t.Fatal(err)
	}
	return "2026/09/24 10:00:03 " + obs.TracePrefix + string(encoded)
}

func TestRelayedConsoleTailDoesNotChangeRecorderClassification(t *testing.T) {
	trace := consoleTailTrace(t, strings.Join([]string{
		"09/24 10:00:01 [Networking] Disconnect reason: " + networkDisconnectMarker,
		"09/24 10:00:02 [zackvideo] capture_failed: " + playbackEndedMarker,
		"09/24 10:00:02 " + unplayableStartPrefix + " probe",
		"09/24 10:00:02 " + resetBreakpadMarker,
		"09/24 10:00:03 " + missingCaptureAttestationMarker,
	}, "\n"))
	for _, marker := range []string{networkDisconnectMarker, playbackEndedMarker, unplayableStartPrefix, resetBreakpadMarker, missingCaptureAttestationMarker} {
		if !strings.Contains(trace, marker) {
			t.Fatalf("test premise: the relayed trace does not carry %q: %s", marker, trace)
		}
	}
	consoleLog := `"C:\\Users\\player\\AppData\\Local\\ClipHub\\work\\console.log"`
	for _, tc := range []struct {
		name       string
		failure    string
		wantReason string
		wantRetry  bool
	}{
		{
			name:       "observer drift",
			failure:    "error: capture POV verification failed: observer target drifted from 7656 to 7657; check CS2 console log " + consoleLog,
			wantReason: "recorder failed: capture POV verification failed: observer target drifted from 7656 to 7657; check CS2 console log " + consoleLog,
		},
		{
			name:       "transient CS2 exit",
			failure:    "error: failure_code=capture_pov_unverified substage=capture; capture POV verification failed: " + missingCaptureAttestationMarker + "; check CS2 console log " + consoleLog,
			wantReason: "recorder failed: capture POV verification failed: " + missingCaptureAttestationMarker + "; check CS2 console log " + consoleLog,
			wantRetry:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, output := range []string{
				"2026/09/24 10:00:00 windowed capture: patched cs2_video.txt\n" + tc.failure + "\n",
				"2026/09/24 10:00:00 windowed capture: patched cs2_video.txt\n" + trace + "\n" + tc.failure + "\n",
			} {
				runErr := newCommandError("zv-recorder.exe", errors.New("exit status 1"), output, output)
				if got := recordFailureReason(runErr, recording.RecordingResult{}, nil); got != tc.wantReason {
					t.Fatalf("recordFailureReason() = %q\nwant                   %q", got, tc.wantReason)
				}
				if got := retryableCaptureCrash(runErr, recording.RecordingResult{}); got != tc.wantRetry {
					t.Fatalf("retryableCaptureCrash() = %v, want %v", got, tc.wantRetry)
				}
			}
		})
	}
}
