package rhythm

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/tickcut/internal/killplan"
	"github.com/rechedev9/tickcut/internal/recording"
)

func TestAnalyzeSamplesDetectsBeatGrid(t *testing.T) {
	const sampleRate = 22050
	const duration = 12.0
	const bpm = 103.0
	period := 60 / bpm
	samples := makePulseTrain(sampleRate, duration, 0.2, period)

	got := AnalyzeSamples(samples, sampleRate, samplesConfig{
		MinBPM:            90,
		MaxBPM:            115,
		KillOffsetSeconds: 0.10,
		MaxBeats:          20,
	})

	if math.Abs(got.EstimatedBPM-bpm) > 2 {
		t.Fatalf("estimated bpm = %.2f, want near %.2f", got.EstimatedBPM, bpm)
	}
	if len(got.BeatTimesSeconds) < 10 {
		t.Fatalf("beat count = %d, want at least 10", len(got.BeatTimesSeconds))
	}
	if math.Abs(got.BeatTimesSeconds[0]-0.2) > 0.08 {
		t.Fatalf("first beat = %.3f, want near 0.2", got.BeatTimesSeconds[0])
	}
}

func TestBuildSegmentSyncAlignsFirstKillAfterBeat(t *testing.T) {
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Segments = []killplan.Segment{
		{
			ID:        "seg-001",
			Round:     4,
			TickStart: 640,
			TickEnd:   1280,
			Kills:     []killplan.Kill{{Tick: 832, Weapon: "awp"}},
		},
	}
	beats := []float64{0.5, 1.0, 1.5, 2.0, 2.5}

	got := BuildSegmentSync(plan, beats, 0.10)

	if len(got) != 1 {
		t.Fatalf("sync entries = %d, want 1", len(got))
	}
	if got[0].SegmentID != "seg-001" {
		t.Fatalf("segment id = %q, want seg-001", got[0].SegmentID)
	}
	if got[0].DeltaToBeatMilliseconds != 100 {
		t.Fatalf("delta to beat = %dms, want 100ms", got[0].DeltaToBeatMilliseconds)
	}
	if got[0].TimelineStartSeconds < 0 {
		t.Fatalf("timeline start = %.3f, want non-negative", got[0].TimelineStartSeconds)
	}
}

func TestRankedPlanAppliesTheSameBestFirstLimitAsRendering(t *testing.T) {
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Segments = []killplan.Segment{
		{ID: "one", TickStart: 0, TickEnd: 640, Kills: []killplan.Kill{{Tick: 64}}},
		{ID: "ace", TickStart: 640, TickEnd: 1280, Kills: []killplan.Kill{{Tick: 704}, {Tick: 768}, {Tick: 832}, {Tick: 896}, {Tick: 960}}},
		{ID: "two", TickStart: 1280, TickEnd: 1920, Kills: []killplan.Kill{{Tick: 1344}, {Tick: 1408}}},
	}

	got := rankedPlan(plan, 2)

	if len(got.Segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(got.Segments))
	}
	if got.Segments[0].ID != "ace" || got.Segments[1].ID != "two" {
		t.Fatalf("ranked ids = %q, %q; want ace, two", got.Segments[0].ID, got.Segments[1].ID)
	}
	if plan.Segments[0].ID != "one" {
		t.Fatal("rankedPlan mutated its input plan")
	}
}

func TestNormalizeTailTrimSecondsUsesStoredMillisecondPrecision(t *testing.T) {
	if got := NormalizeTailTrimSeconds(1.2345); got != 1.235 {
		t.Fatalf("normalized tail trim = %.4f, want 1.235", got)
	}
}

func TestAnalyzeFileRejectsInvalidTailTrimBeforeDecode(t *testing.T) {
	tests := []struct {
		name string
		trim float64
	}{
		{name: "negative", trim: -0.001},
		{name: "not a number", trim: math.NaN()},
		{name: "positive infinity", trim: math.Inf(1)},
		{name: "negative infinity", trim: math.Inf(-1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := AnalyzeFile(context.Background(), Config{
				InputPath:       "input-that-must-not-be-opened.mp4",
				FFmpegPath:      "ffmpeg-that-must-not-run",
				TailTrimSeconds: tt.trim,
			})
			if err == nil || !strings.Contains(err.Error(), "tail trim seconds must be finite and non-negative") {
				t.Fatalf("AnalyzeFile error = %v, want tail trim validation", err)
			}
		})
	}
}

func TestAnalyzeFileRejectsConflictingPlanSourcesBeforeIO(t *testing.T) {
	statCalls := 0
	decodeCalls := 0
	_, err := analyzeFile(
		context.Background(),
		Config{
			InputPath:           "input-that-must-not-be-opened.mp4",
			KillPlanPath:        "killplan-that-must-not-be-opened.json",
			RecordingResultPath: "recording-result-that-must-not-be-opened.json",
			FFmpegPath:          "ffmpeg-that-must-not-run",
		},
		func(string) error {
			statCalls++
			return nil
		},
		func(context.Context, string, string, int) ([]float64, string, error) {
			decodeCalls++
			return nil, "", nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "killplan path and recording result path are mutually exclusive") {
		t.Fatalf("AnalyzeFile error = %v, want conflicting plan-source validation", err)
	}
	if statCalls != 0 || decodeCalls != 0 {
		t.Fatalf("preflight calls = stat %d, decode %d; want zero I/O before rejecting conflicting sources", statCalls, decodeCalls)
	}
}

func TestAnalyzeFileValidatesRecordingResultContract(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*recording.RecordingResult)
		want   string
	}{
		{name: "valid"},
		{
			name: "verified legacy V1",
			mutate: func(result *recording.RecordingResult) {
				result.Plan.CaptureContract = recording.LegacyCaptureContractVersion
				result.Plan.KillPlanSchemaVersion = ""
				result.Plan.DemoSHA256 = ""
				result.Plan.DemoDurationTicks = 0
				result.Plan.EditorialSegmentIDs = nil
				result.CaptureMode = ""
				result.CaptureInputFingerprint = ""
			},
		},
		{
			name: "tampered fingerprint",
			mutate: func(result *recording.RecordingResult) {
				result.CaptureInputFingerprint = strings.Repeat("0", 64)
			},
			want: "capture input fingerprint does not match",
		},
		{
			name: "structured recorder error",
			mutate: func(result *recording.RecordingResult) {
				result.Error = "capture failed"
			},
			want: "recording result error: capture failed",
		},
		{
			name: "unverified capture",
			mutate: func(result *recording.RecordingResult) {
				result.CaptureVerified = false
			},
			want: "lacks completed POV verification",
		},
		{
			name: "fake capture",
			mutate: func(result *recording.RecordingResult) {
				result.CaptureMode = recording.CaptureModeFake
			},
			want: `capture_mode must be "real"`,
		},
		{
			name: "dry-run capture",
			mutate: func(result *recording.RecordingResult) {
				result.CaptureMode = recording.CaptureModeDryRun
			},
			want: `capture_mode must be "real"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validRhythmRecordingResult(t)
			if tt.mutate != nil {
				tt.mutate(&result)
			}
			resultPath := writeRhythmRecordingResult(t, result)
			analysis, err := analyzeFile(
				context.Background(),
				Config{
					InputPath:           "music-that-must-not-be-opened.wav",
					RecordingResultPath: resultPath,
				},
				func(string) error { return nil },
				func(context.Context, string, string, int) ([]float64, string, error) {
					return makePulseTrain(22050, 4, 0.1, 0.5), strings.Repeat("b", 64), nil
				},
			)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("analyzeFile error = %v, want valid recording result", err)
				}
				if analysis.RecordingResultPath != resultPath {
					t.Fatalf("recording result path = %q, want %q", analysis.RecordingResultPath, resultPath)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "validate recording result:") ||
				!strings.Contains(err.Error(), tt.want) {
				t.Fatalf("analyzeFile error = %v, want recording-result rejection containing %q", err, tt.want)
			}
		})
	}
}

func validRhythmRecordingResult(t *testing.T) recording.RecordingResult {
	t.Helper()
	plan := killplan.NewPlan()
	plan.Demo.SHA256 = strings.Repeat("a", 64)
	plan.Demo.Tickrate = 64
	plan.Demo.DurationTicks = 1000
	plan.Target.SteamID64 = "76561197960265729"
	plan.Segments = []killplan.Segment{{
		ID:        "seg-001",
		Round:     1,
		TickStart: 64,
		TickEnd:   512,
		Kills:     []killplan.Kill{{Tick: 192, Weapon: "ak47"}},
	}}
	recordingPlan, err := recording.NewPlanFromKillPlan(
		plan,
		"match.dem",
		"recording-output",
		recording.DefaultStreamConfig(),
	)
	if err != nil {
		t.Fatal(err)
	}
	result := recording.RecordingResult{
		Plan:            recordingPlan,
		CaptureMode:     recording.CaptureModeReal,
		CaptureVerified: true,
	}
	result.CaptureInputFingerprint, err = recording.CaptureInputFingerprint(result.Plan)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func writeRhythmRecordingResult(t *testing.T, result recording.RecordingResult) string {
	t.Helper()
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "recording-result.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func makePulseTrain(sampleRate int, duration, phase, period float64) []float64 {
	total := int(duration * float64(sampleRate))
	samples := make([]float64, total)
	width := int(0.035 * float64(sampleRate))
	for t := phase; t < duration; t += period {
		center := int(t * float64(sampleRate))
		for i := -width; i <= width; i++ {
			idx := center + i
			if idx < 0 || idx >= len(samples) {
				continue
			}
			x := float64(i) / float64(width)
			samples[idx] += math.Exp(-x * x * 8)
		}
	}
	return samples
}
