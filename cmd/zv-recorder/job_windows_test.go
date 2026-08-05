//go:build windows

package main

import (
	"os/exec"
	"testing"
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

	// Kill-on-close must have terminated the child: Wait reaps a forcibly
	// terminated process with a non-nil error, never a clean exit.
	if err := child.Wait(); err == nil {
		t.Fatal("child exited cleanly after job close; want forced kill-on-close termination")
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
