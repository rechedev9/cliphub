//go:build !windows

package main

// captureJob is a no-op outside Windows. HLAE/CS2 capture only runs on Windows,
// and launchAndWait returns before creating a job on other platforms; the stub
// exists only so the shared launch path compiles and stays unit-testable there.
type captureJob struct{}

func newCaptureJob() (*captureJob, error) { return &captureJob{}, nil }

func (j *captureJob) assign(int) error { return nil }

func (j *captureJob) close() error { return nil }
