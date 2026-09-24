package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCrashOutputHelper is the child process: it installs the crash output
// exactly as main does and then dies of an unrecovered panic.
func TestCrashOutputHelper(t *testing.T) {
	path := os.Getenv("CLIPHUB_CRASH_OUTPUT_HELPER")
	if path == "" {
		return
	}
	if err := setCrashOutput(path); err != nil {
		t.Fatal(err)
	}
	panic("crash output canary")
}

func TestCrashOutputKeepsFatalPanicAfterExit(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "orchestrator-crash.txt")
	// Studio may leave an earlier crash in place; the file is appended to.
	if err := os.WriteFile(path, []byte("previous crash\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestCrashOutputHelper$")
	cmd.Env = append(os.Environ(), "CLIPHUB_CRASH_OUTPUT_HELPER="+path)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("the helper did not crash: %s", out)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.HasPrefix(text, "previous crash\n") || !strings.Contains(text, "panic: crash output canary") || !strings.Contains(text, "goroutine ") {
		t.Fatalf("crash output = %q", text)
	}
}

func TestCrashOutputReportsUnwritablePath(t *testing.T) {
	if err := setCrashOutput(filepath.Join(t.TempDir(), "missing", "crash.txt")); err == nil {
		t.Fatal("an unwritable crash output path was accepted")
	}
}
