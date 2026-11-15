package editor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/worldoverlay"
)

// writeWorldTrack writes a track where the POV sees a teammate ahead on every tick of each plan segment.
func writeWorldTrack(t *testing.T, dir string, plan recording.RecordingPlan) string {
	t.Helper()
	track := worldoverlay.Track{Version: worldoverlay.Version, DemoSHA256: plan.DemoSHA256, TickRate: plan.Tickrate}
	for _, s := range plan.Segments {
		seg := worldoverlay.Segment{ID: s.ID, TickStart: s.TickStart, TickEnd: s.TickEnd, Names: []string{"mate"}}
		for tick := s.TickStart - 200; tick <= s.TickEnd+100; tick++ {
			seg.Frames = append(seg.Frames, worldoverlay.Frame{
				Tick:    tick,
				Camera:  &worldoverlay.Camera{Eye: worldoverlay.Vec{0, 0, 64}, Side: "CT"},
				Players: []worldoverlay.Player{{Slot: 0, Side: "CT", Health: 80, Eye: worldoverlay.Vec{500, 0, 64}}},
			})
		}
		track.Segments = append(track.Segments, seg)
	}
	path := filepath.Join(dir, "world-track.json")
	body, err := json.Marshal(track)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func worldOverlayRun(t *testing.T, dir string, cfg Config) (Result, error) {
	t.Helper()
	cfg.OutputDir = filepath.Join(dir, "shorts")
	cfg.FFmpegPath = filepath.Join(dir, "missing-ffmpeg")
	cfg.DryRun = true
	cfg.CompileSegments = true
	cfg.VictimReplay = true
	cfg.PlayerLabels = true
	return Run(context.Background(), cfg)
}

func TestRunDrawsTheWorldOverlayOnEachPlayButNotTheReplay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fixture := replayFixture(dir)
	result, err := worldOverlayRun(t, dir, Config{
		RecordingResultPath: writeRecordingResult(t, dir, fixture),
		WorldTrackPath:      writeWorldTrack(t, dir, fixture.Plan),
	})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	command := strings.Join(result.Shorts[0].FFmpegCommand, " ")
	for _, want := range []string{"[0:v]ass=filename=", "segment-001.ass", "[1:v]setpts=PTS/0.500,", "[2:v]ass=filename=", "segment-002.ass"} {
		if !strings.Contains(command, want) {
			t.Errorf("compilation misses %q:\n%s", want, command)
		}
	}
	script, err := os.ReadFile(filepath.Join(dir, "shorts", "world-overlay", "segment-001.ass"))
	if err != nil || !strings.Contains(string(script), "}mate") {
		t.Fatalf("segment-001 script = %v, want the teammate's label", err)
	}
}

func TestRunWithoutAWorldTrackWarnsAndRendersWithoutIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	result, err := worldOverlayRun(t, dir, Config{RecordingResultPath: writeRecordingResult(t, dir, replayFixture(dir))})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "player labels and grenade trails left out") {
		t.Errorf("warnings = %v, want the missing track named", result.Warnings)
	}
	if command := strings.Join(result.Shorts[0].FFmpegCommand, " "); strings.Contains(command, "world-overlay") {
		t.Errorf("a world overlay was drawn without a track:\n%s", command)
	}
}

func TestRunRefusesAWorldTrackOfAnotherDemo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fixture := replayFixture(dir)
	other := fixture.Plan
	other.DemoSHA256 = strings.Repeat("a", 64)
	_, err := worldOverlayRun(t, dir, Config{
		RecordingResultPath: writeRecordingResult(t, dir, fixture),
		WorldTrackPath:      writeWorldTrack(t, dir, other),
	})
	if err == nil || !strings.Contains(err.Error(), "another demo") {
		t.Fatalf("Run error = %v, want the mismatched track refused", err)
	}
}

func TestWorldOverlayFilterSkipsFullDemoAndUnknownSegments(t *testing.T) {
	short := ShortEdit{SegmentID: "seg-001"}
	short.Overlays.World = &WorldOverlay{FontsDir: "fonts", Scripts: map[string]string{"seg-001": "a.ass"}}
	if got := worldOverlayFilter(short, "seg-001"); !strings.HasPrefix(got, "ass=filename=") || !strings.HasSuffix(got, ",") {
		t.Errorf("filter = %q, want an ass filter ending in a comma", got)
	}
	if got := worldOverlayFilter(short, "seg-002"); got != "" {
		t.Errorf("filter for a segment without a script = %q", got)
	}
	short.FullDemo = &FullDemoRenderEvidence{}
	if got := worldOverlayFilter(short, "seg-001"); got != "" {
		t.Errorf("Full Demo filter = %q, want none", got)
	}
}
