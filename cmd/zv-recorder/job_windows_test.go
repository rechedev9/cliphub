//go:build windows

package main

import (
	"os/exec"
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
