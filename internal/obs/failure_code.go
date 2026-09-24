package obs

import (
	"errors"
	"io"
	"log"
	"regexp"
	"strings"
	"time"
)

// FailureCode is a stable, snake_case identifier for a known failure. It
// travels as a message prefix ("failure_code=<code> substage=<substage>; ")
// so it needs no wire change and survives the diagnostic redaction filters.
// Remote alerting groups issues on it, so never rename a published value.
type FailureCode string

// Substage names where inside a pipeline stage a failure happened.
type Substage string

const (
	// Full Demo render codes.
	FailureLoudnormParamOutOfRange FailureCode = "loudnorm_param_out_of_range"
	FailureAudioMasterExhausted    FailureCode = "audio_master_exhausted"
	FailureAACRecoveryFailed       FailureCode = "aac_recovery_failed"
	FailureDeliveryVerifyFailed    FailureCode = "delivery_verify_failed"
	FailureFFmpegFailed            FailureCode = "ffmpeg_failed"
	FailureRenderInterrupted       FailureCode = "render_interrupted"

	// Capture codes written by the recorder.
	FailureCaptureIncomplete    FailureCode = "capture_incomplete"
	FailureHLAEHookIncompatible FailureCode = "hlae_hook_incompatible"
	FailureCS2AlreadyRunning    FailureCode = "cs2_already_running"
	FailureCapturePOVUnverified FailureCode = "capture_pov_unverified"
)

const (
	SubstageAudioMaster    Substage = "audio_master"
	SubstageAACRecovery    Substage = "aac_recovery"
	SubstageDeliveryVerify Substage = "delivery_verify"
	SubstageOverlay        Substage = "overlay"
	SubstageConcat         Substage = "concat"
	SubstageCapture        Substage = "capture"
)

// Failure is a classified failure: a code and, optionally, the substage.
type Failure struct {
	Code     FailureCode
	Substage Substage
}

// Prefix renders the contract prefix without the trailing space, for example
// "failure_code=audio_master_exhausted substage=audio_master;".
func (f Failure) Prefix() string {
	if f.Substage == "" {
		return "failure_code=" + string(f.Code) + ";"
	}
	return "failure_code=" + string(f.Code) + " substage=" + string(f.Substage) + ";"
}

type failureError struct {
	failure Failure
	err     error
}

func (e *failureError) Error() string { return e.failure.Prefix() + " " + e.err.Error() }

func (e *failureError) Unwrap() error { return e.err }

// WithFailure classifies err so that its Error() starts with
// "failure_code=<code> substage=<substage>; " followed by the original text.
// errors.Is/As keep working through the wrapper. The first classification
// wins: an error that already carries a code is returned unchanged, so the
// site closest to the cause decides and a message never carries two prefixes.
func WithFailure(err error, code FailureCode, substage Substage) error {
	if err == nil || code == "" {
		return err
	}
	if _, ok := FailureOf(err); ok {
		return err
	}
	return &failureError{failure: Failure{Code: code, Substage: substage}, err: err}
}

// FailureOf returns the classification carried anywhere in err's chain.
func FailureOf(err error) (Failure, bool) {
	var classified *failureError
	if errors.As(err, &classified) {
		return classified.failure, true
	}
	return Failure{}, false
}

var failurePrefixPattern = regexp.MustCompile(`(?:^|\s)failure_code=([a-z0-9_]+)(?: substage=([a-z0-9_]+))?;`)

// FindFailure locates the last failure prefix in text written by a child
// process. It returns the failure and that line without the prefix and
// without a leading Go log timestamp. Diagnostic trace lines are skipped: they
// carry copies of messages and are not the child's final failure line.
func FindFailure(text string) (Failure, string, bool) {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.Contains(line, TracePrefix) {
			continue
		}
		match := failurePrefixPattern.FindStringSubmatchIndex(line)
		if match == nil {
			continue
		}
		failure := Failure{Code: FailureCode(line[match[2]:match[3]])}
		if match[4] >= 0 {
			failure.Substage = Substage(line[match[4]:match[5]])
		}
		rest := strings.TrimSpace(logTimestampPattern.ReplaceAllString(line[:match[0]], ""))
		if after := strings.TrimSpace(line[match[1]:]); rest == "" {
			rest = after
		} else if after != "" {
			rest += " " + after
		}
		return failure, rest, true
	}
	return Failure{}, "", false
}

// StripFailurePrefix removes a leading failure prefix, so user-facing reasons
// and class matching see the original message.
func StripFailurePrefix(text string) string {
	match := failurePrefixPattern.FindStringIndex(text)
	if match == nil || strings.TrimSpace(text[:match[0]]) != "" {
		return text
	}
	return strings.TrimSpace(text[match[1]:])
}

// LeadWithFailure makes text start with err's failure prefix when err carries
// one and text does not already start with it. Wrappers such as the recorder's
// concise reason hide the prefix from Error(); journals still need it first.
func LeadWithFailure(err error, text string) string {
	failure, ok := FailureOf(err)
	if !ok || strings.HasPrefix(text, failure.Prefix()) {
		return text
	}
	return failure.Prefix() + " " + text
}

var logTimestampPattern = regexp.MustCompile(`^(?:\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?|\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2}))(?:\s+|$)`)

// LastDiagnosticLine returns the last non-empty line of a subprocess's stderr,
// skipping diagnostic trace lines and dropping a leading Go log timestamp so
// the same failure always produces the same text.
func LastDiagnosticLine(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.Contains(line, TracePrefix) {
			continue
		}
		return logTimestampPattern.ReplaceAllString(line, "")
	}
	return ""
}

// UseRFC3339Log switches the standard logger to RFC3339 timestamps written to
// out. Go's default "2006/01/02" date contains slashes, which the diagnostic
// redaction filter treats as a path.
func UseRFC3339Log(out io.Writer) {
	log.SetFlags(0)
	log.SetOutput(RFC3339LogWriter{Out: out})
}

// RFC3339LogWriter prefixes every Go log record with an RFC3339 timestamp. The
// standard logger writes each record with a single Write call.
type RFC3339LogWriter struct {
	Out io.Writer
	Now func() time.Time
}

func (w RFC3339LogWriter) Write(p []byte) (int, error) {
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	line := make([]byte, 0, len(p)+26)
	line = now().AppendFormat(line, time.RFC3339)
	line = append(line, ' ')
	line = append(line, p...)
	if _, err := w.Out.Write(line); err != nil {
		return 0, err
	}
	return len(p), nil
}
