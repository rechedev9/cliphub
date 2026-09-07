package telemetry

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDiagnosticMessageSharedFixtures(t *testing.T) {
	data, err := os.ReadFile("../../testdata/telemetry-diagnostics.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct{ Name, Input, Expected string }
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			if got := diagnosticMessage(fixture.Input); got != fixture.Expected {
				t.Fatalf("got %q; want %q", got, fixture.Expected)
			}
			if got := diagnosticMessage(fixture.Expected); got != fixture.Expected {
				t.Fatalf("filter is not idempotent: %q", got)
			}
		})
	}
}

func TestDiagnosticMessagePreservesFinalCauseWithinUTF8Limit(t *testing.T) {
	text := diagnosticMessage("recorder failed: " + strings.Repeat("é漢🙂 ", 2000) + "\nFinal cause: encoder device unavailable")
	if len(text) > maxDiagnosticBytes || !utf8.ValidString(text) || !strings.HasPrefix(text, "recorder failed:") || !strings.HasSuffix(text, "Final cause: encoder device unavailable") {
		t.Fatalf("invalid bounded diagnostic (%d bytes): %q", len(text), text)
	}
	if diagnosticMessage(text) != text {
		t.Fatal("bounded message is not idempotent")
	}
}
