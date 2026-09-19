//go:build !windows

package editor

import "os/exec"

// Only Windows exposes a per-process scheduling class through the process
// creation flags os/exec already carries. Elsewhere the mark is inert, so the
// render behaves exactly as it did before.
func setBackgroundProcessPriority(*exec.Cmd) {}
