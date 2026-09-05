//go:build windows

package recording

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// SettingsOwnerAlive fails closed if Windows cannot establish whether the
// process that owns a recovery journal has exited. PID reuse is conservative.
func SettingsOwnerAlive(pid int) (bool, error) {
	if pid <= 0 || uint64(pid) > uint64(^uint32(0)) {
		return false, fmt.Errorf("invalid settings journal owner")
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check settings journal owner: %w", err)
	}
	defer windows.CloseHandle(h)
	state, err := windows.WaitForSingleObject(h, 0)
	if err != nil {
		return false, err
	}
	return state != windows.WAIT_OBJECT_0, nil
}
