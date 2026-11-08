package telemetry

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// Studio's journal forwarder files any stage or class it does not list as "unknown",
// so a value the collector accepts but desktop drops never reaches the diagnostics.
func TestDesktopJournalAllowlistsMatchTheCollector(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "desktop", "src", "telemetry-journal.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for name, collector := range map[string]map[string]bool{
		"ALLOWED_STAGES":  allowedJournalStages,
		"ALLOWED_CLASSES": allowedJournalClasses,
	} {
		desktop := tsStringSet(t, string(src), name)
		for value := range collector {
			if !desktop[value] {
				t.Errorf("desktop %s is missing %q", name, value)
			}
		}
		for value := range desktop {
			if !collector[value] {
				t.Errorf("desktop %s has %q, which the collector rejects", name, value)
			}
		}
	}
}

func tsStringSet(t *testing.T, src, name string) map[string]bool {
	t.Helper()
	m := regexp.MustCompile(`const ` + name + ` = new Set\(\[([^\]]*)\]\)`).FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("telemetry-journal.ts has no %s set", name)
	}
	set := map[string]bool{}
	for _, quoted := range regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(m[1], -1) {
		set[quoted[1]] = true
	}
	if len(set) == 0 {
		t.Fatalf("telemetry-journal.ts %s is empty", name)
	}
	return set
}
