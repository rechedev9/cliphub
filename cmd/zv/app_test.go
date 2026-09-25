package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeSubcommandCall struct {
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}

type fakeRunner struct {
	name string
	args []string
	err  error
}

func TestMain(m *testing.M) {
	if os.Getenv("ZV_FAKE_SUBCOMMAND") == "1" {
		os.Exit(runFakeSubcommand())
	}
	// Redirect observability output to a temp dir so best-effort obs recording
	// (e.g. a failed `zv short` stage) never writes data/obs into the source tree.
	obsDir, _ := os.MkdirTemp("", "zv-test-obs-")
	if obsDir != "" {
		os.Setenv("ZV_DATA_DIR", obsDir)
	}
	code := m.Run()
	for _, dir := range []string{cachedZVBinaryDir, delegatedBinariesDir, fakeSubcommandMasterDir, obsDir} {
		if err := removeAllTestArtifacts(dir); err != nil {
			fmt.Fprintf(os.Stderr, "test cleanup %s: %v\n", dir, err)
			code = 1
		}
	}
	os.Exit(code)
}

func runFakeSubcommand() int {
	logPath := os.Getenv("ZV_FAKE_SUBCOMMAND_LOG")
	if logPath == "" {
		return 1
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 1
	}
	defer f.Close()
	call := fakeSubcommandCall{
		Executable: filepath.Base(os.Args[0]),
		Args:       append([]string(nil), os.Args[1:]...),
	}
	if err := json.NewEncoder(f).Encode(call); err != nil {
		return 1
	}
	if os.Getenv("ZV_FAKE_SUBCOMMAND_ECHO_STDIO") == "1" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return 1
		}
		fmt.Fprintf(os.Stdout, "fake stdout: %s", string(b))
		fmt.Fprintf(os.Stderr, "fake stderr: %s", filepath.Base(os.Args[0]))
	}
	return 0
}

func (f *fakeRunner) Run(_ context.Context, name string, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
	f.name = name
	f.args = append([]string(nil), args...)
	return f.err
}

func TestRunDemoParseDelegatesToParser(t *testing.T) {
	runner := &fakeRunner{}
	var stdout, stderr strings.Builder

	code := Run([]string{"zv", "demo", "parse", "--demo", "inferno.dem", "--steamid", "123", "--out", "plan.json"}, &stdout, &stderr, nil, runner)

	if got, want := code, exitSuccess; got != want {
		t.Fatalf("code = %d, want %d; stderr=%s", got, want, stderr.String())
	}
	if got, want := runner.name, executableName("zv-parser"); got != want && !strings.HasSuffix(got, want) {
		t.Fatalf("runner.name = %q, want suffix %q", got, want)
	}
	if got, want := strings.Join(runner.args, " "), "parse --demo inferno.dem --steamid 123 --out plan.json"; got != want {
		t.Fatalf("runner.args = %q, want %q", got, want)
	}
}

func TestRunUtilityAuditDelegatesToParser(t *testing.T) {
	runner := &fakeRunner{}
	var stdout, stderr strings.Builder

	code := Run([]string{"zv", "utility", "audit", "--plan", "plan.json", "--lineup-catalog", "data/lineups", "--out", "utility-audit.csv", "--format", "json"}, &stdout, &stderr, nil, runner)

	if got, want := code, exitSuccess; got != want {
		t.Fatalf("code = %d, want %d; stderr=%s", got, want, stderr.String())
	}
	if got, want := runner.name, executableName("zv-parser"); got != want && !strings.HasSuffix(got, want) {
		t.Fatalf("runner.name = %q, want suffix %q", got, want)
	}
	if got, want := strings.Join(runner.args, " "), "utility-audit --plan plan.json --lineup-catalog data/lineups --out utility-audit.csv --format json"; got != want {
		t.Fatalf("runner.args = %q, want %q", got, want)
	}
}

func TestRunShortsRenderDelegatesToEditor(t *testing.T) {
	runner := &fakeRunner{}
	var stdout, stderr strings.Builder

	code := Run([]string{"zv", "shorts", "render", "--recording-result", "recording.json", "--out", "shorts"}, &stdout, &stderr, nil, runner)

	if got, want := code, exitSuccess; got != want {
		t.Fatalf("code = %d, want %d; stderr=%s", got, want, stderr.String())
	}
	if got, want := runner.name, executableName("zv-editor"); got != want && !strings.HasSuffix(got, want) {
		t.Fatalf("runner.name = %q, want suffix %q", got, want)
	}
	if got, want := strings.Join(runner.args, " "), "--recording-result recording.json --out shorts"; got != want {
		t.Fatalf("runner.args = %q, want %q", got, want)
	}
}

func TestRunServeDelegatesToOrchestrator(t *testing.T) {
	runner := &fakeRunner{}
	var stdout, stderr strings.Builder

	code := Run([]string{"zv", "serve"}, &stdout, &stderr, nil, runner)

	if got, want := code, exitSuccess; got != want {
		t.Fatalf("code = %d, want %d; stderr=%s", got, want, stderr.String())
	}
	if got, want := runner.name, executableName("zv-orchestrator"); got != want && !strings.HasSuffix(got, want) {
		t.Fatalf("runner.name = %q, want suffix %q", got, want)
	}
	if got, want := len(runner.args), 0; got != want {
		t.Fatalf("runner.args len = %d, want %d: %#v", got, want, runner.args)
	}
}

func TestRunUnknownCommandReturnsInvalidArgs(t *testing.T) {
	runner := &fakeRunner{}
	var stdout, stderr strings.Builder

	code := Run([]string{"zv", "wat"}, &stdout, &stderr, nil, runner)

	if got, want := code, exitInvalidArgs; got != want {
		t.Fatalf("code = %d, want %d", got, want)
	}
	if !strings.Contains(stderr.String(), `unknown command "wat"`) {
		t.Fatalf("stderr = %q, want unknown command", stderr.String())
	}
}

func TestRunStreamValidationErrorsAreMachineReadable(t *testing.T) {
	runner := &fakeRunner{}
	var stdout, stderr strings.Builder
	code := Run([]string{"zv", "stream", "render", "--format", "json"}, &stdout, &stderr, nil, runner)
	if code != exitInvalidArgs || runner.name != "" || stderr.Len() != 0 {
		t.Fatalf("code = %d, delegated to = %q, stderr = %q", code, runner.name, stderr.String())
	}
	if !json.Valid([]byte(stdout.String())) {
		t.Fatalf("stdout = %q, want validation JSON", stdout.String())
	}
}

func TestRunDelegateReportsRunnerError(t *testing.T) {
	runner := &fakeRunner{err: errors.New("boom")}
	var stdout, stderr strings.Builder

	code := Run([]string{"zv", "parser", "--help"}, &stdout, &stderr, nil, runner)

	if got, want := code, exitUnexpected; got != want {
		t.Fatalf("code = %d, want %d", got, want)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr = %q, want runner error", stderr.String())
	}
}

func TestRunLegacyPassThroughsDelegate(t *testing.T) {
	for _, passThrough := range legacyPassThroughs() {
		t.Run(passThrough.Command, func(t *testing.T) {
			runner := &fakeRunner{}
			var stdout, stderr strings.Builder

			code := Run([]string{"zv", passThrough.Command, "--sentinel"}, &stdout, &stderr, nil, runner)

			if got, want := code, exitSuccess; got != want {
				t.Fatalf("code = %d, want %d; stderr=%s", got, want, stderr.String())
			}
			if got, want := runner.name, executableName(passThrough.Binary); got != want && !strings.HasSuffix(got, want) {
				t.Fatalf("runner.name = %q, want suffix %q", got, want)
			}
			if got, want := strings.Join(runner.args, " "), "--sentinel"; got != want {
				t.Fatalf("runner.args = %q, want %q", got, want)
			}
		})
	}
}
