//go:build windows

package cloudbridge

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

const processListTimeout = 15 * time.Second

// diskUsage reports the free and total bytes of the volume holding path.
func diskUsage(path string) (free, total uint64, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, 0, err
	}
	pathUTF16, err := windows.UTF16PtrFromString(abs)
	if err != nil {
		return 0, 0, err
	}
	var freeToCaller, totalBytes, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(pathUTF16, &freeToCaller, &totalBytes, &totalFree); err != nil {
		return 0, 0, fmt.Errorf("disk usage of %s: %w", abs, err)
	}
	return freeToCaller, totalBytes, nil
}

// processRunning uses the same tasklist filter as zv-recorder, so the worker
// and the recorder agree on whether cs2.exe is alive.
func processRunning(ctx context.Context, image string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, processListTimeout)
	defer cancel()
	// #nosec G204 -- tasklist is fixed and image is a constant process name.
	out, err := exec.CommandContext(ctx, "tasklist", "/FI", "IMAGENAME eq "+image, "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false, fmt.Errorf("list %s processes: %w", image, err)
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(image)), nil
}
