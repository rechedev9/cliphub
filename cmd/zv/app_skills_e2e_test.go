package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestZVBinarySkillsCheckAcceptsCaptureWithoutBriefApprovalEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	writeSkillBody(t, tempDir, "alpha", strings.Join([]string{
		"---",
		"name: alpha",
		`description: "Alpha workflow"`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run record -- --killplan plan.json --demo demo.dem --out recording`,
		"```",
		"",
	}, "\n"))

	stdout, stderr := runZVBinarySplit(t, exe, tempDir, "skills", "check", "--format", "json")
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty for json output", stderr)
	}
	var result skillCheckResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout)
	}
	if !result.OK || len(result.Issues) != 0 {
		t.Fatalf("result = %#v, want valid auto-detected record example", result)
	}
}

func TestZVBinarySkillsCheckRejectsRequiredWorkflowRunsOutOfOrderEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	writeSkillBody(t, tempDir, "zackvideo-cs2-utility-shorts", strings.Join([]string{
		"---",
		"name: zackvideo-cs2-utility-shorts",
		`description: "Create CS2 utility Shorts from a demo with ClipHub."`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run demo-parse -- --demo demo.dem --steamid 76561198000000000 --out plan.json`,
		`.\bin\zv.exe workflows run record -- --killplan plan.json --demo demo.dem --out recording --dry-run`,
		`.\bin\zv.exe workflows run utility-audit -- --plan plan.json --lineup-catalog data\lineups --out utility-audit.csv`,
		`.\bin\zv.exe workflows run shorts-render -- --recording-result recording\recording-result.json --out shorts`,
		`.\bin\zv.exe workflows run gallery-open -- --path shorts\publish\index.html`,
		"```",
		"",
	}, "\n"))

	stdout, stderr, code := runZVBinaryFailureSplit(t, exe, tempDir, "skills", "check", "--format", "json")

	if got, want := code, exitInvalidArgs; got != want {
		t.Fatalf("code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty for json output", stderr)
	}
	var result skillCheckResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout)
	}
	if result.OK {
		t.Fatalf("result.OK = true, want false")
	}
	want := "required workflow runs must appear in order: demo-parse, utility-audit, record, shorts-render, gallery-open"
	if !hasIssueContaining(result.Issues, want) {
		t.Fatalf("issues = %#v, want %q", result.Issues, want)
	}
}

func TestZVBinarySkillsCheckRejectsRequiredWorkflowRunDocumentedOnlyAsHelpEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	writeSkillBody(t, tempDir, "zackvideo-lineup-audit", strings.Join([]string{
		"---",
		"name: zackvideo-lineup-audit",
		`description: "Review and correct ClipHub CS2 utility destination labels."`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run utility-audit -- --help`,
		"```",
		"",
	}, "\n"))

	stdout, stderr, code := runZVBinaryFailureSplit(t, exe, tempDir, "skills", "check", "--format", "json")

	if got, want := code, exitInvalidArgs; got != want {
		t.Fatalf("code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty for json output", stderr)
	}
	var result skillCheckResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout)
	}
	if result.OK {
		t.Fatalf("result.OK = true, want false")
	}
	want := "missing required workflow run utility-audit"
	if !hasIssueContaining(result.Issues, want) {
		t.Fatalf("issues = %#v, want %q", result.Issues, want)
	}
}

func TestZVBinarySkillsCheckRejectsCatalogWorkflowRunsOutOfOrderEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	writeSkillBody(t, tempDir, "alpha", strings.Join([]string{
		"---",
		"name: alpha",
		`description: "Alpha workflow"`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run shorts-render -- --recording-result recording\recording-result.json --out shorts`,
		`.\bin\zv.exe workflows run demo-parse -- --demo demo.dem --steamid 76561198000000000 --out plan.json`,
		"```",
		"",
	}, "\n"))

	stdout, stderr, code := runZVBinaryFailureSplit(t, exe, tempDir, "skills", "check", "--format", "json")

	if got, want := code, exitInvalidArgs; got != want {
		t.Fatalf("code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty for json output", stderr)
	}
	var result skillCheckResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout)
	}
	if result.OK {
		t.Fatalf("result.OK = true, want false")
	}
	want := "workflow runs must follow catalog order; demo-parse appears after shorts-render"
	if !hasIssueContaining(result.Issues, want) {
		t.Fatalf("issues = %#v, want %q", result.Issues, want)
	}
}

func TestZVBinarySkillsCheckRejectsUnexpectedRequiredSkillWorkflowRunsEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	writeSkillBody(t, tempDir, "zackvideo-lineup-audit", strings.Join([]string{
		"---",
		"name: zackvideo-lineup-audit",
		`description: "Review and correct ClipHub CS2 utility destination labels."`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run utility-audit -- --plan plan.json --lineup-catalog data\lineups --out utility-audit.csv`,
		`.\bin\zv.exe workflows run gallery-open -- --path shorts\publish\index.html`,
		"```",
		"",
	}, "\n"))

	stdout, stderr, code := runZVBinaryFailureSplit(t, exe, tempDir, "skills", "check", "--format", "json")

	if got, want := code, exitInvalidArgs; got != want {
		t.Fatalf("code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty for json output", stderr)
	}
	var result skillCheckResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout)
	}
	if result.OK {
		t.Fatalf("result.OK = true, want false")
	}
	want := "unexpected workflow run gallery-open; expected only: utility-audit"
	if !hasIssueContaining(result.Issues, want) {
		t.Fatalf("issues = %#v, want %q", result.Issues, want)
	}
}

func TestZVBinarySkillsCheckRejectsClipHubSkillWithoutWorkflowRequirementsEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	writeSkillBody(t, tempDir, "zackvideo-cs2-utility-shorts", strings.Join([]string{
		"---",
		"name: zackvideo-cs2-utility-shorts",
		`description: "Create CS2 utility Shorts from a demo with ClipHub."`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run demo-parse -- --demo demo.dem --steamid 76561198000000000 --out plan.json`,
		`.\bin\zv.exe workflows run utility-audit -- --plan plan.json --lineup-catalog data\lineups --out utility-audit.csv`,
		`.\bin\zv.exe workflows run record -- --killplan plan.json --demo demo.dem --out recording --dry-run`,
		`.\bin\zv.exe workflows run shorts-render -- --recording-result recording\recording-result.json --out shorts`,
		`.\bin\zv.exe workflows run gallery-open -- --path shorts\publish\index.html`,
		"```",
		"",
	}, "\n"))
	writeSkillBody(t, tempDir, "zackvideo-lineup-audit", strings.Join([]string{
		"---",
		"name: zackvideo-lineup-audit",
		`description: "Review and correct ClipHub CS2 utility destination labels."`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run utility-audit -- --plan plan.json --lineup-catalog data\lineups --out utility-audit.csv`,
		"```",
		"",
	}, "\n"))
	writeSkillBody(t, tempDir, "zackvideo-youtube-shorts-publish", strings.Join([]string{
		"---",
		"name: zackvideo-youtube-shorts-publish",
		`description: "Prepare ClipHub YouTube Shorts packs for manual publication."`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run gallery-open -- --path shorts\publish\index.html`,
		"```",
		"",
	}, "\n"))
	writeSkillBody(t, tempDir, "zackvideo-new-skill", strings.Join([]string{
		"---",
		"name: zackvideo-new-skill",
		`description: "New ClipHub workflow skill."`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run demo-parse -- --demo demo.dem --steamid 76561198000000000 --out plan.json`,
		"```",
		"",
	}, "\n"))

	stdout, stderr, code := runZVBinaryFailureSplit(t, exe, tempDir, "skills", "check", "--format", "json")

	if got, want := code, exitInvalidArgs; got != want {
		t.Fatalf("code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty for json output", stderr)
	}
	var result skillCheckResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout)
	}
	if result.OK {
		t.Fatalf("result.OK = true, want false")
	}
	want := "skill:zackvideo-new-skill: missing workflow requirements for repo skill"
	if !hasIssueContaining(result.Issues, want) {
		t.Fatalf("issues = %#v, want %q", result.Issues, want)
	}
}

func TestZVBinarySkillsCheckRejectsMissingRequiredRepoSkillEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	writeSkillBody(t, tempDir, "zackvideo-cs2-utility-shorts", strings.Join([]string{
		"---",
		"name: zackvideo-cs2-utility-shorts",
		`description: "Create CS2 utility Shorts from a demo with ClipHub."`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run demo-parse -- --demo demo.dem --steamid 76561198000000000 --out plan.json`,
		`.\bin\zv.exe workflows run utility-audit -- --plan plan.json --lineup-catalog data\lineups --out utility-audit.csv`,
		`.\bin\zv.exe workflows run record -- --killplan plan.json --demo demo.dem --out recording --dry-run`,
		`.\bin\zv.exe workflows run shorts-render -- --recording-result recording\recording-result.json --out shorts`,
		`.\bin\zv.exe workflows run gallery-open -- --path shorts\publish\index.html`,
		"```",
		"",
	}, "\n"))

	stdout, stderr, code := runZVBinaryFailureSplit(t, exe, tempDir, "skills", "check", "--format", "json")

	if got, want := code, exitInvalidArgs; got != want {
		t.Fatalf("code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty for json output", stderr)
	}
	var result skillCheckResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout)
	}
	if result.OK {
		t.Fatalf("result.OK = true, want false")
	}
	for _, want := range []string{
		"skill:zackvideo-lineup-audit: workflow requirements reference missing repo skill",
		"skill:zackvideo-shorts-production: workflow requirements reference missing repo skill",
		"skill:zackvideo-youtube-shorts-publish: workflow requirements reference missing repo skill",
	} {
		if !hasIssueContaining(result.Issues, want) {
			t.Fatalf("issues = %#v, want %q", result.Issues, want)
		}
	}
}

func TestZVBinarySkillsCheckRejectsDuplicateWorkflowRunsEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	writeSkillBody(t, tempDir, "alpha", strings.Join([]string{
		"---",
		"name: alpha",
		`description: "Alpha workflow"`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run demo-parse -- --demo demo.dem --steamid 76561198000000000 --out plan.json`,
		`.\bin\zv.exe workflows run demo-parse -- --demo other.dem --steamid 76561198000000000 --out other-plan.json`,
		"```",
		"",
	}, "\n"))

	stdout, stderr, code := runZVBinaryFailureSplit(t, exe, tempDir, "skills", "check", "--format", "json")

	if got, want := code, exitInvalidArgs; got != want {
		t.Fatalf("code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty for json output", stderr)
	}
	var result skillCheckResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout)
	}
	if result.OK {
		t.Fatalf("result.OK = true, want false")
	}
	if !hasIssueContaining(result.Issues, "duplicate workflow run demo-parse") {
		t.Fatalf("issues = %#v, want duplicate workflow run issue", result.Issues)
	}
}
