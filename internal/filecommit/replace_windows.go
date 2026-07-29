//go:build windows

package filecommit

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

var replaceFileW = syscall.NewLazyDLL("kernel32.dll").NewProc("ReplaceFileW")

func replace(attempt, destination string) error {
	destinationPtr, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	attemptPtr, err := syscall.UTF16PtrFromString(attempt)
	if err != nil {
		return err
	}
	replaced, _, callErr := replaceFileW.Call(
		// #nosec G103 -- audited Win32 ReplaceFileW FFI; pointers live through Call.
		uintptr(unsafe.Pointer(destinationPtr)),
		// #nosec G103 -- audited Win32 ReplaceFileW FFI; pointers live through Call.
		uintptr(unsafe.Pointer(attemptPtr)),
		0, 0, 0, 0,
	)
	if replaced != 0 {
		return nil
	}
	if errors.Is(callErr, syscall.ERROR_FILE_NOT_FOUND) || errors.Is(callErr, syscall.ERROR_PATH_NOT_FOUND) {
		return os.Rename(attempt, destination)
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return syscall.EINVAL
}
