package main

import (
	"sync"
	"time"
)

// captureQueueGrace covers the gap between starting a capture and a worker
// taking the job, while the job still shows its previous state.
const captureQueueGrace = 2 * time.Minute

// recentCaptures remembers the captures this process started.
var recentCaptures = newCaptureStarts()

type captureStarts struct {
	now func() time.Time
	mu  sync.Mutex
	at  map[string]time.Time
}

func newCaptureStarts() *captureStarts {
	return &captureStarts{now: time.Now, at: map[string]time.Time{}}
}

func (c *captureStarts) mark(jobID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at[jobID] = c.now()
}

// queued is true while a capture started here may not have reached a worker yet.
func (c *captureStarts) queued(jobID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	started, ok := c.at[jobID]
	if ok && c.now().Sub(started) >= captureQueueGrace {
		delete(c.at, jobID)
		return false
	}
	return ok
}

// forget ends the grace once a worker has taken the job.
func (c *captureStarts) forget(jobID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.at, jobID)
}
