//go:build !windows

package workers

import "os"

// killProcessTree ends the command the way os/exec does. Capture and render
// only run on Windows, where the whole tree is ended.
func killProcessTree(process *os.Process, _ string) error {
	return process.Kill()
}

func endProcessesOf([]string) (int, error) {
	return 0, nil
}
