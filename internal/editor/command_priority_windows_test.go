//go:build windows

package editor

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// runDiagnosticFFmpeg is the single execution choke point, so the scheduling
// class must be applied there and the process must still run and be timed
// exactly as before.
func TestRunDiagnosticFFmpegAppliesBackgroundPriority(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	for _, tc := range []struct {
		name       string
		background bool
	}{
		{"critical path", false},
		{"background", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if tc.background {
				ctx = withBackgroundProcessPriority(ctx)
			}
			cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-version")
			if err := runDiagnosticFFmpeg(ctx, cmd, "priority canary"); err != nil {
				t.Fatal(err)
			}
			if !tc.background {
				if cmd.SysProcAttr != nil {
					t.Fatalf("critical-path command was rewritten: %+v", cmd.SysProcAttr)
				}
				return
			}
			if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.BELOW_NORMAL_PRIORITY_CLASS == 0 {
				t.Fatalf("background command kept its scheduling class: %+v", cmd.SysProcAttr)
			}
		})
	}
}

// childFFmpegPriorityClasses reads the live scheduling class of every ffmpeg
// process this test binary has started and that is still running.
func childFFmpegPriorityClasses(t *testing.T) []uint32 {
	t.Helper()
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	self := uint32(os.Getpid())
	var classes []uint32
	for err := windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ParentProcessID != self || !strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "ffmpeg.exe") {
			continue
		}
		process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
		if err != nil {
			continue // Already gone.
		}
		class, err := windows.GetPriorityClass(process)
		_ = windows.CloseHandle(process)
		if err == nil {
			classes = append(classes, class)
		}
	}
	return classes
}

// The wiring, observed on the live child processes: every FFmpeg process the
// video-only item pool starts runs in BELOW_NORMAL, and every process the
// audio-only pool starts, a link of the audio critical path, keeps NORMAL.
func TestFullDemoItemPoolsStartTheirProcessesInTheirPriorityClass(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dir := t.TempDir()
	short, _, _ := fullDemoTransitionCanaryShort(t, ctx, ffmpeg, dir)
	if err := prepareFullDemoTransitions(ctx, &short); err != nil {
		t.Fatal(err)
	}
	if err := prepareFullDemoTracks(ctx, &short, nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		streams fullDemoItemStreams
		want    uint32
	}{
		{"video only", fullDemoItemVideoOnly, windows.BELOW_NORMAL_PRIORITY_CLASS},
		{"audio only", fullDemoItemAudioOnly, windows.NORMAL_PRIORITY_CLASS},
	} {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				_, err := runFullDemoItemPoolJobs(ctx, short, tc.streams, nil, 1)
				done <- err
			}()
			observed := map[uint32]int{}
			for {
				for _, class := range childFFmpegPriorityClasses(t) {
					observed[class]++
				}
				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
					if len(observed) != 1 || observed[tc.want] == 0 {
						t.Fatalf("observed priority classes %#v, want only %#x", observed, tc.want)
					}
					return
				default:
					time.Sleep(time.Millisecond)
				}
			}
		})
	}
}

// Existing creation flags must survive, and a repeated application must not
// accumulate anything.
func TestSetBackgroundProcessPriorityPreservesCreationFlags(t *testing.T) {
	cmd := exec.Command("ffmpeg", "-version")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	setBackgroundProcessPriority(cmd)
	setBackgroundProcessPriority(cmd)
	want := uint32(windows.CREATE_NEW_PROCESS_GROUP | windows.BELOW_NORMAL_PRIORITY_CLASS)
	if cmd.SysProcAttr.CreationFlags != want {
		t.Fatalf("creation flags = %#x, want %#x", cmd.SysProcAttr.CreationFlags, want)
	}
}
