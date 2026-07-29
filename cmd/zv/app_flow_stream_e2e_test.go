package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// streamEnvelope is the {ok, dry_run, executed} success envelope the stream
// stage commands emit; the streamcli result types live in another package, so
// the binary-level suite decodes just the shared fields.
type streamEnvelope struct {
	OK       bool `json:"ok"`
	DryRun   bool `json:"dry_run"`
	Executed bool `json:"executed"`
}

// requireStreamMediaTools skips a stream binary-level e2e unless ffmpeg and
// ffprobe are both resolvable, since the real zv-stream binary probes and
// renders media. It returns the ffmpeg path used to synthesize the source.
func requireStreamMediaTools(t *testing.T) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not found; skipping stream binary-level e2e")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not found; skipping stream binary-level e2e")
	}
	return ffmpeg
}

// TestStreamJourneyBinaryChainsPlanAndRender drives the media-dependent hops of
// the stream journey through the REAL zv-stream binary: it probes a synthesized
// source to persist an edit plan, then renders that exact plan in --dry-run.
// Each hop asserts the envelope and that the plan step's --out is the literal
// --plan the render consumes.
func TestStreamJourneyBinaryChainsPlanAndRender(t *testing.T) {
	ffmpeg := requireStreamMediaTools(t)
	exe := buildDelegatedBinaries(t)

	ws := t.TempDir()
	source := filepath.Join(ws, "source.mp4")
	generateSyntheticSource(t, ffmpeg, source)

	editPlan := filepath.Join(ws, "edit-plan.json")
	stdout, _ := runZVBinarySplit(t, exe, ws, "stream", "plan", "--input", source, "--out", editPlan, "--format", "json")
	var plan streamEnvelope
	decodeJSON(t, "stream plan", stdout, &plan)
	if !plan.OK || plan.DryRun || !plan.Executed {
		t.Fatalf("stream plan envelope = %#v, want executed", plan)
	}
	assertFileExists(t, editPlan)

	renderDir := filepath.Join(ws, "render")
	stdout, _ = runZVBinarySplit(t, exe, ws, "stream", "render", "--input", source, "--plan", editPlan, "--out", renderDir, "--dry-run", "--format", "json")
	var render streamEnvelope
	decodeJSON(t, "stream render", stdout, &render)
	if !render.OK || !render.DryRun || render.Executed {
		t.Fatalf("stream render envelope = %#v, want a successful dry run", render)
	}
}

// TestFlowsRunStreamDryRunChainsPlanAndRender exercises `zv flows run stream
// --dry-run` end to end and proves that both phases are planned without writes.
func TestFlowsRunStreamDryRunChainsPlanAndRender(t *testing.T) {
	ffmpeg := requireStreamMediaTools(t)
	exe := buildDelegatedBinaries(t)

	ws := t.TempDir()
	source := filepath.Join(ws, "source.mp4")
	generateSyntheticSource(t, ffmpeg, source)
	runDir := filepath.Join(ws, "run")

	cmd := exec.Command(exe, "flows", "run", "stream", "--input", source, "--run-dir", runDir, "--dry-run", "--format", "json")
	cmd.Dir = ws
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("flows run stream succeeded, want incomplete dry-run\nstdout:\n%s", stdout.String())
	} else if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != exitUnexpected {
		t.Fatalf("flows run stream: %v, want exit %d\nstdout:\n%s\nstderr:\n%s", err, exitUnexpected, stdout.String(), stderr.String())
	}
	var report flowRunReport
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		t.Fatalf("decode flow run report: %v\n%s", err, stdout.String())
	}
	if report.OK || report.Flow != "stream" {
		t.Fatalf("stream flow report = %#v, want incomplete dependency report", report)
	}

	planPhase, ok := phaseByName(report, "plan")
	if !ok || !planPhase.OK || !planPhase.DryRun || planPhase.Executed {
		t.Fatalf("plan phase = %#v, want a successful declarative dry run", planPhase)
	}
	if _, err := os.Stat(runDir); !os.IsNotExist(err) {
		t.Fatalf("pure dry-run created run dir: %v", err)
	}

	renderPhase, ok := phaseByName(report, "render")
	if !ok || renderPhase.OK || !renderPhase.Skipped || renderPhase.DryRun || renderPhase.Executed ||
		!strings.Contains(renderPhase.Reason, "edit-plan.json") {
		t.Fatalf("render phase = %#v, want unvalidated edit-plan dependency", renderPhase)
	}
}
