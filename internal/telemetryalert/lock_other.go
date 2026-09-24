//go:build !(linux || darwin)

package telemetryalert

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// lockStateDir falls back to an exclusive lock file on platforms without
// flock (local development); a lock older than two budgets is stale.
func lockStateDir(dir string) (func(), error) {
	path := filepath.Join(dir, ".lock")
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = file.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		info, statErr := os.Stat(path)
		if statErr != nil || time.Since(info.ModTime()) < 2*runBudget {
			break
		}
		_ = os.Remove(path)
	}
	return nil, errors.New("another alert run holds the state lock")
}
