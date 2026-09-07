package workers

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// Exercise the real executable's flag defaults, which editor.Config literals
// and fake runners bypass. Synthetic bytes are used only for a dry-run manifest;
// this test does not execute FFmpeg or certify capture/delivery media.
func TestFullDemoWorkerArgumentsThroughEditorCLI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "zv-editor.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/zv-editor")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build editor CLI: %v\n%s", err, output)
	}
	result, dir, recordingPath := fullDemoPublicationFixture(t, "synthetic dry-run input")
	if err := writeJSONFile(recordingPath, result); err != nil {
		t.Fatal(err)
	}
	doc := *result.Plan.FullDemo
	executionPath := filepath.Join(dir, "full-demo-execution.json")
	execution := editor.FullDemoExecution{SchemaVersion: "1.0", Approved: recapplan.Snapshot{
		Document: doc, Approval: recapplan.Approval{PlanHash: doc.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()},
	}}
	if err := writeJSONFile(executionPath, execution); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		args      []string
		wantError bool
	}{
		{"worker arguments", fullDemoExecutionArgs(executionPath), false},
		{"legacy CLI default", []string{"--full-demo-execution", executionPath}, true},
		{"explicit conflicting trim", append(fullDemoExecutionArgs(executionPath), "--tail-trim=1.5"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "render")
			args := []string{"--recording-result", recordingPath, "--out", out,
				"--preset", editor.PresetGameplayPOV60, "--output-format", "landscape-16x9",
				"--compile-segments", "--hook=false", "--kill-counter=false", "--covers=false", "--dry-run"}
			args = append(args, tc.args...)
			output, err := exec.CommandContext(ctx, binary, args...).CombinedOutput()
			if tc.wantError {
				if err == nil || !strings.Contains(string(output), "full demo execution cannot be overridden by legacy editorial flags") {
					t.Fatalf("expected strict legacy flag rejection: %v\n%s", err, output)
				}
				return
			}
			if err != nil {
				t.Fatalf("worker arguments rejected by editor CLI: %v\n%s", err, output)
			}
			var rendered editor.Result
			if err := readJSONFile(filepath.Join(out, "shorts-result.json"), &rendered); err != nil {
				t.Fatal(err)
			}
			if !rendered.DryRun || rendered.Executed || len(rendered.Shorts) != 1 || rendered.Shorts[0].FullDemo == nil {
				t.Fatalf("expected one Full Demo dry-run compilation: %#v", rendered)
			}
		})
	}
}
