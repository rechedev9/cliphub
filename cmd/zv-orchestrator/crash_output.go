package main

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
)

// crashKeep is how long a boot's fatal error stays readable for the scans it left running.
const crashKeep = 30 * 24 * time.Hour

// crashLog keeps each boot's fatal errors in its own file; a forced kill (closing Studio) leaves it empty.
type crashLog struct {
	dir  string
	boot string
}

// openCrashLog names this boot and points the runtime's fatal errors at its file. It drops the
// empty files of earlier boots, which ended without one, and the old reports.
func openCrashLog(dataDir string) (crashLog, error) {
	dir := filepath.Join(dataDir, "crashes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return crashLog{}, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return crashLog{}, err
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || entry.IsDir() || (info.Size() > 0 && time.Since(info.ModTime()) < crashKeep) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
	c := crashLog{dir: dir, boot: uuid.NewString()}
	f, err := os.Create(filepath.Join(dir, c.boot+".txt"))
	if err != nil {
		return crashLog{}, err
	}
	defer f.Close()
	return c, debug.SetCrashOutput(f, debug.CrashOptions{})
}

// crashed reports whether the given boot died of a fatal error.
func (c crashLog) crashed(boot string) bool {
	if _, err := uuid.Parse(boot); err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(c.dir, boot+".txt"))
	return err == nil && info.Size() > 0
}
