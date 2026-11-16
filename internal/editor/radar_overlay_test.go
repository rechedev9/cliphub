package editor

import (
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/radaroverlay"
	"github.com/rechedev9/cliphub/internal/recording"
)

// radarFixture is the compiled replay fixture captured without the CS2 HUD, as the radar needs.
func radarFixture(dir string) recording.RecordingResult {
	fixture := replayFixture(dir)
	fixture.Plan.DemoSHA256 = strings.Repeat("e", 64)
	fixture.Plan.Stream.HUDMode = recording.HUDModeDeathnotices
	return fixture
}

// writeRadarTrack puts one moving player and a smoke in every segment of the plan, and the map art.
func writeRadarTrack(t *testing.T, dir string, plan recording.RecordingPlan) (string, string) {
	t.Helper()
	cal, _ := radarmap.Lookup("de_mirage")
	track := radaroverlay.Track{Version: radaroverlay.Version, DemoSHA256: plan.DemoSHA256, TickRate: plan.Tickrate, Map: "de_mirage", Calibration: cal}
	for _, s := range plan.Segments {
		start := recording.EffectiveRecordStartTick(s, plan.Tickrate)
		track.Segments = append(track.Segments, radaroverlay.Segment{
			ID: s.ID, TickStart: start, TickEnd: s.TickEnd, POV: 0,
			Frames: []radaroverlay.Frame{
				{Tick: start, Players: []radaroverlay.Player{{Slot: 0, Side: "T", X: -1000, Y: -1000}}},
				{Tick: start + 64, Players: []radaroverlay.Player{{Slot: 0, Side: "T", X: -900, Y: -1000}}},
			},
			Utility: []radaroverlay.Utility{{Kind: radaroverlay.KindSmoke, Tick: start, X: -500, Y: -500}},
		})
	}
	trackPath := filepath.Join(dir, "radar-track.json")
	body, err := json.Marshal(track)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(trackPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	artPath := filepath.Join(dir, "radar-upper.png")
	f, err := os.Create(artPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return trackPath, artPath
}

func TestRunDrawsTheRadarOverItsPlatesButNotOverTheVictimReplay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fixture := radarFixture(dir)
	trackPath, artPath := writeRadarTrack(t, dir, fixture.Plan)
	result, err := Run(context.Background(), Config{
		RecordingResultPath: writeRecordingResult(t, dir, fixture),
		OutputDir:           filepath.Join(dir, "shorts"),
		FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
		DryRun:              true, CompileSegments: true, VictimReplay: true, HookText: true,
		Radar: true, RadarPosition: radaroverlay.PositionTopRight, RadarTrackPath: trackPath, RadarImagePath: artPath,
	})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	short := result.Shorts[0]
	command := strings.Join(short.FFmpegCommand, " ")
	radarAt, lastImage := strings.Index(command, ".radar.ass"), strings.LastIndex(command, "[vimages]")
	if radarAt < 0 || lastImage < 0 || radarAt < lastImage {
		t.Fatalf("the radar layer must be drawn after the plates:\n%s", command)
	}
	replay := short.Parts[1]
	var plates []Effect
	for _, e := range short.Effects {
		if e.Source == "radar" {
			plates = append(plates, e)
		}
	}
	if len(plates) != 2 {
		t.Fatalf("plates = %+v, want one per play around the victim replay", plates)
	}
	if plates[0].StartSeconds < 1.8-1e-9 || plates[0].EndSeconds > replay.TimelineStartSeconds+1e-9 || plates[1].StartSeconds < replay.TimelineStartSeconds+replay.DurationSeconds-1e-9 {
		t.Errorf("plates %+v / %+v overlap the hook title or the victim replay %+v", plates[0], plates[1], replay)
	}
	if _, err := os.Stat(plates[0].Path); err != nil || !strings.HasSuffix(plates[0].Path, ".radar-default.png") {
		t.Errorf("plate %q was not baked: %v", plates[0].Path, err)
	}
	if x := plates[0].X; x != "693" {
		t.Errorf("plate x = %s, want the top right corner of a 1080 px frame", x)
	}
	script, err := os.ReadFile(short.Output + ".radar.ass")
	if err != nil || !strings.Contains(string(script), `\move(`) || !strings.Contains(string(script), `\1c&HD2CDC8&`) {
		t.Fatalf("script = %v, want the moving player and the smoke", err)
	}
}

func TestRadarIsLeftOutOverACaptureThatShowsCS2(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]func(*recording.RecordingResult, *Config){
		"native HUD": func(r *recording.RecordingResult, _ *Config) { r.Plan.Stream.HUDMode = recording.HUDModeGameplay },
		"no track":   func(_ *recording.RecordingResult, c *Config) { c.RadarTrackPath = "" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			fixture := radarFixture(dir)
			trackPath, _ := writeRadarTrack(t, dir, fixture.Plan)
			cfg := Config{Radar: true, RadarTrackPath: trackPath}
			change(&fixture, &cfg)
			radar, warnings, err := prepareRadarOverlay(radarOverlayInput{cfg: cfg, plan: fixture.Plan})
			if err != nil || radar != nil || len(warnings) != 1 || !strings.HasPrefix(warnings[0], "radar left out") {
				t.Fatalf("radar=%v warnings=%v err=%v", radar, warnings, err)
			}
		})
	}
	dir := t.TempDir()
	fixture := radarFixture(dir)
	trackPath, _ := writeRadarTrack(t, dir, fixture.Plan)
	other := fixture.Plan
	other.DemoSHA256 = strings.Repeat("f", 64)
	if _, _, err := prepareRadarOverlay(radarOverlayInput{cfg: Config{Radar: true, RadarTrackPath: trackPath}, plan: other}); err == nil {
		t.Fatal("a track of another demo was accepted")
	}
	if _, _, err := prepareRadarOverlay(radarOverlayInput{cfg: Config{Radar: true, RadarSize: "huge", RadarTrackPath: trackPath}, plan: fixture.Plan}); err == nil {
		t.Fatal("an unknown size was accepted")
	}
	radar, _, err := prepareRadarOverlay(radarOverlayInput{cfg: Config{Radar: true, RadarTrackPath: trackPath}, plan: fixture.Plan})
	if err != nil || radar.Starts["seg-001"] != recording.EffectiveRecordStartTick(fixture.Plan.Segments[0], fixture.Plan.Tickrate) {
		t.Fatalf("radar = %+v, %v; want each segment's capture start", radar, err)
	}
}

func TestRadarPlateGroupsJoinQuickCutsOnOneLevel(t *testing.T) {
	t.Parallel()
	groups := radarPlateGroups([]radaroverlay.Span{
		{From: 0, Until: 4, Level: radarmap.LevelDefault},
		{From: 4.1, Until: 8, Level: radarmap.LevelDefault},
		{From: 8, Until: 9, Level: radarmap.LevelLower},
		{From: 12, Until: 14, Level: radarmap.LevelLower},
	})
	if len(groups) != 3 || groups[0].until != 8 || groups[1].level != radarmap.LevelLower || groups[2].from != 12 {
		t.Fatalf("groups = %+v, want the quick cut joined and the level change and the long gap kept apart", groups)
	}
}

func TestRadarAvoidsTheWatermark(t *testing.T) {
	t.Parallel()
	short := ShortEdit{OutputFormat: OutputFormatShort9x16, Effects: []Effect{
		{Type: EffectImage, Source: "brand-watermark", X: "43", Y: "1500", Width: 170, Height: 100},
		{Type: EffectImage, Source: "full-demo-intro", X: "0", Y: "0", Width: 1920, Height: 1080},
	}}
	avoid := radarAvoidRects(short, 1920)
	if len(avoid) != 1 || avoid[0] != (radaroverlay.Rect{X: 43, Y: 1500, W: 170, H: 100}) {
		t.Fatalf("avoid = %+v, want only the watermark", avoid)
	}
}
