package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The tool versions are pinned so a new release never turns the suite red.
const (
	deadcodeTool    = "golang.org/x/tools/cmd/deadcode@v0.52.0"
	staticcheckTool = "honnef.co/go/tools/cmd/staticcheck@v0.8.2"
)

// Studio ships on Windows and CI runs on Linux. A helper in an untagged file
// that only a _windows.go file calls is dead on Linux, so analyze both.
var deadCodeTargets = []string{"linux", "windows"}

// TestNoDeadCode fails when a function no main package reaches stays in
// production code. A function only tests call belongs in a _test.go file.
func TestNoDeadCode(t *testing.T) {
	if testing.Short() {
		t.Skip("whole-program analysis takes several seconds")
	}
	t.Parallel()
	root := repoRoot(t)
	// The tools are built for the host; GOOS then only selects the files they load.
	bin := t.TempDir()
	for _, tool := range []string{deadcodeTool, staticcheckTool} {
		install := exec.Command("go", "install", tool)
		install.Env = append(os.Environ(), "GOBIN="+bin, "GOOS=", "GOARCH=")
		if out, err := install.CombinedOutput(); err != nil {
			t.Fatalf("install %s: %v\n%s", tool, err, out)
		}
	}
	for _, goos := range deadCodeTargets {
		t.Run(goos+"/deadcode", func(t *testing.T) {
			t.Parallel()
			// deadcode exits 0 when it finds dead code, so any output fails.
			out, errOut, err := runCodeAnalysis(root, goos, filepath.Join(bin, executableName("deadcode")), "./...")
			if err != nil {
				t.Fatalf("deadcode: %v\n%s", err, errOut)
			}
			if strings.TrimSpace(out) != "" {
				t.Fatalf("deadcode found functions no %s main package reaches:\n%s", goos, out)
			}
		})
		t.Run(goos+"/staticcheck U1000", func(t *testing.T) {
			t.Parallel()
			if out, errOut, err := runCodeAnalysis(root, goos, filepath.Join(bin, executableName("staticcheck")), "-checks", "U1000", "./..."); err != nil {
				t.Fatalf("staticcheck found unused code for %s (%v):\n%s%s", goos, err, out, errOut)
			}
		})
	}
}

func runCodeAnalysis(root, goos, tool string, args ...string) (string, string, error) {
	cmd := exec.Command(tool, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS="+goos)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	return string(out), stderr.String(), err
}
