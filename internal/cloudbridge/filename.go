package cloudbridge

import (
	"fmt"
	"strings"
)

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
	if originalName == "" {
		originalName = "demo.dem"
	}
	return fmt.Sprintf("cloud-%s-%s-%s", shortID, submitter, originalName)
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
