package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// maxGoFileLines caps every Go file. Long files are hard for agents and people to read
// and edit without breaking something.
const maxGoFileLines = 1000

// oversizedGoFiles holds the files that were already past the cap when it was added, with
// their size then. They may shrink but never grow; drop an entry once its file fits.
var oversizedGoFiles = map[string]int{
	"cmd/zv-orchestrator/inline_queue_test.go":             1971,
	"cmd/zv-recorder/main.go":                              1814,
	"cmd/zv-recorder/main_test.go":                         1492,
	"cmd/zv/app_e2e_test.go":                               1038,
	"cmd/zv/app_workflows_e2e_test.go":                     1377,
	"cmd/zv/short_commands_test.go":                        1134,
	"cmd/zv/workflows_commands_test.go":                    1080,
	"internal/demooverlay/render.go":                       1231,
	"internal/editor/effects.go":                           1717,
	"internal/editor/effects_test.go":                      1173,
	"internal/editor/filter.go":                            1019,
	"internal/editor/killfeed_probe_test.go":               1028,
	"internal/editor/manifest.go":                          1193,
	"internal/editor/manifest_test.go":                     1372,
	"internal/faceit/index.go":                             1097,
	"internal/httpapi/handlers.go":                         2652,
	"internal/httpapi/handlers_generate_test.go":           1040,
	"internal/httpapi/handlers_test.go":                    6350,
	"internal/httpapi/stream_handlers.go":                  1101,
	"internal/httpapi/workbench_htmx.go":                   1154,
	"internal/reconcile/sweep_test.go":                     1136,
	"internal/store/sqlite_library_repo.go":                1233,
	"internal/tacticalplan/scout.go":                       1251,
	"internal/workers/media_worker.go":                     4183,
	"internal/workers/media_worker_segment_select_test.go": 1174,
	"internal/workers/media_worker_test.go":                3140,
}

// TestGoFileLength fails when a Go file passes the line cap, or an oversized file grows.
func TestGoFileLength(t *testing.T) {
	root := repoRoot(t)
	seen := map[string]bool{}
	var problems []string
	// Ask git so ignored build output and nested worktrees never count; untracked new files still do.
	list := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "*.go")
	list.Dir = root
	out, err := list.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	for _, rel := range strings.Split(string(out), "\x00") {
		if rel == "" || strings.HasPrefix(rel, "testdata/") || strings.Contains(rel, "/testdata/") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		lines := bytes.Count(data, []byte("\n"))
		limit, listed := oversizedGoFiles[rel]
		if listed {
			seen[rel] = true
			switch {
			case lines <= maxGoFileLines:
				problems = append(problems, rel+" now fits the cap: remove it from oversizedGoFiles")
			case lines > limit:
				problems = append(problems, rel+" grew past its frozen size: split it instead")
			}
			continue
		}
		if lines > maxGoFileLines {
			problems = append(problems, rel+" passes the line cap: split it into smaller files")
		}
	}
	for rel := range oversizedGoFiles {
		if !seen[rel] {
			problems = append(problems, rel+" no longer exists: remove it from oversizedGoFiles")
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		t.Fatalf("Go files must stay at or under %d lines:\n%s", maxGoFileLines, strings.Join(problems, "\n"))
	}
}
