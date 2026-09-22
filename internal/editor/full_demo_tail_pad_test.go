package editor

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func tailPadTestShort(input string, pads []recording.FullDemoTailPad) ShortEdit {
	options := recapplan.DefaultOptions()
	options.Capture.HUDProfile = "native-clean-spectator"
	options.Overlays.HUDTheme = ""
	options.Audio.Music.Enabled = false
	return ShortEdit{
		Preset: PresetGameplayPOV60, OutputFormat: OutputFormatLandscape16x9, OutputFPS: 60, VideoCRF: 18, VideoPreset: "ultrafast", Threads: 2,
		Parts:    []ShortPart{{SegmentID: "round-006", Input: input}},
		FullDemo: &FullDemoRenderEvidence{Effective: recapplan.Document{Clock: recapplan.Clock{TickRate: 64}, Options: options}, CaptureTailPads: pads},
		fullDemo: &fullDemoRenderContext{
			ffmpeg:    "ffmpeg",
			recording: recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "round-006", TickStart: 50_000}}}},
		},
	}
}

// The incident round: a 1,913-frame window over a 1,912-frame clip. The item
// clones the last frame once before the trim, so it still emits 1,913 frames.
func TestFullDemoItemCommandPadsRecordedTailShortfall(t *testing.T) {
	item := recapplan.TimelineItem{Role: "round", SourceRef: "round-006", SourceStartTick: 50_000, SourceEndTick: 52_040, EndFrame: 1913, EndSample: 1913 * recapplan.SamplesPerFrame}
	pad := recording.FullDemoTailPad{SegmentID: "round-006", ClipFrames: 1912, WindowFrames: 1913, PaddedFrames: 1}
	command, err := fullDemoItemVideoCommand(tailPadTestShort("round-006.mp4", []recording.FullDemoTailPad{pad}), item, "item.nut")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(command, " ")
	if !strings.Contains(joined, "[0:v]fps=60,tpad=stop_mode=clone:stop=1,trim=start_frame=0:end_frame=1913,") {
		t.Fatalf("item video must clone exactly the recorded shortfall before its trim: %v", command)
	}
	if strings.Count(joined, "tpad=") != 1 {
		t.Fatalf("exactly one tail pad expected: %v", command)
	}
	// The audio bus already pads to the canonical sample count; it must not change.
	audio, err := fullDemoItemAudioCommand(tailPadTestShort("round-006.mp4", []recording.FullDemoTailPad{pad}), item, "item-audio.nut")
	if err != nil {
		t.Fatal(err)
	}
	unpadded, err := fullDemoItemAudioCommand(tailPadTestShort("round-006.mp4", nil), item, "item-audio.nut")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(audio, " ") != strings.Join(unpadded, " ") || strings.Contains(strings.Join(audio, " "), "tpad") {
		t.Fatalf("a video tail pad must not alter the sample-exact audio item: %v", audio)
	}
}

func TestFullDemoItemCommandWithoutPadIsUnchanged(t *testing.T) {
	item := recapplan.TimelineItem{Role: "round", SourceRef: "round-006", SourceStartTick: 50_000, SourceEndTick: 52_040, EndFrame: 1913, EndSample: 1913 * recapplan.SamplesPerFrame}
	command, err := fullDemoItemVideoCommand(tailPadTestShort("round-006.mp4", nil), item, "item.nut")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(command, " ")
	if strings.Contains(joined, "tpad") || !strings.Contains(joined, "[0:v]fps=60,trim=start_frame=0:end_frame=1913,") {
		t.Fatalf("an item without a recorded pad must keep the legacy chain: %v", command)
	}
}

func TestFullDemoItemTailPadBoundsEveryItem(t *testing.T) {
	pad := recording.FullDemoTailPad{SegmentID: "round-006", ClipFrames: 1911, WindowFrames: 1913, PaddedFrames: 2}
	for _, tc := range []struct {
		name                 string
		pads                 []recording.FullDemoTailPad
		segment              string
		sourceOffset, frames int64
		want                 int64
		wantErr              bool
	}{
		{"whole round pads the recorded shortfall", []recording.FullDemoTailPad{pad}, "round-006", 0, 1913, 2, false},
		{"split prefix before the sponsor needs no pad", []recording.FullDemoTailPad{pad}, "round-006", 0, 900, 0, false},
		{"split suffix pads the round tail", []recording.FullDemoTailPad{pad}, "round-006", 900, 1013, 2, false},
		{"suffix ending on the last captured frame needs no pad", []recording.FullDemoTailPad{pad}, "round-006", 900, 1011, 0, false},
		{"other rounds are never padded", []recording.FullDemoTailPad{pad}, "round-007", 0, 1913, 0, false},
		{"no recorded pad means no pad", nil, "round-006", 0, 1913, 0, false},
		{"past the recorded window fails", []recording.FullDemoTailPad{pad}, "round-006", 0, 1914, 0, true},
		{"an item made only of clones fails", []recording.FullDemoTailPad{pad}, "round-006", 1911, 2, 0, true},
		{"an unbounded recorded pad fails", []recording.FullDemoTailPad{{SegmentID: "round-006", ClipFrames: 1900, WindowFrames: 1913, PaddedFrames: 13}}, "round-006", 0, 1913, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fullDemoItemTailPad(tc.pads, tc.segment, tc.sourceOffset, tc.frames)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("pad=%d err=%v, want pad=%d error=%t", got, err, tc.want, tc.wantErr)
			}
		})
	}
	for _, frames := range []int64{-1, 0, recording.FullDemoTailPadToleranceFrames + 1, 1 << 40} {
		if filter := fullDemoTailPadFilter(frames); filter != "" {
			t.Fatalf("out-of-range pad %d must never reach tpad: %q", frames, filter)
		}
	}
}

func TestFullDemoCaptureTailPadsValidateAgainstEffectiveRounds(t *testing.T) {
	effective := recapplan.Document{Rounds: []recapplan.Round{{ID: "round-006"}, {ID: "round-007"}}}
	good := recording.FullDemoTailPad{SegmentID: "round-006", ClipFrames: 1912, WindowFrames: 1913, PaddedFrames: 1}
	if err := validateFullDemoCaptureTailPads(nil, effective); err != nil {
		t.Fatal(err)
	}
	if err := validateFullDemoCaptureTailPads([]recording.FullDemoTailPad{good}, effective); err != nil {
		t.Fatal(err)
	}
	for _, pads := range [][]recording.FullDemoTailPad{
		{good, good},
		{{SegmentID: "round-999", ClipFrames: 1912, WindowFrames: 1913, PaddedFrames: 1}},
		{{SegmentID: "round-006", ClipFrames: 1910, WindowFrames: 1913, PaddedFrames: 3}},
	} {
		if err := validateFullDemoCaptureTailPads(pads, effective); err == nil {
			t.Fatalf("pads %+v must be rejected", pads)
		}
	}
}

func TestFullDemoDeliveryDocumentRecordsTailPads(t *testing.T) {
	delivery := &FullDemoDeliveryEvidence{FullDecode: true, FrameCount: 1913, SampleRate: 48000, Channels: 2, DurationSeconds: 1913.0 / 60, ContentSHA256: strings.Repeat("a", 64)}
	legacy, err := json.Marshal(delivery)
	if err != nil {
		t.Fatal(err)
	}
	unpadded, err := json.Marshal(FullDemoDocumentFiles(FullDemoRenderEvidence{Delivery: delivery})["full-demo-delivery.json"])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(legacy, unpadded) {
		t.Fatalf("a render without pads must keep the cached delivery document: %s != %s", unpadded, legacy)
	}
	pad := recording.FullDemoTailPad{SegmentID: "round-006", ClipFrames: 1912, WindowFrames: 1913, PaddedFrames: 1}
	padded, err := json.Marshal(FullDemoDocumentFiles(FullDemoRenderEvidence{Delivery: delivery, CaptureTailPads: []recording.FullDemoTailPad{pad}})["full-demo-delivery.json"])
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		FrameCount      int64                       `json:"frame_count"`
		CaptureTailPads []recording.FullDemoTailPad `json:"capture_tail_pads"`
	}
	if err := json.Unmarshal(padded, &document); err != nil {
		t.Fatal(err)
	}
	if document.FrameCount != 1913 || len(document.CaptureTailPads) != 1 || document.CaptureTailPads[0] != pad {
		t.Fatalf("delivery document must record the padded frames: %s", padded)
	}
}

// Real FFmpeg: a clip two frames short of its window renders an item with the
// canonical frame count, while the same command without the pad comes up short.
func TestFullDemoItemTailPadRendersCanonicalFrameCount(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	dir := t.TempDir()
	source := filepath.Join(dir, "round-006.nut")
	// 64 Hz: 34 ticks round to 32 frames; the clip holds only 30.
	if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=320x180:r=60", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo",
		"-frames:v", "30", "-t", "0.5", "-c:v", "libx264", "-preset", "ultrafast", "-bf", "0", "-c:a", "pcm_f32le", source}, "tail pad fixture"); err != nil {
		t.Fatal(err)
	}
	frames, err := recapplan.TickFrames(34, 64)
	if err != nil || frames != 32 {
		t.Fatalf("fixture window = %d frames (%v), want 32", frames, err)
	}
	item := recapplan.TimelineItem{Role: "round", SourceRef: "round-006", SourceStartTick: 50_000, SourceEndTick: 50_034, EndFrame: frames, EndSample: frames * recapplan.SamplesPerFrame}
	count := func(path string) int {
		t.Helper()
		out, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", path, "-map", "0:v:0", "-fps_mode", "passthrough", "-f", "framemd5", "-").Output()
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, line := range strings.Split(string(out), "\n") {
			if line != "" && !strings.HasPrefix(line, "#") {
				n++
			}
		}
		return n
	}
	render := func(pads []recording.FullDemoTailPad, name string) string {
		t.Helper()
		short := tailPadTestShort(source, pads)
		short.fullDemo.ffmpeg = ffmpeg
		output := filepath.Join(dir, name)
		command, err := fullDemoItemVideoCommand(short, item, output)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runFFmpegOutput(ctx, command, "tail pad item"); err != nil {
			t.Fatal(err)
		}
		return output
	}
	if got := count(source); got != 30 {
		t.Fatalf("fixture clip has %d frames, want 30", got)
	}
	if got := count(render(nil, "unpadded.nut")); got != 30 {
		t.Fatalf("unpadded item has %d frames; the fixture no longer reproduces a short clip", got)
	}
	pad := recording.FullDemoTailPad{SegmentID: "round-006", ClipFrames: 30, WindowFrames: 32, PaddedFrames: 2}
	if got := count(render([]recording.FullDemoTailPad{pad}, "padded.nut")); got != int(frames) {
		t.Fatalf("padded item has %d frames, want the canonical %s", got, strconv.FormatInt(frames, 10))
	}
	// A clip with enough frames keeps its exact trim even with a stale pad:
	// the clone sits after the trim window and is discarded.
	full := filepath.Join(dir, "full.nut")
	if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=320x180:r=60", "-frames:v", "40", "-c:v", "libx264", "-preset", "ultrafast", "-bf", "0", full}, "tail pad full fixture"); err != nil {
		t.Fatal(err)
	}
	short := tailPadTestShort(full, []recording.FullDemoTailPad{pad})
	short.fullDemo.ffmpeg = ffmpeg
	output := filepath.Join(dir, "full-item.nut")
	command, err := fullDemoItemVideoCommand(short, item, output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runFFmpegOutput(ctx, command, "tail pad full item"); err != nil {
		t.Fatal(err)
	}
	if got := count(output); got != int(frames) {
		t.Fatalf("a long enough clip rendered %d frames, want %d", got, frames)
	}
}
