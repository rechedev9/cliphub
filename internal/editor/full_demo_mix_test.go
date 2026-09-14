package editor

import (
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func TestFullDemoCaptureSeekKeepsShortOffsetsUnchanged(t *testing.T) {
	for _, offset := range []int64{0, 1, 61, 120} {
		seek, trim := fullDemoCaptureSeek(offset)
		if seek != 0 || trim != offset {
			t.Fatalf("offset %d: seek=%d trim=%d, want seek 0 trim %d", offset, seek, trim, offset)
		}
	}
}

func TestFullDemoCaptureSeekPrerollsTwoSeconds(t *testing.T) {
	seek, trim := fullDemoCaptureSeek(1000)
	if seek != 880 || trim != 120 {
		t.Fatalf("seek=%d trim=%d, want 880/120 (2s preroll at 60fps)", seek, trim)
	}
	if seek+trim != 1000 {
		t.Fatalf("seek+trim = %d, want original offset 1000", seek+trim)
	}
}

func TestFullDemoItemCommandSeeksCaptureBeforeTrim(t *testing.T) {
	options := recapplan.DefaultOptions()
	options.Capture.HUDProfile = "native-clean-spectator"
	options.Overlays.HUDTheme = ""
	short := ShortEdit{
		Parts:    []ShortPart{{SegmentID: "round-001", Input: "game.nut"}},
		FullDemo: &FullDemoRenderEvidence{Effective: recapplan.Document{Clock: recapplan.Clock{TickRate: 64}, Options: options}},
		fullDemo: &fullDemoRenderContext{
			ffmpeg:    "ffmpeg",
			recording: recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "round-001", TickStart: 64}}}},
		},
	}
	item := recapplan.TimelineItem{
		Role: "round", SourceRef: "round-001",
		SourceStartTick: 64 + 1067, SourceOffsetFrames: 0,
		EndFrame: 60, EndSample: 48000,
	}
	command, err := fullDemoItemCommand(short, item, "round.nut")
	if err != nil {
		t.Fatal(err)
	}
	seconds, ok := firstInputSeek(command)
	if !ok {
		t.Fatalf("missing input -ss before game -i: %v", command)
	}
	seekFrames := int64(seconds*recapplan.OutputFPS + 0.5)
	if seekFrames != 880 {
		t.Fatalf("input seek = %d frames (%v), want 880", seekFrames, command)
	}
	joined := strings.Join(command, " ")
	if !strings.Contains(joined, "trim=start_frame=120:end_frame=180") {
		t.Fatalf("video trim lost its capture-relative window: %v", command)
	}
	if !strings.Contains(joined, "atrim=start_sample=96000:end_sample=144000") {
		t.Fatalf("audio trim lost its capture-relative window: %v", command)
	}
}

func TestFullDemoItemCommandDoesNotSeekShortCaptureOffset(t *testing.T) {
	options := recapplan.DefaultOptions()
	options.Capture.HUDProfile = "native-clean-spectator"
	options.Overlays.HUDTheme = ""
	short := ShortEdit{
		Parts:    []ShortPart{{SegmentID: "round-001", Input: "game.nut"}},
		FullDemo: &FullDemoRenderEvidence{Effective: recapplan.Document{Clock: recapplan.Clock{TickRate: 64}, Options: options}},
		fullDemo: &fullDemoRenderContext{
			ffmpeg:    "ffmpeg",
			recording: recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "round-001", TickStart: 64}}}},
		},
	}
	item := recapplan.TimelineItem{
		Role: "round", SourceRef: "round-001",
		SourceStartTick: 129, SourceOffsetFrames: 0,
		EndFrame: 60, EndSample: 48000,
	}
	command, err := fullDemoItemCommand(short, item, "round.nut")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := firstInputSeek(command); ok {
		t.Fatalf("short capture offset should not input-seek: %v", command)
	}
	joined := strings.Join(command, " ")
	if !strings.Contains(joined, "trim=start_frame=61:end_frame=121") {
		t.Fatalf("video trim = %v, want original capture offset 61", command)
	}
}

func firstInputSeek(command []string) (float64, bool) {
	for i, arg := range command {
		if arg != "-i" || i+1 >= len(command) {
			continue
		}
		if i >= 2 && command[i-2] == "-ss" {
			seconds, err := strconv.ParseFloat(command[i-1], 64)
			return seconds, err == nil
		}
		return 0, false
	}
	return 0, false
}
