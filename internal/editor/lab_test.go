package editor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// labFixtureShort approves a two-round Full Demo plan over synthetic 121-frame
// captures of the given colors, with a team-voice track, the way the render
// lab receives it after attachFullDemoExecution.
func labFixtureShort(t *testing.T, ctx context.Context, ffmpeg, dir, roundOneColor, roundTwoColor string) ShortEdit {
	t.Helper()
	makeMedia := func(name, color string, seconds float64) string {
		t.Helper()
		path := filepath.Join(dir, name+".nut")
		command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=c=" + color + ":s=160x90:r=60:d=" + decimal(seconds), "-f", "lavfi", "-i", "sine=f=440:r=48000:d=" + decimal(seconds), "-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-preset", "ultrafast", "-bf", "0", "-c:a", "pcm_f32le", "-ac", "2", path}
		if _, err := runFFmpegOutput(ctx, command, "create synthetic media"); err != nil {
			t.Fatal(err)
		}
		return path
	}
	roundOne := makeMedia("round-one", roundOneColor, 121.0/60)
	roundTwo := makeMedia("round-two", roundTwoColor, 121.0/60)
	voice := makeMedia("team-voice", "black", 10)
	options := recapplan.DefaultOptions()
	options.Capture.HUDProfile = "native-clean-spectator"
	options.Overlays.HUDTheme = ""
	options.Capture.Crosshair.AllowCaptureDefault = true
	options.Editorial.FreezeSeconds, options.Editorial.RoundTailSeconds = 0, 0
	options.Editorial.KeepFreezeVoice = false
	options.Audio.Voice.Normalization = "none"
	facts := recapplan.Facts{SchemaVersion: "1.0", DemoSHA256: strings.Repeat("a", 64), TargetSteamID64: "76561198000000001", ClockKind: recapplan.ClockIngame, TickRate: 64, EndTick: 640, Complete: true, Rounds: []recapplan.RoundFacts{
		{ID: "round-001", Number: 1, StartTick: 0, FreezeEndTick: 256, RoundEndTick: 256, NextStartTick: 320, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}},
		{ID: "round-002", Number: 2, StartTick: 320, FreezeEndTick: 576, RoundEndTick: 576, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}},
	}}
	voiceEvidence := recapplan.VoiceEvidence{Availability: "available", IndexRef: "synthetic/index.json", IndexHash: strings.Repeat("b", 64), ExtractorVersion: "team-packet-clock-v2", ClockKind: recapplan.ClockIngame, SelectedPackets: 2, Activity: []recapplan.TickRange{{Start: 128, End: 577}}}
	document, err := recapplan.Plan(facts, options, voiceEvidence, nil, "synthetic/facts.json")
	if err != nil || len(document.Blockers) > 0 {
		t.Fatalf("plan: %v; blockers: %+v", err, document.Blockers)
	}
	approval := recapplan.Snapshot{Document: document, Approval: recapplan.Approval{PlanHash: document.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()}}
	execution := FullDemoExecution{SchemaVersion: "1.0", Approved: approval, VoiceTracks: []FullDemoLocalVoice{{SteamID64: facts.TargetSteamID64, StorageKey: "synthetic/team-voice", Path: voice}}}
	short := ShortEdit{Preset: PresetGameplayPOV60, OutputFormat: OutputFormatLandscape16x9, OutputFPS: 60, Tickrate: 64, VideoCRF: 18, VideoPreset: "ultrafast", Threads: 2, Output: filepath.Join(dir, "final.mp4"), DurationSeconds: 242.0 / 60,
		Parts:    []ShortPart{{SegmentID: "round-001", Input: roundOne}, {SegmentID: "round-002", Input: roundTwo}},
		FullDemo: &FullDemoRenderEvidence{SchemaVersion: "1.0", Approved: approval, Effective: document},
		fullDemo: &fullDemoRenderContext{execution: execution, recording: recording.RecordingResult{Plan: recording.RecordingPlan{Tickrate: 64, DemoDurationTicks: 640, Segments: []recording.RecordingSegment{{ID: "round-001", TickStart: 128}, {ID: "round-002", TickStart: 448}}}}, ffmpeg: ffmpeg, workDir: filepath.Join(dir, "media")},
	}
	if err := os.MkdirAll(short.fullDemo.workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return short
}

func labItemIndex(t *testing.T, short ShortEdit, sourceRef string) int {
	t.Helper()
	for i, item := range short.FullDemo.Effective.Timeline {
		if item.SourceRef == sourceRef {
			return i
		}
	}
	t.Fatalf("timeline has no item for %s: %+v", sourceRef, short.FullDemo.Effective.Timeline)
	return -1
}

func TestLabItemRendersOneBoundedItemAndFlagsABlackCapture(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	short := labFixtureShort(t, ctx, ffmpeg, t.TempDir(), "red", "black")

	lit, err := labItem(ctx, &short, ffprobe, LabOptions{Index: labItemIndex(t, short, "round-001"), Seconds: 1})
	if err != nil {
		t.Fatalf("lab item for the lit round: %v", err)
	}
	if !lit.Excerpt || lit.ExpectedFrames != 60 || !lit.Media.FramesMatch || lit.Media.Frames != 60 {
		t.Fatalf("lit excerpt = excerpt %t, expected %d, measured %d (match %t); want a 60-frame excerpt", lit.Excerpt, lit.ExpectedFrames, lit.Media.Frames, lit.Media.FramesMatch)
	}
	if lit.Media.BlackSeconds != 0 || lit.Media.MeanLuma < 40 {
		t.Fatalf("lit round measured black %.2fs, mean luma %.1f; want no black and a lit picture", lit.Media.BlackSeconds, lit.Media.MeanLuma)
	}
	if lit.Media.IntegratedLUFS == nil || lit.Media.TruePeakDBTP == nil {
		t.Fatalf("lit round audio was not measured: %+v", lit.Media)
	}
	if len(lit.Media.Stills) != 3 {
		t.Fatalf("stills = %v, want start, middle and end frames", lit.Media.Stills)
	}
	for _, still := range lit.Media.Stills {
		if info, err := os.Stat(still); err != nil || info.Size() == 0 {
			t.Fatalf("still %s was not written: %v", still, err)
		}
	}

	dark, err := labItem(ctx, &short, ffprobe, LabOptions{Index: labItemIndex(t, short, "round-002"), Seconds: 0})
	if err != nil {
		t.Fatalf("lab item for the black round: %v", err)
	}
	if dark.Excerpt || !dark.Media.FramesMatch {
		t.Fatalf("whole black item = excerpt %t, frames %d/%d; want the full item", dark.Excerpt, dark.Media.Frames, dark.ExpectedFrames)
	}
	itemSeconds := float64(dark.ExpectedFrames) / recapplan.OutputFPS
	if dark.Media.BlackSeconds < itemSeconds-0.1 || dark.Media.MeanLuma > 20 {
		t.Fatalf("black round measured black %.2fs of %.2fs, mean luma %.1f; want it flagged as black", dark.Media.BlackSeconds, itemSeconds, dark.Media.MeanLuma)
	}
}

func TestLabItemRejectsAnIndexOutsideTheTimeline(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	short := labFixtureShort(t, ctx, ffmpeg, t.TempDir(), "red", "blue")
	_, err := labItem(ctx, &short, "ffprobe", LabOptions{Index: len(short.FullDemo.Effective.Timeline)})
	if err == nil || !strings.Contains(err.Error(), "run the plan mode") {
		t.Fatalf("out-of-range index error = %v, want a pointer to the plan mode", err)
	}
}

func TestLabAudioRunsTheMasterLoopToTheLoudnessTarget(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	dir := t.TempDir()
	short := labFixtureShort(t, ctx, ffmpeg, dir, "red", "blue")

	audio, err := labAudio(ctx, &short, filepath.Join(dir, "lab"))
	if err != nil {
		t.Fatalf("lab audio: %v", err)
	}
	if len(audio.Loudness.MasterTargets) == 0 || audio.Loudness.FinalMuxedAAC == nil || audio.Loudness.FinalMuxedAAC.IntegratedLUFS == nil {
		t.Fatalf("lab audio evidence has no master attempts or final measurement: %+v", audio.Loudness)
	}
	target := short.FullDemo.Effective.Options.Audio.Loudness
	if got := *audio.Loudness.FinalMuxedAAC.IntegratedLUFS; got < target.TargetILUFS-1.5 || got > target.TargetILUFS+1.5 {
		t.Fatalf("final integrated loudness = %.2f LUFS, want within 1.5 LU of %.1f", got, target.TargetILUFS)
	}
	if _, err := os.Stat(audio.Output); err != nil {
		t.Fatalf("mastered output %s is missing: %v", audio.Output, err)
	}
}
