package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func installFakeSubcommands(t *testing.T, binDir string, names ...string) {
	t.Helper()
	for _, name := range names {
		dst := filepath.Join(binDir, executableName(name))
		writeFakeSubcommandExecutable(t, dst)
		if runtime.GOOS != "windows" {
			if err := os.Chmod(dst, 0o755); err != nil {
				t.Fatalf("chmod %s: %v", dst, err)
			}
		}
	}
}

func installFakeDelegatedSubcommands(t *testing.T, binDir string) {
	t.Helper()
	installFakeSubcommands(t, binDir, defaultLegacyCommandEntrypointNames()...)
}

var (
	fakeSubcommandMasterOnce sync.Once
	fakeSubcommandMasterDir  string
	fakeSubcommandMasterPath string
	fakeSubcommandMasterErr  error
)

func writeFakeSubcommandExecutable(t *testing.T, dst string) {
	t.Helper()
	fakeSubcommandMasterOnce.Do(func() {
		currentExe, err := os.Executable()
		if err != nil {
			fakeSubcommandMasterErr = fmt.Errorf("current executable: %w", err)
			return
		}
		fakeSubcommandMasterDir, fakeSubcommandMasterErr = os.MkdirTemp("", "zv-fake-subcommand-*")
		if fakeSubcommandMasterErr != nil {
			return
		}
		fakeSubcommandMasterPath = filepath.Join(fakeSubcommandMasterDir, executableName("zv-fake-subcommand"))
		contents, err := os.ReadFile(currentExe)
		if err != nil {
			fakeSubcommandMasterErr = fmt.Errorf("read current executable: %w", err)
			return
		}
		fakeSubcommandMasterErr = os.WriteFile(fakeSubcommandMasterPath, contents, 0o755)
	})
	if fakeSubcommandMasterErr != nil {
		t.Fatal(fakeSubcommandMasterErr)
	}
	if err := os.Link(fakeSubcommandMasterPath, dst); err != nil {
		copyFile(t, fakeSubcommandMasterPath, dst)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		t.Fatalf("copy %s to %s: %v", src, dst, err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close %s: %v", dst, err)
	}
}

func readFakeSubcommandCalls(t *testing.T, path string) []fakeSubcommandCall {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open calls log: %v", err)
	}
	defer f.Close()
	var calls []fakeSubcommandCall
	dec := json.NewDecoder(f)
	for dec.More() {
		var call fakeSubcommandCall
		if err := dec.Decode(&call); err != nil {
			t.Fatalf("decode calls log: %v", err)
		}
		calls = append(calls, call)
	}
	return calls
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", dir)
		}
		dir = parent
	}
}

func currentRepoSkills(t *testing.T, root string) []skillInfo {
	t.Helper()
	skillsDir := filepath.Join(root, ".claude", "skills")
	entries, err := os.ReadDir(skillsDir)
	if errors.Is(err, os.ErrNotExist) {
		return []skillInfo{}
	}
	if err != nil {
		t.Fatalf("read skills dir: %v", err)
	}
	skills := []skillInfo{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			t.Fatalf("stat skill %s: %v", entry.Name(), err)
		}
		skill, err := parseSkill(path)
		if err != nil {
			t.Fatalf("parse skill %s: %v", path, err)
		}
		if skill.Uncataloged {
			continue
		}
		if skill.Name == "" {
			skill.Name = entry.Name()
		}
		skills = append(skills, skill)
	}
	return skills
}

func workflowRunCommandArgs(t *testing.T, workflow workflowInfo) []string {
	t.Helper()
	fields, ok := splitCommandFields(workflow.RunCommand)
	if !ok {
		t.Fatalf("parse workflow run command %q", workflow.RunCommand)
	}
	if len(fields) < 4 || fields[0] != "zv" || fields[1] != "workflows" || fields[2] != "run" {
		t.Fatalf("workflow run command = %#v, want zv workflows run <name>", fields)
	}
	return append([]string(nil), fields[1:]...)
}

func workflowValidateCommandArgs(t *testing.T, workflow workflowInfo) []string {
	t.Helper()
	fields, ok := splitCommandFields(workflow.ValidateCommand)
	if !ok {
		t.Fatalf("parse workflow validate command %q", workflow.ValidateCommand)
	}
	if len(fields) != 4 || fields[0] != "zv" || fields[1] != "workflows" || fields[2] != "validate" || fields[3] != workflow.Name {
		t.Fatalf("workflow validate command = %#v, want zv workflows validate %s", fields, workflow.Name)
	}
	return append([]string(nil), fields[1:]...)
}

func workflowRunSampleForwardedArgs(t *testing.T, workflow workflowInfo, galleryPath string) []string {
	t.Helper()
	if strings.HasPrefix(workflow.Name, "full-demo-") {
		return append([]string{"--"}, fullDemoSampleArgs(t, workflow.Name)...)
	}
	switch workflow.Name {
	case "short":
		return []string{"--", "inferno.dem", "--prompt", "all kills 76561198000000000", "--out", "run/short", "--dry-run"}
	case "faceit-index":
		return []string{"--", "--profile", "m0NESY", "--out", "run/faceit-index.json", "--dry-run"}
	case "demo-parse":
		return []string{"--", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "run/plan.json"}
	case "demo-players":
		return []string{"--", "--demo", "inferno.dem"}
	case "demo-moments":
		planPath := writeDemoReviewPlan(t, filepath.Dir(filepath.Dir(galleryPath)))
		return []string{"--", "--killplan", planPath}
	case "demo-select":
		baseDir := filepath.Dir(filepath.Dir(galleryPath))
		planPath := writeDemoReviewPlan(t, baseDir)
		return []string{"--", "--killplan", planPath, "--segments", "seg-001", "--out", filepath.Join(baseDir, "selected-plan.json"), "--dry-run"}
	case "demo-probe":
		return []string{"--", "--demo", "inferno.dem", "--out", "run/playability.json", "--dry-run"}
	case "demo-voice":
		return []string{"--", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "run/voice-probe.json", "--dry-run"}
	case "utility-audit":
		return []string{"--", "--plan", "run/plan.json", "--lineup-catalog", "data/lineups", "--out", "run/utility-audit.csv"}
	case "record":
		return []string{"--", "--killplan", "run/plan.json", "--demo", "inferno.dem", "--out", "run/recording", "--dry-run"}
	case "compose-final":
		return []string{"--", "--recording-result", "run/recording/recording-result.json", "--out", "run/final.mp4", "--dry-run"}
	case "shorts-render":
		return []string{"--", "--recording-result", "run/recording/recording-result.json", "--out", "run/shorts"}
	case "stream-variants":
		return nil
	case "stream-fetch":
		return []string{"--", "--url", "https://www.twitch.tv/videos/123456789", "--out", "run/stream.mp4", "--dry-run"}
	case "stream-plan":
		return []string{"--", "--input", "stream.mp4", "--out", "run/stream-edit-plan.json", "--dry-run"}
	case "stream-render":
		return []string{"--", "--input", "stream.mp4", "--plan", "run/stream-edit-plan.json", "--out", "run/stream", "--dry-run"}
	case "music-analyze":
		return []string{"--", "--input", "data/music/track.mp4", "--out", "run/rhythm.json"}
	case "analysis-tactical":
		return []string{"--", "--demo", "inferno.dem", "--out", "run/tactical-document.json", "--dry-run"}
	case "analysis-rounds":
		documentPath := writeTacticalDocument(t, filepath.Dir(filepath.Dir(galleryPath)))
		return []string{"--", "--tactical", documentPath}
	case "analysis-tendencies":
		documentPath := writeTacticalDocument(t, filepath.Dir(filepath.Dir(galleryPath)))
		return []string{"--", "--tactical", documentPath, "--team", "t-start"}
	case "analysis-tactical-data":
		return []string{"--", "--demo", "inferno.dem", "--out", "run/tactical.json", "--start", "1000", "--end", "2000"}
	case "analysis-viewer":
		return []string{"--", "--json", "run/analysis.json"}
	case "gallery-open":
		return []string{"--", "--path", galleryPath}
	case "flows-run":
		baseDir := filepath.Dir(filepath.Dir(galleryPath))
		fixtureRoot := repoRoot(t)
		return []string{
			"--", "demo",
			"--demo", filepath.Join(fixtureRoot, "testdata", "agent-demo.fixture"),
			"--killplan", filepath.Join(fixtureRoot, "testdata", "agent-killplan.json"),
			"--run-dir", filepath.Join(baseDir, "flowdry"),
			"--dry-run",
		}
	case "capabilities", "skills-check", "workflows-check", "project-check", "serve":
		return nil
	default:
		t.Fatalf("missing sample forwarded args for workflow %q", workflow.Name)
		return nil
	}
}

func workflowDirectSampleArgs(t *testing.T, workflow workflowInfo, galleryPath string) []string {
	t.Helper()
	args := append([]string(nil), workflow.RunArgs...)
	forwarded := workflowRunSampleForwardedArgs(t, workflow, galleryPath)
	if len(forwarded) == 0 {
		return args
	}
	if forwarded[0] != "--" {
		t.Fatalf("workflow %q sample forwarded args = %#v, want leading --", workflow.Name, forwarded)
	}
	return append(args, forwarded[1:]...)
}

func workflowRunSampleArgsWithoutSeparator(t *testing.T, workflow workflowInfo, galleryPath string) []string {
	t.Helper()
	forwarded := workflowRunSampleForwardedArgs(t, workflow, galleryPath)
	if len(forwarded) > 0 {
		if forwarded[0] != "--" {
			t.Fatalf("workflow %q sample forwarded args = %#v, want leading --", workflow.Name, forwarded)
		}
		return append([]string(nil), forwarded[1:]...)
	}
	switch workflow.Name {
	case "capabilities", "stream-variants", "skills-check", "workflows-check", "project-check":
		return []string{"--format", "json"}
	case "serve":
		return []string{"--help"}
	default:
		t.Fatalf("missing separator sample args for workflow %q", workflow.Name)
		return nil
	}
}

func assertWorkflowDiscoveryMatches(t *testing.T, source string, got workflowInfo, want workflowInfo) {
	t.Helper()
	if got.Name != want.Name {
		t.Fatalf("%s name = %q, want %q", source, got.Name, want.Name)
	}
	if got.Description != want.Description {
		t.Fatalf("%s description for %s = %q, want %q", source, want.Name, got.Description, want.Description)
	}
	if got.Command != want.Command {
		t.Fatalf("%s command for %s = %q, want %q", source, want.Name, got.Command, want.Command)
	}
	if got.RunCommand != want.RunCommand {
		t.Fatalf("%s run_command for %s = %q, want %q", source, want.Name, got.RunCommand, want.RunCommand)
	}
	if got.ValidateCommand != want.ValidateCommand {
		t.Fatalf("%s validate_command for %s = %q, want %q", source, want.Name, got.ValidateCommand, want.ValidateCommand)
	}
	if !reflect.DeepEqual(got.Arguments, want.Arguments) {
		t.Fatalf("%s arguments for %s = %#v, want %#v", source, want.Name, got.Arguments, want.Arguments)
	}
	if !reflect.DeepEqual(got.Safety, want.Safety) {
		t.Fatalf("%s safety for %s = %#v, want %#v", source, want.Name, got.Safety, want.Safety)
	}
	if !reflect.DeepEqual(got.Contract, want.Contract) {
		t.Fatalf("%s contract for %s = %#v, want %#v", source, want.Name, got.Contract, want.Contract)
	}
	if got.RunArgs != nil {
		t.Fatalf("%s run args for %s = %#v, want omitted from json", source, want.Name, got.RunArgs)
	}
}

func assertJSONKeys(t *testing.T, source string, row map[string]json.RawMessage, want ...string) {
	t.Helper()
	wantSet := make(map[string]struct{}, len(want))
	for _, key := range want {
		wantSet[key] = struct{}{}
		if _, ok := row[key]; !ok {
			t.Fatalf("%s missing json key %q in %#v", source, key, row)
		}
	}
	for key := range row {
		if _, ok := wantSet[key]; !ok {
			t.Fatalf("%s has unexpected json key %q in %#v", source, key, row)
		}
	}
}

func assertIssueJSONKeys(t *testing.T, source string, raw json.RawMessage) {
	t.Helper()
	var issues []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &issues); err != nil {
		t.Fatalf("unmarshal %s: %v\n%s", source, err, raw)
	}
	if len(issues) == 0 {
		t.Fatalf("%s len = 0, want issues", source)
	}
	for i, issue := range issues {
		assertJSONKeys(t, fmt.Sprintf("%s[%d]", source, i), issue, "path", "message")
	}
}

func duplicateFlagValue(t *testing.T, args []string, flag string) []string {
	t.Helper()
	dup := append([]string(nil), args...)
	for i, arg := range dup {
		if arg != flag {
			continue
		}
		if i+1 >= len(dup) {
			t.Fatalf("flag %s has no value in %#v", flag, args)
		}
		insert := []string{flag, dup[i+1]}
		dup = append(dup[:i+2], append(insert, dup[i+2:]...)...)
		return dup
	}
	t.Fatalf("flag %s not found in %#v", flag, args)
	return nil
}

func equalsRequiredFlags(t *testing.T, args []string, required []string) []string {
	t.Helper()
	converted := append([]string(nil), args...)
	for _, flag := range required {
		var found bool
		for i := 0; i < len(converted); i++ {
			if converted[i] != flag {
				continue
			}
			if i+1 >= len(converted) {
				t.Fatalf("flag %s has no value in %#v", flag, args)
			}
			converted[i] = flag + "=" + converted[i+1]
			converted = append(converted[:i+1], converted[i+2:]...)
			found = true
			break
		}
		if !found {
			t.Fatalf("flag %s not found in %#v", flag, args)
		}
	}
	return converted
}

func emptyEqualsRequiredFlag(t *testing.T, args []string, flag string) []string {
	t.Helper()
	converted := append([]string(nil), args...)
	for i := 0; i < len(converted); i++ {
		if converted[i] != flag {
			continue
		}
		if i+1 >= len(converted) {
			t.Fatalf("flag %s has no value in %#v", flag, args)
		}
		converted[i] = flag + "="
		return append(converted[:i+1], converted[i+2:]...)
	}
	t.Fatalf("flag %s not found in %#v", flag, args)
	return nil
}

func emptySeparateRequiredFlag(t *testing.T, args []string, flag string) []string {
	t.Helper()
	converted := append([]string(nil), args...)
	for i := 0; i < len(converted); i++ {
		if converted[i] != flag {
			continue
		}
		if i+1 >= len(converted) {
			t.Fatalf("flag %s has no value in %#v", flag, args)
		}
		converted[i+1] = ""
		return converted
	}
	t.Fatalf("flag %s not found in %#v", flag, args)
	return nil
}

func skillListText(skills []skillInfo) string {
	var b strings.Builder
	for _, skill := range skills {
		if skill.Description == "" {
			fmt.Fprintln(&b, skill.Name)
			continue
		}
		fmt.Fprintf(&b, "%s\t%s\n", skill.Name, skill.Description)
	}
	return b.String()
}

func skillNames(skills []skillInfo) []string {
	names := make([]string, 0, len(skills))
	for _, skill := range skills {
		names = append(names, skill.Name)
	}
	return names
}

func workflowListText(workflows []workflowInfo) string {
	var b strings.Builder
	for _, workflow := range workflows {
		fmt.Fprintf(&b, "%s\t%s\n", workflow.Name, workflow.Description)
	}
	return b.String()
}

func workflowShowText(workflow workflowInfo) string {
	return fmt.Sprintf("%s\n%s\n\ncommand: %s\nrun_command: %s\nvalidate_command: %s\n", workflow.Name, workflow.Description, workflow.Command, workflow.RunCommand, workflow.ValidateCommand)
}

func decodeWorkflowCheckResult(t *testing.T, body string) workflowCheckResult {
	t.Helper()
	var result workflowCheckResult
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("unmarshal workflow check json: %v\n%s", err, body)
	}
	return result
}

func helpCommandStem(stem string) string {
	fields, ok := splitCommandFields(stem)
	if !ok || len(fields) == 0 {
		return ""
	}
	switch fields[0] {
	case "./bin/zv", `.\bin\zv.exe`, "zv":
		fields[0] = "zv"
	default:
		return ""
	}
	return strings.Join(fields, " ")
}

func workflowNames(workflows []workflowInfo) []string {
	names := make([]string, 0, len(workflows))
	for _, workflow := range workflows {
		names = append(names, workflow.Name)
	}
	return names
}

func workflowDirectDocCommandIsComparable(workflow workflowInfo) bool {
	return workflowDelegatesExternally(workflow) || workflow.Name == "gallery-open"
}

func assertDiscoveredWorkflowRunMatchesDirect(t *testing.T, exe, root, source string, index int, discovered workflowInfo, catalogWorkflow workflowInfo, galleryPath string) {
	t.Helper()
	runArgs := workflowRunCommandArgs(t, discovered)
	if len(runArgs) < 3 || runArgs[2] != catalogWorkflow.Name {
		t.Fatalf("%s discovered run_command for %s resolved to args %#v", source, catalogWorkflow.Name, runArgs)
	}
	runArgs = append(runArgs, workflowRunSampleForwardedArgs(t, catalogWorkflow, galleryPath)...)
	directArgs := workflowDirectSampleArgs(t, catalogWorkflow, galleryPath)

	prefix := fmt.Sprintf("%02d-%s-%s", index, source, catalogWorkflow.Name)
	runSubcommandLog := filepath.Join(root, prefix+"-discovered-run.jsonl")
	directSubcommandLog := filepath.Join(root, prefix+"-direct.jsonl")
	runOpenLog := filepath.Join(root, prefix+"-discovered-run-open.txt")
	directOpenLog := filepath.Join(root, prefix+"-direct-open.txt")

	runOut := runZVBinaryWithEnv(t, exe, root, []string{
		"ZV_FAKE_SUBCOMMAND=1",
		"ZV_FAKE_SUBCOMMAND_LOG=" + runSubcommandLog,
		"ZV_FAKE_OPEN_PATH_LOG=" + runOpenLog,
	}, runArgs...)
	directOut := runZVBinaryWithEnv(t, exe, root, []string{
		"ZV_FAKE_SUBCOMMAND=1",
		"ZV_FAKE_SUBCOMMAND_LOG=" + directSubcommandLog,
		"ZV_FAKE_OPEN_PATH_LOG=" + directOpenLog,
	}, directArgs...)

	if got, want := runOut, directOut; got != want {
		t.Fatalf("%s discovered run_command output = %q, want direct output %q", source, got, want)
	}
	if catalogWorkflow.Name == "gallery-open" {
		if got, want := strings.Join(readLines(t, runOpenLog), "\n"), strings.Join(readLines(t, directOpenLog), "\n"); got != want {
			t.Fatalf("%s discovered run_command open path log = %q, want direct log %q", source, got, want)
		}
		return
	}

	runCalls := readFakeSubcommandCalls(t, runSubcommandLog)
	directCalls := readFakeSubcommandCalls(t, directSubcommandLog)
	if got, want := len(runCalls), 1; got != want {
		t.Fatalf("%s discovered run_command calls len = %d, want %d: %#v", source, got, want, runCalls)
	}
	if got, want := len(directCalls), 1; got != want {
		t.Fatalf("%s direct calls len = %d, want %d: %#v", source, got, want, directCalls)
	}
	if got, want := runCalls[0].Executable, directCalls[0].Executable; got != want {
		t.Fatalf("%s discovered run_command executable = %q, want direct executable %q", source, got, want)
	}
	if got, want := strings.Join(runCalls[0].Args, "\x00"), strings.Join(directCalls[0].Args, "\x00"); got != want {
		t.Fatalf("%s discovered run_command args = %#v, want direct args %#v", source, runCalls[0].Args, directCalls[0].Args)
	}
}

func workflowDelegatesExternally(workflow workflowInfo) bool {
	if workflow.Name == "flows-run" {
		// The catalog sample supplies a kill plan, so its validated phases run
		// in-process and the missing generated dependencies stay write-free.
		return false
	}
	switch workflow.Name {
	case "demo-moments", "demo-select", "demo-probe", "demo-voice", "analysis-tactical", "analysis-rounds", "analysis-tendencies":
		// These run in-process inside zv itself; they never spawn a subcommand.
		return false
	}
	if len(workflow.RunArgs) == 0 {
		return false
	}
	switch workflow.RunArgs[0] {
	case "capabilities", "check", "faceit", "gallery", "short", "skills", "workflows", "flows", "full-demo":
		return false
	default:
		return true
	}
}

func workflowHelpDelegatesExternally(workflow workflowInfo) bool {
	if workflow.Name == "serve" || workflow.Name == "flows-run" {
		return false
	}
	return workflowDelegatesExternally(workflow)
}

func writeSkill(t *testing.T, root, name, description string) {
	t.Helper()
	writeSkillBody(t, root, name, strings.Join([]string{
		"---",
		"name: " + name,
		`description: "` + description + `"`,
		"---",
		"",
		"# " + name,
		"",
		"Workflow details.",
		"",
	}, "\n"))
}

func writeSkillBody(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".claude", "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o600); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func writeWorkflowDocs(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/test\n")
	writeFile(t, filepath.Join(root, "scripts", "smoke-real.ps1"), strings.Join([]string{
		`Fail "Orchestrator is not reachable. Start bin\zv serve with the current environment and run migrations first."`,
		"",
	}, "\n"))
	writeFile(t, filepath.Join(root, "scripts", "smoke.sh"), strings.Join([]string{
		"#!/usr/bin/env bash",
		"set -euo pipefail",
		`BASE="${ZV_BASE_URL:-http://localhost:8080}"`,
		`curl -fsS "$BASE/api/jobs"`,
		`curl -fsS "$BASE/api/jobs/$ID"`,
		`curl -fsS "$BASE/api/jobs/$ID/plan"`,
		"",
	}, "\n"))
	writeFile(t, filepath.Join(root, "Makefile"), strings.Join([]string{
		"build:",
		"\tgo build -o bin/zv ./cmd/zv",
		"",
		"test:",
		"\tgo test ./... -count=1",
		"\tgo run ./cmd/zv check",
		"\tgo run ./cmd/zv workflows check",
		"",
	}, "\n"))
	writeFile(t, filepath.Join(root, "scripts", "build.ps1"), strings.Join([]string{
		`$commands = @(`,
		`    "zv",`,
		`)`,
		`foreach ($name in $commands) {`,
		`    $out = Join-Path $binDir "$name.exe"`,
		`    $pkg = "./cmd/$name"`,
		`    & go build -o $out $pkg`,
		`}`,
		"",
	}, "\n"))
	writeFile(t, filepath.Join(root, "scripts", "go-gate.sh"), strings.Join([]string{
		`echo "== zv check =="`,
		"go run ./cmd/zv check",
		"",
	}, "\n"))
	writeFile(t, filepath.Join(root, "scripts", "fix-loop.ps1"), strings.Join([]string{
		`Invoke-Step "zv check" {`,
		"    & go run ./cmd/zv check",
		"}",
		"",
	}, "\n"))
	writeFile(t, filepath.Join(root, ".claude", "settings.json"), claudeSettingsFixture())
}

func claudeSettingsFixture() string {
	return strings.Join([]string{
		"{",
		`  "permissions": {`,
		`    "allow": [`,
		`      "Read",`,
		`      "Edit",`,
		`      "Write",`,
		`      "WebSearch",`,
		`      "WebFetch",`,
		`      "Bash(git status*)",`,
		`      "Bash(git diff*)",`,
		`      "Bash(git log*)",`,
		`      "Bash(go test*)",`,
		`      "Bash(go vet*)",`,
		`      "Bash(gofmt*)",`,
		`      "Bash(goimports*)",`,
		`      "Bash(staticcheck*)",`,
		`      "Bash(govulncheck*)",`,
		`      "Bash(gosec*)",`,
		`      "Bash(scripts/go-format-changed.sh*)",`,
		`      "Bash(scripts/go-gate.sh*)",`,
		`      "Bash(scripts/go-tools-check.sh*)",`,
		`      "Bash(powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/check-toolchain.ps1*)",`,
		`      "Bash(pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/check-toolchain.ps1*)",`,
		`      "Bash(*)",`,
		`      "Read(*)",`,
		`      "Edit(*)",`,
		`      "Write(*)"`,
		`    ],`,
		`    "defaultMode": "bypassPermissions"`,
		`  }`,
		"}",
		"",
	}, "\n")
}
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func appendFile(t *testing.T, path, body string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open append %s: %v", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(body); err != nil {
		t.Fatalf("append %s: %v", path, err)
	}
}

func hasIssue(issues []skillIssue, want string) bool {
	for _, issue := range issues {
		if issue.Path+": "+issue.Message == want {
			return true
		}
	}
	return false
}

func hasIssueContaining(issues []skillIssue, want string) bool {
	for _, issue := range issues {
		if strings.Contains(issue.Path+": "+issue.Message, want) {
			return true
		}
	}
	return false
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
}

var (
	cachedZVBinaryOnce   sync.Once
	cachedZVBinaryDir    string
	cachedZVBinaryPath   string
	cachedZVBinaryOutput []byte
	cachedZVBinaryErr    error
)

func buildZVBinary(t *testing.T, _ string) string {
	t.Helper()
	// Every binary E2E test calls this helper. The sole E2E test that mutates
	// process-wide environment and working-directory state deliberately does
	// not, so these expensive subprocess tests can safely run concurrently.
	t.Parallel()
	ensureCachedZVBinary(t)

	testBinDir, err := os.MkdirTemp(cachedZVBinaryDir, "case-*")
	if err != nil {
		t.Fatalf("create test binary directory: %v", err)
	}
	exe := filepath.Join(testBinDir, executableName("zv"))
	// A hardlink gives tests the private path needed for adjacent fake
	// subcommands without rewriting and rescanning the 40 MB executable for
	// every case on Windows. Fall back for filesystems without hardlinks.
	if err := os.Link(cachedZVBinaryPath, exe); err != nil {
		copyFile(t, cachedZVBinaryPath, exe)
	}
	return exe
}

// ensureCachedZVBinary builds the unified zv binary once for the whole package
// test run and returns its path. It performs no t.Parallel(), so both the
// hardlink-per-case E2E path (buildZVBinary) and buildDelegatedBinaries can
// share the single expensive compile.
func ensureCachedZVBinary(t *testing.T) string {
	t.Helper()
	cachedZVBinaryOnce.Do(func() {
		cachedZVBinaryDir, cachedZVBinaryErr = os.MkdirTemp("", "zv-test-bin-*")
		if cachedZVBinaryErr != nil {
			return
		}
		cachedZVBinaryPath = filepath.Join(cachedZVBinaryDir, executableName("zv"))
		cmd := exec.Command("go", "build", "-o", cachedZVBinaryPath, ".")
		cachedZVBinaryOutput, cachedZVBinaryErr = cmd.CombinedOutput()
	})
	if cachedZVBinaryErr != nil {
		t.Fatalf("go build ./cmd/zv: %v\n%s", cachedZVBinaryErr, cachedZVBinaryOutput)
	}
	return cachedZVBinaryPath
}

var (
	delegatedBinariesOnce sync.Once
	delegatedBinariesDir  string
	delegatedBinariesExe  string
	delegatedBinariesErr  error
)

// buildDelegatedBinaries builds the unified zv binary next to every delegated
// stage binary a journey shells out to (parser, recorder, editor, composer,
// stream), so a spawned `zv <stage>` resolves real siblings — delegation looks
// next to the executable. Built once per package test run and shared by the
// stage-contract, flow-run, and journey e2e suites instead of each keeping its
// own build-once path. `demo moments`/`demo select` delegate back to zv itself,
// so no extra binary is needed for them.
func buildDelegatedBinaries(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	zvBinary := ensureCachedZVBinary(t)
	delegatedBinariesOnce.Do(func() {
		delegatedBinariesDir, delegatedBinariesErr = os.MkdirTemp("", "zv-delegated-bin-*")
		if delegatedBinariesErr != nil {
			return
		}
		// Reuse the already-built unified zv binary instead of compiling ./cmd/zv
		// a second time; it must sit beside the delegated stage binaries so a
		// spawned `zv <stage>` resolves real siblings.
		zvDst := filepath.Join(delegatedBinariesDir, executableName("zv"))
		if err := os.Link(zvBinary, zvDst); err != nil {
			if delegatedBinariesErr = copyExecutable(zvBinary, zvDst); delegatedBinariesErr != nil {
				return
			}
		}
		for _, name := range []string{"zv-parser", "zv-recorder", "zv-editor", "zv-composer", "zv-stream"} {
			out := filepath.Join(delegatedBinariesDir, executableName(name))
			cmd := exec.Command("go", "build", "-o", out, "./cmd/"+name)
			cmd.Dir = root
			if combined, err := cmd.CombinedOutput(); err != nil {
				delegatedBinariesErr = fmt.Errorf("build %s: %w\n%s", name, err, combined)
				return
			}
		}
		delegatedBinariesExe = zvDst
	})
	if delegatedBinariesErr != nil {
		t.Fatalf("build delegated binaries: %v", delegatedBinariesErr)
	}
	return delegatedBinariesExe
}

// copyExecutable copies src to dst preserving the executable bit; used as a
// hardlink fallback when the cached zv binary and the delegated bin dir land on
// different filesystems.
func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// generateSyntheticSource builds a 1280x720, 4s, 30fps clip: a solid blue frame
// with a solid red rectangle over the exact top-left quarter (x=[0,320)
// y=[0,180)), plus a sine wave audio track, so the stream binary-level e2e has
// real media to probe and render against without committing any media file
// (repo rule). It is a deliberate ~20-line duplicate of the identical helper in
// cmd/zv-orchestrator/stream_e2e_test.go (keep the two in sync); the packages do
// not share test helpers, so cross-referencing is the least-surprising option.
func generateSyntheticSource(t *testing.T, ffmpegPath, outPath string) {
	t.Helper()
	args := []string{
		"-y",
		"-f", "lavfi", "-i", "color=c=blue:s=1280x720:d=4:r=30",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=4",
		"-filter_complex", "[0:v]drawbox=x=0:y=0:w=320:h=180:color=red:t=fill[v]",
		"-map", "[v]",
		"-map", "1:a",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-shortest",
		outPath,
	}
	runSyntheticFFmpeg(t, ffmpegPath, args...)
}

func runSyntheticFFmpeg(t *testing.T, ffmpegPath string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// #nosec G204 -- ffmpegPath comes from exec.LookPath and args are test-local literals.
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// exec.LookPath only proves an ffmpeg binary exists, not that it can
		// synthesize the source (a build without libx264/lavfi or with an older
		// drawbox fails here). Treat a synthesis failure as a missing capability
		// and skip, not a hard failure. Real stage failures after a successful
		// synth still Fatal in the callers.
		t.Skipf("ffmpeg cannot synthesize the test source (missing capability): %v\n%s", err, out)
	}
}

func removeAllTestArtifacts(path string) error {
	if path == "" {
		return nil
	}
	var err error
	for attempt := 0; attempt < 40; attempt++ {
		err = os.RemoveAll(path)
		if err == nil {
			return nil
		}
		if runtime.GOOS != "windows" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return err
}

func runZVBinary(t *testing.T, exe, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", exe, strings.Join(args, " "), err, out)
	}
	return string(out)
}

func runZVBinarySplit(t *testing.T, exe, dir string, args ...string) (string, string) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s: %v\nstdout:\n%s\nstderr:\n%s", exe, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

func runZVBinarySplitWithEnv(t *testing.T, exe, dir string, env []string, args ...string) (string, string) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), withCaptureToolsEnv(t, env)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s: %v\nstdout:\n%s\nstderr:\n%s", exe, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

func runZVBinarySplitWithEnvAndInput(t *testing.T, exe, dir string, env []string, input string, args ...string) (string, string) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), withCaptureToolsEnv(t, env)...)
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s: %v\nstdout:\n%s\nstderr:\n%s", exe, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

func runZVBinaryFailure(t *testing.T, exe, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("%s %s succeeded unexpectedly\n%s", exe, strings.Join(args, " "), out)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("%s %s failed without exit code: %v\n%s", exe, strings.Join(args, " "), err, out)
	}
	return string(out), exitErr.ExitCode()
}

func runZVBinaryFailureSplit(t *testing.T, exe, dir string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("%s %s succeeded unexpectedly\nstdout:\n%s\nstderr:\n%s", exe, strings.Join(args, " "), stdout.String(), stderr.String())
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("%s %s failed without exit code: %v\nstdout:\n%s\nstderr:\n%s", exe, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String(), exitErr.ExitCode()
}

func runZVBinaryFailureSplitWithEnv(t *testing.T, exe, dir string, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("%s %s succeeded unexpectedly\nstdout:\n%s\nstderr:\n%s", exe, strings.Join(args, " "), stdout.String(), stderr.String())
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("%s %s failed without exit code: %v\nstdout:\n%s\nstderr:\n%s", exe, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String(), exitErr.ExitCode()
}

func testBashExecutable() string {
	if runtime.GOOS == "windows" {
		const gitBash = `C:\Program Files\Git\bin\bash.exe`
		if _, err := os.Stat(gitBash); err == nil {
			return gitBash
		}
	}
	return "bash"
}

func findPowerShell() (string, bool) {
	for _, name := range []string{"pwsh", "powershell.exe", "powershell"} {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, true
		}
	}
	return "", false
}

var (
	fakeCaptureToolsOnce    sync.Once
	fakeCaptureToolsEnvVars []string
	fakeCaptureToolsErr     error
)

// fakeCaptureToolsEnv points ZV_HLAE_PATH and ZV_CS2_PATH at real placeholder
// executables in a temp dir. Documented record examples that omit --hlae/--cs2
// resolve capture tools from these variables, so the built-binary e2e tests run
// deterministically on machines without a real HLAE/CS2 install (CI runners).
// The paths are created once and stay stable for the process, so workflow-run
// and direct invocations append identical --hlae/--cs2 args and still compare
// equal.
func fakeCaptureToolsEnv(t *testing.T) []string {
	t.Helper()
	fakeCaptureToolsOnce.Do(func() {
		dir, err := os.MkdirTemp("", "zv-fake-capture-*")
		if err != nil {
			fakeCaptureToolsErr = err
			return
		}
		hlae := filepath.Join(dir, executableName("HLAE"))
		cs2 := filepath.Join(dir, executableName("cs2"))
		for _, path := range []string{hlae, cs2} {
			if err := os.WriteFile(path, []byte("fake capture tool"), 0o755); err != nil {
				fakeCaptureToolsErr = err
				return
			}
		}
		fakeCaptureToolsEnvVars = []string{"ZV_HLAE_PATH=" + hlae, "ZV_CS2_PATH=" + cs2}
	})
	if fakeCaptureToolsErr != nil {
		t.Fatalf("create fake capture tools: %v", fakeCaptureToolsErr)
	}
	return fakeCaptureToolsEnvVars
}

// withCaptureToolsEnv appends the fake capture tool paths whenever the caller
// stubs delegation with ZV_FAKE_SUBCOMMAND, so record examples that rely on
// capture-tool autodetection resolve without a real HLAE/CS2 install. Callers
// that set ZV_HLAE_PATH or ZV_CS2_PATH themselves keep their explicit values.
func withCaptureToolsEnv(t *testing.T, env []string) []string {
	t.Helper()
	var fakes, hasCapture bool
	for _, entry := range env {
		switch {
		case entry == "ZV_FAKE_SUBCOMMAND=1":
			fakes = true
		case strings.HasPrefix(entry, "ZV_HLAE_PATH=") || strings.HasPrefix(entry, "ZV_CS2_PATH="):
			hasCapture = true
		}
	}
	if !fakes || hasCapture {
		return env
	}
	return append(append([]string(nil), env...), fakeCaptureToolsEnv(t)...)
}

func runZVBinaryWithEnv(t *testing.T, exe, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), withCaptureToolsEnv(t, env)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", exe, strings.Join(args, " "), err, out)
	}
	return string(out)
}

// runZVBinaryWithEnvExpectFlowDryRunIncomplete preserves the generic workflow
// catalog tests while pinning the special flows-run contract: a write-free
// chain that reaches an unmaterialized dependency must return exit 1 and an
// explicit unsuccessful report, not masquerade as a completed workflow.
func runZVBinaryWithEnvExpectFlowDryRunIncomplete(t *testing.T, exe, dir string, env []string, args ...string) string {
	t.Helper()
	directFlowRun := len(args) >= 2 && args[0] == "flows" && args[1] == "run"
	workflowFlowRun := len(args) >= 3 && args[0] == "workflows" && args[1] == "run" && args[2] == "flows-run"
	if !directFlowRun && !workflowFlowRun {
		return runZVBinaryWithEnv(t, exe, dir, env, args...)
	}

	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), withCaptureToolsEnv(t, env)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != exitUnexpected {
		t.Fatalf("%s %s: error = %v, want exit %d\nstdout:\n%s\nstderr:\n%s",
			exe, strings.Join(args, " "), err, exitUnexpected, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("%s %s wrote stderr for expected incomplete flow: %s", exe, strings.Join(args, " "), stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, `"ok": false`) && !strings.Contains(output, "result: failed") {
		t.Fatalf("%s %s output does not report an incomplete flow:\n%s", exe, strings.Join(args, " "), output)
	}
	return output
}

func assertPathDoesNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s exists, want no file", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat %s: %v", path, err)
	}
}
