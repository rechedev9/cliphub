//go:build windows

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestProcessEndsAtItsMemoryLimit(t *testing.T) {
	if os.Getenv("ZV_MCP_MEMORY_CHILD") == "1" {
		if err := limitProcessMemory(256 << 20); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		var hold [][]byte
		for {
			hold = append(hold, bytes.Repeat([]byte{1}, 16<<20))
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessEndsAtItsMemoryLimit$", "-test.count=1")
	cmd.Env = append(os.Environ(), "ZV_MCP_MEMORY_CHILD=1")
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)
	if ctx.Err() != nil {
		t.Fatalf("the child still ran after %s at its memory limit:\n%.2000s", elapsed, stderr.String())
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != memoryLimitExitCode {
		t.Fatalf("child ended with %v, want exit code %d:\n%.2000s", err, memoryLimitExitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "reached its 256 MiB memory limit") {
		t.Fatalf("child stderr lacks the notice:\n%.2000s", stderr.String())
	}
}
