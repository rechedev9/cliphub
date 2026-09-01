package verify

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillReport is the parsed in-repo verification skill.
type SkillReport struct {
	Path        string   `json:"path"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	OK          bool     `json:"ok"`
	Issues      []string `json:"issues"`
}

// RequiredSkillPhrases are the standing rules the skill must keep saying.
var RequiredSkillPhrases = []string{
	"prove against the real artifact",
	"never treat compile",
	"fail closed",
	"HLAE",
	"CS2",
	"./bin/zv",
}

// InspectSkill reads and checks the Cursor skill at the repo root.
func InspectSkill(root string) SkillReport {
	rel := filepath.FromSlash(SkillRelPath)
	path := filepath.Join(root, rel)
	report := SkillReport{Path: filepath.ToSlash(rel)}
	body, err := os.ReadFile(path)
	if err != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("read skill: %v", err))
		return report
	}
	name, description := parseSkillFrontmatter(string(body))
	report.Name = name
	report.Description = description
	if name != "verify-cliphub" {
		report.Issues = append(report.Issues, fmt.Sprintf("skill name %q must be verify-cliphub", name))
	}
	if strings.TrimSpace(description) == "" {
		report.Issues = append(report.Issues, "missing skill description")
	}
	text := string(body)
	lower := strings.ToLower(text)
	if !strings.Contains(text, `.\bin\zv.exe`) && !strings.Contains(text, `./bin/zv`) {
		report.Issues = append(report.Issues, "does not document the unified zv CLI")
	}
	for _, phrase := range RequiredSkillPhrases {
		if phrase == "./bin/zv" {
			continue
		}
		if !strings.Contains(lower, strings.ToLower(phrase)) {
			report.Issues = append(report.Issues, fmt.Sprintf("missing required phrase %q", phrase))
		}
	}
	if !strings.Contains(text, ClosedCaptureGapID) && !strings.Contains(text, "HLAE/CS2") {
		report.Issues = append(report.Issues, "does not name the HLAE/CS2 / Windows Studio gap")
	}
	report.OK = len(report.Issues) == 0
	return report
}

func parseSkillFrontmatter(body string) (name, description string) {
	scanner := bufio.NewScanner(strings.NewReader(body))
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return "", ""
	}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		switch strings.TrimSpace(key) {
		case "name":
			name = value
		case "description":
			description = value
		}
	}
	return name, description
}
