package workers

import (
	"log"
	"os"
	"time"
)

const (
	stageDirRemoveTimeout = 15 * time.Second
	stageDirRemoveDelay   = 500 * time.Millisecond
)

// removeStageDir deletes a stage directory. A capture that was just stopped
// can keep a file open for a moment while CS2 goes away, and a removal that
// gave up at once left gigabytes behind, so a failed removal is repeated for
// a short while before it is reported.
func removeStageDir(dir string) {
	deadline := time.Now().Add(stageDirRemoveTimeout)
	for {
		err := os.RemoveAll(dir)
		if err == nil {
			return
		}
		if !time.Now().Before(deadline) {
			log.Printf("worker: could not remove stage directory %s: %v", dir, err)
			return
		}
		time.Sleep(stageDirRemoveDelay)
	}
}
