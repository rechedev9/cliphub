package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runFlowsRunInProcess dispatches `zv flows run ...` with a no-op delegated
// runner. Local read-only checks still execute; no capture, render, or artifact
// write can escape the process.
func runFlowsRunInProcess(t *testing.T, args ...string) (flowRunReport, int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"zv", "flows", "run"}, args...), &stdout, &stderr, nil, &fakeRunner{})
	var report flowRunReport
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			// Non-JSON stdout (text mode or an argument error) is returned raw via
			// the string result; leave report zero.
			return flowRunReport{}, code, stdout.String() + stderr.String()
		}
	}
	return report, code, stderr.String()
}

func phaseByName(report flowRunReport, name string) (flowRunPhaseReport, bool) {
	for _, phase := range report.Phases {
		if phase.Phase == name {
			return phase, true
		}
	}
	return flowRunPhaseReport{}, false
}

func TestFlowsRunRejectsWithoutDryRun(t *testing.T) {
	ws := t.TempDir()
	plan := writeStageContractPlan(t, ws)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"zv", "flows", "run", "demo", "--killplan", plan, "--run-dir", filepath.Join(ws, "run")}, &stdout, &stderr, nil, &fakeRunner{})
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d\nstderr: %s", code, exitInvalidArgs, stderr.String())
	}
	if !strings.Contains(stderr.String(), "supports only --dry-run") {
		t.Fatalf("stderr = %q, want the stage-by-stage explanation", stderr.String())
	}
}

// TestWorkflowsValidateFlowsRunMatchesRunnerPrerequisites keeps the advertised
// zero-execution preflight aligned with the runner's fail-fast requirements.
func TestWorkflowsValidateFlowsRunMatchesRunnerPrerequisites(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "dry run is required",
			args: []string{"demo", "--killplan", "plan.json", "--run-dir", "run"},
			want: "supports only --dry-run",
		},
		{
			name: "demo needs an input source",
			args: []string{"demo", "--run-dir", "run", "--dry-run"},
			want: "requires --demo for capture and render",
		},
		{
			name: "demo parsing needs a target player",
			args: []string{"demo", "--demo", "match.dem", "--run-dir", "run", "--dry-run"},
			want: "--demo requires --steamid",
		},
		{
			name: "stream needs a source video",
			args: []string{"stream", "--run-dir", "run", "--dry-run"},
			want: "stream flow requires --input",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var validatedOut, validatedErr bytes.Buffer
			validateArgs := append([]string{"zv", "workflows", "validate", "flows-run", "--format", "json", "--"}, tc.args...)
			code := Run(validateArgs, &validatedOut, &validatedErr, nil, &fakeRunner{})
			if got, want := code, exitInvalidArgs; got != want {
				t.Fatalf("validator code = %d, want %d; stderr=%s", got, want, validatedErr.String())
			}
			var result workflowValidationResult
			if err := json.Unmarshal(validatedOut.Bytes(), &result); err != nil {
				t.Fatalf("unmarshal validator output: %v\n%s", err, validatedOut.String())
			}
			if result.OK || !strings.Contains(result.Error, tc.want) {
				t.Fatalf("validator result = %#v, want error containing %q", result, tc.want)
			}

			var runOut, runErr bytes.Buffer
			code = Run(append([]string{"zv", "flows", "run"}, tc.args...), &runOut, &runErr, nil, &fakeRunner{})
			if got, want := code, exitInvalidArgs; got != want {
				t.Fatalf("runner code = %d, want %d; stderr=%s", got, want, runErr.String())
			}
			if output := runOut.String() + runErr.String(); !strings.Contains(output, tc.want) {
				t.Fatalf("runner output = %q, want error containing %q", output, tc.want)
			}
		})
	}
}

func TestFlowsRunRequiresFlowNameFirst(t *testing.T) {
	ws := t.TempDir()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing flow name",
			args: []string{"--run-dir", filepath.Join(ws, "a"), "--dry-run"},
			want: "missing flow name",
		},
		{
			// The flow name must be the first token; a leading flag must not be
			// mistaken for it, and its value must not be stolen as the flow.
			name: "flow name after flags is rejected",
			args: []string{"--run-dir", filepath.Join(ws, "b"), "demo", "--dry-run"},
			want: "missing flow name",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(append([]string{"zv", "flows", "run"}, tc.args...), &stdout, &stderr, nil, &fakeRunner{})
			if code != exitInvalidArgs {
				t.Fatalf("code = %d, want %d\nstderr: %s", code, exitInvalidArgs, stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tc.want)
			}
		})
	}
}

// TestFlowsRunRejectsTemplateFlowWithoutCreatingRunDir pins that the literal
// documentation token "<demo|stream>" is rejected at runtime with a non-zero
// exit BEFORE the run dir is created, rather than exiting 0 with an empty report.
func TestFlowsRunRejectsTemplateFlowWithoutCreatingRunDir(t *testing.T) {
	ws := t.TempDir()
	runDir := filepath.Join(ws, "run")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"zv", "flows", "run", "<demo|stream>", "--run-dir", runDir, "--dry-run"}, &stdout, &stderr, nil, &fakeRunner{})
	if code == exitSuccess {
		t.Fatalf("code = %d, want a non-zero exit for the template token\nstdout: %s", code, stdout.String())
	}
	if !strings.Contains(stderr.String()+stdout.String(), "unknown flow") {
		t.Fatalf("output = %q/%q, want unknown flow error", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(runDir); err == nil {
		t.Fatalf("run dir %s was created for a rejected flow, want no directory", runDir)
	}
}

// TestFlowRunnerStepsCoverRegistryPhases links the runner (flow_run.go) to the
// descriptive registry (productionFlows): every runner step must drive a real
// registry phase, and every registry phase must be either driven by a runner
// step or explicitly exempt (doctor, players, *-preflight, gates, review...).
// A new registry phase added without runner coverage or an exemption fails
// here. Runner step ids intentionally differ from a few phase ids
// (record->capture, shorts-render->edit), so the link is a documented
// correspondence map rather than raw id equality.
func TestFlowRunnerStepsCoverRegistryPhases(t *testing.T) {
	setOf := func(ids ...string) map[string]bool {
		m := make(map[string]bool, len(ids))
		for _, id := range ids {
			m[id] = true
		}
		return m
	}
	cases := []struct {
		flow        string
		steps       []flowRunStep
		runnerPhase map[string]string
		exempt      map[string]bool
	}{
		{
			flow:  "demo",
			steps: demoFlowRunSteps("run", "match.dem", "76561198000000000", ""),
			runnerPhase: map[string]string{
				"probe":   "probe",
				"parse":   "parse",
				"moments": "moments",

				"select":              "select",
				"record":              "capture",
				"shorts-render":       "edit",
				"thumbnail-selection": "thumbnail-selection",
			},
			exempt: setOf("doctor", "players", "parse-preflight", "moments-preflight", "select-preflight", "capture-preflight", "edit-preflight", "review"),
		},
		{
			flow:  "stream",
			steps: streamFlowRunSteps("run", "stream.mp4"),
			runnerPhase: map[string]string{

				"plan":   "plan",
				"render": "render",
			},
			exempt: setOf("doctor", "layouts", "plan-preflight", "plan-review", "render-preflight", "review"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.flow, func(t *testing.T) {
			flow, ok := findProductionFlow(tc.flow)
			if !ok {
				t.Fatalf("production flow %q not found", tc.flow)
			}
			phaseIDs := make(map[string]bool, len(flow.Phases))
			for _, phase := range flow.Phases {
				phaseIDs[phase.ID] = true
			}
			covered := make(map[string]bool)
			for _, step := range tc.steps {
				target, ok := tc.runnerPhase[step.id]
				if !ok {
					t.Fatalf("runner step %q has no documented registry phase; map it or exempt it", step.id)
				}
				if !phaseIDs[target] {
					t.Fatalf("runner step %q maps to phase %q, which is not in flow %q", step.id, target, tc.flow)
				}
				covered[target] = true
			}
			for _, phase := range flow.Phases {
				if covered[phase.ID] || tc.exempt[phase.ID] {
					continue
				}
				if phase.When == "full-demo-pov-chill-v1" {
					// This conditional branch is driven by the job-based CLI,
					// whose admission/dry-run contracts are exercised in
					// full_demo_commands_test.go. The legacy flows run command
					// only preflights its selected-kill chain and cannot approve
					// a Full Demo document implicitly.
					fields := strings.Fields(phase.Command)
					workflow, found := workflowForRunArgsPrefix(fields[1:])
					if !found || !strings.HasPrefix(workflow.Name, "full-demo-") {
						t.Fatalf("conditional Full Demo phase has no CLI owner: %s", phase.ID)
					}
					continue
				}
				t.Fatalf("flow %q phase %q has no runner step and is not exempt; add runner coverage or exempt it", tc.flow, phase.ID)
			}
		})
	}
}

func TestFlowsRunDemoFailsFastOnMissingParseInputs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "demo without steamid",
			args: []string{"--demo", "match.dem"},
			want: "--demo requires --steamid",
		},
		{
			name: "neither demo nor killplan",
			args: []string{},
			want: "requires --demo for capture and render",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runDir := filepath.Join(t.TempDir(), "run")
			args := append([]string{"zv", "flows", "run", "demo", "--run-dir", runDir, "--dry-run"}, tc.args...)
			var stdout, stderr bytes.Buffer
			code := Run(args, &stdout, &stderr, nil, &fakeRunner{})
			if got, want := code, exitInvalidArgs; got != want {
				t.Fatalf("code = %d, want %d; stderr=%s", got, want, stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tc.want)
			}
			if _, err := os.Stat(runDir); !os.IsNotExist(err) {
				t.Fatalf("run dir %s exists after fail-fast rejection; stat err = %v", runDir, err)
			}
		})
	}
}

func TestFlowPhaseFailureReason(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		stdout string
		code   int
		want   string
	}{
		{
			name:   "json error field wins over raw json lines",
			stdout: "{\n  \"ok\": false,\n  \"error\": \"clip clip-001 is outside the source duration\"\n}",
			code:   1,
			want:   "clip clip-001 is outside the source duration",
		},
		{
			name:   "stderr line wins when present",
			stderr: "error: plan file not found: plan.json",
			stdout: `{"ok":false,"error":"unused"}`,
			code:   3,
			want:   "error: plan file not found: plan.json",
		},
		{
			name:   "plain stdout falls back to first line",
			stdout: "something went wrong\nmore detail",
			code:   1,
			want:   "something went wrong",
		},
		{
			name: "empty output falls back to exit code",
			code: 4,
			want: "exit 4",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := flowPhaseFailureReason(tc.stderr, tc.stdout, tc.code)
			if got != tc.want {
				t.Fatalf("flowPhaseFailureReason() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFlowsRunUnknownFlowIsRejected(t *testing.T) {
	ws := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"zv", "flows", "run", "movie", "--run-dir", ws, "--dry-run"}, &stdout, &stderr, nil, &fakeRunner{})
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(stderr.String(), `unknown flow "movie"`) {
		t.Fatalf("stderr = %q, want unknown flow error", stderr.String())
	}
}

func TestFlowsRunDemoRejectsKillPlanWithoutCaptureSource(t *testing.T) {
	ws := t.TempDir()
	plan := writeStageContractPlan(t, ws)
	runDir := filepath.Join(ws, "run")

	report, code, _ := runFlowsRunInProcess(t, "demo", "--killplan", plan, "--run-dir", runDir, "--dry-run", "--format", "json")
	if code != exitInvalidArgs || report.OK {
		t.Fatalf("code = %d, report = %#v, want fail-fast invalid args", code, report)
	}
	if _, err := os.Stat(runDir); !os.IsNotExist(err) {
		t.Fatalf("run dir exists after rejected incomplete flow: %v", err)
	}
}

func TestFlowsRunDryRunStopsAtInvalidSuppliedKillPlan(t *testing.T) {
	ws := t.TempDir()
	runDir := filepath.Join(ws, "run")
	missing := filepath.Join(ws, "does-not-exist.json")
	demo := filepath.Join(ws, "demo.dem")
	if err := os.WriteFile(demo, []byte("dummy demo"), 0o600); err != nil {
		t.Fatal(err)
	}

	report, code, _ := runFlowsRunInProcess(t, "demo", "--demo", demo, "--killplan", missing, "--run-dir", runDir, "--dry-run", "--format", "json")
	if code == exitSuccess {
		t.Fatalf("code = %d, want failure", code)
	}
	if report.OK {
		t.Fatalf("report.OK = true, want false: %#v", report)
	}

	probe, _ := phaseByName(report, "probe")
	if !probe.OK || !probe.DryRun || probe.Executed {
		t.Fatalf("probe = %#v, want successful read-only preflight before the supplied-plan check", probe)
	}
	parse, _ := phaseByName(report, "parse")
	if parse.OK || parse.Skipped || !strings.Contains(parse.Reason, "read kill plan") {
		t.Fatalf("parse = %#v, want supplied-plan preflight failure", parse)
	}
	// Every phase after the failure is reported as not run.
	for _, id := range []string{"moments", "select", "record", "shorts-render", "thumbnail-selection"} {
		phase, ok := phaseByName(report, id)
		if !ok {
			t.Fatalf("phase %s missing from report", id)
		}
		if phase.OK || !phase.Skipped || !strings.Contains(phase.Reason, "not run") {
			t.Fatalf("phase %s = %#v, want not-run", id, phase)
		}
	}
	if _, err := os.Stat(runDir); !os.IsNotExist(err) {
		t.Fatalf("pure dry-run created run dir: %v", err)
	}
}

func TestFlowsRunDryRunRejectsMissingSourceFiles(t *testing.T) {
	ws := t.TempDir()
	tests := []struct {
		name      string
		args      []string
		phase     string
		wantError string
	}{
		{
			name:      "demo",
			args:      []string{"demo", "--demo", filepath.Join(ws, "missing.dem"), "--steamid", "76561198377256168"},
			phase:     "probe",
			wantError: "validate --demo",
		},
		{
			name:      "stream",
			args:      []string{"stream", "--input", filepath.Join(ws, "missing.mp4")},
			phase:     "plan",
			wantError: "validate --input",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runDir := filepath.Join(ws, tt.name+"-run")
			args := append(append([]string(nil), tt.args...), "--run-dir", runDir, "--dry-run", "--format", "json")
			report, code, _ := runFlowsRunInProcess(t, args...)
			if code == exitSuccess || report.OK {
				t.Fatalf("code = %d, report = %#v, want failed source preflight", code, report)
			}
			phase, ok := phaseByName(report, tt.phase)
			if !ok || phase.OK || phase.Skipped || !strings.Contains(phase.Reason, tt.wantError) {
				t.Fatalf("%s phase = %#v, want error containing %q", tt.phase, phase, tt.wantError)
			}
			if _, err := os.Stat(runDir); !os.IsNotExist(err) {
				t.Fatalf("pure dry-run created run dir: %v", err)
			}
		})
	}
}

func TestFlowsRunDemoDryRunRejectsKillPlanDemoHashMismatch(t *testing.T) {
	ws := t.TempDir()
	plan := writeStageContractPlan(t, ws)
	demo := filepath.Join(ws, "different.dem")
	if err := os.WriteFile(demo, []byte("different demo"), 0o600); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(ws, "run")

	report, code, _ := runFlowsRunInProcess(t, "demo", "--demo", demo, "--killplan", plan, "--run-dir", runDir, "--dry-run", "--format", "json")
	if code == exitSuccess || report.OK {
		t.Fatalf("code = %d, report = %#v, want failed SHA-256 preflight", code, report)
	}
	parse, ok := phaseByName(report, "parse")
	if !ok || parse.OK || parse.Skipped || !strings.Contains(parse.Reason, "sha256 does not match --demo") {
		t.Fatalf("parse = %#v, want demo binding mismatch", parse)
	}
	if _, err := os.Stat(runDir); !os.IsNotExist(err) {
		t.Fatalf("pure dry-run created run dir: %v", err)
	}
}

func TestFlowsRunDemoDryRunMarksGeneratedKillPlanDependenciesUnvalidated(t *testing.T) {
	ws := t.TempDir()
	demo := filepath.Join(ws, "demo.dem")
	if err := os.WriteFile(demo, []byte("dummy demo"), 0o600); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(ws, "run")

	report, code, _ := runFlowsRunInProcess(t, "demo", "--demo", demo, "--steamid", "76561198377256168", "--run-dir", runDir, "--dry-run", "--format", "json")
	if code != exitUnexpected || report.OK {
		t.Fatalf("code = %d, report = %#v, want incomplete dry-run", code, report)
	}
	probe, _ := phaseByName(report, "probe")
	if !probe.OK || !probe.DryRun || probe.Executed {
		t.Fatalf("probe = %#v, want successful read-only preflight", probe)
	}
	parse, _ := phaseByName(report, "parse")
	if !parse.OK || !parse.DryRun || parse.Executed {
		t.Fatalf("parse = %#v, want successful read-only preflight", parse)
	}
	for _, id := range []string{"moments", "select"} {
		phase, ok := phaseByName(report, id)
		if !ok || phase.OK || !phase.Skipped || phase.DryRun || phase.Executed ||
			!strings.Contains(phase.Reason, "killplan.json") {
			t.Fatalf("%s = %#v, want unvalidated generated-plan dependency", id, phase)
		}
	}
	if _, err := os.Stat(runDir); !os.IsNotExist(err) {
		t.Fatalf("pure dry-run created run dir: %v", err)
	}
}

func TestFlowsRunDemoDryRunReportsUnmaterializedCaptureDependencies(t *testing.T) {
	t.Parallel()
	exe := buildDelegatedBinaries(t)

	ws := t.TempDir()
	plan := writeStageContractPlan(t, ws)
	demo := filepath.Join(ws, "demo.dem")
	if err := os.WriteFile(demo, []byte("dummy demo"), 0o600); err != nil {
		t.Fatalf("write demo fixture: %v", err)
	}
	runDir := filepath.Join(ws, "run")

	cmd := exec.Command(exe, "flows", "run", "demo",
		"--demo", demo, "--killplan", plan, "--run-dir", runDir, "--dry-run", "--format", "json")
	cmd.Dir = ws
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("flows run demo succeeded, want incomplete dry-run\nstdout:\n%s", stdout.String())
	} else if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != exitUnexpected {
		t.Fatalf("flows run demo: %v, want exit %d\nstdout:\n%s\nstderr:\n%s", err, exitUnexpected, stdout.String(), stderr.String())
	}

	var report flowRunReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout.String())
	}
	if report.OK {
		t.Fatalf("report.OK = true, want incomplete dependency report: %s", stdout.String())
	}

	probe, _ := phaseByName(report, "probe")
	if !probe.OK || !probe.DryRun || probe.Executed {
		t.Fatalf("probe phase = %#v, want a successful read-only preflight", probe)
	}
	for _, id := range []string{"moments", "select"} {
		phase, _ := phaseByName(report, id)
		if !phase.OK || !phase.DryRun || phase.Executed {
			t.Fatalf("%s phase = %#v, want a successful read-only preflight", id, phase)
		}
	}
	record, _ := phaseByName(report, "record")
	if record.OK || !record.Skipped || record.DryRun || record.Executed ||
		!strings.Contains(record.Reason, "selected-plan.json") {
		t.Fatalf("record phase = %#v, want unvalidated selected-plan dependency", record)
	}
	render, _ := phaseByName(report, "shorts-render")
	if render.OK || !render.Skipped || render.DryRun || render.Executed ||
		!strings.Contains(render.Reason, "recording-result.json") {
		t.Fatalf("shorts-render phase = %#v, want unvalidated recording-result dependency", render)
	}

	if _, err := os.Stat(runDir); !os.IsNotExist(err) {
		t.Fatalf("pure dry-run created run dir: %v", err)
	}
	for _, phase := range report.Phases {
		if len(phase.Outputs) != 0 {
			t.Fatalf("phase %s outputs = %#v, pure dry-run must not claim written artifacts", phase.Phase, phase.Outputs)
		}
	}

	// The planned chain remains auditable even though it is not materialized.
	selectedPlan := filepath.Join(runDir, "selected-plan.json")
	if got, ok := flagValue(record.Argv, "--killplan"); !ok || got != selectedPlan {
		t.Fatalf("record --killplan = %q (ok=%v), want the selected plan %q", got, ok, selectedPlan)
	}
	recordingResult := filepath.Join(runDir, "recording", "recording-result.json")
	if got, ok := flagValue(render.Argv, "--recording-result"); !ok || got != recordingResult {
		t.Fatalf("shorts-render --recording-result = %q (ok=%v), want %q", got, ok, recordingResult)
	}
}
