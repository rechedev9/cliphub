//go:build windows

package storage

import (
	"errors"
	"os"
	"syscall"
	"time"
	"unsafe"
)

var replaceFileW = syscall.NewLazyDLL("kernel32.dll").NewProc("ReplaceFileW")

const (
	windowsAccessDenied     syscall.Errno = 5
	windowsSharingViolation syscall.Errno = 32
)

// Retry shape for a denied open of a file that is being replaced: exponential
// backoff from a sub-millisecond-scale first wait, capped per sleep so a long
// wait stays responsive, and bounded overall by openRetryBudget.
const (
	openRetryBudget    = 250 * time.Millisecond
	openRetryFirstWait = time.Millisecond
	openRetryMaxWait   = 25 * time.Millisecond
)

// openLocalFile opens a stored artifact for reading, tolerating the short
// windows in which Windows refuses a new handle on a file that is being
// replaced.
//
// ReplaceFileW is atomic: a reader sees either the previous or the next
// complete generation, never a partial one. The operating system around it is
// not as quiet. Virus scanners and filesystem filter drivers hold transient
// handles on a file that was just written, and while they do CreateFile answers
// ERROR_SHARING_VIOLATION (32) or ERROR_ACCESS_DENIED (5). Studio polls render
// status.json continuously for the whole length of a render, so that
// millisecond-scale window must not turn into a terminal 500 response: for a
// localhost status document, waiting a few hundred milliseconds is strictly
// better than failing the poll. Every other error, a missing key above all, is
// returned on the first attempt, and when the budget is spent the caller still
// gets the original *os.PathError.
func openLocalFile(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	var handle syscall.Handle
	deadline := time.Now().Add(openRetryBudget)
	wait := openRetryFirstWait
	for {
		handle, err = syscall.CreateFile(
			name,
			syscall.GENERIC_READ,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
			nil,
			syscall.OPEN_EXISTING,
			syscall.FILE_ATTRIBUTE_NORMAL,
			0,
		)
		if err == nil || (!errors.Is(err, windowsSharingViolation) && !errors.Is(err, windowsAccessDenied)) {
			break
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		sleep := wait
		if sleep > remaining {
			sleep = remaining
		}
		time.Sleep(sleep)
		if wait < openRetryMaxWait {
			wait *= 2
			if wait > openRetryMaxWait {
				wait = openRetryMaxWait
			}
		}
	}
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		_ = syscall.CloseHandle(handle)
		return nil, &os.PathError{Op: "open", Path: path, Err: syscall.EINVAL}
	}
	return file, nil
}

func replaceLocalFile(tempPath, destinationPath string) error {
	destination, err := syscall.UTF16PtrFromString(destinationPath)
	if err != nil {
		return err
	}
	temp, err := syscall.UTF16PtrFromString(tempPath)
	if err != nil {
		return err
	}
	replaced, _, callErr := replaceFileW.Call(
		// #nosec G103 -- audited Win32 ReplaceFileW FFI; pointers remain live for the duration of Call.
		uintptr(unsafe.Pointer(destination)),
		// #nosec G103 -- audited Win32 ReplaceFileW FFI; pointers remain live for the duration of Call.
		uintptr(unsafe.Pointer(temp)),
		0,
		0,
		0,
		0,
	)
	if replaced != 0 {
		return nil
	}
	if errors.Is(callErr, syscall.ERROR_FILE_NOT_FOUND) || errors.Is(callErr, syscall.ERROR_PATH_NOT_FOUND) {
		return os.Rename(tempPath, destinationPath)
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return syscall.EINVAL
}
