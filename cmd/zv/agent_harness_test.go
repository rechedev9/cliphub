package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRepoSkillsUseUnifiedCLI(t *testing.T) {
	root := repoRoot(t)
	for _, skill := range currentRepoSkills(t, root) {
		path, body := skill.Path, skill.Body
		if !strings.Contains(body, `.\bin\zv.exe`) {
			t.Errorf("%s does not document the unified zv CLI", path)
		}
		if !strings.Contains(body, `.\bin\zv.exe workflows run`) {
			t.Errorf("%s does not document a cataloged workflow run command", path)
		}
		for _, legacy := range legacySkillBinaries() {
			if strings.Contains(body, legacy) {
				t.Errorf("%s documents legacy direct binary %s", path, legacy)
			}
		}
	}
}

func TestAgentInstructionsUseCLIAndNoExternalMCP(t *testing.T) {
	root := repoRoot(t)

	agentPath := filepath.Join(root, "CLAUDE.md")
	agentInstructions, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("read %s: %v", agentPath, err)
	}
	agentBody := string(agentInstructions)
	for _, want := range []string{
		"## CLI-first",
		`.\bin\zv.exe capabilities --format json`,
		`.\bin\zv.exe workflows show short --format json`,
		`.\bin\zv.exe workflows validate short --format json -- match.dem --prompt "all kills 76561198000000000" --dry-run --format json`,
		`.\bin\zv.exe workflows run short -- match.dem --prompt "all kills 76561198000000000" --dry-run --format json`,
		"retired external MCP server",
		"Studio ships no assistant surface",
	} {
		if !strings.Contains(agentBody, want) {
			t.Fatalf("%s does not contain %q", agentPath, want)
		}
	}
}

func TestGoGateRunsProjectCheck(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "scripts", "go-gate.sh")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body := string(b)
	for _, want := range []string{
		"== zv check ==",
		"go run ./cmd/zv check",
		"go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...",
		"go run github.com/securego/gosec/v2/cmd/gosec@v2.28.0 ./...",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("%s does not contain %q", path, want)
		}
	}
}

func TestGoBashScriptsBootstrapWindowsGoToolchain(t *testing.T) {
	root := repoRoot(t)
	tests := []string{
		filepath.Join(root, "scripts", "go-gate.sh"),
		filepath.Join(root, "scripts", "go-format-changed.sh"),
	}
	for _, path := range tests {
		t.Run(filepath.Base(path), func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			body := string(b)
			for _, want := range []string{
				"source scripts/go-env.sh",
				"ensure_go_toolchain",
			} {
				if !strings.Contains(body, want) {
					t.Fatalf("%s does not contain %q", path, want)
				}
			}
		})
	}

	path := filepath.Join(root, "scripts", "go-env.sh")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body := string(b)
	for _, want := range []string{
		"ensure_go_toolchain()",
		"/c/Program Files/Go/bin",
		"/mnt/c/Program Files/Go/bin",
		"command -v go.exe",
		"command -v gofmt.exe",
		"go not found: install Go or add it to PATH",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("%s does not contain %q", path, want)
		}
	}
}

func TestRootShellScriptsParseEndToEnd(t *testing.T) {
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "scripts"))
	if err != nil {
		t.Fatalf("read scripts dir: %v", err)
	}
	args := []string{"-n"}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sh" {
			continue
		}
		args = append(args, filepath.ToSlash(filepath.Join("scripts", entry.Name())))
	}
	if len(args) == 1 {
		t.Fatalf("no root shell scripts found")
	}
	cmd := exec.Command(testBashExecutable(), args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestRootPowerShellScriptsParseEndToEnd(t *testing.T) {
	root := repoRoot(t)
	powerShell, ok := findPowerShell()
	if !ok {
		t.Skip("powershell or pwsh not found")
	}
	entries, err := os.ReadDir(filepath.Join(root, "scripts"))
	if err != nil {
		t.Fatalf("read scripts dir: %v", err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".ps1" {
			continue
		}
		files = append(files, filepath.Join("scripts", entry.Name()))
	}
	if len(files) == 0 {
		t.Fatalf("no root PowerShell scripts found")
	}

	script := strings.Join([]string{
		"param([string[]]$Paths)",
		"$failed = $false",
		"foreach ($path in $Paths) {",
		"  $tokens = $null",
		"  $errors = $null",
		"  [System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path -LiteralPath $path).Path, [ref]$tokens, [ref]$errors) | Out-Null",
		"  if ($errors.Count -gt 0) {",
		"    Write-Error (\"${path}: \" + (($errors | ForEach-Object { $_.Message }) -join '; '))",
		"    $failed = $true",
		"  }",
		"}",
		"if ($failed) { exit 1 }",
	}, "\n")
	scriptPath := filepath.Join(t.TempDir(), "parse-powershell-scripts.ps1")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write parse script: %v", err)
	}

	args := []string{"-NoProfile"}
	if strings.Contains(strings.ToLower(filepath.Base(powerShell)), "powershell") {
		args = append(args, "-ExecutionPolicy", "Bypass")
	}
	args = append(args, "-File", scriptPath)
	args = append(args, files...)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, powerShell, args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("%s %s timed out\n%s", powerShell, strings.Join(args, " "), out)
	}
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", powerShell, strings.Join(args, " "), err, out)
	}
}

func TestFixLoopRunsProjectCheck(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "scripts", "fix-loop.ps1")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body := string(b)
	for _, want := range []string{
		`Invoke-Step "zv check"`,
		"go run ./cmd/zv check",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("%s does not contain %q", path, want)
		}
	}
}

func TestMakefileRunsProjectCheck(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "Makefile")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body := string(b)
	for _, want := range []string{
		"check:",
		"go run ./cmd/zv check",
		"workflows-check:",
		"go run ./cmd/zv workflows check",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("%s does not contain %q", path, want)
		}
	}
	if !strings.Contains(body, ".PHONY:") || !strings.Contains(body, "check") || !strings.Contains(body, "workflows-check") {
		t.Fatalf("%s does not mark check targets as phony", path)
	}
}

func TestCurrentBuildScriptsCoverCommandEntrypoints(t *testing.T) {
	root := repoRoot(t)
	commands, err := commandEntrypoints(root)
	if err != nil {
		t.Fatalf("command entrypoints: %v", err)
	}
	if len(commands) == 0 {
		t.Fatalf("no command entrypoints found")
	}

	makefileBody := readFileString(t, filepath.Join(root, "Makefile"))
	buildScriptBody := readFileString(t, filepath.Join(root, "scripts", "build.ps1"))
	known := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		known[command] = struct{}{}
		makeTarget := fmt.Sprintf("go build -o bin/%s ./cmd/%s", command, command)
		if !strings.Contains(makefileBody, makeTarget) {
			t.Fatalf("Makefile does not build %s with %q", command, makeTarget)
		}
		buildEntry := fmt.Sprintf(`"%s"`, command)
		if !strings.Contains(buildScriptBody, buildEntry) {
			t.Fatalf("scripts/build.ps1 does not include command entry %s", buildEntry)
		}
	}
	for _, target := range makefileCommandBuildTargets(makefileBody) {
		if _, ok := known[target.Command]; !ok {
			t.Fatalf("Makefile builds stale command %q with %q", target.Command, target.Line)
		}
	}
	for _, command := range buildScriptCommandEntries(buildScriptBody) {
		if _, ok := known[command]; !ok {
			t.Fatalf("scripts/build.ps1 includes stale command entry %q", command)
		}
	}
}

// TestAgentGuideUsesUnifiedCLI keeps the agent-facing guide free of the retired
// per-stage binaries. It replaces the PRODUCT.md variant that was dropped with
// the product guide itself; .claude/GUIDE.md is now the only tracked doc that
// enumerates the workflow command surface.
func TestAgentGuideUsesUnifiedCLI(t *testing.T) {
	root := repoRoot(t)
	guidePath := filepath.Join(root, ".claude", "GUIDE.md")
	b, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatalf("read .claude/GUIDE.md: %v", err)
	}
	body := string(b)
	for _, legacy := range legacyWorkflowCommands() {
		if strings.Contains(body, legacy) {
			t.Fatalf(".claude/GUIDE.md contains legacy workflow command %q; use ./bin/zv instead", legacy)
		}
	}
	for _, want := range []string{
		"./bin/zv workflows run demo-parse",
		"./bin/zv workflows run demo-players",
		"./bin/zv workflows run record",
		"./bin/zv workflows run compose-final",
		"./bin/zv workflows run music-analyze",
		"./bin/zv workflows run shorts-render",
		"./bin/zv check",
		"./bin/zv workflows run serve",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf(".claude/GUIDE.md does not contain unified workflow command %q", want)
		}
	}
}
