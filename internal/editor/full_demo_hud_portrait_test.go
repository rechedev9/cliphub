package editor

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func TestFullDemoFocusPortraitPreservesAlphaAspectAndFrameCount(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	dir := t.TempDir()
	source, portrait := filepath.Join(dir, "source.nut"), filepath.Join(dir, "portrait.png")
	if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=black:s=320x180:r=60:d=0.5", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo:d=0.5", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "pcm_f32le", source}, "portrait fixture"); err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 40, 80))
	for y := 20; y < 80; y++ {
		for x := 0; x < 40; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	f, err := os.Create(portrait)
	if err != nil {
		t.Fatal(err)
	}
	err = png.Encode(f, img)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	ref := recapplan.AssetRef{ID: "22222222-2222-4222-8222-222222222222", SHA256: strings.Repeat("a", 64)}
	options := recapplan.DefaultOptions()
	options.Audio.Voice.Enabled = false
	options.Overlays.HUDTheme, options.Overlays.HUDPortrait = "focus", &ref
	options.Transitions.Enabled = false
	item := recapplan.TimelineItem{Role: "round", SourceRef: "a", EndFrame: 30, EndSample: 24000}
	d := recapplan.Document{Clock: recapplan.Clock{TickRate: 60}, Options: options, Timeline: []recapplan.TimelineItem{item}}
	timeline := customhud.Timeline{Version: customhud.TelemetryVersion, DemoSHA256: strings.Repeat("a", 64), TargetSteamID: customhud.ExampleTarget, TickRate: 60, EndTick: 60, Snapshots: []customhud.Snapshot{customhud.Example()}}
	short := ShortEdit{Preset: PresetGameplayPOV60, OutputFormat: OutputFormatLandscape16x9, OutputFPS: 60, VideoCRF: 18, VideoPreset: "ultrafast", Threads: 2, Parts: []ShortPart{{SegmentID: "a", Input: source}}, FullDemo: &FullDemoRenderEvidence{Effective: d}, fullDemo: &fullDemoRenderContext{hud: &timeline, ffmpeg: ffmpeg, execution: FullDemoExecution{Assets: []FullDemoLocalMedia{{Ref: ref, Path: portrait}}}, recording: recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "a", TickStart: 0}}}}}}
	for _, videoOnly := range []bool{false, true} {
		output := filepath.Join(dir, "output.nut")
		var command []string
		if videoOnly {
			command, err = fullDemoItemVideoCommand(short, item, output)
		} else {
			command, err = fullDemoItemCommand(short, item, output)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runFFmpegOutput(ctx, command, "portrait composition"); err != nil {
			t.Fatal(err)
		}
		// Sampling every decoded frame also proves the still neither shortens
		// nor extends the 30-frame main stream, for muxed and split generation.
		for _, probe := range []struct {
			crop string
			red  bool
		}{{"1:1:712:970", true}, {"1:1:712:908", false}, {"1:1:660:908", false}} {
			data, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", output, "-vf", "format=rgb24,crop="+probe.crop, "-pix_fmt", "rgb24", "-f", "rawvideo", "pipe:1").Output()
			if err != nil || len(data) != 30*3 {
				t.Fatalf("portrait frame count: %d %v", len(data)/3, err)
			}
			for i := 0; i < len(data); i += 3 {
				if probe.red && (data[i] < 220 || data[i+1] > 30) {
					t.Fatal("portrait missing or distorted")
				}
				if !probe.red && (data[i] > 15 || data[i+1] > 15 || data[i+2] > 15) {
					t.Fatal("portrait lost transparency or aspect ratio")
				}
			}
		}
	}
	short.fullDemo.execution.Assets = nil
	if _, err := fullDemoItemCommand(short, item, filepath.Join(dir, "missing.nut")); err == nil {
		t.Fatal("missing approved portrait silently omitted")
	}
}
