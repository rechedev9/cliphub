package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// flowRunReport is the machine-readable summary of a `zv flows run` dry-run: one
// entry per phase in flow order, plus the global outcome. Global OK is false
// when any read-only preflight failed or a dependent phase could not be
// validated. Intentional creative gates do not flip it.
type flowRunReport struct {
	OK     bool                 `json:"ok"`
	Flow   string               `json:"flow"`
	RunDir string               `json:"run_dir"`
	DryRun bool                 `json:"dry_run"`
	Phases []flowRunPhaseReport `json:"phases"`
}

type flowRunPhaseReport struct {
	Phase    string   `json:"phase"`
	Argv     []string `json:"argv,omitempty"`
	OK       bool     `json:"ok"`
	DryRun   bool     `json:"dry_run"`
	Executed bool     `json:"executed"`
	Skipped  bool     `json:"skipped,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Outputs  []string `json:"outputs,omitempty"`
}

// flowRunStep is one phase of a production flow's declarative dry-run.
type flowRunStep struct {
	id    string
	build func() (flowRunAction, error)
}

// flowRunAction describes the argv a phase would run, or why it is gated/skipped.
type flowRunAction struct {
	argv        []string
	gate        bool
	skip        bool
	unvalidated bool
	reason      string
}

// runFlowsRun executes only read-only phase preflights and never writes a run
// artifact. Real execution stays stage by stage behind the creative gates, so
// a non-dry-run invocation is rejected.
func runFlowsRun(args []string, stdout, stderr io.Writer, stdin io.Reader, runner commandRunner) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, flowsRunUsage)
		return exitSuccess
	}
	// Validate the argv with the same canonical rules as every other command
	// (unknown/duplicate/missing flags, stray positionals) so direct invocations
	// and documented command lines report identical errors.
	if issue := validateFlowsRunCommand(args); issue != "" {
		return writeFlowError(args, stdout, stderr, fmt.Errorf("%s", issue), flowsRunUsage)
	}

	// The flow name is always the first token (validateFlowsRunCommand enforces
	// this), so the rest are the flow flags. Never scan for the first non-dash
	// token: that stole flag values from lines like "flows run --run-dir X demo".
	flowName := args[0]
	rest := args[1:]

	fs := flag.NewFlagSet("flows run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	runDir := fs.String("run-dir", "", "run output directory")
	dryRun := fs.Bool("dry-run", false, "resolve and chain every phase safely")
	format := fs.String("format", "text", "text or json")
	demo := fs.String("demo", "", "demo .dem path")
	steamid := fs.String("steamid", "", "target SteamID64")
	killplanPath := fs.String("killplan", "", "existing kill plan JSON; skips demo parse")
	input := fs.String("input", "", "stream video path")
	if err := fs.Parse(rest); err != nil {
		return writeFlowError(args, stdout, stderr, err, flowsRunUsage)
	}
	if *format != "text" && *format != "json" {
		return writeFlowError(args, stdout, stderr, fmt.Errorf("unsupported format %q", *format), flowsRunUsage)
	}
	if !*dryRun {
		return writeFlowError(args, stdout, stderr,
			fmt.Errorf("%q currently supports only --dry-run; real execution remains stage by stage behind the creative gates (see %q)", "flows run", "zv flows show "+flowName),
			flowsRunUsage)
	}
	// Reject the documentation template token "<demo|stream>" and any other
	// non-runnable flow BEFORE creating the run dir. validateFlowsRunCommand
	// tolerates the template for doc/catalog validation, so the runner must fail
	// closed here rather than exit 0 with an empty {ok:true} report after
	// creating a directory.
	var steps []flowRunStep
	switch flowName {
	case "demo":
		// A kill plan can skip parsing, but capture still needs the source demo.
		// Never report an end-to-end demo flow as successful when its two central
		// media phases are structurally impossible.
		if strings.TrimSpace(*demo) == "" {
			return writeFlowError(args, stdout, stderr,
				fmt.Errorf(`the demo flow requires --demo for capture and render; --killplan only skips parse`), flowsRunUsage)
		}
		if strings.TrimSpace(*killplanPath) == "" {
			if strings.TrimSpace(*steamid) == "" {
				return writeFlowError(args, stdout, stderr,
					fmt.Errorf(`--demo requires --steamid for "demo parse"`), flowsRunUsage)
			}
		}
		steps = demoFlowRunSteps(*runDir, *demo, *steamid, *killplanPath)
	case "stream":
		steps = streamFlowRunSteps(*runDir, *input)
	default:
		return writeFlowError(args, stdout, stderr,
			fmt.Errorf(`unknown flow %q for "flows run"; expected demo or stream`, flowName), flowsRunUsage)
	}

	report := flowRunReport{OK: true, Flow: flowName, RunDir: *runDir, DryRun: true}
	failed := false
	for _, step := range steps {
		if failed {
			report.Phases = append(report.Phases, flowRunPhaseReport{Phase: step.id, Skipped: true, Reason: "not run: an earlier phase failed"})
			continue
		}
		action, err := step.build()
		if err != nil {
			report.OK = false
			failed = true
			report.Phases = append(report.Phases, flowRunPhaseReport{Phase: step.id, Reason: err.Error()})
			continue
		}
		if action.gate || action.skip {
			report.Phases = append(report.Phases, flowRunPhaseReport{Phase: step.id, OK: true, Skipped: true, Reason: action.reason})
			continue
		}
		argv := append([]string(nil), action.argv...)
		if len(argv) > 0 && !containsArg(argv, "--dry-run") {
			argv = append(argv, "--dry-run")
		}
		if action.unvalidated {
			report.OK = false
			report.Phases = append(report.Phases, flowRunPhaseReport{
				Phase:   step.id,
				Argv:    argv,
				Skipped: true,
				Reason:  action.reason,
			})
			continue
		}
		var out, errBuf bytes.Buffer
		code := Run(append([]string{"zv"}, argv...), &out, &errBuf, stdin, runner)
		phase := flowRunPhaseReport{
			Phase:    step.id,
			Argv:     argv,
			DryRun:   true,
			Executed: false,
		}
		if code == exitSuccess {
			phase.OK = true
		} else {
			phase.Reason = flowPhaseFailureReason(errBuf.String(), out.String(), code)
			report.OK = false
			failed = true
		}
		report.Phases = append(report.Phases, phase)
	}

	if *format == "json" {
		if err := writeJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "error: write flow run report: %v\n", err)
			return exitUnexpected
		}
	} else {
		writeFlowRunText(stdout, report)
	}
	if !report.OK {
		return exitUnexpected
	}
	return exitSuccess
}

// demoFlowRunSteps mirrors the demo journey's chain: playability probe, parse
// (skipped when a kill plan is supplied), moments,
// select (every segment in plan order, the documented dry-run default), the
// capture and render dry runs, and the thumbnail gate.
func demoFlowRunSteps(runDir, demo, steamid, killplanFlag string) []flowRunStep {
	killplanPath := killplanFlag
	if strings.TrimSpace(killplanPath) == "" {
		killplanPath = filepath.Join(runDir, "killplan.json")
	}
	playabilityPath := filepath.Join(runDir, "playability.json")
	momentsPath := filepath.Join(runDir, "moments.json")
	selectedPath := filepath.Join(runDir, "selected-plan.json")
	recordingDir := filepath.Join(runDir, "recording")
	recordingResult := filepath.Join(recordingDir, "recording-result.json")
	renderDir := filepath.Join(runDir, "render")
	publishDir := filepath.Join(runDir, "shortslistosparasubir")

	return []flowRunStep{
		{id: "probe", build: func() (flowRunAction, error) {
			if err := validateFlowInputFile("--demo", demo); err != nil {
				return flowRunAction{}, err
			}
			return flowRunAction{
				argv: []string{"demo", "probe", "--demo", demo, "--out", playabilityPath, "--format", "json"},
			}, nil
		}},
		{id: "parse", build: func() (flowRunAction, error) {
			if err := validateFlowInputFile("--demo", demo); err != nil {
				return flowRunAction{}, err
			}
			if strings.TrimSpace(killplanFlag) != "" {
				if err := validateDemoFlowBinding(killplanFlag, demo); err != nil {
					return flowRunAction{}, err
				}
				return flowRunAction{skip: true, reason: "kill plan supplied; demo and plan binding validated without parsing"}, nil
			}
			if strings.TrimSpace(demo) == "" {
				return flowRunAction{}, fmt.Errorf("demo parse requires --demo or --killplan")
			}
			if strings.TrimSpace(steamid) == "" {
				return flowRunAction{}, fmt.Errorf("demo parse requires --steamid")
			}
			return flowRunAction{
				argv: []string{"demo", "parse", "--demo", demo, "--steamid", steamid, "--out", killplanPath},
			}, nil
		}},
		{id: "moments", build: func() (flowRunAction, error) {
			action := flowRunAction{
				argv: []string{"demo", "moments", "--killplan", killplanPath, "--out", momentsPath, "--format", "json"},
			}
			if strings.TrimSpace(killplanFlag) == "" {
				action.unvalidated = true
				action.reason = "unvalidated: requires killplan.json from parse, which a write-free dry-run does not create"
			}
			return action, nil
		}},
		{id: "select", build: func() (flowRunAction, error) {
			if strings.TrimSpace(killplanFlag) == "" {
				return flowRunAction{
					unvalidated: true,
					reason:      "unvalidated: requires killplan.json from parse, which a write-free dry-run does not create",
				}, nil
			}
			ids, err := demoFlowSegmentIDs(killplanPath)
			if err != nil {
				return flowRunAction{}, err
			}
			return flowRunAction{
				argv: []string{"demo", "select", "--killplan", killplanPath, "--segments", strings.Join(ids, ","), "--out", selectedPath, "--format", "json"},
			}, nil
		}},
		{id: "record", build: func() (flowRunAction, error) {
			return flowRunAction{
				argv:        []string{"record", "--killplan", selectedPath, "--demo", demo, "--out", recordingDir, "--dry-run", "--format", "json"},
				unvalidated: true,
				reason:      "unvalidated: requires selected-plan.json from select, which a write-free dry-run does not create",
			}, nil
		}},
		{id: "shorts-render", build: func() (flowRunAction, error) {
			return flowRunAction{
				argv:        []string{"shorts", "render", "--recording-result", recordingResult, "--killplan", selectedPath, "--out", renderDir, "--publish-dir", publishDir, "--dry-run"},
				unvalidated: true,
				reason:      "unvalidated: requires recording-result.json from record, which a write-free dry-run does not create",
			}, nil
		}},
		{id: "thumbnail-selection", build: func() (flowRunAction, error) {
			return flowRunAction{gate: true, reason: "creative gate: choose a cover candidate or delegate automatic selection"}, nil
		}},
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want || strings.HasPrefix(arg, want+"=") {
			return true
		}
	}
	return false
}

// streamFlowRunSteps mirrors the stream journey's chain: a
// read-only plan preflight, and the dependent render phase.
func streamFlowRunSteps(runDir, input string) []flowRunStep {
	editPlan := filepath.Join(runDir, "edit-plan.json")
	renderDir := filepath.Join(runDir, "render")

	return []flowRunStep{
		{id: "plan", build: func() (flowRunAction, error) {
			if strings.TrimSpace(input) == "" {
				return flowRunAction{}, fmt.Errorf("stream plan requires --input")
			}
			if err := validateFlowInputFile("--input", input); err != nil {
				return flowRunAction{}, err
			}
			return flowRunAction{
				argv: []string{"stream", "plan", "--input", input, "--out", editPlan, "--format", "json"},
			}, nil
		}},
		{id: "render", build: func() (flowRunAction, error) {
			return flowRunAction{
				argv:        []string{"stream", "render", "--input", input, "--plan", editPlan, "--out", renderDir, "--dry-run", "--format", "json"},
				unvalidated: true,
				reason:      "unvalidated: requires edit-plan.json from plan, which a write-free dry-run does not create",
			}, nil
		}},
	}
}

func validateFlowInputFile(flagName, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("validate %s %q: %w", flagName, path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("validate %s %q: input is not a regular file", flagName, path)
	}
	return nil
}

func validateDemoFlowBinding(killplanPath, demoPath string) error {
	plan, err := loadDemoKillPlan(killplanPath)
	if err != nil {
		return fmt.Errorf("read kill plan: %w", err)
	}
	if len(plan.Demo.SHA256) != sha256.Size*2 {
		return fmt.Errorf("kill plan demo sha256 must be a 64-character digest")
	}
	// #nosec G304 -- demoPath is the explicit local dry-run input and is opened
	// only to verify the durable plan's SHA-256 binding.
	file, err := os.Open(demoPath)
	if err != nil {
		return fmt.Errorf("open demo for SHA-256 validation: %w", err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("hash demo: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close demo after hashing: %w", closeErr)
	}
	if actual := hex.EncodeToString(hash.Sum(nil)); plan.Demo.SHA256 != actual {
		return fmt.Errorf("kill plan demo sha256 does not match --demo")
	}
	return nil
}

func demoFlowSegmentIDs(path string) ([]string, error) {
	plan, err := loadDemoKillPlan(path)
	if err != nil {
		return nil, fmt.Errorf("read kill plan: %w", err)
	}
	ids := make([]string, 0, len(plan.Segments))
	for _, seg := range plan.Segments {
		if strings.TrimSpace(seg.ID) == "" {
			return nil, fmt.Errorf("kill plan contains a segment without an id")
		}
		ids = append(ids, seg.ID)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("kill plan has no segments to select")
	}
	return ids, nil
}

func flowPhaseFailureReason(stderr, stdout string, code int) string {
	if line := firstNonEmptyLine(stderr); line != "" {
		return line
	}
	// Stage commands report failures as {ok:false, error:...} JSON on stdout;
	// surface the message instead of the raw JSON's first line ("{").
	var envelope struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err == nil && strings.TrimSpace(envelope.Error) != "" {
		return envelope.Error
	}
	if line := firstNonEmptyLine(stdout); line != "" {
		return line
	}
	return fmt.Sprintf("exit %d", code)
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func writeFlowRunText(w io.Writer, report flowRunReport) {
	fmt.Fprintf(w, "flow: %s (run-dir: %s, dry-run)\n", report.Flow, report.RunDir)
	for i, phase := range report.Phases {
		fmt.Fprintf(w, "%d. %-20s %s\n", i+1, phase.Phase, flowPhaseStatus(phase))
	}
	if report.OK {
		fmt.Fprintln(w, "result: ok")
	} else {
		fmt.Fprintln(w, "result: failed")
	}
}

func flowPhaseStatus(phase flowRunPhaseReport) string {
	switch {
	case phase.Skipped:
		if phase.Reason != "" {
			return "skipped (" + phase.Reason + ")"
		}
		return "skipped"
	case !phase.OK:
		if phase.Reason != "" {
			return "failed (" + phase.Reason + ")"
		}
		return "failed"
	case phase.DryRun:
		return "ok (dry-run)"
	default:
		return "ok (executed)"
	}
}
