package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runningCS2(string) (bool, string, error) { return true, "Counter-Strike 2", nil }

// captureLog sends the standard logger to a buffer for the rest of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previous)
		log.SetFlags(flags)
	})
	return &buf
}

func TestWaitForWindowsProcessRunAndExitStopsCS2OnHookError(t *testing.T) {
	var stopped string
	err := waitForWindowsProcessRunAndExitWith(context.Background(), captureWait{
		image:        "cs2.exe",
		firstWait:    time.Second,
		pollInterval: time.Millisecond,
		closeGrace:   time.Hour,
		status:       func(string) (bool, string, error) { return true, "Error - AfxHookSource2", nil },
		terminate: func(image string) error {
			stopped = image
			return nil
		},
	})
	var hookErr *hookIncompatibleError
	if !errors.As(err, &hookErr) {
		t.Fatalf("error = %v, want hookIncompatibleError", err)
	}
	if stopped != "cs2.exe" {
		t.Fatalf("terminated image = %q, want cs2.exe", stopped)
	}
}

func TestWaitForWindowsProcessRunAndExitStopsCS2OnLauncherError(t *testing.T) {
	tests := []struct {
		name, text, wantPrefix string
	}{
		{"steamworks", "FATAL ERROR: Failed to initialize the Steamworks SDK. ConnectToGlobalUser failed.", "steam_not_running: "},
		{"other launcher failure", "Failed to load the launcher DLL", "cs2_launcher_error: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := readDialogText
			readDialogText = func(string, string) string { return tt.text }
			t.Cleanup(func() { readDialogText = previous })
			var stopped string
			err := waitForWindowsProcessRunAndExitWith(context.Background(), captureWait{
				image:        "cs2.exe",
				firstWait:    time.Second,
				pollInterval: time.Millisecond,
				closeGrace:   time.Hour,
				status:       func(string) (bool, string, error) { return true, launcherErrorWindowTitle, nil },
				terminate: func(image string) error {
					stopped = image
					return nil
				},
			})
			if err == nil || !strings.HasPrefix(err.Error(), tt.wantPrefix) || !strings.Contains(err.Error(), tt.text) {
				t.Fatalf("error = %v, want %q with the dialog text", err, tt.wantPrefix)
			}
			if stopped != "cs2.exe" {
				t.Fatalf("terminated image = %q, want cs2.exe", stopped)
			}
		})
	}
}

func TestWaitForWindowsProcessRunAndExitReportsCleanupFailure(t *testing.T) {
	err := waitForWindowsProcessRunAndExitWith(context.Background(), captureWait{
		image:        "cs2.exe",
		firstWait:    time.Second,
		pollInterval: time.Millisecond,
		closeGrace:   time.Hour,
		status:       func(string) (bool, string, error) { return true, "Error - AfxHookSource2", nil },
		terminate:    func(string) error { return errors.New("access denied") },
	})
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
	err := waitForWindowsProcessRunAndExitWith(context.Background(), captureWait{
		image:        "cs2.exe",
		firstWait:    time.Second,
		pollInterval: time.Millisecond,
		closeGrace:   time.Hour,
		status:       func(string) (bool, string, error) { return true, "Counter-Strike 2", wantErr },
		terminate: func(image string) error {
			stopped = image
			return nil
		},
	})
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
	err := waitForWindowsProcessRunAndExitWith(ctx, captureWait{
		image:        "cs2.exe",
		firstWait:    time.Hour,
		pollInterval: time.Hour,
		closeGrace:   time.Hour,
		status:       func(string) (bool, string, error) { return false, "", nil },
		terminate: func(image string) error {
			stopped = image
			return nil
		},
	})
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

			err := waitForWindowsProcessRunAndExitWith(ctx, captureWait{
				image:        "cs2.exe",
				firstWait:    time.Hour,
				pollInterval: time.Hour,
				closeGrace:   time.Hour,
				status:       func(string) (bool, string, error) { return true, "Counter-Strike 2", tt.statusErr },
				terminate: func(image string) error {
					stopped = image
					return nil
				},
			})

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

	err := waitForWindowsProcessRunAndExitWith(ctx, captureWait{
		image:        "cs2.exe",
		firstWait:    time.Hour,
		pollInterval: time.Millisecond,
		closeGrace:   time.Hour,
		status:       status,
		terminate: func(image string) error {
			stopped = image
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, statusErr) {
		t.Fatalf("error = %v, want cancellation and status error", err)
	}
	if stopped != "cs2.exe" {
		t.Fatalf("terminated image = %q, want cs2.exe", stopped)
	}
}

func TestWaitForWindowsProcessRunAndExitStopsOwnedProcessOnDemoParseFailure(t *testing.T) {
	tests := []struct {
		name      string
		firstWait time.Duration
		poll      time.Duration
	}{
		{name: "detected while polling", firstWait: time.Second, poll: time.Millisecond},
		{name: "detected at first deadline", firstWait: time.Millisecond, poll: time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stopped string
			err := waitForWindowsProcessRunAndExitWith(context.Background(), captureWait{
				image:        "cs2.exe",
				firstWait:    tt.firstWait,
				pollInterval: tt.poll,
				closeGrace:   time.Hour,
				status: func(string) (bool, string, error) {
					return false, "", &demoParseError{path: `C:\game\csgo\console.log`}
				},
				terminate: func(image string) error {
					stopped = image
					return nil
				},
			})
			var parseErr *demoParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("error = %v, want demoParseError", err)
			}
			if stopped != "cs2.exe" {
				t.Fatalf("terminated image = %q, want cs2.exe", stopped)
			}
		})
	}
}

func TestWaitForWindowsProcessRunAndExitDoesNotStopUnobservedProcessOnGenericStatusFailure(t *testing.T) {
	wantErr := errors.New("tasklist unavailable")
	var stopped string
	err := waitForWindowsProcessRunAndExitWith(context.Background(), captureWait{
		image:        "cs2.exe",
		firstWait:    time.Second,
		pollInterval: time.Millisecond,
		closeGrace:   time.Hour,
		status:       func(string) (bool, string, error) { return false, "", wantErr },
		terminate: func(image string) error {
			stopped = image
			return nil
		},
	})
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
	runningForever := func() func(string) (bool, string, error) { return runningCS2 }

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
			err := waitForWindowsProcessRunAndExitWith(context.Background(), captureWait{
				image:        "cs2.exe",
				firstWait:    firstWait,
				pollInterval: poll,
				closeGrace:   tt.closeGrace,
				verified:     tt.newVerified(),
				status:       tt.newStatus(),
				terminate: func(image string) error {
					terminated = image
					return tt.terminateErr
				},
			})
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
	// The grace gate must arm only after the capture-verified marker. A grace
	// so short it would fire at once if it ever armed makes the gating
	// explicit: the wait must still return context.Canceled.
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

	err := waitForWindowsProcessRunAndExitWith(ctx, captureWait{
		image:        "cs2.exe",
		firstWait:    time.Hour,
		pollInterval: time.Millisecond,
		closeGrace:   time.Nanosecond, // would fire immediately if the gate ignored verification
		verified:     func() bool { return false },
		status:       status,
		terminate: func(image string) error {
			terminated = image
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled (grace must not complete an unverified wait)", err)
	}
	// The observed process is stopped via the cancellation path, not the grace gate.
	if terminated != "cs2.exe" {
		t.Fatalf("terminated image = %q, want cs2.exe via the cancellation path", terminated)
	}
}

// quietStall is a watchdog whose capture never changes.
func quietStall(t *testing.T, limit time.Duration) *captureStallWatch {
	t.Helper()
	consoleLog := filepath.Join(t.TempDir(), "console.log")
	if err := os.WriteFile(consoleLog, []byte("Steam is probably not running.\nmap loaded\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &captureStallWatch{
		limit:      limit,
		every:      time.Millisecond,
		consoleLog: consoleLog,
		probe:      func() captureProgress { return captureProgress{consoleBytes: 1} },
		titles:     func() []string { return []string{"Counter-Strike 2"} },
	}
}

func TestWaitForWindowsProcessRunAndExitStopsACaptureWithoutProgress(t *testing.T) {
	logs := captureLog(t)
	var terminated string
	err := waitForWindowsProcessRunAndExitWith(context.Background(), captureWait{
		image:        "cs2.exe",
		firstWait:    time.Hour,
		pollInterval: time.Millisecond,
		closeGrace:   time.Hour,
		verified:     func() bool { return false },
		status:       runningCS2,
		terminate: func(image string) error {
			terminated = image
			return nil
		},
		stall: quietStall(t, 30*time.Millisecond),
	})
	var stallErr *captureStallError
	if !errors.As(err, &stallErr) || !strings.HasPrefix(err.Error(), "capture_stalled:") {
		t.Fatalf("error = %v, want a capture_stalled error", err)
	}
	if terminated != "cs2.exe" {
		t.Fatalf("terminated image = %q, want the observed cs2.exe stopped", terminated)
	}
	for _, want := range []string{"cs2 console: map loaded", `cs2 windows: ["Counter-Strike 2"]`, "cs2 stall: no activity for"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("logs = %q, want %q", logs, want)
		}
	}
}

func TestWaitForWindowsProcessRunAndExitKeepsACaptureThatProgresses(t *testing.T) {
	progress := 0
	stall := quietStall(t, 30*time.Millisecond)
	stall.probe = func() captureProgress {
		progress++
		return captureProgress{fileBytes: int64(progress)}
	}
	started := time.Now()
	var terminated string
	err := waitForWindowsProcessRunAndExitWith(context.Background(), captureWait{
		image:        "cs2.exe",
		firstWait:    time.Hour,
		pollInterval: time.Millisecond,
		closeGrace:   time.Hour,
		verified:     func() bool { return false },
		// cs2.exe outlives several stall limits, then exits on its own.
		status: func(string) (bool, string, error) {
			return time.Since(started) < 300*time.Millisecond, "Counter-Strike 2", nil
		},
		terminate: func(image string) error {
			terminated = image
			return nil
		},
		stall: stall,
	})
	if err != nil || terminated != "" {
		t.Fatalf("error = %v, terminated = %q; want a progressing capture left to exit on its own", err, terminated)
	}
}

func TestWaitForWindowsProcessRunAndExitLeavesAVerifiedCaptureToTheGraceClose(t *testing.T) {
	var terminated string
	err := waitForWindowsProcessRunAndExitWith(context.Background(), captureWait{
		image:        "cs2.exe",
		firstWait:    time.Hour,
		pollInterval: time.Millisecond,
		closeGrace:   50 * time.Millisecond,
		verified:     func() bool { return true },
		status:       runningCS2,
		terminate: func(image string) error {
			terminated = image
			return nil
		},
		stall: quietStall(t, time.Nanosecond),
	})
	if err != nil || terminated != "cs2.exe" {
		t.Fatalf("error = %v, terminated = %q; want the grace close, not a stall", err, terminated)
	}
}

func TestWaitForWindowsProcessRunAndExitWatchdogNeverStopsAnUnobservedProcess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var terminated string
	err := waitForWindowsProcessRunAndExitWith(ctx, captureWait{
		image:        "cs2.exe",
		firstWait:    time.Hour,
		pollInterval: time.Millisecond,
		closeGrace:   time.Hour,
		status:       func(string) (bool, string, error) { return false, "", nil },
		terminate: func(image string) error {
			terminated = image
			return nil
		},
		stall: quietStall(t, time.Nanosecond),
	})
	if !errors.Is(err, context.DeadlineExceeded) || terminated != "" {
		t.Fatalf("error = %v, terminated = %q; want the deadline and no process stopped", err, terminated)
	}
}

func TestWaitForWindowsProcessRunAndExitLogsDiagnosticsAtTheDeadline(t *testing.T) {
	logs := captureLog(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var terminated string
	err := waitForWindowsProcessRunAndExitWith(ctx, captureWait{
		image:        "cs2.exe",
		firstWait:    time.Hour,
		pollInterval: time.Millisecond,
		closeGrace:   time.Hour,
		verified:     func() bool { return false },
		status:       runningCS2,
		terminate: func(image string) error {
			terminated = image
			return nil
		},
		stall: quietStall(t, time.Hour),
	})
	if !errors.Is(err, context.DeadlineExceeded) || terminated != "cs2.exe" {
		t.Fatalf("error = %v, terminated = %q; want the deadline and the observed cs2.exe stopped", err, terminated)
	}
	for _, want := range []string{"cs2 console: Steam is probably not running.", "cs2 console: map loaded", "cs2 stall: no activity for"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("logs = %q, want %q", logs, want)
		}
	}
}

func TestProbeCaptureProgressSeesConsoleAndTakeFiles(t *testing.T) {
	dir := t.TempDir()
	consoleLog := filepath.Join(dir, "console.log")
	out := filepath.Join(dir, "out")
	take := filepath.Join(out, "take0000", "video.mp4")
	if err := os.MkdirAll(filepath.Dir(take), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(consoleLog, []byte("boot\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := probeCaptureProgress(consoleLog, out)
	if again := probeCaptureProgress(consoleLog, out); again != before {
		t.Fatalf("an unchanged capture probed %+v then %+v", before, again)
	}
	for _, change := range []func() error{
		func() error { return os.WriteFile(consoleLog, []byte("boot\nplaying\n"), 0o600) },
		func() error { return os.WriteFile(take, []byte("frame"), 0o600) },
		func() error { return os.WriteFile(take, []byte("frame frame"), 0o600) },
	} {
		if err := change(); err != nil {
			t.Fatal(err)
		}
		next := probeCaptureProgress(consoleLog, out)
		if next == before {
			t.Fatalf("progress %+v did not change after a write", next)
		}
		before = next
	}
}
