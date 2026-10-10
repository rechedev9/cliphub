package workers

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// stageDirPattern matches what prepareStageDir creates in the temp directory:
// zv-<stage>-<job uuid>-<random>.
var stageDirPattern = regexp.MustCompile(`^zv-[a-z-]+-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}-[0-9]+$`)

// SweepStageDirs removes the stage directories earlier runs left in the temp
// directory: each is removed by a deferred call, which a killed orchestrator
// never makes, and a raw capture is several gigabytes. It must only be called
// before any task runs, and only where this orchestrator is the single one
// on the machine (the cloud worker), because the temp directory is shared.
func SweepStageDirs() (removed int, err error) {
	return sweepStageDirs(os.TempDir())
}

func sweepStageDirs(base string) (removed int, err error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !stageDirPattern.MatchString(entry.Name()) {
			continue
		}
		if removeErr := os.RemoveAll(filepath.Join(base, entry.Name())); removeErr != nil {
			err = removeErr
			continue
		}
		removed++
	}
	return removed, err
}

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
