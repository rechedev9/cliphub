//go:build windows

package main

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	jobMsgProcessMemoryLimit = 9
	jobMsgNotificationLimit  = 11
	memoryLimitExitCode      = 3
)

var procGetQueuedCompletionStatus = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetQueuedCompletionStatus")

type jobCompletionPort struct {
	CompletionKey  uintptr
	CompletionPort windows.Handle
}

type memoryJob struct {
	job  windows.Handle
	port windows.Handle
}

type jobNotificationLimit struct {
	IoReadBytesLimit             uint64
	IoWriteBytesLimit            uint64
	PerJobUserTimeLimit          int64
	JobMemoryLimit               uint64
	RateControlTolerance         uint32
	RateControlToleranceInterval uint32
	LimitFlags                   uint32
}

func limitProcessMemory(limit uintptr) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("create the memory job: %w", err)
	}
	port, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 1)
	if err != nil {
		_ = windows.CloseHandle(job)
		return fmt.Errorf("create the memory job port: %w", err)
	}
	j := memoryJob{job: job, port: port}
	if err := j.setLimits(limit); err != nil {
		j.close()
		return err
	}
	if err := windows.AssignProcessToJobObject(job, windows.CurrentProcess()); err != nil {
		j.close()
		return fmt.Errorf("join the memory job: %w", err)
	}
	go endAtMemoryLimit(port, limit)
	return nil
}

func (j memoryJob) setLimits(limit uintptr) error {
	assoc := jobCompletionPort{CompletionKey: uintptr(j.job), CompletionPort: j.port}
	if _, err := windows.SetInformationJobObject(j.job, windows.JobObjectAssociateCompletionPortInformation, uintptr(unsafe.Pointer(&assoc)), uint32(unsafe.Sizeof(assoc))); err != nil {
		return fmt.Errorf("watch the memory job: %w", err)
	}
	notify := jobNotificationLimit{JobMemoryLimit: uint64(limit), LimitFlags: windows.JOB_OBJECT_LIMIT_JOB_MEMORY}
	if _, err := windows.SetInformationJobObject(j.job, windows.JobObjectNotificationLimitInformation, uintptr(unsafe.Pointer(&notify)), uint32(unsafe.Sizeof(notify))); err != nil {
		return fmt.Errorf("set the memory notice: %w", err)
	}
	hard := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY},
		ProcessMemoryLimit:    limit + limit/2,
	}
	if _, err := windows.SetInformationJobObject(j.job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&hard)), uint32(unsafe.Sizeof(hard))); err != nil {
		return fmt.Errorf("set the memory limit: %w", err)
	}
	return nil
}

func endAtMemoryLimit(port windows.Handle, limit uintptr) {
	notice := fmt.Sprintf("zv-mcp: the process reached its %d MiB memory limit, most likely in run_script; exiting\n", limit>>20)
	for {
		var msg uint32
		var key, pid uintptr
		ok, _, _ := procGetQueuedCompletionStatus.Call(uintptr(port), uintptr(unsafe.Pointer(&msg)), uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&pid)), uintptr(windows.INFINITE))
		if ok == 0 {
			return
		}
		if msg == jobMsgNotificationLimit || msg == jobMsgProcessMemoryLimit {
			_, _ = os.Stderr.WriteString(notice)
			_ = windows.TerminateProcess(windows.CurrentProcess(), memoryLimitExitCode)
		}
	}
}

func (j memoryJob) close() {
	_ = windows.CloseHandle(j.job)
	_ = windows.CloseHandle(j.port)
}
