package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// The tool versions are pinned so a new release never turns the suite red.
const (
	deadcodeTool    = "golang.org/x/tools/cmd/deadcode@v0.52.0"
	staticcheckTool = "honnef.co/go/tools/cmd/staticcheck@v0.8.2"
)

// TestNoDeadCode fails when a function no main package reaches stays in
// production code. A function only tests call belongs in a _test.go file.
func TestNoDeadCode(t *testing.T) {
	if testing.Short() {
		t.Skip("whole-program analysis takes several seconds")
	}
	t.Parallel()
	root := repoRoot(t)
	t.Run("deadcode", func(t *testing.T) {
		t.Parallel()
		// deadcode exits 0 when it finds dead code, so any output fails.
		out, errOut, err := runGoTool(root, deadcodeTool, "./...")
		if err != nil {
			t.Fatalf("deadcode: %v\n%s", err, errOut)
		}
		if strings.TrimSpace(out) != "" {
			t.Fatalf("deadcode found functions no main package reaches:\n%s", out)
		}
	})
	t.Run("staticcheck U1000", func(t *testing.T) {
		t.Parallel()
		if out, errOut, err := runGoTool(root, staticcheckTool, "-checks", "U1000", "./..."); err != nil {
			t.Fatalf("staticcheck found unused code (%v):\n%s%s", err, out, errOut)
		}
	})
}

// runGoTool keeps stderr apart because go run reports module downloads there.
func runGoTool(root, tool string, args ...string) (string, string, error) {
	cmd := exec.Command("go", append([]string{"run", tool}, args...)...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	return string(out), stderr.String(), err
}
