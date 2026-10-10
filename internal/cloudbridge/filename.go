package cloudbridge

import (
	"fmt"
	"strings"
	"unicode"
)

// maxOriginalNameRunes keeps the whole handle under the 128 runes the local
// pipeline stores, so its own cap never cuts the name mid-way.
const maxOriginalNameRunes = 72

// cloudFileName builds the DemoFileName Studio displays for a bridge-admitted
// job. web/'s Studio UI shows only DemoFileName, and the submitter's note
// only ever appears in the portal's /admin, so without a human-identifiable
// handle here the owner has no way to match a Studio job back to the request
// that produced it.
func cloudFileName(requestID, submitterLabel, originalName string) string {
	shortID := requestID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	submitter := sanitizeHandleComponent(submitterLabel)
	if submitter == "" {
		submitter = "unknown"
	}
	originalName = sanitizeOriginalName(originalName)
	if originalName == "" {
		originalName = "demo.dem"
	}
	return fmt.Sprintf("cloud-%s-%s-%s", shortID, submitter, originalName)
}

// sanitizeOriginalName reduces the name a stranger gave their demo to a
// display name: no directory part, no control characters and no invisible
// format characters (RTL overrides, zero-width runes) that could spoof what
// an operator reads. A manual upload gets the same treatment at admission.
func sanitizeOriginalName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	var b strings.Builder
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		b.WriteRune(r)
	}
	cleaned := strings.TrimSpace(b.String())
	if runes := []rune(cleaned); len(runes) > maxOriginalNameRunes {
		cleaned = strings.TrimSpace(string(runes[:maxOriginalNameRunes]))
	}
	return cleaned
}

// sanitizeHandleComponent keeps a filename segment filesystem- and
// display-safe: letters, digits, dot, dash, underscore pass through;
// everything else (spaces, @, unicode, path separators) becomes an
// underscore. Capped short since it is only ever a display label.
func sanitizeHandleComponent(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
		if b.Len() >= 40 {
			break
		}
	}
	return b.String()
}
