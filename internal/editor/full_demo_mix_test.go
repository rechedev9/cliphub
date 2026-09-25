package editor

import (
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// Capture offsets up to the 2 s preroll (120 frames at 60 fps) trim from the
// start of the capture; longer offsets input-seek to 2 s before the item and
// keep a 120-frame trim, so the trimmed window is the same capture frames.
func TestFullDemoItemCommandSeeksCaptureBeforeTrim(t *testing.T) {
	tests := []struct {
		name         string
		startTick    int
		offsetFrames int64
		wantSeek     int64
		wantTrim     string
		wantAtrim    string
	}{
		{name: "short tick offset 61", startTick: 129, wantTrim: "trim=start_frame=61:end_frame=121", wantAtrim: "atrim=start_sample=48800:end_sample=96800"},
		{name: "preroll boundary 120", startTick: 64, offsetFrames: 120, wantTrim: "trim=start_frame=120:end_frame=180", wantAtrim: "atrim=start_sample=96000:end_sample=144000"},
		{name: "one past preroll 121", startTick: 64, offsetFrames: 121, wantSeek: 1, wantTrim: "trim=start_frame=120:end_frame=180", wantAtrim: "atrim=start_sample=96000:end_sample=144000"},
		{name: "long tick offset 1000", startTick: 64 + 1067, wantSeek: 880, wantTrim: "trim=start_frame=120:end_frame=180", wantAtrim: "atrim=start_sample=96000:end_sample=144000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				SourceStartTick: tt.startTick, SourceOffsetFrames: tt.offsetFrames,
				EndFrame: 60, EndSample: 48000,
			}
			command, err := fullDemoItemCommand(short, item, "round.nut")
			if err != nil {
				t.Fatal(err)
			}
			seconds, ok := firstInputSeek(command)
			if tt.wantSeek == 0 {
				if ok {
					t.Fatalf("offset within preroll should not input-seek: %v", command)
				}
			} else {
				if !ok {
					t.Fatalf("missing input -ss before game -i: %v", command)
				}
				if seekFrames := int64(seconds*recapplan.OutputFPS + 0.5); seekFrames != tt.wantSeek {
					t.Fatalf("input seek = %d frames, want %d: %v", seekFrames, tt.wantSeek, command)
				}
			}
			joined := strings.Join(command, " ")
			if !strings.Contains(joined, tt.wantTrim) {
				t.Fatalf("video trim missing %q: %v", tt.wantTrim, command)
			}
			if !strings.Contains(joined, tt.wantAtrim) {
				t.Fatalf("audio trim missing %q: %v", tt.wantAtrim, command)
			}
		})
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
