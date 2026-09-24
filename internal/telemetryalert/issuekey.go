package telemetryalert

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/rechedev9/cliphub/internal/obs"
)

// Issue keys never leave the VPS. They are derived only from labels and the
// already-filtered message, so no unfiltered text is ever hashed.

var (
	failureCodePattern = regexp.MustCompile(`^failure_code=([a-z0-9_]+)( substage=([a-z0-9_]+))?;`)
	keyLinePattern     = regexp.MustCompile(`(?i)error|failed|invalid|out of range|exit status|panic|crash`)

	signatureRewrites = []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		// Timestamps, including the over-redacted "2026[path] 17:09:49" form.
		{regexp.MustCompile(`\b\d{4}[-/]\d{2}[-/]\d{2}(?:[T ]\d{2}:\d{2}(?::\d{2}(?:[.,]\d+)?)?(?:Z|[+-]\d{2}:?\d{2})?)?`), ""},
		{regexp.MustCompile(`\b(?:19|20)\d{2}\[path\]`), ""},
		{regexp.MustCompile(`\b\d{1,2}:\d{2}:\d{2}(?:[.,]\d+)?\b`), ""},
		{regexp.MustCompile(`(?i)\b0x[0-9a-f]+\b`), "<n>"},
		{regexp.MustCompile(`\bv\d+(?:\.\d+)+(?:-[a-z0-9.]+)?\b`), "v<n>"},
		{regexp.MustCompile(`\b\d+(?:\.\d+)?(?:ns|us|µs|ms|s|m|h)(?:\d+(?:\.\d+)?(?:ms|s|m))*\b`), "<n>"},
		{regexp.MustCompile(`\b\d+(?:\.\d+)*\b`), "<n>"},
		{regexp.MustCompile(`(\[(?:path|url|media|identifier)\])(?:[\s,;:]*\[(?:path|url|media|identifier)\])+`), "$1"},
		{regexp.MustCompile(`\s+`), " "},
	}
	uuidPattern    = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	longHexPattern = regexp.MustCompile(`(?i)\b[0-9a-f]{8,}\b`)
)

const maxSignatureRunes = 160

// ParseFailureCode reads the Go-written "failure_code=<code> substage=<sub>;"
// prefix. Both results are empty when the message has none.
func ParseFailureCode(message string) (code, substage string) {
	match := failureCodePattern.FindStringSubmatch(strings.TrimSpace(message))
	if match == nil {
		return "", ""
	}
	return match[1], match[3]
}

// Signature normalizes the key line of a filtered message so rewordings of
// numbers, times, ids and redacted tokens do not create new issues.
func Signature(message string) string {
	var line string
	for _, candidate := range strings.Split(message, "\n") {
		candidate = strings.TrimSpace(candidate)
		// Relayed trace lines (a CS2 console tail, tool events) copy other
		// text and must never become the key line of an issue.
		if candidate == "" || obs.IsTraceLine(candidate) {
			continue
		}
		if line == "" {
			line = candidate
		}
		if keyLinePattern.MatchString(candidate) {
			line = candidate
			break
		}
	}
	line = uuidPattern.ReplaceAllString(line, "<n>")
	line = longHexPattern.ReplaceAllStringFunc(line, func(token string) string {
		if strings.ContainsAny(token, "0123456789") {
			return "<n>"
		}
		return token
	})
	for _, rewrite := range signatureRewrites {
		line = rewrite.pattern.ReplaceAllString(line, rewrite.replacement)
	}
	line = strings.TrimSpace(line)
	if utf8.RuneCountInString(line) > maxSignatureRunes {
		line = string([]rune(line)[:maxSignatureRunes])
	}
	return line
}

// IssueKey is sha256(component|name|stage|class|code_or_signature)[:16].
func IssueKey(l Labels, codeOrSignature string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{l.Component, l.Name, l.Stage, l.Class, codeOrSignature}, "|")))
	return hex.EncodeToString(sum[:])[:16]
}

// Labels are the release-independent event labels of an issue.
type Labels struct {
	Component, Name, Stage, Class string
}

func (l Labels) String() string {
	return strings.Join([]string{l.Component, l.Name, l.Stage, l.Class}, "|")
}

// Short is the label shown on the phone: the class when it names a task,
// otherwise the event name.
func (l Labels) Short() string {
	if l.Class != "" && l.Class != "unknown" {
		return l.Class
	}
	return l.Name
}

func parseLabels(value string) Labels {
	parts := strings.SplitN(value, "|", 4)
	for len(parts) < 4 {
		parts = append(parts, "")
	}
	return Labels{parts[0], parts[1], parts[2], parts[3]}
}
