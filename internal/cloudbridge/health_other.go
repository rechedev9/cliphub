//go:build !windows

package cloudbridge

import (
	"context"
	"fmt"

	"golang.org/x/sys/unix"
)

// diskUsage reports the free and total bytes of the volume holding path.
func diskUsage(path string) (free, total uint64, err error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, 0, fmt.Errorf("disk usage of %s: %w", path, err)
	}
	blockSize := uint64(stat.Bsize) //nolint:gosec // block sizes are positive
	return uint64(stat.Bavail) * blockSize, uint64(stat.Blocks) * blockSize, nil
}

// processRunning always answers no: capture, Steam and CS2 are Windows only,
// so a worker on any other system stays blocked instead of claiming jobs.
func processRunning(context.Context, string) (bool, error) {
	return false, nil
}
