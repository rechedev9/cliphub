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

func TestDiagnosticModulePathMatchesGoModInBothFilters(t *testing.T) {
	goMod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	module := ""
	for _, line := range strings.Split(strings.ReplaceAll(string(goMod), "\r\n", "\n"), "\n") {
		if value, ok := strings.CutPrefix(line, "module "); ok {
			module = strings.TrimSpace(value)
		}
	}
	if module != diagnosticModulePath {
		t.Fatalf("diagnosticModulePath = %q; go.mod module = %q", diagnosticModulePath, module)
	}
	desktop, err := os.ReadFile("../../desktop/src/diagnostic-message.ts")
	if err != nil {
		t.Fatal(err)
	}
	escaped := `\b` + strings.NewReplacer(".", `\.`, "/", `\/`).Replace(module) + `\/`
	if !strings.Contains(string(desktop), escaped) {
		t.Fatalf("desktop filter does not protect frames of module %s (want %s)", module, escaped)
	}
}

func TestDiagnosticPlaceholdersNeverLeakPastTheirCapacity(t *testing.T) {
	text := strings.Repeat("1/2 ", maxDiagnosticPlaceholders+100)
	got := filterDiagnosticMessage(text, maxDiagnosticInput)
	if diagnosticPlaceholders.MatchString(got) {
		t.Fatal("a placeholder survived the filter")
	}
	if kept := strings.Count(got, "1/2"); kept != maxDiagnosticPlaceholders {
		t.Fatalf("kept %d fractions; want the %d that fit the placeholder range", kept, maxDiagnosticPlaceholders)
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
