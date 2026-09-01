package verify

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindRepoRoot walks from cwd and the executable looking for go.mod plus the
// in-repo verification skill. The skill is the lever; a go.mod alone is not.
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
	return "", fmt.Errorf("cliphub repo root not found: missing go.mod or %s", SkillRelPath)
}

func isRepoRoot(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(SkillRelPath)))
	return err == nil && st.Mode().IsRegular()
}
