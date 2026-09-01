package workers

import (
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/recording"
)

// demoIncompatiblePrefix is the stable, machine-readable marker the web UI
// matches on to explain that CS2 cannot replay a demo recorded on an older
// build. Keep it exactly in sync with the frontend.
const demoIncompatiblePrefix = "demo_incompatible:"

// unplayableStartPrefix is the stable marker for a CS2 playdemo tick-0 crash.
const unplayableStartPrefix = "unplayable_start:"

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
		!strings.Contains(text, unplayableStartPrefix)
}

// recordFailureReason condenses a noisy recorder run error into a concise
// reason. An incompatible-demo failure (keyed on the stable CS2 marker) becomes
// the demo_incompatible: prefix plus an optional captured-progress suffix; any
// other failure is reduced to its last "error: " line, falling back to the
// original text when there is none.
func recordFailureReason(runErr error, result recording.RecordingResult, requested []string) string {
	text := runErr.Error()
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
