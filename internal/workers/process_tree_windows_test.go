//go:build windows

package workers

import (
	"context"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func processAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	event, _ := windows.WaitForSingleObject(handle, 0)
	return event == uint32(windows.WAIT_TIMEOUT)
}

// endHelpers makes sure no helper outlives the test, whatever it proved.
func endHelpers(t *testing.T, pids []int) {
	t.Cleanup(func() {
		for _, pid := range pids {
			if process, err := os.FindProcess(pid); err == nil {
				_ = process.Kill()
			}
		}
	})
}

// A render is the editor plus the ffmpeg and renderer processes it starts.
// Canceling it must take all of them down before the worker moves on, or
// they keep the GPU busy under the next capture.
func TestCancelingACommandEndsEverythingItStartedBeforeRunReturns(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(treeHelperDirEnv, dir)
	t.Setenv(treeHelperDepthEnv, "2")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := execCommandRunner{}.Run(ctx, os.Args[0])
		done <- err
	}()
	// The tool, what it started, and what that started in turn.
	pids := treeHelperPIDs(t, dir, 3)
	endHelpers(t, pids)

	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run of a canceled command returned no error")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Run did not return after its context was canceled")
	}
	for generation, pid := range pids {
		if processAlive(pid) {
			t.Fatalf("generation %d of the command's tree (pid %d) is still running after Run returned", generation, pid)
		}
	}
}

func TestACommandTimeoutEndsEverythingItStarted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(treeHelperDirEnv, dir)
	t.Setenv(treeHelperDepthEnv, "1")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := make(chan []int, 1)
	go func() { started <- treeHelperPIDs(t, dir, 2) }()

	if _, err := (execCommandRunner{}).Run(ctx, os.Args[0]); err == nil {
		t.Fatal("Run of a command that timed out returned no error")
	}

	pids := <-started
	endHelpers(t, pids)
	for generation, pid := range pids {
		if processAlive(pid) {
			t.Fatalf("generation %d of the command's tree (pid %d) is still running after the timeout", generation, pid)
		}
	}
}

// Only a canceled command loses its tree: one that ends by itself keeps
// whatever it left running, as before.
func TestACommandThatEndsByItselfLeavesWhatItStartedAlone(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(treeHelperDirEnv, dir)
	t.Setenv(treeHelperDepthEnv, "1")
	t.Setenv(treeHelperExitEnv, "1")

	if _, err := (execCommandRunner{}).Run(context.Background(), os.Args[0]); err != nil {
		t.Fatalf("Run error = %v", err)
	}

	pids := treeHelperPIDs(t, dir, 2)
	endHelpers(t, pids)
	if started := pids[0]; !processAlive(started) {
		t.Fatalf("the process (pid %d) a finished command left running was ended", started)
	}
}
