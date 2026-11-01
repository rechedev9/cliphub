package main

import (
	"testing"
	"time"
)

func TestCaptureStartsCoverOnlyTheQueueGap(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	c := newCaptureStarts()
	c.now = func() time.Time { return now }
	if c.queued("job") {
		t.Fatal("a job never started here is not queued")
	}
	c.mark("job")
	if !c.queued("job") {
		t.Fatal("a capture started a moment ago is queued")
	}
	now = now.Add(captureQueueGrace)
	if c.queued("job") {
		t.Fatal("the grace ends after captureQueueGrace")
	}
	c.mark("job")
	c.forget("job")
	if c.queued("job") {
		t.Fatal("a job a worker took is no longer queued")
	}
}
