// Package filecommit publishes generated files through same-directory attempt
// paths so a failed producer can never corrupt the last committed artifact.
package filecommit

import (
	"fmt"
	"os"
	"path/filepath"
)

// Attempt returns a unique, absent path beside destination with the same
// extension. Producers such as FFmpeg infer the container from that extension.
func Attempt(destination string) (string, func(), error) {
	dir := filepath.Dir(destination)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", nil, err
	}
	ext := filepath.Ext(destination)
	base := filepath.Base(destination)
	base = base[:len(base)-len(ext)]
	f, err := os.CreateTemp(dir, "."+base+".attempt-*"+ext)
	if err != nil {
		return "", nil, err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", nil, err
	}
	if err := os.Remove(name); err != nil {
		return "", nil, err
	}
	return name, func() { _ = os.Remove(name) }, nil
}

// Commit atomically replaces destination with a completed attempt.
func Commit(attempt, destination string) error {
	info, err := os.Lstat(attempt)
	if err != nil {
		return fmt.Errorf("inspect attempt: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("attempt output is not a regular file")
	}
	if info.Size() == 0 {
		return fmt.Errorf("attempt output is empty")
	}
	if err := replace(attempt, destination); err != nil {
		return fmt.Errorf("commit attempt: %w", err)
	}
	return nil
}
