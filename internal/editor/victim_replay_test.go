package editor

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recording"
)

// replayFixture gives seg-001 a recorded victim replay clip; seg-002 has none.
func replayFixture(dir string) recording.RecordingResult {
	result := testRecordingResult(dir)
	result.Plan.Stream.VictimReplay = true
	result.Plan.Segments[0].VictimReplay = &recording.VictimReplay{SteamID64: "76561198000000001", KillTick: 14450, TickStart: 14258, TickEnd: 14482}
	result.Artifacts = append(result.Artifacts, recording.RecordingArtifact{
		SegmentID: "seg-001-victim", Role: "segment", Type: "video", DurationSeconds: 3.5,
		Path: filepath.Join(dir, "recording", "segments", "seg-001-victim.mp4"),
	})
	return result
}

func TestRunCompilationPlaysTheVictimReplayAfterItsPlay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	result, err := Run(context.Background(), Config{
		RecordingResultPath: writeRecordingResult(t, dir, replayFixture(dir)),
		OutputDir:           filepath.Join(dir, "shorts"),
		FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
		DryRun:              true,
		CompileSegments:     true,
		VictimReplay:        true,
	})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	parts := result.Shorts[0].Parts
	var ids []string
	for _, part := range parts {
		ids = append(ids, part.SegmentID)
	}
	if strings.Join(ids, ",") != "seg-001,seg-001-victim,seg-002" {
		t.Fatalf("parts = %v, want the replay right after seg-001", ids)
	}
	replay := parts[1]
	if replay.Speed != victimReplaySpeed || replay.DurationSeconds != 7 || replay.TimelineStartSeconds != parts[0].DurationSeconds {
		t.Fatalf("replay part = %+v", replay)
	}
	if parts[2].TimelineStartSeconds != parts[0].DurationSeconds+7 {
		t.Fatalf("seg-002 starts at %v, want after the replay", parts[2].TimelineStartSeconds)
	}
	filter := CompilationFilter(ShortEdit{Parts: parts, Transition: TransitionCut, Tickrate: 64})
	for _, want := range []string{"[1:v]setpts=PTS/0.500,", ",fade=t=in:st=0:d=0.150:color=white,trim=end_frame=420", "aresample=48000,atempo=0.500,atrim=duration=7.000000"} {
		if !strings.Contains(filter, want) {
			t.Errorf("compilation filter misses %q:\n%s", want, filter)
		}
	}
	if strings.Count(filter, "atempo") != 1 {
		t.Errorf("only the replay part may be slowed:\n%s", filter)
	}
}

func TestRunSingleShortBecomesPlayPlusReplay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	result, err := Run(context.Background(), Config{
		RecordingResultPath: writeRecordingResult(t, dir, replayFixture(dir)),
		OutputDir:           filepath.Join(dir, "shorts"),
		FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
		DryRun:              true,
		VictimReplay:        true,
	})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	for _, short := range result.Shorts {
		switch short.SegmentID {
		case "seg-001":
			if len(short.Parts) != 2 || short.Parts[1].SegmentID != "seg-001-victim" || short.DurationSeconds != short.Parts[0].DurationSeconds+7 {
				t.Errorf("seg-001 = %d parts, %v s; want play plus 7 s replay", len(short.Parts), short.DurationSeconds)
			}
		case "seg-002":
			if len(short.Parts) != 0 {
				t.Errorf("seg-002 has no replay clip but got %d parts", len(short.Parts))
			}
		}
	}
}

func TestRunWithoutVictimReplayIgnoresRecordedReplays(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	result, err := Run(context.Background(), Config{
		RecordingResultPath: writeRecordingResult(t, dir, replayFixture(dir)),
		OutputDir:           filepath.Join(dir, "shorts"),
		FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
		DryRun:              true,
		CompileSegments:     true,
	})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	if got := len(result.Shorts[0].Parts); got != 2 {
		t.Fatalf("parts = %d, want the two plays only", got)
	}
}
