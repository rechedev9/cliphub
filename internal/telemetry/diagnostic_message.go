package telemetry

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxDiagnosticBytes = 2048
const maxDiagnosticInput = 64 * 1024

// The desktop filters messages before upload; the collector repeats the filter
// before persistence. Shared fixtures exercise both implementations.
var diagnosticRedactions = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`[^\s]{256,}`), "[long-token]"},
	{regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`), ""},
	{regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`), "[credential]"},
	{regexp.MustCompile(`(?i)\b(?:[a-z0-9]+[_-])*(?:authorization|proxy-authorization|password|passwd|token|access_token|refresh_token|api[_-]?key|ingest[_-]?key|secret|cookie|set-cookie)\s*["']?\s*[:=]\s*(?:"[^"\r\n]*"|'[^'\r\n]*'|(?:bearer|basic)\s+[^\s,;]+|[^\s,;]+)`), "[credential]"},
	{regexp.MustCompile(`(?i)\b(?:bearer|basic)\s+[^\s,;]+`), "[credential]"},
	{regexp.MustCompile(`(?i)\b(?:https?|ftp|steam):\/\/[^\s<>"']+`), "[url]"},
	{regexp.MustCompile(`(?i)"[a-z]:[\\/][^"\r\n]*"|'[a-z]:[\\/][^'\r\n]*'|[a-z]:[\\/][^:\r\n<>"',;]*`), "[path]"},
	{regexp.MustCompile(`"\\\\[^"\r\n]*"|'\\\\[^'\r\n]*'|\\\\[^\s<>"']+`), "[path]"},
	{regexp.MustCompile(`"\/[^"\r\n]*"|'\/[^'\r\n]*'|\/[^\s<>"']+`), "[path]"},
	{regexp.MustCompile("(?i)[a-z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-z0-9-]+(?:\\.[a-z0-9-]+)+"), "[email]"},
	{regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`), "[address]"},
	{regexp.MustCompile(`\b[0-9]{17}\b`), "[steamid]"},
	{regexp.MustCompile(`(?i)[^\s\\/<>:"']+\.(?:dem|mp4|mkv|avi|webm|wav|mp3|m4a|flac)\b`), "[media]"},
	{regexp.MustCompile(`(?i)\b[a-z0-9_-]{40,}\b`), "[identifier]"},
}

var diagnosticControls = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f]`)

func diagnosticMessage(text string) string {
	return filterDiagnosticMessage(text, maxDiagnosticBytes)
}

func filterDiagnosticMessage(text string, limit int) string {
	if len(text) > maxDiagnosticInput {
		text = fitDiagnosticBytes(text, maxDiagnosticInput/2, false) + "\n[truncated]\n" + fitDiagnosticBytes(text, maxDiagnosticInput/2, true)
	}
	for _, rule := range diagnosticRedactions {
		text = rule.pattern.ReplaceAllString(text, rule.replacement)
	}
	text = strings.TrimSpace(diagnosticControls.ReplaceAllString(text, ""))
	if len(text) <= limit {
		return text
	}
	const marker = "\n[truncated]\n"
	half := (limit - len(marker)) / 2
	return fitDiagnosticBytes(text, half, false) + marker + fitDiagnosticBytes(text, half, true)
}

func fitDiagnosticBytes(text string, budget int, tail bool) string {
	if len(text) <= budget {
		return text
	}
	if tail {
		start := len(text) - budget
		for start < len(text) && !utf8.RuneStart(text[start]) {
			start++
		}
		return text[start:]
	}
	end := budget
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end]
}
