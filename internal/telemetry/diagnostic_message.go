package telemetry

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxDiagnosticBytes = 2048
const maxDiagnosticInput = 64 * 1024

type diagnosticRedaction struct {
	pattern     *regexp.Regexp
	replacement string
}

// The desktop filters messages before upload; the collector repeats the filter
// before persistence. Shared fixtures exercise both implementations. The rules
// run in three groups: secrets first, then URLs and paths (which a few known
// technical tokens are protected from), then identifiers.
var diagnosticSecretRedactions = []diagnosticRedaction{
	{regexp.MustCompile(`[^\s]{256,}`), "[long-token]"},
	{regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`), ""},
	{regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`), "[credential]"},
	{regexp.MustCompile(`(?i)\b(?:[a-z0-9]+[_-])*(?:authorization|proxy-authorization|password|passwd|token|access_token|refresh_token|api[_-]?key|ingest[_-]?key|secret|cookie|set-cookie)\s*["']?\s*[:=]\s*(?:"[^"\r\n]*"|'[^'\r\n]*'|(?:bearer|basic)\s+[^\s,;]+|[^\s,;]+)`), "[credential]"},
	{regexp.MustCompile(`(?i)\b(?:bearer|basic)\s+[^\s,;]+`), "[credential]"},
}

var diagnosticPathRedactions = []diagnosticRedaction{
	{regexp.MustCompile(`(?i)\b(?:https?|ftp|steam):\/\/[^\s<>"']+`), "[url]"},
	{regexp.MustCompile(`(?i)"[a-z]:[\\/][^"\r\n]*"|'[a-z]:[\\/][^'\r\n]*'|[a-z]:[\\/][^:\r\n<>"',;]*`), "[path]"},
	{regexp.MustCompile(`"\\\\[^"\r\n]*"|'\\\\[^'\r\n]*'|\\\\[^\s<>"']+`), "[path]"},
	{regexp.MustCompile(`"\/[^"\r\n]*"|'\/[^'\r\n]*'|\/[^\s<>"']+`), "[path]"},
}

var diagnosticIdentifierRedactions = []diagnosticRedaction{
	{regexp.MustCompile("(?i)[a-z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-z0-9-]+(?:\\.[a-z0-9-]+)+"), "[email]"},
	{regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`), "[address]"},
	{regexp.MustCompile(`\b[0-9]{17}\b`), "[steamid]"},
	{regexp.MustCompile(`(?i)[^\s\\/<>:"']+\.(?:dem|mp4|mkv|avi|webm|wav|mp3|m4a|flac)\b`), "[media]"},
	{regexp.MustCompile(`(?i)\b[a-z0-9_-]{40,}\b`), "[identifier]"},
}

// diagnosticModulePath is this repository's Go module path (go.mod). Binaries
// built with -trimpath print panic frames relative to it instead of a local
// checkout path.
const diagnosticModulePath = "github.com/rechedev9/cliphub"

// Protected tokens are swapped for private-use placeholders before the URL and
// path rules and restored right after them. Only tokens that the identifier
// rules would leave unchanged qualify, so the filter stays idempotent.
const (
	diagnosticPlaceholderBase = 0xE000
	maxDiagnosticPlaceholders = 0xF8FF - diagnosticPlaceholderBase + 1
)

var (
	diagnosticPlaceholders = regexp.MustCompile(`[\x{E000}-\x{F8FF}]`)
	// The Studio web UI is served from loopback; its stack frames keep the
	// bundle location but not the port.
	protectedAppAsset    = regexp.MustCompile(`(?:http://127\.0\.0\.1:[0-9]{1,5}|\[app\])/_next/static/chunks/[A-Za-z0-9_.\-()\[\]@~/]+\.js:[0-9]{1,7}:[0-9]{1,7}`)
	protectedWhitespace  = regexp.MustCompile(`[^\t\n\f\r ]+`)
	protectedRoute       = regexp.MustCompile(`^/api/[A-Za-z0-9{}_.\-/]+$`)
	protectedModuleFrame = regexp.MustCompile(`\b` + regexp.QuoteMeta(diagnosticModulePath) + `/[A-Za-z0-9_./\-]+`)
	protectedDate        = regexp.MustCompile(`\b[0-9]{4}/[0-9]{2}/[0-9]{2}\b`)
	protectedFraction    = regexp.MustCompile(`\b[0-9]{1,6}/[0-9]{1,6}\b`)
	unsafePathSegment    = regexp.MustCompile(`(?:^|/)\.{1,2}(?:/|$)|//`)
)

var diagnosticControls = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f]`)

func diagnosticMessage(text string) string {
	return filterDiagnosticMessage(text, maxDiagnosticBytes)
}

func filterDiagnosticMessage(text string, limit int) string {
	if len(text) > maxDiagnosticInput {
		text = fitDiagnosticBytes(text, maxDiagnosticInput/2, false) + "\n[truncated]\n" + fitDiagnosticBytes(text, maxDiagnosticInput/2, true)
	}
	text = diagnosticPlaceholders.ReplaceAllString(text, "")
	text = applyDiagnosticRedactions(text, diagnosticSecretRedactions)
	text, protected := protectDiagnosticTokens(text)
	text = applyDiagnosticRedactions(text, diagnosticPathRedactions)
	text = diagnosticPlaceholders.ReplaceAllStringFunc(text, func(placeholder string) string {
		r, _ := utf8.DecodeRuneInString(placeholder)
		index := int(r) - diagnosticPlaceholderBase
		if index < len(protected) {
			return protected[index]
		}
		return ""
	})
	text = applyDiagnosticRedactions(text, diagnosticIdentifierRedactions)
	text = strings.TrimSpace(diagnosticControls.ReplaceAllString(text, ""))
	if len(text) <= limit {
		return text
	}
	const marker = "\n[truncated]\n"
	half := (limit - len(marker)) / 2
	return fitDiagnosticBytes(text, half, false) + marker + fitDiagnosticBytes(text, half, true)
}

func applyDiagnosticRedactions(text string, rules []diagnosticRedaction) string {
	for _, rule := range rules {
		text = rule.pattern.ReplaceAllString(text, rule.replacement)
	}
	return text
}

// protectDiagnosticTokens keeps API route patterns, this module's panic frames,
// dates, fractions and Studio bundle frames out of the path rules, which would
// otherwise turn them into [path]. Anything else behaves exactly as before.
func protectDiagnosticTokens(text string) (string, []string) {
	var protected []string
	keep := func(candidate, fallback string) string {
		if len(protected) >= maxDiagnosticPlaceholders || unsafePathSegment.MatchString(candidate) ||
			applyDiagnosticRedactions(candidate, diagnosticIdentifierRedactions) != candidate {
			return fallback
		}
		protected = append(protected, candidate)
		return string(rune(diagnosticPlaceholderBase + len(protected) - 1))
	}
	text = protectedAppAsset.ReplaceAllStringFunc(text, func(match string) string {
		return keep("[app]"+match[strings.Index(match, "/_next/"):], match)
	})
	text = protectedWhitespace.ReplaceAllStringFunc(text, func(token string) string {
		if !protectedRoute.MatchString(token) {
			return token
		}
		return keep(token, token)
	})
	for _, pattern := range []*regexp.Regexp{protectedModuleFrame, protectedDate, protectedFraction} {
		text = pattern.ReplaceAllStringFunc(text, func(match string) string { return keep(match, match) })
	}
	return text, protected
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
