package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func findWorkflowRoot() (string, error) {
	var starts []string
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		for dir := start; ; dir = filepath.Dir(dir) {
			if hasWorkflowRootMarker(dir) {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	return "", fmt.Errorf("workflow root not found")
}

func hasWorkflowRootMarker(dir string) bool {
	if isRepoSkillsDir(filepath.Join(dir, ".claude", "skills")) {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return true
	}
	return false
}

// isRepoSkillsDir reports whether candidate is a repo-local skills catalog.
// The user-global Claude Code skills dir (~/.claude/skills) has the same
// shape and sits on the walk-up path of every directory under the home
// folder, so it is never accepted as the zv catalog.
func isRepoSkillsDir(candidate string) bool {
	st, err := os.Stat(candidate)
	if err != nil || !st.IsDir() {
		return false
	}
	if home, err := os.UserHomeDir(); err == nil {
		if same, err := samePath(candidate, filepath.Join(home, ".claude", "skills")); err == nil && same {
			return false
		}
	}
	return true
}

func samePath(a, b string) (bool, error) {
	absA, err := filepath.Abs(a)
	if err != nil {
		return false, err
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		return false, err
	}
	return filepath.Clean(absA) == filepath.Clean(absB), nil
}
