package editor

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// Synthetic sources exercise composition only, without claiming a real capture
// attestation. Camera transitions must not distort the broadcast scoreboard.
func TestFullDemoHUDRemainsStableDuringRoundTransition(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	dir := t.TempDir()
	source := filepath.Join(dir, "source.nut")
	if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=320x180:r=60:d=0.5", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo:d=0.5", "-c:v", "libx264", "-preset", "ultrafast", "-bf", "0", "-c:a", "pcm_f32le", source}, "HUD transition source"); err != nil {
		t.Fatal(err)
	}
	timeline := customhud.Timeline{Version: customhud.TelemetryVersion, DemoSHA256: strings.Repeat("a", 64), TargetSteamID: customhud.ExampleTarget, TickRate: 60, EndTick: 90, Snapshots: []customhud.Snapshot{customhud.Example()}}
	timeline.Snapshots[0].Tick = 0
	d := recapplan.Document{Clock: recapplan.Clock{TickRate: 60}, Options: recapplan.DefaultOptions(), Timeline: []recapplan.TimelineItem{
		{Role: "round", SourceRef: "a", SourceStartTick: 0, SourceEndTick: 30, StartFrame: 0, EndFrame: 30, StartSample: 0, EndSample: 24000},
		{Role: "round", SourceRef: "b", SourceStartTick: 60, SourceEndTick: 90, StartFrame: 30, EndFrame: 60, StartSample: 24000, EndSample: 48000},
	}}
	d.Options.Audio.Music.Enabled = false
	d.Options.Overlays.HUDTheme = "arena"
	d.Options.Capture.HUDProfile = customhud.CaptureProfile
	pixels := func(path, crop string) []byte {
		t.Helper()
		data, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", path, "-vf", "select=eq(n\\,29),crop="+crop, "-frames:v", "1", "-pix_fmt", "rgb24", "-f", "rawvideo", "pipe:1").Output()
		if err != nil || len(data) == 0 {
			t.Fatalf("decode HUD transition sample: %v", err)
		}
		return data
	}
	var cleanHUD, cleanGame []byte
	for _, enabled := range []bool{false, true} {
		o := recapplan.DefaultTransitions()
		o.Enabled, o.Whip, o.Zoom, o.Flash = enabled, true, false, true
		o.Direction, o.FlashIntensity = "left", .2
		o.Whoosh, o.Impact = false, false
		d.Options.Transitions = &o
		output := filepath.Join(dir, "clean.nut")
		if enabled {
			output = filepath.Join(dir, "transition.nut")
		}
		short := ShortEdit{Preset: PresetGameplayPOV60, OutputFormat: OutputFormatLandscape16x9, OutputFPS: 60, VideoCRF: 18, VideoPreset: "ultrafast", Threads: 2, Parts: []ShortPart{{SegmentID: "a", Input: source}}, FullDemo: &FullDemoRenderEvidence{Effective: d}, fullDemo: &fullDemoRenderContext{hud: &timeline, ffmpeg: ffmpeg, recording: recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "a", TickStart: 0}}}}}}
		command, err := fullDemoItemCommand(short, d.Timeline[0], 0, output)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runFFmpegOutput(ctx, command, "HUD transition composition"); err != nil {
			t.Fatal(err)
		}
		hud, game := pixels(output, "640:48:640:40"), pixels(output, "1120:450:400:340")
		if !enabled {
			cleanHUD, cleanGame = hud, game
			continue
		}
		difference := func(a, b []byte) float64 {
			if len(a) != len(b) {
				t.Fatal("transition changed the output geometry")
			}
			total := 0
			for i := range a {
				total += absInt(int(a[i]) - int(b[i]))
			}
			return float64(total) / float64(len(a))
		}
		if got := difference(cleanHUD, hud); got > 1.5 {
			t.Fatalf("transition distorted the fixed HUD: mean pixel difference %.3f", got)
		}
		if got := difference(cleanGame, game); got < 4 {
			t.Fatalf("transition did not affect gameplay: mean pixel difference %.3f", got)
		}
	}
}
