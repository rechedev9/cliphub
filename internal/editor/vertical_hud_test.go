package editor

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/recording"
)

// verticalFixture is a compiled replay fixture captured without the CS2 HUD, as the vertical HUD needs.
func verticalFixture(dir string) recording.RecordingResult {
	fixture := replayFixture(dir)
	fixture.Plan.DemoSHA256 = strings.Repeat("c", 64)
	fixture.Plan.Stream.HUDMode = recording.HUDModeDeathnotices
	return fixture
}

// writeVerticalTrack gives each segment two states that split one second after its capture starts.
func writeVerticalTrack(t *testing.T, dir string, plan recording.RecordingPlan) string {
	t.Helper()
	track := customhud.VerticalTrack{Version: customhud.VerticalVersion, DemoSHA256: plan.DemoSHA256, TickRate: plan.Tickrate}
	for _, s := range plan.Segments {
		start := recording.EffectiveRecordStartTick(s, plan.Tickrate)
		first, second := customhud.VerticalExample(), customhud.VerticalExample()
		second.TAlive = 3
		track.Segments = append(track.Segments, customhud.VerticalSegment{ID: s.ID, Spans: []customhud.VerticalSpan{
			{TickStart: start - 500, TickEnd: start + 64, State: first},
			{TickStart: start + 64, TickEnd: s.TickEnd + 200, State: second},
		}})
	}
	path := filepath.Join(dir, "vertical-hud.json")
	body, err := json.Marshal(track)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func verticalRun(t *testing.T, dir string, cfg Config) (Result, error) {
	t.Helper()
	cfg.OutputDir = filepath.Join(dir, "shorts")
	cfg.FFmpegPath = filepath.Join(dir, "missing-ffmpeg")
	cfg.DryRun, cfg.CompileSegments, cfg.VictimReplay = true, true, true
	cfg.VerticalHUD, cfg.CenteredKillfeed = true, true
	return Run(context.Background(), cfg)
}

func TestVerticalHUDFollowsThePlaysButNotTheReplay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fixture := verticalFixture(dir)
	cfg := Config{VerticalHUD: true, VerticalHUDTheme: "prism", VerticalHUDTrackPath: writeVerticalTrack(t, dir, fixture.Plan)}
	hud, warnings, err := prepareVerticalHUD(verticalHUDInput{cfg: cfg, plan: fixture.Plan})
	if err != nil || len(warnings) != 0 || hud.Theme != "prism" {
		t.Fatalf("hud=%+v warnings=%v err=%v", hud, warnings, err)
	}
	if got := hud.Segments["seg-001"]; len(got) != 2 || got[0].StartSeconds != 0 || math.Abs(got[1].StartSeconds-1) > 1e-9 {
		t.Fatalf("seg-001 cues = %+v, want them on the clip clock", got)
	}
	opts := testManifestOptions(dir, nil)
	opts.CompileSegments, opts.VictimReplay, opts.HookText = true, true, true
	opts.Overlays = ShortOverlays{VerticalHUD: hud}
	short := mustBuildManifest(t, fixture, opts).Shorts[0]
	cues := verticalHUDCues(short)
	replay, seg2 := short.Parts[1], short.Parts[2]
	if cues[0].StartSeconds < 1.8-1e-9 {
		t.Errorf("the HUD starts at %v, under the hook title", cues[0].StartSeconds)
	}
	for _, cue := range cues {
		if cue.EndSeconds > replay.TimelineStartSeconds+1e-9 && cue.StartSeconds < seg2.TimelineStartSeconds-1e-9 {
			t.Errorf("HUD drawn over the victim replay: %+v", cue)
		}
	}
	if last := cues[len(cues)-1]; math.Abs(last.EndSeconds-(seg2.TimelineStartSeconds+seg2.DurationSeconds)) > 1e-9 || last.State.TAlive != 3 {
		t.Errorf("last cue = %+v, want seg-002's second state to its end", last)
	}
}

func TestRunDrawsTheVerticalHUDWithItsKillFeed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fixture := verticalFixture(dir)
	result, err := verticalRun(t, dir, Config{
		RecordingResultPath:  writeRecordingResult(t, dir, fixture),
		VerticalHUDTrackPath: writeVerticalTrack(t, dir, fixture.Plan),
	})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	short := result.Shorts[0]
	if command := strings.Join(short.FFmpegCommand, " "); !strings.Contains(command, ".vertical-hud.ass") || !strings.Contains(command, ".killfeed.ass") {
		t.Fatalf("render command misses the HUD or its kill feed:\n%s", command)
	}
	script, err := os.ReadFile(short.Output + ".vertical-hud.ass")
	if err != nil || !strings.Contains(string(script), "PlayResY: 1920") || !strings.Contains(string(script), "}RONDA 17") {
		t.Fatalf("script = %v, want the 9:16 scoreboard", err)
	}
}

func TestVerticalHUDIsLeftOutOverACaptureThatShowsCS2(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]func(*recording.RecordingResult, *Config){
		"native HUD":           func(r *recording.RecordingResult, _ *Config) { r.Plan.Stream.HUDMode = recording.HUDModeGameplay },
		"notices in the frame": func(r *recording.RecordingResult, _ *Config) { r.Plan.Stream.PortraitSafeKillfeed = true },
		"landscape":            func(_ *recording.RecordingResult, c *Config) { c.OutputFormat = OutputFormatLandscape16x9 },
		"no track":             func(_ *recording.RecordingResult, c *Config) { c.VerticalHUDTrackPath = "" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			fixture := verticalFixture(dir)
			cfg := Config{VerticalHUD: true, VerticalHUDTrackPath: writeVerticalTrack(t, dir, fixture.Plan)}
			change(&fixture, &cfg)
			hud, warnings, err := prepareVerticalHUD(verticalHUDInput{cfg: cfg, plan: fixture.Plan})
			if err != nil || hud != nil || len(warnings) != 1 || !strings.HasPrefix(warnings[0], "vertical HUD left out") {
				t.Fatalf("hud=%v warnings=%v err=%v", hud, warnings, err)
			}
		})
	}
	dir := t.TempDir()
	fixture := verticalFixture(dir)
	path := writeVerticalTrack(t, dir, fixture.Plan)
	other := fixture.Plan
	other.DemoSHA256 = strings.Repeat("d", 64)
	if _, _, err := prepareVerticalHUD(verticalHUDInput{cfg: Config{VerticalHUD: true, VerticalHUDTrackPath: path}, plan: other}); err == nil {
		t.Fatal("a track of another demo was accepted")
	}
	if _, _, err := prepareVerticalHUD(verticalHUDInput{cfg: Config{VerticalHUD: true, VerticalHUDTheme: "nope", VerticalHUDTrackPath: path}, plan: fixture.Plan}); err == nil {
		t.Fatal("an unknown design was accepted")
	}
}

func TestVerticalHUDEndsBeforeTheStatsCard(t *testing.T) {
	t.Parallel()
	short := ShortEdit{
		SegmentID: "seg-001", OutputFormat: OutputFormatShort9x16, DurationSeconds: 10,
		Kills: []KillCue{{TimeSeconds: 2}},
		Overlays: ShortOverlays{
			StatsCard:   &StatsCard{Player: "ropz", Kills: 3, Deaths: 1},
			VerticalHUD: &VerticalHUD{Segments: map[string][]VerticalHUDCue{"seg-001": {{StartSeconds: 0, EndSeconds: 12, State: customhud.VerticalExample()}}}},
		},
	}
	cardStart, _, _ := statsCardWindow(short)
	cues := verticalHUDCues(short)
	if len(cues) != 1 || cues[0].StartSeconds != 0 || math.Abs(cues[0].EndSeconds-cardStart) > 1e-9 {
		t.Fatalf("cues = %+v, want 0..%v", cues, cardStart)
	}
	if filters := appendShortASSFilters(nil, short); len(filters) != 0 {
		t.Fatalf("filters before the script is written = %v", filters)
	}
}

func TestVerticalHUDIsDrawnOnceAfterTheConcat(t *testing.T) {
	t.Parallel()
	short := ShortEdit{
		OutputFormat: OutputFormatShort9x16, OutputFPS: 60, DurationSeconds: 12, Transition: TransitionZoomWhip,
		VerticalHUDASS: "hud.ass", VerticalHUDFontsDir: "fonts",
		Parts: []ShortPart{{Input: "a.mp4", DurationSeconds: 6}, {Input: "b.mp4", DurationSeconds: 6, TimelineStartSeconds: 6}},
	}
	filter := CompilationFilter(short)
	// A part that burns the HUD in shows its own clock from 0, and the post-concat zoom scales that stale copy.
	if got := strings.Count(filter, "hud.ass"); got != 1 {
		t.Fatalf("the HUD script is drawn %d times, want once:\n%s", got, filter)
	}
	if strings.Index(filter, "hud.ass") < strings.Index(filter, "concat=") {
		t.Errorf("the HUD is drawn before the concat:\n%s", filter)
	}
}
