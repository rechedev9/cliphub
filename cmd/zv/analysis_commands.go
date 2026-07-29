package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/rechedev9/fragforge/internal/pathguard"
	"github.com/rechedev9/fragforge/internal/tactical"
	"github.com/rechedev9/fragforge/internal/tacticalplan"
)

type analysisTacticalResult struct {
	OK        bool     `json:"ok"`
	DryRun    bool     `json:"dry_run"`
	Executed  bool     `json:"executed"`
	Demo      string   `json:"demo"`
	Output    string   `json:"output"`
	Positions string   `json:"positions,omitempty"`
	SampleHZ  float64  `json:"sample_hz"`
	CellSize  float64  `json:"cell_size"`
	Map       string   `json:"map,omitempty"`
	Tickrate  float64  `json:"tickrate,omitempty"`
	Rounds    int      `json:"rounds"`
	Frames    int      `json:"frames"`
	Warnings  []string `json:"warnings,omitempty"`
}

type analysisRoundsResult struct {
	OK     bool                   `json:"ok"`
	Input  string                 `json:"input"`
	Map    string                 `json:"map,omitempty"`
	Filter tacticalplan.Filter    `json:"filter"`
	Count  int                    `json:"count"`
	Total  int                    `json:"total"`
	Rounds []analysisRoundSummary `json:"rounds"`
}

// analysisRoundSummary is the flat per-round view the CLI table and the JSON
// output share, so a scripted reader sees exactly the columns an analyst reads.
type analysisRoundSummary struct {
	Number        int      `json:"number"`
	Half          int      `json:"half"`
	Overtime      int      `json:"overtime,omitempty"`
	ScoreCTBefore int      `json:"score_ct_before"`
	ScoreTBefore  int      `json:"score_t_before"`
	TBuy          string   `json:"t_buy"`
	CTBuy         string   `json:"ct_buy"`
	Site          string   `json:"site"`
	TPattern      string   `json:"t_pattern"`
	CTPattern     string   `json:"ct_pattern"`
	Winner        string   `json:"winner"`
	EndReason     string   `json:"end_reason"`
	Tags          []string `json:"tags"`
}

type analysisTendenciesResult struct {
	OK         bool                    `json:"ok"`
	Input      string                  `json:"input"`
	Map        string                  `json:"map,omitempty"`
	MinSample  int                     `json:"min_reliable_sample"`
	Tendencies tacticalplan.Tendencies `json:"tendencies"`
}

func runAnalysisTactical(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, analysisTacticalUsage)
		return exitSuccess
	}
	fs := flag.NewFlagSet("analysis tactical", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	demoPath := fs.String("demo", "", "CS2 demo to scan")
	outPath := fs.String("out", "", "tactical document JSON artifact")
	positionsPath := fs.String("positions", "", "optional sidecar position blob")
	hz := fs.Float64("hz", tactical.DefaultSampleHZ, "position sample rate in Hz")
	cellSize := fs.Float64("cell-size", tactical.DefaultCellSize, "occupancy grid cell size in world units")
	format := fs.String("format", "text", "text or json")
	dryRun := fs.Bool("dry-run", false, "validate inputs and print the plan without scanning or writing")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(args, stdout, stderr, err, analysisTacticalUsage, exitInvalidArgs)
	}
	if fs.NArg() != 0 {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unexpected positional arg %q", fs.Arg(0)), analysisTacticalUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(*demoPath) == "" || strings.TrimSpace(*outPath) == "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("--demo and --out are required"), analysisTacticalUsage, exitInvalidArgs)
	}
	if *format != "text" && *format != "json" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unsupported format %q", *format), analysisTacticalUsage, exitInvalidArgs)
	}
	if math.IsNaN(*hz) || math.IsInf(*hz, 0) || *hz <= 0 || *hz > tactical.MaxSampleHZ {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("--hz %v must be in (0, %d]", *hz, tactical.MaxSampleHZ), analysisTacticalUsage, exitInvalidArgs)
	}
	if math.IsNaN(*cellSize) || math.IsInf(*cellSize, 0) || *cellSize <= 0 {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("--cell-size %v must be positive", *cellSize), analysisTacticalUsage, exitInvalidArgs)
	}
	if err := pathguard.RejectOutputAliases(*outPath, pathguard.Input{Flag: "--demo", Path: *demoPath}); err != nil {
		return writeCommandError(args, stdout, stderr, err, analysisTacticalUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(*positionsPath) != "" {
		if err := pathguard.RejectOutputAliases(*positionsPath,
			pathguard.Input{Flag: "--demo", Path: *demoPath},
			pathguard.Input{Flag: "--out", Path: *outPath},
		); err != nil {
			return writeCommandError(args, stdout, stderr, err, analysisTacticalUsage, exitInvalidArgs)
		}
	}
	// A dry run neither reads the demo nor writes an artifact: it settles the
	// argv and prints the plan, so it stays usable before the demo is on disk.
	absDemo, _ := filepath.Abs(*demoPath)
	absOut, _ := filepath.Abs(*outPath)
	result := analysisTacticalResult{
		OK:       true,
		DryRun:   *dryRun,
		Executed: !*dryRun,
		Demo:     absDemo,
		Output:   absOut,
		SampleHZ: *hz,
		CellSize: *cellSize,
	}
	if strings.TrimSpace(*positionsPath) != "" {
		result.Positions, _ = filepath.Abs(*positionsPath)
	}

	if !*dryRun {
		scan, err := tactical.ScanFile(context.Background(), *demoPath, tactical.Options{
			SampleHZ: *hz,
			CellSize: *cellSize,
		})
		if err != nil {
			return writeCommandError(args, stdout, stderr, fmt.Errorf("scan tactical: %w", err), "", exitUnexpected)
		}
		result.Map = scan.Document.Demo.Map
		result.Tickrate = scan.Document.Demo.Tickrate
		result.Rounds = len(scan.Document.Rounds)
		result.Frames = scan.Document.Positions.FrameCount
		result.Warnings = scan.Document.Warnings
		if err := writeJSONArtifact(absOut, scan.Document); err != nil {
			return writeCommandError(args, stdout, stderr, fmt.Errorf("write tactical document: %w", err), "", exitUnexpected)
		}
		if result.Positions != "" {
			if err := writeArtifact(result.Positions, scan.Positions.Data); err != nil {
				return writeCommandError(args, stdout, stderr, fmt.Errorf("write position blob: %w", err), "", exitUnexpected)
			}
		}
	}

	if *format == "json" {
		if err := writeJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "error: write analysis tactical result: %v\n", err)
			return exitUnexpected
		}
		return exitSuccess
	}
	fmt.Fprintf(stdout, "demo: %s\n", result.Demo)
	fmt.Fprintf(stdout, "sample: %g Hz, cell size: %g units\n", result.SampleHZ, result.CellSize)
	if *dryRun {
		fmt.Fprintf(stdout, "document: %s (not written)\n", result.Output)
		if result.Positions != "" {
			fmt.Fprintf(stdout, "positions: %s (not written)\n", result.Positions)
		}
		return exitSuccess
	}
	fmt.Fprintf(stdout, "map: %s, tickrate: %g, rounds: %d, frames: %d\n", result.Map, result.Tickrate, result.Rounds, result.Frames)
	fmt.Fprintf(stdout, "document: %s\n", result.Output)
	if result.Positions != "" {
		fmt.Fprintf(stdout, "positions: %s\n", result.Positions)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(stdout, "warning: %s\n", warning)
	}
	return exitSuccess
}

func runAnalysisRounds(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, analysisRoundsUsage)
		return exitSuccess
	}
	fs := flag.NewFlagSet("analysis rounds", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tacticalPath := fs.String("tactical", "", "tactical document JSON to read")
	format := fs.String("format", "text", "text or json")
	values := registerTacticalFilterFlags(fs)
	if err := fs.Parse(args); err != nil {
		return writeCommandError(args, stdout, stderr, err, analysisRoundsUsage, exitInvalidArgs)
	}
	doc, filter, code := loadFilteredTactical(args, stdout, stderr, fs, *tacticalPath, *format, values, analysisRoundsUsage)
	if code != exitSuccess {
		return code
	}

	rounds := filter.Apply(doc)
	absInput, _ := filepath.Abs(*tacticalPath)
	result := analysisRoundsResult{
		OK:     true,
		Input:  absInput,
		Map:    doc.Demo.Map,
		Filter: filter,
		Count:  len(rounds),
		Total:  len(doc.Rounds),
		Rounds: make([]analysisRoundSummary, 0, len(rounds)),
	}
	for _, r := range rounds {
		tags := r.Class.Tags
		if tags == nil {
			tags = []string{}
		}
		result.Rounds = append(result.Rounds, analysisRoundSummary{
			Number:        r.Number,
			Half:          r.Half,
			Overtime:      r.Overtime,
			ScoreCTBefore: r.ScoreCTBefore,
			ScoreTBefore:  r.ScoreTBefore,
			TBuy:          string(r.Economy.TBuy),
			CTBuy:         string(r.Economy.CTBuy),
			Site:          string(r.Class.Site),
			TPattern:      string(r.Class.TSide),
			CTPattern:     string(r.Class.CTSide),
			Winner:        string(r.Winner),
			EndReason:     r.EndReason,
			Tags:          tags,
		})
	}

	if *format == "json" {
		if err := writeJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "error: write analysis rounds result: %v\n", err)
			return exitUnexpected
		}
		return exitSuccess
	}

	// A fixed-width table, not tabs: an analyst scans these columns by eye and
	// the widths must not move between rounds or between runs.
	const rowFormat = "%4s  %4s  %7s  %-7s  %-7s  %-4s  %-9s  %-10s  %-6s  %s\n"
	fmt.Fprintf(stdout, rowFormat, "#", "half", "ct-t", "t buy", "ct buy", "site", "t pattern", "ct pattern", "winner", "tags")
	for _, r := range result.Rounds {
		fmt.Fprintf(stdout, rowFormat,
			fmt.Sprintf("%d", r.Number),
			fmt.Sprintf("%d", r.Half),
			fmt.Sprintf("%d-%d", r.ScoreCTBefore, r.ScoreTBefore),
			r.TBuy, r.CTBuy, r.Site, r.TPattern, r.CTPattern, dashIfEmpty(r.Winner),
			dashIfEmpty(strings.Join(r.Tags, ",")))
	}
	fmt.Fprintf(stdout, "rounds: %d of %d\n", result.Count, result.Total)
	return exitSuccess
}

func runAnalysisTendencies(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, analysisTendenciesUsage)
		return exitSuccess
	}
	fs := flag.NewFlagSet("analysis tendencies", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	tacticalPath := fs.String("tactical", "", "tactical document JSON to read")
	format := fs.String("format", "text", "text or json")
	values := registerTacticalFilterFlags(fs)
	if err := fs.Parse(args); err != nil {
		return writeCommandError(args, stdout, stderr, err, analysisTendenciesUsage, exitInvalidArgs)
	}
	doc, filter, code := loadFilteredTactical(args, stdout, stderr, fs, *tacticalPath, *format, values, analysisTendenciesUsage)
	if code != exitSuccess {
		return code
	}

	absInput, _ := filepath.Abs(*tacticalPath)
	result := analysisTendenciesResult{
		OK:         true,
		Input:      absInput,
		Map:        doc.Demo.Map,
		MinSample:  tacticalplan.MinReliableSample,
		Tendencies: tacticalplan.Aggregate(doc, filter),
	}
	if *format == "json" {
		if err := writeJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "error: write analysis tendencies result: %v\n", err)
			return exitUnexpected
		}
		return exitSuccess
	}
	writeTendenciesText(stdout, result)
	return exitSuccess
}

// loadFilteredTactical performs the validation every reader of a tactical
// document shares: no positional args, a supported format, a readable document,
// and a filter parsed through the one shared vocabulary.
func loadFilteredTactical(args []string, stdout, stderr io.Writer, fs *flag.FlagSet, tacticalPath, format string, values url.Values, commandUsage string) (tacticalplan.Document, tacticalplan.Filter, int) {
	if fs.NArg() != 0 {
		return tacticalplan.Document{}, tacticalplan.Filter{}, writeCommandError(args, stdout, stderr, fmt.Errorf("unexpected positional arg %q", fs.Arg(0)), commandUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(tacticalPath) == "" {
		return tacticalplan.Document{}, tacticalplan.Filter{}, writeCommandError(args, stdout, stderr, fmt.Errorf("--tactical is required"), commandUsage, exitInvalidArgs)
	}
	if format != "text" && format != "json" {
		return tacticalplan.Document{}, tacticalplan.Filter{}, writeCommandError(args, stdout, stderr, fmt.Errorf("unsupported format %q", format), commandUsage, exitInvalidArgs)
	}
	filter, err := tacticalplan.FilterFromValues(values)
	if err != nil {
		return tacticalplan.Document{}, tacticalplan.Filter{}, writeCommandError(args, stdout, stderr, err, commandUsage, exitInvalidArgs)
	}
	doc, err := loadTacticalDocument(tacticalPath)
	if err != nil {
		return tacticalplan.Document{}, tacticalplan.Filter{}, writeCommandError(args, stdout, stderr, fmt.Errorf("read tactical document: %w", err), "", exitUnexpected)
	}
	return doc, filter, exitSuccess
}

// tacticalFilterFlags maps each CLI filter flag onto the query key
// tacticalplan.FilterFromValues already understands, so the CLI and the HTTP
// API share one filter vocabulary instead of growing a second parser.
func tacticalFilterFlags() []struct {
	Name  string
	Key   string
	Usage string
} {
	return []struct {
		Name  string
		Key   string
		Usage string
	}{
		{"side", "side", "perspective side: CT or T"},
		{"team", "team", "team key to follow across the side swap"},
		{"buy", "buy", "buy type; repeat or comma-separate to OR"},
		{"opponent-buy", "opponent_buy", "opponent buy type; repeat or comma-separate to OR"},
		{"site", "site", "site: a, b, mid, or none; repeatable"},
		{"outcome", "outcome", "win or loss, from the perspective"},
		{"t-pattern", "t_pattern", "T-side round pattern; repeatable"},
		{"ct-pattern", "ct_pattern", "CT-side round pattern; repeatable"},
		{"tag", "tag", "round tag that must be present; repeatable (AND)"},
		{"slot", "slot", "player slot that must have played; repeatable"},
		{"round-from", "round_from", "first round number"},
		{"round-to", "round_to", "last round number"},
		{"phase", "phase", "regulation or overtime"},
	}
}

func registerTacticalFilterFlags(fs *flag.FlagSet) url.Values {
	values := url.Values{}
	for _, mapping := range tacticalFilterFlags() {
		fs.Var(filterQueryFlag{values: values, key: mapping.Key}, mapping.Name, mapping.Usage)
	}
	return values
}

// filterQueryFlag accumulates one CLI flag into the shared filter query.
// Repeating a flag ORs its values, matching "buy=eco&buy=force" on the API.
type filterQueryFlag struct {
	values url.Values
	key    string
}

func (f filterQueryFlag) String() string { return strings.Join(f.values[f.key], ",") }

func (f filterQueryFlag) Set(value string) error {
	f.values.Add(f.key, value)
	return nil
}

func loadTacticalDocument(path string) (tacticalplan.Document, error) {
	// #nosec G304 -- the CLI operator explicitly supplies the local document path.
	body, err := os.ReadFile(path)
	if err != nil {
		return tacticalplan.Document{}, err
	}
	var doc tacticalplan.Document
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return tacticalplan.Document{}, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return tacticalplan.Document{}, err
	}
	if doc.SchemaVersion != tacticalplan.SchemaVersion {
		return tacticalplan.Document{}, fmt.Errorf("unsupported tactical schema %q (want %q)", doc.SchemaVersion, tacticalplan.SchemaVersion)
	}
	return doc, nil
}

func writeTendenciesText(stdout io.Writer, result analysisTendenciesResult) {
	t := result.Tendencies
	fmt.Fprintf(stdout, "tactical: %s\n", result.Input)
	if result.Map != "" {
		fmt.Fprintf(stdout, "map: %s\n", result.Map)
	}
	fmt.Fprintf(stdout, "rounds: %d, wins: %d, perspective: %s\n", t.RoundCount, t.Wins, dashIfEmpty(string(t.Perspective)))
	fmt.Fprintf(stdout, "rates below n=%d are marked low-sample\n\n", result.MinSample)

	fmt.Fprintln(stdout, "buys")
	for _, b := range t.Buys {
		fmt.Fprintf(stdout, "  %-8s rounds %3d  share %s  win %s  conversion %s\n",
			b.Buy, b.Rounds, formatRate(b.Share), formatRate(b.WinRate), formatRate(b.PlantOrDefuse))
	}

	fmt.Fprintln(stdout, "matchups")
	for _, m := range t.Matchups {
		fmt.Fprintf(stdout, "  %-8s vs %-8s rounds %3d  win %s\n", m.Buy, m.OpponentBuy, m.Rounds, formatRate(m.WinRate))
	}

	fmt.Fprintln(stdout, "sites")
	for _, s := range t.Sites {
		fmt.Fprintf(stdout, "  %-8s rounds %3d  share %s  win %s\n", s.Site, s.Rounds, formatRate(s.Share), formatRate(s.WinRate))
	}

	fmt.Fprintln(stdout, "buy x site")
	for _, b := range t.BuySites {
		fmt.Fprintf(stdout, "  %-8s %-6s rounds %3d  share %s  win %s\n",
			b.Buy, b.Site, b.Rounds, formatRate(b.Share), formatRate(b.WinRate))
	}

	fmt.Fprintln(stdout, "t patterns")
	writePatternBuckets(stdout, t.TPatterns)
	fmt.Fprintln(stdout, "ct patterns")
	writePatternBuckets(stdout, t.CTPatterns)

	fmt.Fprintln(stdout, "openings")
	fmt.Fprintf(stdout, "  duels %d\n", t.Openings.Rounds)
	fmt.Fprintf(stdout, "  won                       %s\n", formatRate(t.Openings.Won))
	fmt.Fprintf(stdout, "  traded after losing entry %s\n", formatRate(t.Openings.TradedOnLoss))
	fmt.Fprintf(stdout, "  round won after entry win %s\n", formatRate(t.Openings.RoundWinAfter))
	fmt.Fprintf(stdout, "  round won after entry loss %s\n", formatRate(t.Openings.RoundWinLost))

	fmt.Fprintln(stdout, "timings (seconds from freeze-time end)")
	writeHistogram(stdout, "first contact", t.Timings.FirstContact)
	writeHistogram(stdout, "plant", t.Timings.Plant)
	writeHistogram(stdout, "round duration", t.Timings.RoundDuration)

	fmt.Fprintln(stdout, "players")
	fmt.Fprintf(stdout, "  %-4s %-18s %5s %4s %4s %4s %6s %4s %4s %5s  %s\n",
		"slot", "name", "rnds", "k", "d", "a", "adr", "ok", "od", "trade", "survival")
	for _, p := range t.Players {
		fmt.Fprintf(stdout, "  %-4d %-18s %5d %4d %4d %4d %6.1f %4d %4d %5d  %s\n",
			p.Slot, truncate(p.Name, 18), p.Rounds, p.Kills, p.Deaths, p.Assists, p.ADR,
			p.OpeningKills, p.OpeningDeaths, p.TradeKills, formatRate(p.SurvivalRate))
	}
}

func writePatternBuckets(stdout io.Writer, buckets []tacticalplan.PatternBucket) {
	for _, b := range buckets {
		fmt.Fprintf(stdout, "  %-11s rounds %3d  share %s  win %s\n",
			b.Pattern, b.Rounds, formatRate(b.Share), formatRate(b.WinRate))
	}
}

func writeHistogram(stdout io.Writer, label string, h tacticalplan.Histogram) {
	fmt.Fprintf(stdout, "  %-15s n=%d median %.1fs\n", label, h.Samples, h.Median)
	for _, bucket := range h.Buckets {
		if bucket.Count == 0 {
			continue
		}
		fmt.Fprintf(stdout, "    %3d-%3ds %s (%d)\n",
			bucket.FromSeconds, bucket.FromSeconds+h.BucketSeconds, strings.Repeat("#", bucket.Count), bucket.Count)
	}
}

// formatRate always prints the denominator, and marks any rate whose sample is
// below tacticalplan.MinReliableSample: a 1-of-1 site take is not a tendency.
func formatRate(r tacticalplan.Rate) string {
	out := fmt.Sprintf("%5.1f%% (%d of n=%d)", r.Pct, r.Count, r.Total)
	if !r.Reliable {
		out += " low-sample"
	}
	return out
}

func dashIfEmpty(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
