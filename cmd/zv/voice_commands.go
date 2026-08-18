package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/pathguard"
	"github.com/rechedev9/cliphub/internal/voicecomms"
)

type demoVoiceResult struct {
	OK       bool               `json:"ok"`
	DryRun   bool               `json:"dry_run"`
	Executed bool               `json:"executed"`
	Input    string             `json:"input"`
	Output   string             `json:"output,omitempty"`
	Extract  string             `json:"extract,omitempty"`
	Tracks   []voicecomms.Track `json:"tracks,omitempty"`
	Report   voicecomms.Report  `json:"report"`
}

func runDemoVoice(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, demoVoiceUsage)
		return exitSuccess
	}
	fs := flag.NewFlagSet("demo voice", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	demoPath := fs.String("demo", "", "path to .dem file")
	steamID := fs.String("steamid", "", "POV SteamID64 whose teammates are listed")
	outPath := fs.String("out", "", "voice probe JSON artifact")
	extractDir := fs.String("extract", "", "write POV-team Ogg Opus tracks into this directory")
	format := fs.String("format", "text", "text or json")
	dryRun := fs.Bool("dry-run", false, "validate inputs without parsing or writing")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(args, stdout, stderr, err, demoVoiceUsage, exitInvalidArgs)
	}
	if fs.NArg() != 0 {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unexpected positional arg %q", fs.Arg(0)), demoVoiceUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(*demoPath) == "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("--demo is required"), demoVoiceUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(*steamID) == "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("--steamid is required"), demoVoiceUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(*outPath) == "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("--out is required"), demoVoiceUsage, exitInvalidArgs)
	}
	if *format != "text" && *format != "json" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unsupported format %q", *format), demoVoiceUsage, exitInvalidArgs)
	}
	if err := pathguard.RejectOutputAliases(*outPath, pathguard.Input{Flag: "--demo", Path: *demoPath}); err != nil {
		return writeCommandError(args, stdout, stderr, err, demoVoiceUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(*extractDir) != "" {
		if err := pathguard.RejectOutputAliases(*extractDir, pathguard.Input{Flag: "--demo", Path: *demoPath}); err != nil {
			return writeCommandError(args, stdout, stderr, err, demoVoiceUsage, exitInvalidArgs)
		}
	}

	absOut, _ := filepath.Abs(*outPath)
	absDemo, _ := filepath.Abs(*demoPath)
	if *dryRun {
		result := demoVoiceResult{
			OK: true, DryRun: true, Executed: false, Input: absDemo, Output: absOut,
			Report: voicecomms.Report{
				SchemaVersion: voicecomms.SchemaVersion,
				Demo:          absDemo,
				Format:        voicecomms.FormatNone,
				Teammates:     []voicecomms.PlayerVoice{},
			},
		}
		return writeDemoVoiceResult(args, stdout, stderr, *format, result)
	}

	var (
		report voicecomms.Report
		index  voicecomms.Index
		err    error
	)
	if strings.TrimSpace(*extractDir) != "" {
		index, report, err = voicecomms.ExtractFile(*demoPath, *steamID, *extractDir)
	} else {
		report, err = voicecomms.ProbeFile(*demoPath, *steamID)
	}
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, "", exitUnexpected)
	}
	if err := writeJSONArtifact(absOut, report); err != nil {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("write voice probe: %w", err), "", exitUnexpected)
	}
	absExtract := ""
	if strings.TrimSpace(*extractDir) != "" {
		absExtract, _ = filepath.Abs(*extractDir)
	}
	return writeDemoVoiceResult(args, stdout, stderr, *format, demoVoiceResult{
		OK: true, DryRun: false, Executed: true, Input: absDemo, Output: absOut, Extract: absExtract, Tracks: index.Tracks, Report: report,
	})
}

func writeDemoVoiceResult(args []string, stdout, stderr io.Writer, format string, result demoVoiceResult) int {
	if format == "json" {
		if err := writeJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "error: write demo voice result: %v\n", err)
			return exitUnexpected
		}
		return exitSuccess
	}
	if result.DryRun {
		fmt.Fprintf(stdout, "voice probe: %s (not written)\n", result.Output)
		return exitSuccess
	}
	fmt.Fprintf(stdout, "voice_present\t%t\n", result.Report.VoicePresent)
	fmt.Fprintf(stdout, "format\t%s\n", result.Report.Format)
	fmt.Fprintf(stdout, "packets\t%d\n", result.Report.TotalPackets)
	fmt.Fprintf(stdout, "target\t%s\t%d\n", result.Report.Target.SteamID64, result.Report.Target.Packets)
	fmt.Fprintf(stdout, "teammates\t%d\n", len(result.Report.Teammates))
	fmt.Fprintf(stdout, "others\t%d\t%d\n", result.Report.Others.Players, result.Report.Others.Packets)
	fmt.Fprintf(stdout, "voice probe: %s\n", result.Output)
	return exitSuccess
}
