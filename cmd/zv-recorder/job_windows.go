//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// captureJob is a Windows job object that owns the HLAE launcher and, through
// it, the cs2.exe capture process. It is configured with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE so that closing the job handle terminates
// every process still in the job — an OS-guaranteed teardown of the whole
// capture tree that backstops the grace-gate close in
// waitForWindowsProcessRunAndExit. If a capture ever leaves cs2.exe alive on a
// return path — a failed taskkill, an early error, or a hang the grace window
// did not resolve — closing the job still tears it down deterministically.
type captureJob struct {
	handle windows.Handle
}

// newCaptureJob creates a kill-on-close job object. Its failure is reported to
// the caller so the launch can log it and continue: the grace-gate close, not
// the job, is the primary deterministic teardown.
func newCaptureJob() (*captureJob, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create capture job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("configure capture job object kill-on-close: %w", err)
	}
	return &captureJob{handle: handle}, nil
}

// assign puts the process (by PID) into the job. cs2.exe, launched by the HLAE
// loader as a descendant, inherits the job unless it breaks away; when it does,
// the grace-gate close in waitForWindowsProcessRunAndExit remains the
// deterministic fallback. Assignment happens right after the launcher starts,
// well before the loader spawns cs2.exe, so the whole tree is captured.
func (j *captureJob) assign(pid int) error {
	if j == nil || j.handle == 0 {
		return nil
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("open HLAE process %d for capture job: %w", pid, err)
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(j.handle, process); err != nil {
		return fmt.Errorf("assign HLAE process %d to capture job: %w", pid, err)
	}
	return nil
}

// close terminates every process still in the job (kill-on-close) and releases
// the handle. It is safe to call more than once and on a nil job.
func (j *captureJob) close() error {
	if j == nil || j.handle == 0 {
		return nil
	}
	handle := j.handle
	j.handle = 0
	if err := windows.CloseHandle(handle); err != nil {
		return fmt.Errorf("close capture job object: %w", err)
	}
	return nil
}
