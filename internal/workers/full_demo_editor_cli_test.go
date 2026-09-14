package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// Exercise the real executable's flag defaults, which editor.Config literals
// and fake runners bypass. Synthetic bytes are used only for a dry-run manifest;
// this test does not execute FFmpeg or certify capture/delivery media.
func TestFullDemoWorkerArgumentsThroughEditorCLI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if renderer := localOverlayRendererPath(); renderer != "" {
		t.Setenv("ZV_OVERLAY_RENDERER_PATH", renderer)
	} else {
		t.Skip("Studio Chromium renderer is required to materialize the mandatory Full Demo overlays")
	}
	binary := filepath.Join(t.TempDir(), "zv-editor.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/zv-editor")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build editor CLI: %v\n%s", err, output)
	}
	result, dir, recordingPath := currentFullDemoPublicationFixture(t, "synthetic dry-run input")
	if err := writeJSONFile(recordingPath, result); err != nil {
		t.Fatal(err)
	}
	doc := *result.Plan.FullDemo
	executionPath := filepath.Join(dir, "full-demo-execution.json")
	execution := editor.FullDemoExecution{SchemaVersion: "1.0", Approved: recapplan.Snapshot{
		Document: doc, Approval: recapplan.Approval{PlanHash: doc.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()},
	}}
	if err := attachCurrentHUDTelemetry(&execution, dir); err != nil {
		t.Fatal(err)
	}
	overlayPath := writeCurrentOverlayFixture(t, dir, doc.Input.TargetSteamID64)
	if err := writeJSONFile(executionPath, execution); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		args      []string
		wantError bool
	}{
		{"worker arguments", append(fullDemoExecutionArgs(executionPath), "--full-demo-overlay", overlayPath), false},
		{"legacy CLI default", []string{"--full-demo-execution", executionPath}, true},
		{"explicit conflicting trim", append(fullDemoExecutionArgs(executionPath), "--tail-trim=1.5"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "render")
			args := []string{"--recording-result", recordingPath, "--out", out,
				"--preset", editor.PresetGameplayPOV60, "--output-format", "landscape-16x9",
				"--hook=false", "--kill-counter=false", "--covers=false", "--dry-run"}
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

func localOverlayRendererPath() string {
	if path := os.Getenv("ZV_OVERLAY_RENDERER_PATH"); path != "" {
		return path
	}
	candidate := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "ClipHub Studio Local", "ClipHub Studio.exe")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

func writeCurrentOverlayFixture(t *testing.T, dir, target string) string {
	t.Helper()
	doc := demooverlay.BuildForSource(demooverlay.Roster{
		TargetSteamID64: target,
		Map:             "de_mirage",
		ScoreCT:         13,
		ScoreT:          9,
		Rounds:          22,
		Players: []demooverlay.RosterPlayer{
			{SteamID64: target, Name: "observed", Team: "CT", Kills: 20, Deaths: 12, Assists: 4, ADR: 88.4},
			{SteamID64: "76561198000000002", Name: "opponent", Team: "T", Kills: 15, Deaths: 16, Assists: 2, ADR: 71.2},
		},
	}, "", nil)
	doc.Theme = demooverlay.ThemeNeonViolet
	path := filepath.Join(dir, "full-demo-overlay.json")
	if err := demooverlay.Write(path, doc); err != nil {
		t.Fatal(err)
	}
	return path
}

// The stored metadata is deliberately optimistic, while the localized file
// has only 60 generated frames. No native capture or delivery is certified.
func TestFullDemoEditorChecksActualSourceFramesBeforePreparation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ffmpeg, ffprobe := recording.FindFFmpeg(), recording.FindFFprobe()
	if ffmpeg == "" || ffprobe == "" {
		t.Fatal("FFmpeg and ffprobe are required for Full Demo source preflight")
	}
	result, dir, recordingPath := currentFullDemoPublicationFixture(t, "replaced with generated media")
	clip := result.Artifacts[0].Path
	output, err := exec.CommandContext(ctx, ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=s=160x90:r=60:d=1",
		"-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo:d=1", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", clip).CombinedOutput()
	if err != nil {
		t.Fatalf("generate source: %v\n%s", err, output)
	}
	info, err := os.Stat(clip)
	if err != nil {
		t.Fatal(err)
	}
	result.Artifacts[0].SizeBytes = info.Size()
	result.Artifacts[0].ContentSHA256 = ""
	if err := result.DigestSegmentFiles(ctx); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(recordingPath, result); err != nil {
		t.Fatal(err)
	}
	doc := *result.Plan.FullDemo
	executionPath := filepath.Join(dir, "full-demo-execution.json")
	execution := editor.FullDemoExecution{SchemaVersion: "1.0", Approved: recapplan.Snapshot{
		Document: doc, Approval: recapplan.Approval{PlanHash: doc.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()},
	}}
	if err := attachCurrentHUDTelemetry(&execution, dir); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(executionPath, execution); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "render")
	_, err = editor.Run(ctx, editor.Config{RecordingResultPath: recordingPath, FullDemoExecutionPath: executionPath,
		OutputDir: out, Preset: editor.PresetGameplayPOV60, OutputFormat: editor.OutputFormatLandscape16x9,
		CompileSegments: true, DisableCovers: true, FFmpegPath: ffmpeg, FFprobePath: ffprobe})
	if err == nil || !recording.IsNotReusableMessage(err.Error()) || !strings.Contains(err.Error(), "clip has 60 frames") {
		t.Fatalf("actual short source was not rejected before preparation: %v", err)
	}
	var failed editor.Result
	if err := readJSONFile(filepath.Join(out, "shorts-result.json"), &failed); err != nil {
		t.Fatal(err)
	}
	if failed.Executed || !recording.IsNotReusableMessage(failed.Error) {
		t.Fatalf("worker lost the structured recapture reason: %s", failed.Error)
	}
	if _, err := os.Stat(filepath.Join(out, "full-demo-media")); !os.IsNotExist(err) {
		t.Fatal("invalid source reached media preparation")
	}
}

// CLI tests exercise a current Full Demo execution, which requires materialized
// broadcast HUD telemetry. The synthetic timeline is tied to the fixture's
// demo/player identity and only reaches the dry-run/preflight paths.
func currentFullDemoPublicationFixture(t *testing.T, content string) (recording.RecordingResult, string, string) {
	t.Helper()
	return fullDemoPublicationFixture(t, content, func(_ *recapplan.Facts, o *recapplan.Options) {
		o.Overlays.HUDTheme = "arena"
		o.Capture.HUDProfile = customhud.CaptureProfile
	})
}

func attachCurrentHUDTelemetry(execution *editor.FullDemoExecution, dir string) error {
	doc := execution.Approved.Document
	snapshot := customhud.Example()
	snapshot.Tick = 0
	endTick := 1
	for _, round := range doc.Rounds {
		if round.RequestedEndTick > endTick {
			endTick = round.RequestedEndTick
		}
	}
	timeline := customhud.Timeline{
		Version:       customhud.TelemetryVersion,
		DemoSHA256:    doc.Input.DemoSHA256,
		TargetSteamID: doc.Input.TargetSteamID64,
		TickRate:      doc.Clock.TickRate,
		EndTick:       endTick,
		Snapshots:     []customhud.Snapshot{snapshot},
	}
	body, err := json.Marshal(timeline)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "full-demo-hud-telemetry.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		return err
	}
	digest := sha256.Sum256(body)
	execution.HUDTelemetry = &editor.FullDemoLocalHUD{Path: path, SHA256: hex.EncodeToString(digest[:])}
	return nil
}
