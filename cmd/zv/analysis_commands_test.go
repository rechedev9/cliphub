package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/fragforge/internal/tacticalplan"
)

// writeTacticalDocument writes the committed reference tactical document into
// dir, so a test exercises the same artifact the docs point agents at.
func writeTacticalDocument(t *testing.T, dir string) string {
	t.Helper()
	source := filepath.Join(repoRoot(t), "testdata", "agent-tactical.json")
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read reference tactical document: %v", err)
	}
	path := filepath.Join(dir, "tactical.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunAnalysisTacticalFlagValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing demo and out", args: []string{"--dry-run"}, want: "--demo and --out are required"},
		{name: "missing out", args: []string{"--demo", "match.dem"}, want: "--demo and --out are required"},
		{name: "unsupported format", args: []string{"--demo", "match.dem", "--out", "t.json", "--format", "yaml"}, want: `unsupported format "yaml"`},
		{name: "hz above maximum", args: []string{"--demo", "match.dem", "--out", "t.json", "--hz", "128"}, want: "--hz 128 must be in (0, 64]"},
		{name: "zero hz", args: []string{"--demo", "match.dem", "--out", "t.json", "--hz", "0"}, want: "--hz 0 must be in (0, 64]"},
		{name: "negative cell size", args: []string{"--demo", "match.dem", "--out", "t.json", "--cell-size", "-1"}, want: "--cell-size -1 must be positive"},
		{name: "unknown flag", args: []string{"--demo", "match.dem", "--out", "t.json", "--nope"}, want: "flag provided but not defined: -nope"},
		{name: "positional arg", args: []string{"--demo", "match.dem", "--out", "t.json", "extra"}, want: `unexpected positional arg "extra"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runAnalysisTactical(tt.args, &stdout, &stderr); code != exitInvalidArgs {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, exitInvalidArgs, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), tt.want)
			}
		})
	}
}

func TestRunAnalysisTacticalRejectsOutputAliasingTheDemo(t *testing.T) {
	dir := t.TempDir()
	demo := filepath.Join(dir, "match.dem")
	if err := os.WriteFile(demo, []byte("not a real demo"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runAnalysisTactical([]string{"--demo", demo, "--out", demo, "--dry-run"}, &stdout, &stderr); code != exitInvalidArgs {
		t.Fatalf("exit code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(stderr.String(), "--out must not overwrite --demo") {
		t.Fatalf("stderr = %q, want the output-alias rejection", stderr.String())
	}
}

func TestRunAnalysisTacticalRejectsPositionsAliasingTheDocument(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "tactical.json")
	var stdout, stderr bytes.Buffer
	args := []string{"--demo", filepath.Join(dir, "match.dem"), "--out", out, "--positions", out, "--dry-run"}
	if code := runAnalysisTactical(args, &stdout, &stderr); code != exitInvalidArgs {
		t.Fatalf("exit code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(stderr.String(), "--out must not overwrite --out") {
		t.Fatalf("stderr = %q, want the output-alias rejection", stderr.String())
	}
}

func TestRunAnalysisTacticalDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "tactical.json")
	positions := filepath.Join(dir, "positions.zvpos")

	var stdout, stderr bytes.Buffer
	args := []string{"--demo", filepath.Join(dir, "match.dem"), "--out", out, "--positions", positions, "--dry-run", "--format", "json"}
	if code := runAnalysisTactical(args, &stdout, &stderr); code != exitSuccess {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, exitSuccess, stderr.String())
	}

	var result analysisTacticalResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal result: %v\n%s", err, stdout.String())
	}
	if !result.OK || !result.DryRun || result.Executed {
		t.Fatalf("result = %#v, want ok dry run that did not execute", result)
	}
	if result.SampleHZ != 8 || result.CellSize != 64 {
		t.Fatalf("result sample = %v Hz, cell size = %v; want the package defaults", result.SampleHZ, result.CellSize)
	}
	if result.Rounds != 0 || result.Frames != 0 {
		t.Fatalf("result = %#v, want no scan totals from a dry run", result)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("dry run created %d entries in %s, want none", len(entries), dir)
	}
}

func TestRunAnalysisRoundsFilterFlagsUseTheSharedVocabulary(t *testing.T) {
	// Every documented CLI filter flag must land on a key
	// tacticalplan.FilterFromValues understands, so the CLI never grows a
	// second filter parser.
	dir := t.TempDir()
	document := writeTacticalDocument(t, dir)

	tests := []struct {
		name  string
		args  []string
		check func(t *testing.T, got tacticalplan.Filter)
	}{
		{
			name: "side and outcome",
			args: []string{"--side", "T", "--outcome", "win"},
			check: func(t *testing.T, got tacticalplan.Filter) {
				if got.Side != tacticalplan.SideT || got.Outcome != tacticalplan.OutcomeWin {
					t.Fatalf("filter = %#v", got)
				}
			},
		},
		{
			name: "repeated buy flags are ORed",
			args: []string{"--buy", "eco", "--buy", "force"},
			check: func(t *testing.T, got tacticalplan.Filter) {
				if len(got.Buys) != 2 || got.Buys[0] != tacticalplan.BuyEco || got.Buys[1] != tacticalplan.BuyForce {
					t.Fatalf("filter buys = %#v", got.Buys)
				}
			},
		},
		{
			name: "comma-separated sites match repeated flags",
			args: []string{"--site", "a,b"},
			check: func(t *testing.T, got tacticalplan.Filter) {
				if len(got.Sites) != 2 || got.Sites[0] != tacticalplan.SiteA || got.Sites[1] != tacticalplan.SiteB {
					t.Fatalf("filter sites = %#v", got.Sites)
				}
			},
		},
		{
			name: "patterns, tags, slots, bounds, and phase",
			args: []string{
				"--t-pattern", "execute", "--ct-pattern", "retake", "--tag", "postplant",
				"--slot", "2", "--round-from", "1", "--round-to", "4", "--phase", "regulation",
				"--team", "t-start", "--opponent-buy", "full",
			},
			check: func(t *testing.T, got tacticalplan.Filter) {
				if len(got.TPatterns) != 1 || got.TPatterns[0] != tacticalplan.TExecute {
					t.Fatalf("filter t patterns = %#v", got.TPatterns)
				}
				if len(got.CTPatterns) != 1 || got.CTPatterns[0] != tacticalplan.CTRetake {
					t.Fatalf("filter ct patterns = %#v", got.CTPatterns)
				}
				if len(got.Tags) != 1 || got.Tags[0] != tacticalplan.TagPostPlant {
					t.Fatalf("filter tags = %#v", got.Tags)
				}
				if len(got.Slots) != 1 || got.Slots[0] != 2 {
					t.Fatalf("filter slots = %#v", got.Slots)
				}
				if got.RoundFrom != 1 || got.RoundTo != 4 || got.Phase != tacticalplan.PhaseRegulation {
					t.Fatalf("filter bounds = %#v", got)
				}
				if got.TeamKey != "t-start" {
					t.Fatalf("filter team = %q", got.TeamKey)
				}
				if len(got.OpponentBuys) != 1 || got.OpponentBuys[0] != tacticalplan.BuyFull {
					t.Fatalf("filter opponent buys = %#v", got.OpponentBuys)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"--tactical", document, "--format", "json"}, tt.args...)
			var stdout, stderr bytes.Buffer
			if code := runAnalysisRounds(args, &stdout, &stderr); code != exitSuccess {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, exitSuccess, stderr.String())
			}
			var result analysisRoundsResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("unmarshal result: %v\n%s", err, stdout.String())
			}
			tt.check(t, result.Filter)
		})
	}
}

func TestRunAnalysisRoundsRejectsUnknownFilterValues(t *testing.T) {
	dir := t.TempDir()
	document := writeTacticalDocument(t, dir)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown side", args: []string{"--side", "spectator"}, want: `filter: unknown side "spectator"`},
		{name: "unknown buy", args: []string{"--buy", "rich"}, want: `filter: unknown buy type "rich"`},
		{name: "unknown site", args: []string{"--site", "c"}, want: `filter: unknown site "c"`},
		{name: "inverted bounds", args: []string{"--round-from", "9", "--round-to", "2"}, want: "round_from 9 is after round_to 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := append([]string{"--tactical", document}, tt.args...)
			if code := runAnalysisRounds(args, &stdout, &stderr); code != exitInvalidArgs {
				t.Fatalf("exit code = %d, want %d", code, exitInvalidArgs)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), tt.want)
			}
		})
	}
}

func TestRunAnalysisRoundsRequiresTacticalDocument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runAnalysisRounds([]string{"--side", "T"}, &stdout, &stderr); code != exitInvalidArgs {
		t.Fatalf("exit code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(stderr.String(), "--tactical is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunAnalysisRoundsJSONShape(t *testing.T) {
	dir := t.TempDir()
	document := writeTacticalDocument(t, dir)

	var stdout, stderr bytes.Buffer
	args := []string{"--tactical", document, "--format", "json", "--side", "T", "--buy", "full"}
	if code := runAnalysisRounds(args, &stdout, &stderr); code != exitSuccess {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, exitSuccess, stderr.String())
	}
	var result analysisRoundsResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal result: %v\n%s", err, stdout.String())
	}
	if !result.OK || result.Map != "de_inferno" || result.Total != 6 {
		t.Fatalf("result = %#v, want the reference document totals", result)
	}
	if result.Count != len(result.Rounds) {
		t.Fatalf("count = %d but %d rounds were listed", result.Count, len(result.Rounds))
	}
	if result.Count != 2 {
		t.Fatalf("count = %d, want the two T full-buy rounds", result.Count)
	}
	for _, round := range result.Rounds {
		if round.TBuy != string(tacticalplan.BuyFull) {
			t.Fatalf("round %d t buy = %q, want full", round.Number, round.TBuy)
		}
		if round.Number == 0 || round.Site == "" || round.TPattern == "" || round.CTPattern == "" {
			t.Fatalf("round summary is missing scannable columns: %#v", round)
		}
	}
}

func TestRunAnalysisRoundsTextTableIsFixedWidth(t *testing.T) {
	dir := t.TempDir()
	document := writeTacticalDocument(t, dir)

	var stdout, stderr bytes.Buffer
	if code := runAnalysisRounds([]string{"--tactical", document}, &stdout, &stderr); code != exitSuccess {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, exitSuccess, stderr.String())
	}
	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 8 {
		t.Fatalf("got %d lines, want a header, six rounds, and a summary:\n%s", len(lines), stdout.String())
	}
	if strings.Contains(stdout.String(), "\t") {
		t.Fatalf("round table used tabs, want fixed-width columns:\n%s", stdout.String())
	}
	header := lines[0]
	for _, column := range []string{"half", "t buy", "ct buy", "site", "t pattern", "ct pattern", "winner", "tags"} {
		index := strings.Index(header, column)
		if index < 0 {
			t.Fatalf("header %q is missing column %q", header, column)
		}
		// A fixed-width table keeps every value aligned under its header.
		for _, row := range lines[1 : len(lines)-1] {
			if len(row) <= index {
				t.Fatalf("row %q is shorter than column %q at %d", row, column, index)
			}
		}
	}
	if !strings.HasPrefix(lines[len(lines)-1], "rounds: 6 of 6") {
		t.Fatalf("summary line = %q", lines[len(lines)-1])
	}
}

func TestRunAnalysisTendenciesJSONShape(t *testing.T) {
	dir := t.TempDir()
	document := writeTacticalDocument(t, dir)

	var stdout, stderr bytes.Buffer
	args := []string{"--tactical", document, "--format", "json", "--team", "t-start"}
	if code := runAnalysisTendencies(args, &stdout, &stderr); code != exitSuccess {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, exitSuccess, stderr.String())
	}
	var result analysisTendenciesResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal result: %v\n%s", err, stdout.String())
	}
	if !result.OK || result.MinSample != tacticalplan.MinReliableSample {
		t.Fatalf("result = %#v, want the package's reliable-sample threshold", result)
	}
	if result.Tendencies.RoundCount != 6 || result.Tendencies.Perspective != tacticalplan.SideT {
		t.Fatalf("tendencies = %#v, want six rounds seen from T", result.Tendencies)
	}
	if len(result.Tendencies.Buys) == 0 || len(result.Tendencies.Players) == 0 {
		t.Fatalf("tendencies are missing buys or players: %#v", result.Tendencies)
	}
	for _, buy := range result.Tendencies.Buys {
		if buy.WinRate.Total != buy.Rounds {
			t.Fatalf("buy %q win rate denominator = %d, want its %d rounds", buy.Buy, buy.WinRate.Total, buy.Rounds)
		}
		if buy.WinRate.Reliable != (buy.WinRate.Total >= tacticalplan.MinReliableSample) {
			t.Fatalf("buy %q reliability disagrees with its sample: %#v", buy.Buy, buy.WinRate)
		}
	}
}

func TestRunAnalysisTendenciesTextMarksLowSampleWithDenominators(t *testing.T) {
	dir := t.TempDir()
	document := writeTacticalDocument(t, dir)

	var stdout, stderr bytes.Buffer
	if code := runAnalysisTendencies([]string{"--tactical", document, "--side", "T"}, &stdout, &stderr); code != exitSuccess {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, exitSuccess, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "rates below n=4 are marked low-sample") {
		t.Fatalf("output is missing the low-sample legend:\n%s", out)
	}
	// Every printed percentage must carry its denominator.
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "%") {
			continue
		}
		if !strings.Contains(line, "n=") {
			t.Fatalf("line %q prints a rate without its denominator", line)
		}
	}
	// The pistol round is a sample of one and must be visibly flagged.
	if !strings.Contains(out, "low-sample") {
		t.Fatalf("output never marks a low sample:\n%s", out)
	}
}

func TestRunAnalysisTendenciesRejectsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	document := writeTacticalDocument(t, dir)
	var stdout, stderr bytes.Buffer
	if code := runAnalysisTendencies([]string{"--tactical", document, "--format", "csv"}, &stdout, &stderr); code != exitInvalidArgs {
		t.Fatalf("exit code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(stderr.String(), `unsupported format "csv"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestLoadTacticalDocumentRejectsUnknownSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tactical.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"0.9"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTacticalDocument(path); err == nil || !strings.Contains(err.Error(), "unsupported tactical schema") {
		t.Fatalf("err = %v, want an unsupported-schema rejection", err)
	}
}

func TestAnalysisFilterFlagsCoverTheDocumentedVocabulary(t *testing.T) {
	// The validator's allowlist and the flag registration must not drift apart.
	got := map[string]bool{}
	for _, name := range tacticalFilterFlagNames() {
		got[name] = true
	}
	want := []string{
		"--side", "--team", "--buy", "--opponent-buy", "--site", "--outcome",
		"--t-pattern", "--ct-pattern", "--tag", "--slot", "--round-from", "--round-to", "--phase",
	}
	if len(got) != len(want) {
		t.Fatalf("filter flags = %#v, want %#v", tacticalFilterFlagNames(), want)
	}
	for _, name := range want {
		if !got[name] {
			t.Fatalf("filter flag %s is not registered", name)
		}
	}
	for _, command := range []string{`"analysis rounds"`, `"analysis tendencies"`} {
		flags := commandValueFlags(command, []string{"--tactical"})
		for _, name := range want {
			if !containsString(flags, name) {
				t.Fatalf("%s does not accept filter flag %s", command, name)
			}
		}
	}
}

func TestValidateSkillCommandAnalysisSubcommands(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		want    string
	}{
		{name: "tactical ok", command: []string{"analysis", "tactical", "--demo", "m.dem", "--out", "t.json"}},
		{name: "tactical dry run ok", command: []string{"analysis", "tactical", "--demo", "m.dem", "--out", "t.json", "--dry-run", "--format", "json"}},
		{name: "tactical missing out", command: []string{"analysis", "tactical", "--demo", "m.dem"}, want: `missing required flag --out for "analysis tactical"`},
		{name: "tactical unknown flag", command: []string{"analysis", "tactical", "--demo", "m.dem", "--out", "t.json", "--sample", "4"}, want: `unknown flag --sample for "analysis tactical"`},
		{name: "rounds ok", command: []string{"analysis", "rounds", "--tactical", "t.json", "--side", "T"}},
		{name: "rounds repeated buy ok", command: []string{"analysis", "rounds", "--tactical", "t.json", "--buy", "eco", "--buy", "force"}},
		{name: "rounds missing tactical", command: []string{"analysis", "rounds", "--side", "T"}, want: `missing required flag --tactical for "analysis rounds"`},
		{name: "tendencies ok", command: []string{"analysis", "tendencies", "--tactical", "t.json", "--tag", "postplant", "--tag", "anti_eco"}},
		{name: "tendencies unknown flag", command: []string{"analysis", "tendencies", "--tactical", "t.json", "--positions", "p.zvpos"}, want: `unknown flag --positions for "analysis tendencies"`},
		{name: "unknown subcommand", command: []string{"analysis", "wat"}, want: `uses non-standard zv command "analysis"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateSkillCommand(tt.command)
			if tt.want == "" {
				if got != "" {
					t.Fatalf("issue = %q, want none", got)
				}
				return
			}
			if !strings.Contains(got, tt.want) {
				t.Fatalf("issue = %q, want it to contain %q", got, tt.want)
			}
		})
	}
}

func TestAnalysisWorkflowSafetyMetadata(t *testing.T) {
	tests := []struct {
		name        string
		readOnly    bool
		longRunning bool
		dryRun      bool
	}{
		{name: "analysis-tactical", readOnly: false, longRunning: true, dryRun: true},
		{name: "analysis-rounds", readOnly: true, longRunning: false, dryRun: false},
		{name: "analysis-tendencies", readOnly: true, longRunning: false, dryRun: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow, ok := findWorkflow(tt.name)
			if !ok {
				t.Fatalf("workflow %q is not cataloged", tt.name)
			}
			if workflow.Safety.ReadOnly != tt.readOnly {
				t.Fatalf("read only = %v, want %v", workflow.Safety.ReadOnly, tt.readOnly)
			}
			if workflow.Safety.LongRunning != tt.longRunning {
				t.Fatalf("long running = %v, want %v", workflow.Safety.LongRunning, tt.longRunning)
			}
			if workflow.Safety.SupportsDryRun != tt.dryRun {
				t.Fatalf("supports dry run = %v, want %v", workflow.Safety.SupportsDryRun, tt.dryRun)
			}
		})
	}
}
