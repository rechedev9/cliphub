package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func skillFixtureBody(name, description string, commands ...string) string {
	lines := []string{
		"---",
		"name: " + name,
		`description: "` + description + `"`,
		"---",
		"",
		"```powershell",
	}
	lines = append(lines, commands...)
	return strings.Join(append(lines, "```", ""), "\n")
}

func TestZVBinarySkillsCheckWorkflowContractsEndToEnd(t *testing.T) {
	const (
		demoParse    = `.\bin\zv.exe workflows run demo-parse -- --demo demo.dem --steamid 76561198000000000 --out plan.json`
		utilityAudit = `.\bin\zv.exe workflows run utility-audit -- --plan plan.json --lineup-catalog data\lineups --out utility-audit.csv`
		recordDryRun = `.\bin\zv.exe workflows run record -- --killplan plan.json --demo demo.dem --out recording --dry-run`
		shortsRender = `.\bin\zv.exe workflows run shorts-render -- --recording-result recording\recording-result.json --out shorts`
		galleryOpen  = `.\bin\zv.exe workflows run gallery-open -- --path shorts\publish\index.html`
	)
	type skill struct{ dir, body string }
	utilityShorts := skill{"zackvideo-cs2-utility-shorts", skillFixtureBody("zackvideo-cs2-utility-shorts", "Create CS2 utility Shorts from a demo with ClipHub.",
		demoParse, utilityAudit, recordDryRun, shortsRender, galleryOpen)}
	tests := []struct {
		name       string
		skills     []skill
		wantIssues []string
	}{
		{
			name: "accepts capture without brief approval",
			skills: []skill{{"alpha", skillFixtureBody("alpha", "Alpha workflow",
				`.\bin\zv.exe workflows run record -- --killplan plan.json --demo demo.dem --out recording`)}},
		},
		{
			name: "rejects required workflow runs out of order",
			skills: []skill{{"zackvideo-cs2-utility-shorts", skillFixtureBody("zackvideo-cs2-utility-shorts", "Create CS2 utility Shorts from a demo with ClipHub.",
				demoParse, recordDryRun, utilityAudit, shortsRender, galleryOpen)}},
			wantIssues: []string{"required workflow runs must appear in order: demo-parse, utility-audit, record, shorts-render, gallery-open"},
		},
		{
			name: "rejects required workflow run documented only as help",
			skills: []skill{{"zackvideo-lineup-audit", skillFixtureBody("zackvideo-lineup-audit", "Review and correct ClipHub CS2 utility destination labels.",
				`.\bin\zv.exe workflows run utility-audit -- --help`)}},
			wantIssues: []string{"missing required workflow run utility-audit"},
		},
		{
			name:       "rejects catalog workflow runs out of order",
			skills:     []skill{{"alpha", skillFixtureBody("alpha", "Alpha workflow", shortsRender, demoParse)}},
			wantIssues: []string{"workflow runs must follow catalog order; demo-parse appears after shorts-render"},
		},
		{
			name: "rejects unexpected required skill workflow runs",
			skills: []skill{{"zackvideo-lineup-audit", skillFixtureBody("zackvideo-lineup-audit", "Review and correct ClipHub CS2 utility destination labels.",
				utilityAudit, galleryOpen)}},
			wantIssues: []string{"unexpected workflow run gallery-open; expected only: utility-audit"},
		},
		{
			name: "rejects ClipHub skill without workflow requirements",
			skills: []skill{
				utilityShorts,
				{"zackvideo-lineup-audit", skillFixtureBody("zackvideo-lineup-audit", "Review and correct ClipHub CS2 utility destination labels.", utilityAudit)},
				{"zackvideo-youtube-shorts-publish", skillFixtureBody("zackvideo-youtube-shorts-publish", "Prepare ClipHub YouTube Shorts packs for manual publication.", galleryOpen)},
				{"zackvideo-new-skill", skillFixtureBody("zackvideo-new-skill", "New ClipHub workflow skill.", demoParse)},
			},
			wantIssues: []string{"skill:zackvideo-new-skill: missing workflow requirements for repo skill"},
		},
		{
			name:   "rejects missing required repo skill",
			skills: []skill{utilityShorts},
			wantIssues: []string{
				"skill:zackvideo-lineup-audit: workflow requirements reference missing repo skill",
				"skill:zackvideo-shorts-production: workflow requirements reference missing repo skill",
				"skill:zackvideo-youtube-shorts-publish: workflow requirements reference missing repo skill",
			},
		},
		{
			name: "rejects duplicate workflow runs",
			skills: []skill{{"alpha", skillFixtureBody("alpha", "Alpha workflow", demoParse,
				`.\bin\zv.exe workflows run demo-parse -- --demo other.dem --steamid 76561198000000000 --out other-plan.json`)}},
			wantIssues: []string{"duplicate workflow run demo-parse"},
		},
	}
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for _, skill := range tt.skills {
				writeSkillBody(t, root, skill.dir, skill.body)
			}

			var stdout, stderr string
			if len(tt.wantIssues) == 0 {
				stdout, stderr = runZVBinarySplit(t, exe, root, "skills", "check", "--format", "json")
			} else {
				var code int
				stdout, stderr, code = runZVBinaryFailureSplit(t, exe, root, "skills", "check", "--format", "json")
				if got, want := code, exitInvalidArgs; got != want {
					t.Fatalf("code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
				}
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty for json output", stderr)
			}
			var result skillCheckResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("unmarshal stdout: %v\n%s", err, stdout)
			}
			if got, want := result.OK, len(tt.wantIssues) == 0; got != want {
				t.Fatalf("result.OK = %v, want %v: %#v", got, want, result)
			}
			if len(tt.wantIssues) == 0 && len(result.Issues) != 0 {
				t.Fatalf("issues = %#v, want none", result.Issues)
			}
			for _, want := range tt.wantIssues {
				if !hasIssueContaining(result.Issues, want) {
					t.Fatalf("issues = %#v, want %q", result.Issues, want)
				}
			}
		})
	}
}
