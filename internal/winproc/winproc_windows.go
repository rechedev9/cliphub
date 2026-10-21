//go:build windows

package winproc

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	getWindowTextW = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowTextW")

	// NewCallback slots are never freed, so the window walk owns exactly one
	// and passes its state through enumState under enumMu.
	enumMu       sync.Mutex
	enumState    map[uint32]bool
	enumTitles   []string
	enumCallback = windows.NewCallback(collectWindowTitle)
)

// List returns every process on the system.
func List() ([]Process, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ProcessEntry32
	// #nosec G103 G115 -- Process32First needs the size of its own struct.
	entry.Size = uint32(unsafe.Sizeof(entry))
	var listed []Process
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		listed = append(listed, Process{
			PID:       entry.ProcessID,
			ParentPID: entry.ParentProcessID,
			Exe:       windows.UTF16ToString(entry.ExeFile[:]),
		})
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	return listed, nil
}

// Running reports whether a process with this image name exists, compared
// without case as Windows compares file names.
func Running(image string) (bool, error) {
	pids, err := pidsOf(image)
	return len(pids) > 0, err
}

// WindowTitles reports whether image runs and the titles of its visible
// top-level windows, including a modal dialog that replaced the main window.
func WindowTitles(image string) (running bool, titles []string, err error) {
	pids, err := pidsOf(image)
	if err != nil || len(pids) == 0 {
		return false, nil, err
	}
	enumMu.Lock()
	defer enumMu.Unlock()
	enumState, enumTitles = pids, nil
	defer func() { enumState, enumTitles = nil, nil }()
	if err := windows.EnumWindows(enumCallback, nil); err != nil {
		return true, nil, fmt.Errorf("list windows of %s: %w", image, err)
	}
	return true, enumTitles, nil
}

func pidsOf(image string) (map[uint32]bool, error) {
	listed, err := List()
	if err != nil {
		return nil, err
	}
	pids := map[uint32]bool{}
	for _, process := range listed {
		if strings.EqualFold(process.Exe, image) {
			pids[process.PID] = true
		}
	}
	return pids, nil
}

func collectWindowTitle(hwnd windows.HWND, _ uintptr) uintptr {
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil || !enumState[pid] || !windows.IsWindowVisible(hwnd) {
		return 1
	}
	var buffer [512]uint16
	// #nosec G103 -- GetWindowTextW writes at most len(buffer) UTF-16 units.
	length, _, _ := getWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if length > 0 {
		enumTitles = append(enumTitles, windows.UTF16ToString(buffer[:length]))
	}
	return 1
}
