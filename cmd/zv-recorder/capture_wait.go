package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// captureCloseGracePeriod bounds how long the recorder waits for the in-engine
// soft-quit (scriptgen's mirv.exec("quit")) to close cs2.exe on its own once the
// capture-verified attestation marker has appeared. CS2 occasionally hangs on
// shutdown — a stalled render thread, a native modal, or client frames that stop
// advancing before the quit countdown lands — so relying on the in-engine quit
// alone makes the close non-deterministic. After the attestation marker proves
// this launch owns cs2.exe and the capture is complete, force-closing the
// process past this grace window is deterministic and safe, instead of blocking
// until the recorder's global timeout fires.
const captureCloseGracePeriod = 15 * time.Second

// captureWait configures waitForWindowsProcessRunAndExitWith. A nil stall
// disables the inactivity watchdog and the cut diagnostics.
type captureWait struct {
	image        string
	firstWait    time.Duration
	pollInterval time.Duration
	closeGrace   time.Duration
	verified     func() bool
	status       func(string) (bool, string, error)
	terminate    func(string) error
	stall        *captureStallWatch
}

// waitForCaptureExit waits for the cs2.exe this capture launched, writing
// into outputDir, to run and exit.
func (m *cs2ConsoleLogMonitor) waitForCaptureExit(ctx context.Context, outputDir string) error {
	return waitForWindowsProcessRunAndExitWith(ctx, captureWait{
		image:        "cs2.exe",
		firstWait:    60 * time.Second,
		pollInterval: 500 * time.Millisecond,
		closeGrace:   captureCloseGracePeriod,
		verified:     func() bool { return m.captureVerified },
		status: func(image string) (bool, string, error) {
			running, title, err := processWindowTitle(image)
			if err != nil {
				return running, title, err
			}
			if err := m.failure(); err != nil {
				return running, title, err
			}
			return running, title, nil
		},
		terminate: terminateWindowsProcess,
		stall:     newCaptureStallWatch(m.path, outputDir),
	})
}

func waitForWindowsProcessRunAndExitWith(ctx context.Context, w captureWait) error {
	image, status, terminate := w.image, w.status, w.terminate
	seen := false
	firstDeadline := time.NewTimer(w.firstWait)
	defer firstDeadline.Stop()
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// The grace timer arms only once the capture-verified marker proves this run
	// owns cs2.exe and recording is complete. Until then graceC stays nil so the
	// select never takes that branch, and an unverified process is never
	// force-closed. When the timer fires, cs2.exe has outlived the in-engine
	// soft-quit and is force-closed deterministically.
	graceTimer := time.NewTimer(w.closeGrace)
	if !graceTimer.Stop() {
		select {
		case <-graceTimer.C:
		default:
		}
	}
	defer graceTimer.Stop()
	var graceC <-chan time.Time
	graceArmed := false
	armGraceIfVerified := func() {
		if graceArmed || w.verified == nil || !w.verified() {
			return
		}
		graceArmed = true
		graceTimer.Reset(w.closeGrace)
		graceC = graceTimer.C
	}

	var stallC <-chan time.Time
	if w.stall != nil {
		w.stall.start(time.Now())
		stallTicker := time.NewTicker(w.stall.every)
		defer stallTicker.Stop()
		stallC = stallTicker.C
	}

	for {
		select {
		case <-ctx.Done():
			_, _, statusErr := status(image)
			w.stall.report(time.Now())
			if statusErr != nil {
				cause := errors.Join(ctx.Err(), fmt.Errorf("inspect %s during cancellation: %w", image, statusErr))
				return stopProcessAfterWaitFailure(
					image,
					cause,
					shouldTerminateAfterStatusFailure(statusErr, seen),
					terminate,
				)
			}
			return stopProcessAfterWaitFailure(image, ctx.Err(), seen, terminate)
		case <-firstDeadline.C:
			if !seen {
				running, title, err := status(image)
				if running {
					seen = true
				}
				if err != nil {
					return stopProcessAfterWaitFailure(image, err, shouldTerminateAfterStatusFailure(err, seen), terminate)
				}
				if isHookErrorWindowTitle(title) {
					return stopProcessAfterWaitFailure(image, &hookIncompatibleError{windowTitle: title}, true, terminate)
				}
				if !running {
					return fmt.Errorf("%s did not appear within %s", image, w.firstWait)
				}
				armGraceIfVerified()
			}
		case <-graceC:
			// Capture is verified but cs2.exe is still up past the grace window:
			// the in-engine quit did not land. Force-close the proven-owned
			// process so the close is deterministic rather than waiting on the
			// recorder's global timeout.
			running, _, err := status(image)
			if err != nil {
				return stopProcessAfterWaitFailure(image, err, shouldTerminateAfterStatusFailure(err, seen), terminate)
			}
			if !running {
				return nil
			}
			if err := terminate(image); err != nil {
				return fmt.Errorf("force-close %s after verified capture: %w", image, err)
			}
			return nil
		case now := <-stallC:
			idle := w.stall.observe(now)
			armGraceIfVerified()
			// A verified capture belongs to the grace close; an unseen process is not ours to stop.
			if !seen || graceArmed || idle < w.stall.limit {
				continue
			}
			running, _, err := status(image)
			if err != nil {
				return stopProcessAfterWaitFailure(image, err, shouldTerminateAfterStatusFailure(err, seen), terminate)
			}
			if !running {
				return nil
			}
			w.stall.report(now)
			return stopProcessAfterWaitFailure(image, &captureStallError{idle: idle}, true, terminate)
		case <-ticker.C:
			running, title, err := status(image)
			if running {
				seen = true
			}
			if err != nil {
				return stopProcessAfterWaitFailure(image, err, shouldTerminateAfterStatusFailure(err, seen), terminate)
			}
			if isHookErrorWindowTitle(title) {
				return stopProcessAfterWaitFailure(image, &hookIncompatibleError{windowTitle: title}, true, terminate)
			}
			if running {
				armGraceIfVerified()
				continue
			}
			if seen {
				return nil
			}
		}
	}
}

func shouldTerminateAfterStatusFailure(err error, processSeen bool) bool {
	var parseErr *demoParseError
	// The console marker can only come from the CS2 instance launched for this
	// capture. It is therefore ownership evidence even when process polling
	// has not successfully observed the process yet.
	return processSeen || errors.As(err, &parseErr)
}

func stopProcessAfterWaitFailure(image string, cause error, processMayBeRunning bool, terminate func(string) error) error {
	if !processMayBeRunning {
		return cause
	}
	if err := terminate(image); err != nil {
		return fmt.Errorf("%w; stop %s after capture failure: %v", cause, image, err)
	}
	return cause
}
