package editor

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// bumperShort builds the smallest ShortEdit whose effective document opens
// with a 3 s intro bumper (with audio) and closes with a 4 s silent outro
// bumper around a single 10 s round.
func bumperShort(t *testing.T) (ShortEdit, recapplan.AssetRef, recapplan.AssetRef) {
	t.Helper()
	intro := recapplan.AssetRef{ID: uuid.NewString(), SHA256: strings.Repeat("1", 64)}
	outro := recapplan.AssetRef{ID: uuid.NewString(), SHA256: strings.Repeat("2", 64)}
	options := recapplan.DefaultOptions()
	options.Bumpers = &recapplan.BumperOptions{Intro: recapplan.BumperSlot{Enabled: true, Video: &intro}, Outro: recapplan.BumperSlot{Enabled: true, Video: &outro}}
	document := recapplan.Document{Options: options, Clock: recapplan.Clock{TickRate: 64, FPS: 60, SampleRate: 48000},
		Assets: []recapplan.AssetEvidence{
			{Ref: intro, DurationFrames: 180, HasVideo: true, HasAudio: true},
			{Ref: outro, DurationFrames: 240, HasVideo: true, HasAudio: false},
		},
		Timeline: []recapplan.TimelineItem{
			{Role: "bumper", SourceRef: intro.ID, StartFrame: 0, EndFrame: 180, StartSample: 0, EndSample: 180 * 800, Reason: recapplan.BumperRoleIntro},
			{Role: "round", SourceRef: "round-001", StartFrame: 180, EndFrame: 780, StartSample: 180 * 800, EndSample: 780 * 800},
			{Role: "bumper", SourceRef: outro.ID, StartFrame: 780, EndFrame: 1020, StartSample: 780 * 800, EndSample: 1020 * 800, Reason: recapplan.BumperRoleOutro},
		}}
	short := ShortEdit{Preset: PresetGameplayPOV60, OutputFormat: OutputFormatLandscape16x9, OutputFPS: 60, DurationSeconds: 17, Output: "program.mp4",
		FullDemo: &FullDemoRenderEvidence{SchemaVersion: "1.0", Effective: document},
		fullDemo: &fullDemoRenderContext{ffmpeg: "ffmpeg", execution: FullDemoExecution{Assets: []FullDemoLocalMedia{{Ref: intro, Path: "intro.mp4"}, {Ref: outro, Path: "outro.mp4"}}}},
	}
	return short, intro, outro
}

func TestFullDemoBumperItemsUseTheirOwnClipAndAudioEvidence(t *testing.T) {
	short, _, _ := bumperShort(t)
	items := short.FullDemo.Effective.Timeline
	introCommand, err := fullDemoItemStreamCommand(short, items[0], "intro.nut", fullDemoItemMuxed)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(introCommand, " ")
	if !strings.Contains(joined, "-i intro.mp4") || !strings.Contains(joined, "[0:a]aresample=48000") || strings.Contains(joined, "anullsrc") {
		t.Fatalf("intro bumper must play its embedded audio:\n%s", joined)
	}
	if !strings.Contains(joined, "trim=start_frame=0:end_frame=180") || !strings.Contains(joined, "scale=1920:1080:force_original_aspect_ratio=decrease") {
		t.Fatalf("intro bumper must be letterboxed onto the program frame:\n%s", joined)
	}
	outroCommand, err := fullDemoItemStreamCommand(short, items[2], "outro.nut", fullDemoItemMuxed)
	if err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(outroCommand, " ")
	if !strings.Contains(joined, "-i outro.mp4") || !strings.Contains(joined, "anullsrc=r=48000:cl=stereo,atrim=end_sample=192000[a]") || strings.Contains(joined, "[0:a]") {
		t.Fatalf("silent outro bumper must synthesize silence instead of mapping a missing track:\n%s", joined)
	}
	if !strings.Contains(joined, "trim=start_frame=0:end_frame=240") {
		t.Fatalf("outro bumper frame window:\n%s", joined)
	}
	videoOnly, err := fullDemoItemStreamCommand(short, items[2], "outro.nut", fullDemoItemVideoOnly)
	if err != nil {
		t.Fatal(err)
	}
	if joined = strings.Join(videoOnly, " "); strings.Contains(joined, "anullsrc") || strings.Contains(joined, "-c:a") {
		t.Fatalf("video-only bumper item must not carry audio:\n%s", joined)
	}

	// A bumper item that the options no longer approve is an execution error,
	// not a silent fallback onto some other asset.
	stale := items[0]
	stale.SourceRef = uuid.NewString()
	if _, err := fullDemoItemStreamCommand(short, stale, "stale.nut", fullDemoItemMuxed); err == nil || !strings.Contains(err.Error(), "full_demo_asset_missing") {
		t.Fatalf("unapproved bumper error = %v", err)
	}
}

func TestFullDemoNeonOverlaysFollowTheGameplaySpanBetweenBumpers(t *testing.T) {
	short, _, _ := bumperShort(t)
	short.FullDemoIntroImagePath, short.FullDemoOutroImagePath = "intro.png", "outro.png"
	spanStart, spanEnd := fullDemoGameplaySpanSeconds(short)
	if spanStart != 3 || spanEnd != 13 {
		t.Fatalf("gameplay span = %.2f-%.2f, want 3-13", spanStart, spanEnd)
	}
	effects := generatedFullDemoOverlayEffects(short)
	if len(effects) != 2 {
		t.Fatalf("effects = %+v", effects)
	}
	wantIntroStart, wantIntroEnd, wantOutroStart, wantOutroEnd := demooverlay.OverlayWindows(10)
	if effects[0].StartSeconds != wantIntroStart+3 || effects[0].EndSeconds != wantIntroEnd+3 {
		t.Fatalf("neon intro = %.2f-%.2f, want it on the first gameplay seconds after the bumper", effects[0].StartSeconds, effects[0].EndSeconds)
	}
	if effects[1].StartSeconds != wantOutroStart+3 || effects[1].EndSeconds != wantOutroEnd+3 || effects[1].EndSeconds != 13 {
		t.Fatalf("neon outro = %.2f-%.2f, want it to end where the outro bumper starts", effects[1].StartSeconds, effects[1].EndSeconds)
	}

	// Without bumpers the windows are the whole program, byte for byte as before.
	plain := short
	plain.FullDemo = nil
	if start, end := fullDemoGameplaySpanSeconds(plain); start != 0 || end != 17 {
		t.Fatalf("plain span = %.2f-%.2f", start, end)
	}
}

func TestFullDemoOutroScoreboardWaitsASecondAfterTheLastKill(t *testing.T) {
	short, _, _ := bumperShort(t)
	short.FullDemoIntroImagePath, short.FullDemoOutroImagePath = "intro.png", "outro.png"
	outro := func(kills ...float64) []Effect {
		short.Kills = nil
		for _, at := range kills {
			short.Kills = append(short.Kills, KillCue{TimeSeconds: at})
		}
		var out []Effect
		for _, effect := range generatedFullDemoOverlayEffects(short) {
			if effect.Source == "full-demo-outro" {
				out = append(out, effect)
			}
		}
		return out
	}
	// Gameplay spans 3-13 s, so the default outro window is 8-13 s.
	for _, tt := range []struct {
		name       string
		kills      []float64
		wantStart  float64
		wantOutros int
	}{
		{"early kills keep the default window", []float64{4, 6.5}, 8, 1},
		{"a late kill delays the scoreboard", []float64{4, 10.5}, 11.5, 1},
		{"kills inside the outro bumper are ignored", []float64{6, 14}, 8, 1},
		{"no room after the last kill drops the scoreboard", []float64{12.5}, 0, 0},
	} {
		got := outro(tt.kills...)
		if len(got) != tt.wantOutros {
			t.Fatalf("%s: outro effects = %+v", tt.name, got)
		}
		if tt.wantOutros == 1 && (got[0].StartSeconds != tt.wantStart || got[0].EndSeconds != 13) {
			t.Fatalf("%s: outro = %.2f-%.2f, want %.2f-13", tt.name, got[0].StartSeconds, got[0].EndSeconds, tt.wantStart)
		}
	}
}

func TestFullDemoCoverNeverLandsInsideABumper(t *testing.T) {
	short, _, _ := bumperShort(t)
	sheet := strings.Join(BuildCoverSheetFFmpegCommand("ffmpeg", short), " ")
	if !strings.Contains(sheet, "select='not(between(n,0,179))'") || !strings.Contains(sheet, "select='not(between(n,780,1019))'") {
		t.Fatalf("cover sheet must skip both bumpers:\n%s", sheet)
	}
}

func TestFullDemoCoverFallbackSkipsAdjacentNonRoundItems(t *testing.T) {
	items := []recapplan.TimelineItem{
		{Role: "bumper", StartFrame: 0, EndFrame: 180},
		{Role: "bumper", StartFrame: 180, EndFrame: 420},
		{Role: "round", StartFrame: 420, EndFrame: 780},
		{Role: "bumper", StartFrame: 780, EndFrame: 1020},
		{Role: "bumper", StartFrame: 1020, EndFrame: 1260},
	}
	for i, want := range map[int]float64{0: 7, 1: 7, 3: 779.0 / 60, 4: 779.0 / 60} {
		if got := fullDemoCoverRoundFallback(items, i); got != want {
			t.Fatalf("fallback from item %d = %.4f, want %.4f", i, got, want)
		}
	}
}
