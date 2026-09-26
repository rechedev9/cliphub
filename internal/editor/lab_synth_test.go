package editor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// A synthetic bundle is only useful if the render accepts it as it accepts a
// real capture: the recording result, the approved execution and every clip
// must pass the editor's own loading and Full Demo checks.
func TestSynthLabBundlePassesTheRenderInputChecks(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dir := filepath.Join(t.TempDir(), "synth")

	bundle, err := SynthLabBundle(ctx, LabSynthOptions{Rounds: 2, Dir: dir, FFmpeg: ffmpeg, Width: 160, Height: 90})
	if err != nil {
		t.Fatalf("synthetic bundle: %v", err)
	}
	if !bundle.Synthetic || !strings.Contains(bundle.SourcePlan, "synthetic") {
		t.Fatalf("bundle = synthetic %t, source %q; want it marked synthetic", bundle.Synthetic, bundle.SourcePlan)
	}
	result, err := ReadRecordingResult(filepath.Join(dir, "recording-result.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := result.FullDemoEvidence.Validate(result.Plan); err != nil {
		t.Fatalf("synthetic capture evidence rejected by the capture contract: %v", err)
	}
	if len(result.Artifacts) != 2 {
		t.Fatalf("artifacts = %d, want one capture per round", len(result.Artifacts))
	}
	for _, artifact := range result.Artifacts {
		media, err := measureLabMedia(ctx, ffmpeg, ffprobe, artifact.Path, filepath.Join(t.TempDir(), artifact.SegmentID), artifact.FrameCount)
		if err != nil {
			t.Fatal(err)
		}
		if !media.FramesMatch || media.BlackSeconds > 0 || media.IntegratedLUFS == nil {
			t.Fatalf("%s measured %d/%d frames, black %.2fs, loudness %v; want exact frames, a lit picture and audio", artifact.SegmentID, media.Frames, artifact.FrameCount, media.BlackSeconds, media.IntegratedLUFS)
		}
	}
	overlay, err := demooverlay.Load(filepath.Join(dir, "full-demo-overlay.json"))
	if err != nil {
		t.Fatalf("synthetic overlay: %v", err)
	}
	if len(overlay.Intro.Left)+len(overlay.Intro.Right) != 10 || len(overlay.Outro.Teams) == 0 {
		t.Fatalf("synthetic overlay has no roster: %+v", overlay)
	}

	// Prepare the render the way the lab does, without the overlay: its intro
	// and outro PNGs come from the Chromium renderer, which this test does
	// not need, so they are stand-ins. The checks under test are the capture's.
	in, err := prepareRunInputs(ctx, Config{
		RecordingResultPath: filepath.Join(dir, "recording-result.json"), KillPlanPath: filepath.Join(dir, "killplan.json"),
		OutputDir: filepath.Join(dir, "out"), Preset: PresetGameplayPOV60, OutputFormat: OutputFormatLandscape16x9,
		CompileSegments: true, SegmentIDs: recording.EditorialSegmentIDs(result), FullDemoExecutionPath: filepath.Join(dir, "full-demo-execution.json"),
		FFmpegPath: ffmpeg, DisableCovers: true, DryRun: true,
	})
	if err != nil {
		t.Fatalf("synthetic bundle rejected while preparing the render: %v", err)
	}
	if in.fullDemoExecution == nil || len(in.fullDemoExecution.VoiceTracks) != 1 {
		t.Fatalf("execution = %+v, want the approved plan with the target's team voice", in.fullDemoExecution)
	}
	in.manifest.Shorts[0].FullDemoIntroImagePath, in.manifest.Shorts[0].FullDemoOutroImagePath = "intro.png", "outro.png"
	if err := attachFullDemoExecution(&in.manifest, in.recordingResult, in.fullDemoExecution, ffmpeg); err != nil {
		t.Fatalf("synthetic capture rejected against its approval: %v", err)
	}
	if got, want := len(in.manifest.Shorts[0].FullDemo.Effective.Timeline), len(in.fullDemoExecution.Approved.Document.Timeline); got != want {
		t.Fatalf("effective timeline has %d items, want the approved %d", got, want)
	}
}

func TestSynthLabBundleRejectsPlansItCannotSupply(t *testing.T) {
	document, err := labSynthSnapshot(1)
	if err != nil {
		t.Fatal(err)
	}
	stale := document.Document
	stale.PlanHash = strings.Repeat("0", 64)
	// A custom HUD plan with a matching hash reaches the synthetic bundle's own
	// guard instead of the render's staleness check.
	withHUD := document.Document
	withHUD.Options.Overlays.HUDTheme = recapplan.DefaultOptions().Overlays.HUDTheme
	withHUD.Options.Capture.HUDProfile = recapplan.DefaultOptions().Capture.HUDProfile
	if withHUD.PlanHash, err = withHUD.Hash(); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		write func(string) error
		want  string
	}{
		"edited without re-planning": {func(path string) error { return writeLabJSON(path, stale) }, "not renderable"},
		"custom HUD":                 {func(path string) error { return writeLabJSON(path, withHUD) }, "cannot draw"},
		"not a plan":                 {func(path string) error { return os.WriteFile(path, []byte(`{"document": 3}`), 0o600) }, "decode plan"},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "plan.json")
			if err := tc.write(path); err != nil {
				t.Fatal(err)
			}
			_, err := SynthLabBundle(context.Background(), LabSynthOptions{PlanPath: path, Dir: t.TempDir()})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("synthetic bundle error = %v, want it rejected with %q", err, tc.want)
			}
		})
	}
	if _, err := labSynthSnapshot(0); err == nil {
		t.Fatal("a synthetic demo with no rounds was planned")
	}
}

// The synthetic bundle writes capture evidence from these values, so they
// must satisfy the capture contract for every capture profile a plan can use.
func TestFullDemoExpectedCaptureCvarsSatisfyTheCaptureContract(t *testing.T) {
	snapshot, err := labSynthSnapshot(1)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*recapplan.Options){
		"native HUD":         func(*recapplan.Options) {},
		"custom HUD capture": func(o *recapplan.Options) { o.Capture.HUDProfile = recapplan.DefaultOptions().Capture.HUDProfile },
		"true view":          func(o *recapplan.Options) { o.Capture.TrueView = true },
		"provided crosshair": func(o *recapplan.Options) {
			o.Capture.Crosshair.Mode, o.Capture.Crosshair.Code = "provided-code", "CSGO-WsnnD-eHaMw-QNDf9-oxuDh-ydOUD"
		},
	} {
		t.Run(name, func(t *testing.T) {
			d := snapshot.Document
			mutate(&d.Options)
			plan := recording.RecordingPlan{FullDemo: &d, Segments: []recording.RecordingSegment{{ID: "round-001", TickStart: 0, TickEnd: 100, LiveEndTick: 90}}}
			expected, err := recording.FullDemoExpectedCaptureCvars(plan)
			if err != nil {
				t.Fatal(err)
			}
			evidence := recording.FullDemoCaptureEvidence{SchemaVersion: "1.0", Restored: true, FilesRestored: true, CertifiedEnds: map[string]int{"round-001": 100}}
			for name, value := range expected {
				evidence.Before = append(evidence.Before, recording.CvarValue{Name: name, Value: []byte("1")})
				evidence.Applied = append(evidence.Applied, recording.CvarValue{Name: name, Value: value})
			}
			if err := evidence.Validate(plan); err != nil {
				t.Fatalf("expected cvars fail the capture contract: %v", err)
			}
		})
	}
}
