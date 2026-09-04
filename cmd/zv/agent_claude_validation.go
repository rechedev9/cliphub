package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func readWorkflowDocBody(root, relPath string) (string, error) {
	path := filepath.Join(root, filepath.FromSlash(relPath))
	// #nosec G304 -- root is the checked repository and relPath comes from the fixed workflow catalog.
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", relPath, err)
	}
	return string(b), nil
}

// Style and operational norms live directly in CLAUDE.md; the .claude/rules
// mirrors were removed. Validate that the load-bearing guidance is present in
// CLAUDE.md and that no rules mirror reappears.
func checkClaudeRuleDocs() ([]skillIssue, error) {
	root, err := findWorkflowRoot()
	if err != nil {
		return nil, err
	}
	claudeBody, err := readWorkflowDocBody(root, "CLAUDE.md")
	if err != nil {
		return nil, err
	}
	webClaudeBody, err := readWorkflowDocBody(root, "web/CLAUDE.md")
	if err != nil {
		return nil, err
	}
	var issues []skillIssue
	for _, required := range claudeStyleRequiredText() {
		if !strings.Contains(claudeBody, required) {
			issues = append(issues, skillIssue{Path: "CLAUDE.md", Message: fmt.Sprintf("missing style guidance %q", required)})
		}
	}
	for _, required := range webClaudeStyleRequiredText() {
		if !strings.Contains(webClaudeBody, required) {
			issues = append(issues, skillIssue{Path: "web/CLAUDE.md", Message: fmt.Sprintf("missing style guidance %q", required)})
		}
	}
	rulesDir := filepath.Join(root, ".claude", "rules")
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return issues, nil
		}
		return nil, fmt.Errorf("read claude rules: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		relPath := filepath.ToSlash(filepath.Join(".claude", "rules", entry.Name()))
		issues = append(issues, skillIssue{Path: relPath, Message: "style rules live in CLAUDE.md; remove this .claude/rules mirror"})
	}
	return issues, nil
}

func claudeStyleRequiredText() []string {
	return []string{
		"Write boring, idiomatic Go.",
		"Do not introduce `util`, `common`, `helper`, `manager`",
		"Add useful context when returning errors.",
		"Every goroutine must have a clear owner and stop condition.",
		"Do not add generated video/audio/image artifacts to git.",
	}
}

// The web frontend style guidance lives in web/CLAUDE.md so it only loads when
// working under web/; validate it there rather than in the root file.
func webClaudeStyleRequiredText() []string {
	return []string{
		"## TypeScript style (web/)",
		"No `any`, ever",
		"No re-exports",
	}
}

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
