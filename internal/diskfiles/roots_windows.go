package diskfiles

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// Roots returns the root of every fixed local drive, whatever its letter.
func Roots() []string {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		if drive := os.Getenv("SystemDrive"); drive != "" {
			return []string{drive + `\`}
		}
		return nil
	}
	roots := []string{}
	for i := range 26 {
		if mask&(1<<i) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		pointer, err := windows.UTF16PtrFromString(root)
		if err == nil && windows.GetDriveType(pointer) == windows.DRIVE_FIXED {
			roots = append(roots, root)
		}
	}
	return roots
}

// Reveal opens File Explorer with path selected. The command line is built by hand
// because Explorer splits an unquoted path at a comma.
func Reveal(path string) error {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	// explorer.exe exits 1 even when it opened the window, so its error is ignored.
	_ = cmd.Run()
	return nil
}
