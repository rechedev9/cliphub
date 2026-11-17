package workers

import (
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/demostart"
	"github.com/rechedev9/cliphub/internal/recording"
)

// demoIncompatiblePrefix is the stable, machine-readable marker the web UI
// matches on to explain that CS2 cannot replay a demo recorded on an older
// build. Keep it exactly in sync with the frontend.
const demoIncompatiblePrefix = "demo_incompatible:"

// unplayableStartPrefix is the stable marker for a CS2 playdemo tick-0 crash.
const unplayableStartPrefix = "unplayable_start:"

// captureStalledPrefix is the recorder's stable marker for a cs2.exe that ran
// without making capture progress. Relaunching against the same machine state repeats it.
const captureStalledPrefix = "capture_stalled:"

// steamNotRunningPrefix is the stable reason when CS2 could not reach the Steam client.
const steamNotRunningPrefix = "steam_not_running:"

// steamNotRunningMarker is the Steamworks SteamAPI_Init failure text the
// recorder forwards from the CS2 console tail.
const steamNotRunningMarker = "Steam is probably not running"

// steamUnreachable reports a CS2 that could not reach Steam: the console marker, or the
// recorder's own prefix for a missing steam.exe or a Steamworks "Launcher Error" dialog.
func steamUnreachable(text string) bool {
	return strings.Contains(text, steamNotRunningMarker) || strings.Contains(text, steamNotRunningPrefix)
}

const resetBreakpadMarker = "ResetBreakpadAppId"

// networkDisconnectMarker is the CS2 playback error substring that is stable
// across both the old and the new zv-recorder wording.
const networkDisconnectMarker = "NETWORK_DISCONNECT_MESSAGE_PARSE_ERROR"

// playbackEndedMarker is the recorder failure reason when demo playback ended
// before every protected segment could be recorded. Like the network parse
// marker it is deterministic in the demo itself, so retrying against the same
// CS2 build cannot resolve it.
const playbackEndedMarker = "demo playback ended before every protected segment completed"

// missingCaptureAttestationMarker is emitted when CS2 exits unexpectedly after
// playback began but before the HLAE runtime could attest the whole capture.
// Unlike POV drift and incompatible-demo failures, this is an external process
// crash and can succeed after a clean relaunch.
const missingCaptureAttestationMarker = "CS2 exited without the completed POV verification marker"

// maxTransientCaptureRestarts bounds clean CS2/HLAE relaunches inside one
// record task. Queue-level retries remain disabled so deterministic capture
// failures are never repeated automatically.
const maxTransientCaptureRestarts = 2

// recordFailure carries a concise, user-facing job failure reason while still
// wrapping the original noisy recorder error so logs and tests can unwrap the
// full chain.
type recordFailure struct {
	reason string
	err    error
}

func (f *recordFailure) Error() string { return f.reason }

func (f *recordFailure) Unwrap() error { return f.err }

// newRecordFailure wraps a recorder run error with a concise reason derived
// from its text, the decoded (possibly zero) recording result, and the segment
// ids this reel requested.
func newRecordFailure(runErr error, result recording.RecordingResult, requested []string) error {
	return &recordFailure{reason: recordFailureReason(runErr, result, requested), err: runErr}
}

// retryableCaptureCrash reports whether the recorder observed a native CS2
// exit without a runtime failure attestation. The result error is included
// because command wrappers can truncate or replace stderr in some hosts.
func retryableCaptureCrash(runErr error, result recording.RecordingResult) bool {
	if runErr == nil {
		return false
	}
	text := runErr.Error() + "\n" + result.Error
	if !strings.Contains(text, missingCaptureAttestationMarker) {
		return false
	}
	return !strings.Contains(text, networkDisconnectMarker) &&
		!strings.Contains(text, playbackEndedMarker) &&
		!strings.Contains(text, unplayableStartPrefix) &&
		!strings.Contains(text, captureStalledPrefix) &&
		!steamUnreachable(text)
}

// tickZeroCopyCanHelp reports a CS2 crash in its opening rewind to demo tick 0
// on a demo that opens at demo tick 1. The same demo crashes on every relaunch,
// but a copy that already starts at tick 0 has no rewind to crash in.
func tickZeroCopyCanHelp(runErr error, result recording.RecordingResult, demoPath string) bool {
	if runErr == nil || !strings.Contains(runErr.Error()+"\n"+result.Error, unplayableStartPrefix) {
		return false
	}
	_, err := demostart.TickOneOffsets(demoPath)
	return err == nil
}

// recordFailureReason condenses a noisy recorder run error into a concise
// reason. An incompatible-demo failure (keyed on the stable CS2 marker) becomes
// the demo_incompatible: prefix plus an optional captured-progress suffix; any
// other failure is reduced to its last "error: " line, falling back to the
// original text when there is none.
func recordFailureReason(runErr error, result recording.RecordingResult, requested []string) string {
	text := runErr.Error()
	// Without Steam nothing else can work, so it outranks whatever CS2 did next.
	if steamUnreachable(text + "\n" + result.Error) {
		return steamNotRunningPrefix + " CS2 could not reach the Steam client (Steam is not running or not signed in)"
	}
	if strings.Contains(text, networkDisconnectMarker) || strings.Contains(text, playbackEndedMarker) {
		reason := demoIncompatiblePrefix + demoIncompatibleMessage(text)
		if captured := capturedSegmentCount(result); captured > 0 {
			reason += fmt.Sprintf("; captured %d/%d segments before the failure", captured, len(requested))
		}
		return reason
	}
	if strings.Contains(text, unplayableStartPrefix) || strings.Contains(text, resetBreakpadMarker) {
		reason := unplayableStartPrefix + " CS2 crashed rewinding playdemo to tick 0"
		if captured := capturedSegmentCount(result); captured > 0 {
			reason += fmt.Sprintf("; captured %d/%d segments before the failure", captured, len(requested))
		}
		return reason
	}
	if strings.Contains(text, captureStalledPrefix) {
		if line, ok := lastErrorLine(text); ok && strings.HasPrefix(line, captureStalledPrefix) {
			return line
		}
		return captureStalledPrefix + " cs2.exe kept running but made no capture progress"
	}
	if line, ok := lastErrorLine(text); ok {
		return "recorder failed: " + line
	}
	return text
}

// demoIncompatibleMessage selects the explanation for a deterministic demo
// replay failure: the network parse marker means an older CS2 build recorded
// the demo, while a playback-ended failure means the demo stops before every
// protected segment could be recorded.
func demoIncompatibleMessage(text string) string {
	if strings.Contains(text, playbackEndedMarker) {
		return " cs2 cannot replay this demo to the end (playback stops before every protected segment completes)"
	}
	return " cs2 cannot replay this demo (it was recorded on an older cs2 build)"
}

// capturedSegmentCount counts the distinct segment ids that produced a segment
// video artifact, i.e. the reels the recorder finished before it failed.
func capturedSegmentCount(result recording.RecordingResult) int {
	seen := map[string]struct{}{}
	for _, a := range result.Artifacts {
		if a.Role == "segment" && a.SegmentID != "" {
			seen[a.SegmentID] = struct{}{}
		}
	}
	return len(seen)
}

// lastErrorLine returns the last line beginning with "error: " with that prefix
// stripped, reporting whether such a line existed.
func lastErrorLine(text string) (string, bool) {
	line, ok := "", false
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if rest, cut := strings.CutPrefix(l, "error: "); cut {
			line, ok = strings.TrimSpace(rest), true
		}
	}
	return line, ok
}
