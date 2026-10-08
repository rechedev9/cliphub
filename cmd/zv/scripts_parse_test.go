package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
