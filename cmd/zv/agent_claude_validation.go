package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

type claudeSettingsFile struct {
	Permissions struct {
		Allow       []string `json:"allow"`
		Ask         []string `json:"ask"`
		DefaultMode string   `json:"defaultMode"`
	} `json:"permissions"`
}

func checkClaudeSettings() ([]skillIssue, error) {
	root, err := findWorkflowRoot()
	if err != nil {
		return nil, err
	}
	const relPath = ".claude/settings.json"
	path := filepath.Join(root, filepath.FromSlash(relPath))
	// #nosec G304 -- both root and relPath are repository-owned validation inputs.
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []skillIssue{{Path: relPath, Message: "missing claude settings"}}, nil
		}
		return nil, fmt.Errorf("read %s: %w", relPath, err)
	}
	var settings claudeSettingsFile
	if err := json.Unmarshal(b, &settings); err != nil {
		return []skillIssue{{Path: relPath, Message: fmt.Sprintf("invalid json: %v", err)}}, nil
	}

	var issues []skillIssue
	for _, permission := range claudeRequiredAllowPermissions() {
		if !containsString(settings.Permissions.Allow, permission) {
			issues = append(issues, skillIssue{Path: relPath, Message: fmt.Sprintf("missing allow permission %q", permission)})
		}
	}
	if len(settings.Permissions.Ask) > 0 {
		issues = append(issues, skillIssue{Path: relPath, Message: "permissions.ask must be empty: the harness runs without permission prompts"})
	}
	if settings.Permissions.DefaultMode != "bypassPermissions" {
		issues = append(issues, skillIssue{Path: relPath, Message: fmt.Sprintf("permissions.defaultMode = %q, want \"bypassPermissions\"", settings.Permissions.DefaultMode)})
	}
	return issues, nil
}

func containsString(values []string, want string) bool {
	return slices.Contains(values, want)
}
