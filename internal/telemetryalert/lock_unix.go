//go:build linux || darwin

package telemetryalert

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// lockStateDir takes an exclusive, non-blocking flock so an overlapping
// manual run cannot race the timer. The kernel drops it if the process dies.
func lockStateDir(dir string) (func(), error) {
	file, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, errors.New("another alert run holds the state lock")
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}
