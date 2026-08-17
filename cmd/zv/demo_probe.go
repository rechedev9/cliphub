package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/pathguard"
)

type demoProbeResult struct {
	OK       bool                     `json:"ok"`
	DryRun   bool                     `json:"dry_run"`
	Executed bool                     `json:"executed"`
	Output   string                   `json:"output,omitempty"`
	Report   parser.PlayabilityReport `json:"report"`
}

func runDemoProbe(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, demoProbeUsage)
		return exitSuccess
	}
	fs := flag.NewFlagSet("demo probe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	demoPath := fs.String("demo", "", "path to .dem file")
	outPath := fs.String("out", "", "playability JSON artifact")
	format := fs.String("format", "text", "text or json")
	dryRun := fs.Bool("dry-run", false, "classify without writing --out")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(args, stdout, stderr, err, demoProbeUsage, exitInvalidArgs)
	}
	if fs.NArg() != 0 {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unexpected positional arg %q", fs.Arg(0)), demoProbeUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(*demoPath) == "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("--demo is required"), demoProbeUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(*outPath) == "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("--out is required"), demoProbeUsage, exitInvalidArgs)
	}
	if *format != "text" && *format != "json" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unsupported format %q", *format), demoProbeUsage, exitInvalidArgs)
	}
	if err := pathguard.RejectOutputAliases(*outPath, pathguard.Input{Flag: "--demo", Path: *demoPath}); err != nil {
		return writeCommandError(args, stdout, stderr, err, demoProbeUsage, exitInvalidArgs)
	}

	absOut, _ := filepath.Abs(*outPath)
	if *dryRun {
		absDemo, _ := filepath.Abs(*demoPath)
		result := demoProbeResult{
			OK: true, DryRun: true, Executed: false, Output: absOut,
			Report: parser.PlayabilityReport{
				OK: true, SchemaVersion: parser.PlayabilitySchemaVersion, Demo: absDemo, CS2Smoke: "not_run",
			},
		}
		if *format == "json" {
			if err := writeJSON(stdout, result); err != nil {
				fmt.Fprintf(stderr, "error: write demo probe result: %v\n", err)
				return exitUnexpected
			}
			return exitSuccess
		}
		fmt.Fprintf(stdout, "playability: %s (not written)\n", absOut)
		return exitSuccess
	}

	report, err := parser.ProbeDemo(*demoPath)
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, "", exitUnexpected)
	}
	if err := writeJSONArtifact(absOut, report); err != nil {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("write playability: %w", err), "", exitUnexpected)
	}
	result := demoProbeResult{OK: report.OK, DryRun: false, Executed: true, Output: absOut, Report: report}
	if *format == "json" {
		if err := writeJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "error: write demo probe result: %v\n", err)
			return exitUnexpected
		}
		return exitSuccess
	}
	fmt.Fprintf(stdout, "class\t%s\n", report.Class)
	fmt.Fprintf(stdout, "tick\t%d\n", report.FirstFullPacketTick)
	fmt.Fprintf(stdout, "tickrate\t%d\n", report.Tickrate)
	fmt.Fprintf(stdout, "map\t%s\n", report.Map)
	fmt.Fprintf(stdout, "sha256\t%s\n", report.SHA256)
	fmt.Fprintf(stdout, "reason\t%s\n", report.Reason)
	fmt.Fprintf(stdout, "playability: %s\n", absOut)
	return exitSuccess
}
