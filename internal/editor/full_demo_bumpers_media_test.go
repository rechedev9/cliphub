package editor

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/recording"
)

func TestFullDemoBumperTransitionsRenderWithRoundEffectsOff(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	dir := t.TempDir()
	short, _, _ := bumperShort(t)
	d := &short.FullDemo.Effective
	d.Options.Transitions.Enabled = false
	d.Options.Capture.HUDProfile, d.Options.Overlays.HUDTheme = "native-clean-spectator", ""
	d.Options.Audio.Voice.Enabled, d.Options.Audio.Music.Enabled = false, false
	short.VideoPreset, short.VideoCRF, short.Threads = "ultrafast", 18, 2
	short.Output, short.DurationSeconds = filepath.Join(dir, "bumpers.mp4"), 3
	short.fullDemo.ffmpeg, short.fullDemo.workDir = ffmpeg, filepath.Join(dir, "prepared")
	for i, color := range []string{"navy", "maroon", "green"} {
		name := filepath.Join(dir, color+".mp4")
		command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=c=" + color + ":s=320x180:r=60:d=1", "-f", "lavfi", "-i", "sine=f=440:r=48000:d=1", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", name}
		if _, err := runFFmpegOutput(ctx, command, "bumper source"); err != nil {
			t.Fatal(err)
		}
		item := &d.Timeline[i]
		item.StartFrame, item.EndFrame = int64(i*60), int64((i+1)*60)
		item.StartSample, item.EndSample = item.StartFrame*800, item.EndFrame*800
		if i == 1 {
			item.SourceStartTick, item.SourceEndTick = 0, 64
			short.Parts = []ShortPart{{SegmentID: item.SourceRef, Input: name}}
			short.fullDemo.recording = recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: item.SourceRef}}}}
		} else {
			assetIndex := i / 2
			short.fullDemo.execution.Assets[assetIndex].Path = name
			d.Assets[assetIndex].DurationFrames = 60
		}
	}
	if err := prepareFullDemoCompilation(ctx, &short, nil); err != nil {
		t.Fatal(err)
	}
	if len(short.FullDemo.Transitions) != 2 || short.FullDemo.Transitions[0].Frame != 60 || short.FullDemo.Transitions[1].Frame != 120 {
		t.Fatalf("missing bumper cuts: %+v", short.FullDemo.Transitions)
	}
	if err := short.FullDemo.validateTransitions(); err != nil {
		t.Fatal(err)
	}
	if _, err := runFFmpegOutput(ctx, buildFullDemoCompilationCommand(ffmpeg, short), "bumper concat"); err != nil {
		t.Fatal(err)
	}
	// A pixel from every decoded frame proves both order and exact duration.
	pixels, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", fullDemoProgramPath(short), "-vf", "format=rgb24,crop=1:1:960:540", "-pix_fmt", "rgb24", "-f", "rawvideo", "pipe:1").Output()
	if err != nil || len(pixels) != 180*3 {
		t.Fatalf("frame count: %d, %v", len(pixels)/3, err)
	}
	for i, channel := range []int{2, 0, 1} {
		pixel := pixels[(i*60+30)*3 : (i*60+30)*3+3]
		if pixel[channel] < 90 || pixel[(channel+1)%3] > 10 || pixel[(channel+2)%3] > 10 {
			t.Fatalf("intro/demo/outro order at item %d: %v", i, pixel)
		}
	}
	for _, frame := range []int{59, 60, 119, 120} {
		channel := []int{2, 0, 1}[frame/60]
		if int(pixels[frame*3+channel])-int(pixels[(frame/60*60+30)*3+channel]) < 10 {
			t.Fatalf("transition has no visible flash at frame %d", frame)
		}
	}
	pcm, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", fullDemoProgramAudioPath(short), "-f", "f32le", "-acodec", "pcm_f32le", "pipe:1").Output()
	if err != nil || len(pcm) != 144000*2*4 {
		t.Fatalf("audio sample count: %d, %v", len(pcm)/8, err)
	}
	energy := func(start, end int) float64 {
		var sum float64
		for i := start * 8; i < end*8; i += 4 {
			v := float64(math.Float32frombits(binary.LittleEndian.Uint32(pcm[i : i+4])))
			sum += v * v
		}
		return sum
	}
	if energy(96000, 98000) == 0 || energy(120000, 124800) != 0 || energy(24000, 28800) == 0 {
		t.Fatal("bumper audio must retain intro sound, add the outro transition and keep a silent outro silent afterward")
	}
	// Export an optional review artifact; the test itself verifies lossless media.
	if root := os.Getenv("FULL_DEMO_EVIDENCE_DIR"); root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		command := []string{ffmpeg, "-y", "-v", "error", "-i", fullDemoProgramPath(short), "-i", fullDemoProgramAudioPath(short), "-map", "0:v", "-map", "1:a", "-c:v", "copy", "-c:a", "aac", "-movflags", "+faststart", filepath.Join(root, "bumper-transitions.mp4")}
		if _, err := runFFmpegOutput(ctx, command, "bumper review artifact"); err != nil {
			t.Fatal(err)
		}
	}
}
