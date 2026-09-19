//go:build windows

package editor

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// setBackgroundProcessPriority starts the process in BELOW_NORMAL. It is a
// scheduling class only: FFmpeg's options, filters and encoders are untouched,
// so the produced bytes are identical to a NORMAL run. Existing creation flags
// are preserved.
func setBackgroundProcessPriority(cmd *exec.Cmd) {
	attributes := cmd.SysProcAttr
	if attributes == nil {
		attributes = &syscall.SysProcAttr{}
	}
	attributes.CreationFlags |= windows.BELOW_NORMAL_PRIORITY_CLASS
	cmd.SysProcAttr = attributes
}
