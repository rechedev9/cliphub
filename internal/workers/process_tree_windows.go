//go:build windows

package workers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	treeAccess = windows.PROCESS_TERMINATE | windows.SYNCHRONIZE | windows.PROCESS_QUERY_LIMITED_INFORMATION
	// treeExitTimeout bounds the wait for terminated processes to be gone; a
	// CS2 that holds the GPU takes a few seconds to leave.
	treeExitTimeout = 20 * time.Second
	// treeMaxRounds bounds the walk; every round ends one more generation.
	treeMaxRounds = 32
)

type listedProcess struct {
	pid    uint32
	parent uint32
	exe    string
}

func listProcesses() ([]listedProcess, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ProcessEntry32
	// #nosec G103 G115 -- Process32First needs the size of its own struct.
	entry.Size = uint32(unsafe.Sizeof(entry))
	var listed []listedProcess
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		listed = append(listed, listedProcess{
			pid:    entry.ProcessID,
			parent: entry.ParentProcessID,
			exe:    windows.UTF16ToString(entry.ExeFile[:]),
		})
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	return listed, nil
}

// treeMember is a process that is being ended, held open so its pid cannot
// be given to another process before the walk is over.
type treeMember struct {
	handle  windows.Handle
	created int64
}

func openTreeMember(pid uint32) (treeMember, error) {
	handle, err := windows.OpenProcess(treeAccess, false, pid)
	if err != nil {
		return treeMember{}, err
	}
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		_ = windows.CloseHandle(handle)
		return treeMember{}, err
	}
	return treeMember{handle: handle, created: created.Nanoseconds()}, nil
}

// endDescendants terminates every process the members started, directly or
// through a child, waits until all of them (members included) are gone, and
// closes their handles. The members themselves must already be terminated:
// a dead process cannot start anything, so each round of the walk finds one
// more generation and the walk ends when a listing shows nothing new.
func endDescendants(members map[uint32]treeMember) (ended int, err error) {
	defer func() {
		for _, member := range members {
			_ = windows.CloseHandle(member.handle)
		}
	}()
	for range treeMaxRounds {
		listed, listErr := listProcesses()
		if listErr != nil {
			err = listErr
			break
		}
		var listedAt windows.Filetime
		windows.GetSystemTimePreciseAsFileTime(&listedAt)
		found := 0
		for _, process := range listed {
			parent, hasParent := members[process.parent]
			if _, known := members[process.pid]; known || !hasParent {
				continue
			}
			member, openErr := openTreeMember(process.pid)
			if openErr != nil {
				// Already gone, or a process this user may not end.
				continue
			}
			// A process older than its recorded parent only shares a reused
			// pid with it; one newer than the listing took a listed pid over.
			if member.created < parent.created || member.created > listedAt.Nanoseconds() {
				_ = windows.CloseHandle(member.handle)
				continue
			}
			_ = windows.TerminateProcess(member.handle, 1)
			members[process.pid] = member
			found++
		}
		ended += found
		if found == 0 {
			break
		}
	}
	deadline := time.Now().Add(treeExitTimeout)
	for pid, member := range members {
		remaining := max(time.Until(deadline), 0)
		// #nosec G115 -- remaining is between zero and treeExitTimeout.
		event, waitErr := windows.WaitForSingleObject(member.handle, uint32(remaining.Milliseconds()))
		if waitErr != nil || event != windows.WAIT_OBJECT_0 {
			err = errors.Join(err, fmt.Errorf("process %d was still running %s after it was terminated", pid, treeExitTimeout))
		}
	}
	return ended, err
}

// killProcessTree ends a command and everything it started, and returns once
// they are gone. The command dies first, exactly as os/exec would kill it.
func killProcessTree(process *os.Process, tool string) error {
	// #nosec G115 -- the pid of a started process is positive.
	pid := uint32(process.Pid)
	// Opened before the kill: afterwards the pid may no longer resolve.
	root, openErr := openTreeMember(pid)
	if err := process.Kill(); err != nil {
		if openErr == nil {
			_ = windows.CloseHandle(root.handle)
		}
		return err
	}
	if openErr != nil {
		return nil
	}
	ended, err := endDescendants(map[uint32]treeMember{pid: root})
	logProcessTree(tool, ended, err)
	return nil
}

func imagePath(handle windows.Handle) (string, error) {
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buffer[:size]), nil
}

// endProcessesOf terminates every running process whose executable is one of
// the given files, with everything those processes started, and waits for
// them. It returns how many processes it ended.
func endProcessesOf(executables []string) (int, error) {
	wanted := map[string]bool{}
	names := map[string]bool{}
	for _, executable := range executables {
		absolute, err := filepath.Abs(executable)
		if executable == "" || err != nil {
			continue
		}
		wanted[strings.ToLower(filepath.Clean(absolute))] = true
		names[strings.ToLower(filepath.Base(absolute))] = true
	}
	if len(wanted) == 0 {
		return 0, nil
	}
	listed, err := listProcesses()
	if err != nil {
		return 0, err
	}
	// #nosec G115 -- a pid is positive.
	self := uint32(os.Getpid())
	roots := map[uint32]treeMember{}
	for _, process := range listed {
		if process.pid == self || !names[strings.ToLower(process.exe)] {
			continue
		}
		member, err := openTreeMember(process.pid)
		if err != nil {
			continue
		}
		path, err := imagePath(member.handle)
		if err != nil || !wanted[strings.ToLower(filepath.Clean(path))] {
			_ = windows.CloseHandle(member.handle)
			continue
		}
		_ = windows.TerminateProcess(member.handle, 1)
		roots[process.pid] = member
	}
	if len(roots) == 0 {
		return 0, nil
	}
	found := len(roots)
	ended, err := endDescendants(roots)
	return found + ended, err
}
