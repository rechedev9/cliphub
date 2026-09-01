package obs

import "strings"

// Stable, queryable error classes. Stage/task names stay on Event.Task;
// Class is the code an operator selects without reading Message.
const (
	ClassMissingPlate           = "missing_plate"
	ClassCaptureFlake           = "capture_flake"
	ClassDemoIncompatible       = "demo_incompatible"
	ClassUnplayableStart        = "unplayable_start"
	ClassRecordingNotReusable   = "recording_not_reusable"
	ClassInterrupted            = "interrupted"
	ClassTargetNotFound         = "target_not_found"
	prefixDemoIncompatible      = "demo_incompatible:"
	prefixUnplayableStart       = "unplayable_start:"
	prefixRecordingNotReusable  = "recording_not_reusable:"
	prefixInterrupted           = "interrupted:"
	phrasePlateMissing          = "plate is missing"
	phraseObserverTarget        = "observer target"
	phraseDriftedFrom           = "drifted from"
	phraseDoesNotMatch          = "does not match"
	phraseTargetNotFound        = "not found in demo"
	phraseOrchestratorRestarted = "orchestrator restarted"
)

// ClassOf maps a failure message onto a stable class. Unknown text returns
// empty so callers can fall back to an existing task type rather than invent
// a parallel taxonomy.
func ClassOf(message string) string {
	switch {
	case strings.HasPrefix(message, prefixDemoIncompatible):
		return ClassDemoIncompatible
	case strings.HasPrefix(message, prefixUnplayableStart):
		return ClassUnplayableStart
	case strings.HasPrefix(message, prefixRecordingNotReusable):
		return ClassRecordingNotReusable
	case strings.HasPrefix(message, prefixInterrupted) || strings.Contains(message, phraseOrchestratorRestarted):
		return ClassInterrupted
	case strings.Contains(message, phrasePlateMissing):
		return ClassMissingPlate
	case isCaptureFlake(message):
		return ClassCaptureFlake
	case strings.Contains(message, phraseTargetNotFound):
		return ClassTargetNotFound
	default:
		return ""
	}
}

func isCaptureFlake(message string) bool {
	if !strings.Contains(message, phraseObserverTarget) {
		return false
	}
	return strings.Contains(message, phraseDriftedFrom) || strings.Contains(message, phraseDoesNotMatch)
}
