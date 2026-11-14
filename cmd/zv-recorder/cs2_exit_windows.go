//go:build windows

package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Job object messages posted to the associated completion port (winnt.h).
const (
	jobMsgNewProcess          = 6
	jobMsgAbnormalExitProcess = 8
	stillActiveExitCode       = 259
)

// crashReportWait covers the seconds Windows Error Reporting takes to log a crash.
const crashReportWait = 6 * time.Second

var procGetQueuedCompletionStatus = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetQueuedCompletionStatus")

type jobAssociateCompletionPort struct {
	CompletionKey  uintptr
	CompletionPort windows.Handle
}

type jobProcess struct {
	pid      uint32
	handle   windows.Handle
	abnormal bool
}

type cs2ExitInfo struct {
	known    bool
	pid      uint32
	code     uint32
	abnormal bool
	lifetime time.Duration
}

type cs2Crash struct {
	module    string
	version   string
	exception string
	offset    string
}

func (j *captureJob) watchProcesses() error {
	port, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 1)
	if err != nil {
		return fmt.Errorf("create capture job completion port: %w", err)
	}
	assoc := jobAssociateCompletionPort{CompletionKey: uintptr(j.handle), CompletionPort: port}
	if _, err := windows.SetInformationJobObject(
		j.handle,
		windows.JobObjectAssociateCompletionPortInformation,
		// #nosec G103 -- SetInformationJobObject takes a raw pointer to the struct.
		uintptr(unsafe.Pointer(&assoc)),
		uint32(unsafe.Sizeof(assoc)),
	); err != nil {
		_ = windows.CloseHandle(port)
		return fmt.Errorf("associate capture job completion port: %w", err)
	}
	j.port = port
	j.watched = make(chan struct{})
	go j.readJobMessages(port)
	return nil
}

// readJobMessages calls the raw API because a job message carries a PID, not
// an OVERLAPPED pointer, in the overlapped slot.
func (j *captureJob) readJobMessages(port windows.Handle) {
	defer close(j.watched)
	for {
		var msg uint32
		var key, pid uintptr
		ok, _, _ := procGetQueuedCompletionStatus.Call(
			uintptr(port),
			// #nosec G103 -- out parameters of GetQueuedCompletionStatus.
			uintptr(unsafe.Pointer(&msg)), uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&pid)),
			uintptr(windows.INFINITE),
		)
		if ok == 0 {
			return
		}
		// #nosec G115 -- job messages carry a DWORD process id.
		j.recordJobMessage(msg, uint32(pid))
	}
}

func (j *captureJob) recordJobMessage(msg, pid uint32) {
	switch msg {
	case jobMsgNewProcess:
		j.trackIfCS2(pid)
	case jobMsgAbnormalExitProcess:
		j.mu.Lock()
		if j.cs2 != nil && j.cs2.pid == pid {
			j.cs2.abnormal = true
		}
		j.mu.Unlock()
	}
}

func (j *captureJob) trackIfCS2(pid uint32) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return
	}
	if !strings.EqualFold(filepath.Base(processImage(handle)), "cs2.exe") {
		_ = windows.CloseHandle(handle)
		return
	}
	j.mu.Lock()
	previous := j.cs2
	j.cs2 = &jobProcess{pid: pid, handle: handle}
	j.mu.Unlock()
	if previous != nil {
		_ = windows.CloseHandle(previous.handle)
	}
}

func processImage(handle windows.Handle) string {
	buf := make([]uint16, 1024)
	// #nosec G115 -- the buffer length is a small constant.
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:size])
}

// stopWatching runs after the job handle is closed, so no new process can join.
func (j *captureJob) stopWatching() {
	if j.port != 0 {
		_ = windows.CloseHandle(j.port)
		j.port = 0
		<-j.watched
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.cs2 != nil {
		_ = windows.CloseHandle(j.cs2.handle)
		j.cs2 = nil
	}
}

func (j *captureJob) cs2Exit() cs2ExitInfo {
	if j == nil {
		return cs2ExitInfo{}
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.cs2 == nil {
		return cs2ExitInfo{}
	}
	info := cs2ExitInfo{pid: j.cs2.pid, abnormal: j.cs2.abnormal}
	if err := windows.GetExitCodeProcess(j.cs2.handle, &info.code); err != nil || info.code == stillActiveExitCode {
		return info
	}
	info.known = true
	// The abnormal-exit message can land after the caller saw the exit; an NTSTATUS error code says the same.
	info.abnormal = info.abnormal || info.code&0xC0000000 == 0xC0000000
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(j.cs2.handle, &created, &exited, &kernel, &user) == nil {
		info.lifetime = time.Duration(exited.Nanoseconds() - created.Nanoseconds())
	}
	return info
}

// explainExit logs how cs2.exe ended and adds it to a missing-attestation
// reason, which otherwise only says that CS2 exited.
func (j *captureJob) explainExit(err error) error {
	exit := j.cs2Exit()
	since := time.Now().Add(-10 * time.Minute)
	if j != nil {
		since = j.created
	}
	crash, crashed := findCS2Crash(exit, since)
	summary := cs2ExitSummary(exit, crash, crashed)
	log.Printf("cs2 exit: %s", summary)
	var verification *captureVerificationError
	if errors.As(err, &verification) {
		verification.reason += " (" + summary + ")"
	}
	return err
}

func findCS2Crash(exit cs2ExitInfo, since time.Time) (cs2Crash, bool) {
	deadline := time.Now().Add(crashReportWait)
	for {
		if crash, ok := queryCS2Crash(exit.pid, since); ok {
			return crash, true
		}
		if (exit.known && !exit.abnormal) || time.Now().After(deadline) {
			return cs2Crash{}, false
		}
		time.Sleep(time.Second)
	}
}

func queryCS2Crash(pid uint32, since time.Time) (cs2Crash, bool) {
	lookback := time.Since(since) + time.Minute
	query := fmt.Sprintf(
		"*[System[Provider[@Name='Application Error'] and EventID=1000 and TimeCreated[timediff(@SystemTime) <= %d]]]",
		lookback.Milliseconds(),
	)
	// #nosec G204 -- wevtutil is fixed and the query is built from numbers only.
	out, err := exec.Command("wevtutil", "qe", "Application", "/q:"+query, "/rd:true", "/c:20", "/f:xml").Output()
	if err != nil {
		return cs2Crash{}, false
	}
	return parseCS2Crash(out, pid)
}

// parseCS2Crash returns the newest Application Error event for cs2.exe, and
// only for pid when the job saw that process.
func parseCS2Crash(events []byte, pid uint32) (cs2Crash, bool) {
	decoder := xml.NewDecoder(bytes.NewReader(bytes.ToValidUTF8(events, []byte("?"))))
	for {
		var event struct {
			Data []struct {
				Name  string `xml:"Name,attr"`
				Value string `xml:",chardata"`
			} `xml:"EventData>Data"`
		}
		if err := decoder.Decode(&event); err != nil {
			return cs2Crash{}, false
		}
		fields := make(map[string]string, len(event.Data))
		for _, data := range event.Data {
			fields[data.Name] = strings.TrimSpace(data.Value)
		}
		if !strings.EqualFold(fields["AppName"], "cs2.exe") {
			continue
		}
		if pid != 0 && parseHex(fields["ProcessId"]) != uint64(pid) {
			continue
		}
		return cs2Crash{
			module:    fields["ModuleName"],
			version:   fields["ModuleVersion"],
			exception: fields["ExceptionCode"],
			offset:    fields["FaultingOffset"],
		}, true
	}
}

func parseHex(value string) uint64 {
	parsed, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(value), "0x"), 16, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func cs2ExitSummary(exit cs2ExitInfo, crash cs2Crash, crashed bool) string {
	parts := []string{"cs2.exe exit code unknown"}
	if exit.known {
		parts[0] = fmt.Sprintf("cs2.exe exit code 0x%08X%s after %.1fs", exit.code, exitCodeName(exit.code), exit.lifetime.Seconds())
	}
	switch {
	case crashed:
		parts = append(parts, fmt.Sprintf("Windows crash report: exception 0x%08X in %s %s at offset 0x%X",
			parseHex(crash.exception), crash.module, crash.version, parseHex(crash.offset)))
	case exit.abnormal:
		parts = append(parts, "abnormal exit with no Windows crash report")
	default:
		parts = append(parts, "no Windows crash report")
	}
	return strings.Join(parts, "; ")
}

func exitCodeName(code uint32) string {
	names := map[uint32]string{
		0:          "clean exit",
		0x40010004: "terminated by another process",
		0x80000003: "breakpoint",
		0xC0000005: "access violation",
		0xC000001D: "illegal instruction",
		0xC00000FD: "stack overflow",
		0xC0000135: "DLL not found",
		0xC0000142: "DLL initialization failed",
		0xC0000374: "heap corruption",
		0xC0000409: "stack buffer overrun or fail fast",
		0xC0000602: "fail fast",
		0xE06D7363: "unhandled C++ exception",
	}
	if name, ok := names[code]; ok {
		return " (" + name + ")"
	}
	return ""
}
