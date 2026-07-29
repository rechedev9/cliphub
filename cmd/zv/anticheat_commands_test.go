package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/fragforge/internal/anticheat"
)

func runAnticheat(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"zv", "demo", "anticheat"}, args...), &stdout, &stderr, nil, &fakeRunner{})
	return code, stdout.String(), stderr.String()
}

// anticheatError runs the command in JSON mode and returns its exit code plus
// the machine-readable reason, which JSON mode reports on stdout rather than
// as human text on stderr.
func anticheatError(t *testing.T, args ...string) (int, string) {
	t.Helper()
	code, stdout, stderr := runAnticheat(t, append(args, "--format", "json")...)
	if stderr != "" {
		t.Fatalf("stderr = %q, want JSON mode to keep the reason on stdout", stderr)
	}
	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode result %q: %v", stdout, err)
	}
	if result.OK {
		t.Fatalf("result reported ok = true, want a failure: %s", stdout)
	}
	return code, result.Error
}

func TestRunDemoAnticheatRequiresADemo(t *testing.T) {
	code, reason := anticheatError(t)
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(reason, `missing required flag --demo for "demo anticheat"`) {
		t.Fatalf("reason = %q, want the missing-flag reason", reason)
	}
}

func TestRunDemoAnticheatRejectsAnUnsupportedFormat(t *testing.T) {
	code, _, stderr := runAnticheat(t, "--demo", "match.dem", "--format", "yaml")
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(stderr, "unsupported format") {
		t.Fatalf("stderr = %q, want the unsupported-format reason", stderr)
	}
}

func TestRunDemoAnticheatReportsHumanErrorsOnStderr(t *testing.T) {
	code, stdout, stderr := runAnticheat(t)
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want text mode to keep errors off stdout", stdout)
	}
	if !strings.Contains(stderr, `missing required flag --demo for "demo anticheat"`) {
		t.Fatalf("stderr = %q, want the canonical missing-flag reason", stderr)
	}
}

func TestRunDemoAnticheatRejectsWritingOverItsInput(t *testing.T) {
	demo := filepath.Join(t.TempDir(), "match.dem")
	if err := os.WriteFile(demo, []byte("PBDEMS2\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _ := anticheatError(t, "--demo", demo, "--out", demo)
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
}

func TestRunDemoAnticheatRejectsWritingOverItsBaseline(t *testing.T) {
	root := t.TempDir()
	demo := filepath.Join(root, "match.dem")
	baseline := filepath.Join(root, "baseline.json")
	if err := os.WriteFile(demo, []byte("PBDEMS2\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONArtifact(baseline, anticheat.DefaultBaseline()); err != nil {
		t.Fatal(err)
	}
	code, reason := anticheatError(t, "--demo", demo, "--baseline", baseline, "--out", baseline)
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(reason, "--baseline") {
		t.Fatalf("reason = %q, want baseline alias rejection", reason)
	}
}

func TestRunDemoAnticheatReportsAnUnreadableDemo(t *testing.T) {
	code, reason := anticheatError(t, "--demo", filepath.Join(t.TempDir(), "missing.dem"))
	if code != exitUnexpected {
		t.Fatalf("code = %d, want %d", code, exitUnexpected)
	}
	if !strings.Contains(reason, "open demo") {
		t.Fatalf("reason = %q, want the open failure", reason)
	}
}

func TestRunDemoAnticheatShowsUsageOnHelp(t *testing.T) {
	code, stdout, _ := runAnticheat(t, "--help")
	if code != exitSuccess {
		t.Fatalf("code = %d, want %d", code, exitSuccess)
	}
	for _, want := range []string{"--dossier", "calibrate", "not a verdict of guilt"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("usage missing %q:\n%s", want, stdout)
		}
	}
}

func TestRunDemoAnticheatCalibrateRequiresItsFlags(t *testing.T) {
	code, reason := anticheatError(t, "calibrate")
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(reason, `missing required flags --demos, --id for "demo anticheat calibrate"`) {
		t.Fatalf("reason = %q, want the missing-flag reason", reason)
	}
}

func TestRunDemoAnticheatCalibrateRequiresAnOutputUnlessDryRun(t *testing.T) {
	code, reason := anticheatError(t, "calibrate", "--demos", t.TempDir(), "--id", "x")
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(reason, "--out is required") {
		t.Fatalf("reason = %q, want the missing-output reason", reason)
	}
}

func TestRunDemoAnticheatCalibrateRejectsAnEmptyCorpus(t *testing.T) {
	code, reason := anticheatError(t, "calibrate", "--demos", t.TempDir(), "--id", "x", "--dry-run")
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(reason, "no .dem files") {
		t.Fatalf("reason = %q, want the empty-corpus reason", reason)
	}
}

func TestRunDemoAnticheatCalibrateRejectsWritingOverCorpusDemo(t *testing.T) {
	corpus := t.TempDir()
	demo := filepath.Join(corpus, "match.dem")
	if err := os.WriteFile(demo, []byte("PBDEMS2\x00"), 0o600); err != nil {
		t.Fatal(err)
	}

	code, reason := anticheatError(t, "calibrate", "--demos", corpus, "--id", "pro-2026", "--out", demo)
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(reason, "must not overwrite --demos") {
		t.Fatalf("reason = %q, want output alias rejection", reason)
	}
}

// A baseline written by calibrate must be loadable by a later analysis run;
// this is the contract that makes recalibration a supported workflow rather
// than a one-way export.
func TestAnticheatBaselineArtifactRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := writeJSONArtifact(path, anticheat.DefaultBaseline()); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadAnticheatBaseline(path)
	if err != nil {
		t.Fatalf("loadAnticheatBaseline() = %v", err)
	}
	if loaded.ID != anticheat.DefaultBaseline().ID {
		t.Fatalf("id = %q, want %q", loaded.ID, anticheat.DefaultBaseline().ID)
	}
}

func TestRunDemoAnticheatRejectsABrokenBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, []byte(`{"id":"x","metrics":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, reason := anticheatError(t, "--demo", "match.dem", "--baseline", path)
	if code != exitUnexpected {
		t.Fatalf("code = %d, want %d", code, exitUnexpected)
	}
	if !strings.Contains(reason, "missing metric") {
		t.Fatalf("reason = %q, want the incomplete-baseline reason", reason)
	}
}

// The shipped baseline is data, so a bad edit to it must fail loudly in CI
// rather than silently skewing every score in the product.
func TestShippedBaselineIsValidAndNamesItsProvenance(t *testing.T) {
	b := anticheat.DefaultBaseline()
	if err := b.Validate(); err != nil {
		t.Fatalf("shipped baseline is invalid: %v", err)
	}
	if b.Source == "" || b.Description == "" {
		t.Fatalf("shipped baseline must name where it came from, got %+v", b)
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(b); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSkillCommandChecksDemoAnticheatFlags(t *testing.T) {
	cases := []struct {
		name    string
		command []string
		want    string
	}{
		{
			name:    "missing demo",
			command: []string{"demo", "anticheat", "--format", "json"},
			want:    `missing required flag --demo for "demo anticheat"`,
		},
		{
			name:    "unknown flag",
			command: []string{"demo", "anticheat", "--demo", "match.dem", "--bogus", "x"},
			want:    `unknown flag --bogus for "demo anticheat"`,
		},
		{
			name:    "duplicate demo",
			command: []string{"demo", "anticheat", "--demo", "first.dem", "--demo", "second.dem"},
			want:    `duplicate flag --demo for "demo anticheat"`,
		},
		{
			name:    "valid screening",
			command: []string{"demo", "anticheat", "--demo", "match.dem", "--dry-run", "--format", "json"},
			want:    "",
		},
		{
			name:    "calibrate missing id",
			command: []string{"demo", "anticheat", "calibrate", "--demos", "pro"},
			want:    `missing required flag --id for "demo anticheat calibrate"`,
		},
		{
			name:    "valid calibrate",
			command: []string{"demo", "anticheat", "calibrate", "--demos", "pro", "--id", "pro-2026", "--out", "b.json"},
			want:    "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateSkillCommand(tc.command); got != tc.want {
				t.Fatalf("validateSkillCommand(%q) = %q, want %q", tc.command, got, tc.want)
			}
		})
	}
}
