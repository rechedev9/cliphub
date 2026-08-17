package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"github.com/rechedev9/cliphub/internal/anticheat"
	"github.com/rechedev9/cliphub/internal/pathguard"
)

// demoAnticheatResult is the JSON contract of `zv demo anticheat`.
type demoAnticheatResult struct {
	OK       bool               `json:"ok"`
	DryRun   bool               `json:"dry_run"`
	Executed bool               `json:"executed"`
	Input    string             `json:"input"`
	Output   string             `json:"output,omitempty"`
	Report   anticheat.Report   `json:"report"`
	Dossier  *anticheat.Dossier `json:"dossier,omitempty"`
}

// demoAnticheatCalibrateResult is the JSON contract of the calibrate pass.
type demoAnticheatCalibrateResult struct {
	OK       bool     `json:"ok"`
	DryRun   bool     `json:"dry_run"`
	Executed bool     `json:"executed"`
	Demos    []string `json:"demos"`
	// DistinctDemos is how many of Demos held unique content; duplicates are
	// parsed but counted once so one lobby cannot dominate the baseline.
	DistinctDemos int                `json:"distinct_demos"`
	Output        string             `json:"output,omitempty"`
	Baseline      anticheat.Baseline `json:"baseline"`
	Skipped       map[string]string  `json:"skipped,omitempty"`
}

func runDemoAnticheat(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, demoAnticheatUsage)
		return exitSuccess
	}
	if len(args) > 0 && args[0] == "calibrate" {
		return runDemoAnticheatCalibrate(args[1:], stdout, stderr)
	}

	fs := flag.NewFlagSet("demo anticheat", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	demoPath := fs.String("demo", "", "CS2 demo to screen")
	baselinePath := fs.String("baseline", "", "optional baseline JSON; defaults to the shipped professional-play distribution")
	outPath := fs.String("out", "", "optional analysis JSON artifact")
	dossierID := fs.String("dossier", "", "optional SteamID64 to render a report dossier for")
	format := fs.String("format", "text", "text or json")
	dryRun := fs.Bool("dry-run", false, "analyse without writing the artifact")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(args, stdout, stderr, err, demoAnticheatUsage, exitInvalidArgs)
	}
	if fs.NArg() != 0 {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unexpected positional arg %q", fs.Arg(0)), demoAnticheatUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(*demoPath) == "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("--demo is required"), demoAnticheatUsage, exitInvalidArgs)
	}
	if *format != "text" && *format != "json" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unsupported format %q", *format), demoAnticheatUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(*outPath) != "" {
		inputs := []pathguard.Input{{Flag: "--demo", Path: *demoPath}}
		if strings.TrimSpace(*baselinePath) != "" {
			inputs = append(inputs, pathguard.Input{Flag: "--baseline", Path: *baselinePath})
		}
		if err := pathguard.RejectOutputAliases(*outPath, inputs...); err != nil {
			return writeCommandError(args, stdout, stderr, err, demoAnticheatUsage, exitInvalidArgs)
		}
	}

	baseline := anticheat.DefaultBaseline()
	if strings.TrimSpace(*baselinePath) != "" {
		loaded, err := loadAnticheatBaseline(*baselinePath)
		if err != nil {
			return writeCommandError(args, stdout, stderr, err, "", exitUnexpected)
		}
		baseline = loaded
	}

	report, err := analyzeDemoFile(*demoPath, baseline)
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, "", exitUnexpected)
	}

	absInput, _ := filepath.Abs(*demoPath)
	result := demoAnticheatResult{
		OK:       true,
		DryRun:   *dryRun,
		Executed: !*dryRun,
		Input:    absInput,
		Report:   report,
	}
	if id := strings.TrimSpace(*dossierID); id != "" {
		player, found := report.Player(id)
		if !found {
			return writeCommandError(args, stdout, stderr,
				fmt.Errorf("steamid %s is not in this demo's analysis", id), "", exitInvalidArgs)
		}
		dossier := anticheat.BuildDossier(report, player)
		result.Dossier = &dossier
	}

	if !*dryRun && strings.TrimSpace(*outPath) != "" {
		if err := writeJSONArtifact(*outPath, report); err != nil {
			fmt.Fprintf(stderr, "error: write anticheat report: %v\n", err)
			return exitUnexpected
		}
		abs, _ := filepath.Abs(*outPath)
		result.Output = abs
	}

	if *format == "json" {
		return writeJSONResult(stdout, stderr, result)
	}
	writeAnticheatText(stdout, result)
	return exitSuccess
}

func runDemoAnticheatCalibrate(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, demoAnticheatCalibrateUsage)
		return exitSuccess
	}

	fs := flag.NewFlagSet("demo anticheat calibrate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	demosDir := fs.String("demos", "", "directory of professional .dem files to measure")
	id := fs.String("id", "", "identifier for the produced baseline")
	outPath := fs.String("out", "", "baseline JSON to write")
	format := fs.String("format", "text", "text or json")
	dryRun := fs.Bool("dry-run", false, "measure without writing the baseline")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(args, stdout, stderr, err, demoAnticheatCalibrateUsage, exitInvalidArgs)
	}
	if fs.NArg() != 0 {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unexpected positional arg %q", fs.Arg(0)), demoAnticheatCalibrateUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(*demosDir) == "" || strings.TrimSpace(*id) == "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("--demos and --id are required"), demoAnticheatCalibrateUsage, exitInvalidArgs)
	}
	if !*dryRun && strings.TrimSpace(*outPath) == "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("--out is required unless --dry-run"), demoAnticheatCalibrateUsage, exitInvalidArgs)
	}
	if *format != "text" && *format != "json" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unsupported format %q", *format), demoAnticheatCalibrateUsage, exitInvalidArgs)
	}

	demos, err := listDemoFiles(*demosDir)
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, "", exitUnexpected)
	}
	if len(demos) == 0 {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("no .dem files under %s", *demosDir), "", exitInvalidArgs)
	}
	if !*dryRun {
		inputs := make([]pathguard.Input, 0, len(demos))
		for _, demo := range demos {
			inputs = append(inputs, pathguard.Input{Flag: "--demos", Path: demo})
		}
		if err := pathguard.RejectOutputAliases(*outPath, inputs...); err != nil {
			return writeCommandError(args, stdout, stderr, err, demoAnticheatCalibrateUsage, exitInvalidArgs)
		}
	}

	// A calibration run is long and unattended, so one unreadable demo must
	// not discard the work already done on the others; the skipped map is
	// reported so the operator knows the sample it actually got.
	skipped := map[string]string{}
	reports := make([]anticheat.Report, 0, len(demos))
	measured := make([]string, 0, len(demos))
	for _, demo := range demos {
		report, err := analyzeDemoFile(demo, anticheat.DefaultBaseline())
		if err != nil {
			skipped[demo] = err.Error()
			continue
		}
		reports = append(reports, report)
		measured = append(measured, demo)
	}
	if len(reports) == 0 {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("every demo under %s failed to parse", *demosDir), "", exitUnexpected)
	}

	baseline, distinct, err := anticheat.Calibrate(*id, fmt.Sprintf("medida sobre demos locales de %s", filepath.Base(strings.TrimRight(*demosDir, `/\`))), reports)
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, "", exitUnexpected)
	}

	result := demoAnticheatCalibrateResult{
		OK:            true,
		DryRun:        *dryRun,
		Executed:      !*dryRun,
		Demos:         measured,
		DistinctDemos: distinct,
		Baseline:      baseline,
	}
	if len(skipped) > 0 {
		result.Skipped = skipped
	}
	if !*dryRun {
		if err := writeJSONArtifact(*outPath, baseline); err != nil {
			fmt.Fprintf(stderr, "error: write baseline: %v\n", err)
			return exitUnexpected
		}
		abs, _ := filepath.Abs(*outPath)
		result.Output = abs
	}

	if *format == "json" {
		return writeJSONResult(stdout, stderr, result)
	}
	fmt.Fprintf(stdout, "baseline %s medida sobre %d demos distintas (%d analizadas)", baseline.ID, distinct, len(reports))
	if len(skipped) > 0 {
		fmt.Fprintf(stdout, " (%d omitidas)", len(skipped))
	}
	if result.Output != "" {
		fmt.Fprintf(stdout, " -> %s", result.Output)
	}
	fmt.Fprintln(stdout)
	return exitSuccess
}

// analyzeDemoFile screens one demo file and stamps the report with the demo's
// name and content hash, so a dossier can name the exact file it came from.
func analyzeDemoFile(path string, baseline anticheat.Baseline) (anticheat.Report, error) {
	// #nosec G304 -- the demo path is an explicit operator-supplied argument.
	f, err := os.Open(path)
	if err != nil {
		return anticheat.Report{}, fmt.Errorf("open demo: %w", err)
	}
	defer f.Close()

	sum, err := hashAndRewind(f)
	if err != nil {
		return anticheat.Report{}, err
	}

	p := demoinfocs.NewParser(f)
	defer p.Close()

	report, err := anticheat.Analyze(p, anticheat.Options{
		Baseline: baseline,
		DemoPath: filepath.Base(path),
		SHA256:   sum,
	})
	if err != nil {
		return anticheat.Report{}, fmt.Errorf("analyze %s: %w", filepath.Base(path), err)
	}
	return report, nil
}

// hashAndRewind returns the SHA-256 of f and leaves it positioned at the start
// so the parser reads the same bytes that were hashed.
func hashAndRewind(f *os.File) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash demo: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind demo: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// listDemoFiles returns the .dem files directly under dir, sorted, so a
// calibration run is reproducible.
func listDemoFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read demo directory: %w", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".dem") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}

func loadAnticheatBaseline(path string) (anticheat.Baseline, error) {
	// #nosec G304 -- the baseline path is an explicit operator-supplied argument.
	f, err := os.Open(path)
	if err != nil {
		return anticheat.Baseline{}, fmt.Errorf("open baseline: %w", err)
	}
	defer f.Close()
	return anticheat.LoadBaseline(f)
}

func writeJSONResult(stdout, stderr io.Writer, v any) int {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "error: encode result: %v\n", err)
		return exitUnexpected
	}
	fmt.Fprintln(stdout, string(b))
	return exitSuccess
}

// writeAnticheatText prints the scoreboard a reviewer reads first: worst score
// on top, with the verdict spelled out rather than left as a raw number.
func writeAnticheatText(stdout io.Writer, result demoAnticheatResult) {
	r := result.Report
	fmt.Fprintf(stdout, "CheaterDetect · %s · %d rondas · base %s\n",
		orUnknown(r.Match.Map), r.Match.Rounds, r.Baseline.ID)
	if !r.Baseline.Measured {
		fmt.Fprintln(stdout, "aviso: la línea base no está medida; recalíbrala con `zv demo anticheat calibrate`")
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "%-24s %-20s %6s %6s %5s  %s\n", "JUGADOR", "STEAMID64", "SCORE", "CONF", "KILLS", "VEREDICTO")
	for _, p := range r.Players {
		fmt.Fprintf(stdout, "%-24s %-20s %6.1f %5.0f%% %5d  %s\n",
			truncate(p.Name, 24), p.SteamID64, p.Score, p.Confidence*100, p.GunKills, anticheat.VerdictLabel(p.Verdict))
	}
	if result.Dossier != nil {
		fmt.Fprintf(stdout, "\n%s\n", result.Dossier.Markdown)
	}
	if result.Output != "" {
		fmt.Fprintf(stdout, "\nescrito: %s\n", result.Output)
	}
}

func orUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "mapa desconocido"
	}
	return s
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
