//go:build windows

package main

import (
	"math"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestCaptureJobKillsAssignedProcessOnClose(t *testing.T) {
	job, err := newCaptureJob()
	if err != nil {
		t.Fatalf("newCaptureJob: %v", err)
	}

	// A long-lived child gives the job a live member to terminate on close.
	// ping -n 30 runs for ~30s without needing a console or stdin, far longer
	// than the kill-on-close latency this test measures.
	child := exec.Command("ping", "-n", "30", "127.0.0.1")
	if err := child.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	t.Cleanup(func() { _ = child.Process.Kill() })

	if err := job.assign(child.Process.Pid); err != nil {
		t.Fatalf("assign child to job: %v", err)
	}
	if err := job.close(); err != nil {
		t.Fatalf("close job: %v", err)
	}

	// Kill-on-close terminates the job's members as if TerminateJobObject had
	// been called with exit code 0, so the child's exit status is
	// indistinguishable from a clean exit and cannot prove anything. The proof
	// is timing: the child would otherwise run for ~30s, so a Wait that returns
	// promptly means the job tore it down. The threshold is deliberately far
	// above the observed sub-millisecond teardown and far below the child's own
	// runtime, so the test cannot flake on a loaded machine.
	waitStarted := time.Now()
	_ = child.Wait()
	if elapsed := time.Since(waitStarted); elapsed > 10*time.Second {
		t.Fatalf("child outlived the job by %s; want prompt kill-on-close termination", elapsed)
	}
}

func TestCaptureJobAssignRejectsInvalidPID(t *testing.T) {
	job, err := newCaptureJob()
	if err != nil {
		t.Fatalf("newCaptureJob: %v", err)
	}
	t.Cleanup(func() { _ = job.close() })

	tests := []struct {
		name string
		pid  int
	}{
		{name: "zero", pid: 0},
		{name: "negative", pid: -1},
		{name: "above uint32", pid: int(uint64(math.MaxUint32) + 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := job.assign(tt.pid)
			if err == nil {
				t.Fatalf("assign(%d) succeeded, want error", tt.pid)
			}
			if !strings.Contains(err.Error(), "invalid PID") {
				t.Fatalf("assign(%d) error = %q, want it to contain %q", tt.pid, err, "invalid PID")
			}
		})
	}
}

func TestCaptureJobCloseIsIdempotent(t *testing.T) {
	job, err := newCaptureJob()
	if err != nil {
		t.Fatalf("newCaptureJob: %v", err)
	}
	if err := job.close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := job.close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}
