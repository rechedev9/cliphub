//go:build windows

package cloudbridge

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

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
