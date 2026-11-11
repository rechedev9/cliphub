package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// copyFileToClipboard puts a file on the Windows clipboard as a file, the way
// Explorer's Copy does, so a document pastes it whole. The path travels in the
// environment, never in the command line.
func copyFileToClipboard(ctx context.Context, path string) error {
	if runtime.GOOS != "windows" {
		return errors.New("the clipboard copy needs Windows")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Set-Clipboard -LiteralPath $env:CLIPHUB_CLIPBOARD_FILE")
	cmd.Env = append(os.Environ(), "CLIPHUB_CLIPBOARD_FILE="+path)
	return cmd.Run()
}
