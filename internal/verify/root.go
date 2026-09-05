package verify

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindRepoRoot walks from cwd and the executable looking for the Go module.
func FindRepoRoot() (string, error) {
	var starts []string
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		for dir := start; ; dir = filepath.Dir(dir) {
			if isRepoRoot(dir) {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	return "", fmt.Errorf("cliphub repo root not found: missing go.mod")
}

func isRepoRoot(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil && st.Mode().IsRegular()
}
