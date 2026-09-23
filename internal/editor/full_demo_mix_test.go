package editor

import (
	"runtime"
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

func TestFullDemoVideoFilterBudgetDoesNotChangeEncoderOrAudioThreads(t *testing.T) {
	short := fullDemoItemOverlayFixtureShort(t.TempDir())
	short.Threads = 7 // Explicit encoder setting remains independent.
	item := recapplan.TimelineItem{Role: "round", SourceRef: "round-001", SourceStartTick: 64, EndFrame: 60, EndSample: 48000}
	for _, streams := range []fullDemoItemStreams{fullDemoItemVideoOnly, fullDemoItemAudioOnly, fullDemoItemMuxed} {
		command, err := fullDemoItemStreamCommand(short, item, "item.nut", streams)
		if err != nil {
			t.Fatal(err)
		}
		i := argIndex(command, "-filter_complex_threads")
		if streams == fullDemoItemVideoOnly {
			if i < 0 {
				t.Fatal("video item has no filter budget")
			}
			threads, err := strconv.Atoi(command[i+1])
			if err != nil || threads < 1 || threads > 4 || threads > runtime.NumCPU() {
				t.Fatalf("unbounded filter budget: %v", command)
			}
		} else if i >= 0 {
			t.Fatalf("filter budget changed another stream kind: %v", command)
		}
		if streams != fullDemoItemAudioOnly {
			j := argIndex(command, "-threads")
			if j < 0 || command[j+1] != "7" {
				t.Fatalf("encoder setting changed: %v", command)
			}
		}
	}
}

// Audio-only items run no encoder, so they get their own budget instead of the
// item encoder budget. It still has to be explicitly bounded and CPU-aware.
func TestFullDemoAudioItemJobsIsBoundedAndCPUAware(t *testing.T) {
	jobs := fullDemoAudioItemJobs()
	if jobs < 1 || jobs > fullDemoAudioItemJobsMax {
		t.Fatalf("audio item jobs = %d, want 1..%d", jobs, fullDemoAudioItemJobsMax)
	}
	if jobs > runtime.NumCPU() {
		t.Fatalf("audio item jobs = %d, above %d CPUs", jobs, runtime.NumCPU())
	}
	if jobs < fullDemoItemJobs() {
		t.Fatalf("audio item jobs = %d, below the %d encoder jobs it must not be more conservative than", jobs, fullDemoItemJobs())
	}
	// Pin the measured ceiling: collapsing it back to the encoder budget (3)
	// would silently lose the audio-items gain without failing any other test.
	if fullDemoAudioItemJobsMax <= 3 {
		t.Fatalf("fullDemoAudioItemJobsMax = %d, want above the 3-worker encoder budget it was measured against", fullDemoAudioItemJobsMax)
	}
	if runtime.NumCPU() >= fullDemoAudioItemJobsMax && jobs != fullDemoAudioItemJobsMax {
		t.Fatalf("audio item jobs = %d on %d CPUs, want the full %d budget", jobs, runtime.NumCPU(), fullDemoAudioItemJobsMax)
	}
}

// Each item stream kind must draw from its own budget: the video pool from the
// encoder budget, the audio pool from the filter-graph budget.
func TestFullDemoItemPoolJobsPerStreamKind(t *testing.T) {
	for streams, want := range map[fullDemoItemStreams]int{
		fullDemoItemAudioOnly: fullDemoAudioItemJobs(),
		fullDemoItemVideoOnly: fullDemoItemJobs(),
		fullDemoItemMuxed:     fullDemoItemJobs(),
	} {
		if got := fullDemoItemPoolJobs(streams); got != want {
			t.Fatalf("pool jobs for stream kind %d = %d, want %d", streams, got, want)
		}
	}
}

// Voice preparation overlaps the video branch since the concurrent pipelines
// change, so its pool is sized to cover a full team instead of staying out of
// the item encoders' way. It must still be explicitly bounded and CPU-aware.
func TestFullDemoVoiceJobsCoverAFullTeamWithinTheCPUBound(t *testing.T) {
	if got := fullDemoVoiceJobs(5); got != min(5, runtime.NumCPU()) {
		t.Fatalf("jobs for a five-track team = %d, want every track running at once on %d CPUs", got, runtime.NumCPU())
	}
	for _, count := range []int{1, 2, 3, 5, 20} {
		jobs := fullDemoVoiceJobs(count)
		if jobs < 1 || jobs > fullDemoVoiceJobsMax || jobs > count || jobs > runtime.NumCPU() {
			t.Fatalf("jobs for %d tracks = %d, want 1..min(%d, count, %d CPUs)", count, jobs, fullDemoVoiceJobsMax, runtime.NumCPU())
		}
	}
}
