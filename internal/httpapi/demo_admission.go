package httpapi

import (
	"bytes"
	"net/http"
	"strings"
	"unicode"
)

// maxDemoFileNameRunes caps the stored original demo file name so a hostile or
// accidental upload cannot bloat the persisted job document.
const maxDemoFileNameRunes = 128

// sanitizeDemoFileName reduces an uploaded multipart file name to a safe display
// name: it strips any directory prefix using either separator (a client may send
// a Windows path or a URL-style one, so filepath.Base alone is not enough), drops
// control characters and invisible format characters (Cf: RTL overrides,
// zero-width runes, BOM) that could spoof the displayed name, and caps the
// result at maxDemoFileNameRunes runes. It returns "" when nothing usable
// remains, so the caller leaves the field empty.
func sanitizeDemoFileName(name string) string {
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
	if runes := []rune(cleaned); len(runes) > maxDemoFileNameRunes {
		cleaned = strings.TrimSpace(string(runes[:maxDemoFileNameRunes]))
	}
	return cleaned
}

// Stable codes for a demo rejected at admission, so the web client can tell
// the user why instead of a generic "could not scan" message.
const (
	codeCSGODemo       = "csgo_demo"
	codeNotADemo       = "not_a_demo"
	codeUnreadableDemo = "unreadable_demo"
	codeTruncatedDemo  = "truncated_demo"
)

// isDemoHeader reports whether the leading bytes look like a CS2 (Source 2)
// demo, the only format the parser (demoinfocs v5) can read.
func isDemoHeader(header []byte) bool {
	return bytes.HasPrefix(header, []byte("PBDEMS2"))
}

// isCSGODemoHeader reports whether the leading bytes belong to a legacy CS:GO
// (Source 1) demo. demoinfocs v5 refuses those, so admitting one only queues a
// scan that is certain to fail.
func isCSGODemoHeader(header []byte) bool {
	return bytes.HasPrefix(header, []byte("HL2DEMO"))
}

// rejectDemoHeader writes a coded 400 and returns true when header is not a
// CS2 demo. subject names the file in the message ("uploaded", "downloaded").
func rejectDemoHeader(w http.ResponseWriter, header []byte, subject string) bool {
	switch {
	case isDemoHeader(header):
		return false
	case isCSGODemoHeader(header):
		writeCodedError(w, http.StatusBadRequest, codeCSGODemo, subject+" file is a CS:GO demo; only CS2 demos are supported")
	default:
		writeCodedError(w, http.StatusBadRequest, codeNotADemo, subject+" file is not a CS2 demo")
	}
	return true
}
