package telemetry

import (
	"net/http"
	"strconv"
	"sync"
)

const (
	rejectionChannelEvents = "events"
	rejectionChannelLogs   = "logs"
)

// rejectionCounter counts refused ingest requests since process start, keyed
// "<channel>:<status>:<code>". A client drops an isolated rejected event, so
// without this an allowlist drift loses errors with no trace on the server.
// The external alerter diffs the counts between runs.
type rejectionCounter struct {
	mu     sync.Mutex
	counts map[string]int64
}

func newRejectionCounter() *rejectionCounter {
	return &rejectionCounter{counts: make(map[string]int64)}
}

func (c *rejectionCounter) add(channel string, status int, code string) {
	key := channel + ":" + strconv.Itoa(status) + ":" + code
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[key]++
}

func (c *rejectionCounter) snapshot() map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.counts))
	for key, count := range c.counts {
		out[key] = count
	}
	return out
}

// rejectIngest counts, logs and answers one refused ingest request. The log
// line carries labels only: never the body, client address, keys or messages.
func (a *API) rejectIngest(w http.ResponseWriter, channel string, status int, code string) {
	a.rejections.add(channel, status, code)
	stage := "ingest"
	if channel == rejectionChannelLogs {
		stage = "logs"
	}
	a.logf("telemetry stage=%s class=rejected status=%d code=%s", stage, status, code)
	writeError(w, status, code)
}
