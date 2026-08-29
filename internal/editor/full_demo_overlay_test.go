package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/demooverlay"
)

func TestBuildManifestFullDemoAttachesIntroAndOutroOverlays(t *testing.T) {
	dir := t.TempDir()
	result := testRecordingResult(dir)
	result.Plan.Segments[0].TickEnd = result.Plan.Segments[0].TickStart + 64*40
	result.Plan.Segments[1].TickEnd = result.Plan.Segments[1].TickStart + 64*40
	for i := range result.Artifacts {
		result.Artifacts[i].DurationSeconds = 40
	}
	intro := filepath.Join(dir, "shorts", "full-demo-intro.png")
	outro := filepath.Join(dir, "shorts", "full-demo-outro.png")
	if err := os.MkdirAll(filepath.Dir(intro), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(intro, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outro, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	doc := demooverlay.Build(demooverlay.Roster{
		TargetSteamID64: "76561198148986856",
		Map:             "de_mirage",
		ScoreCT:         13,
		ScoreT:          8,
		Players: []demooverlay.RosterPlayer{
			{SteamID64: "76561198148986856", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14, Assists: 4, ADR: 101.6, Rating: 1.35},
			{SteamID64: "9", Name: "KingwayO", Team: "T", Kills: 18, Deaths: 16},
		},
	}, nil)

	opts := testManifestOptions(dir, nil)
	opts.Preset = PresetGameplayPOV60
	opts.OutputFormat = OutputFormatLandscape16x9
	opts.CompileSegments = true
	opts.KillEffect = KillEffectClean
	opts.Transition = TransitionCut
	opts.FullDemoOverlay = &doc
	opts.FullDemoIntroImagePath = intro
	opts.FullDemoOutroImagePath = outro

	manifest := mustBuildManifest(t, result, opts)
	if len(manifest.Shorts) != 1 {
		t.Fatalf("shorts = %d", len(manifest.Shorts))
	}
	short := manifest.Shorts[0]
	if short.Title != "donk666 (23-14) Mirage | CS2 DEMO POV + VOICECOMMS" {
		t.Fatalf("title = %q", short.Title)
	}
	if strings.Contains(short.Title, "TOP #1") {
		t.Fatal("title invented FACEIT rank")
	}
	var introFx, outroFx *Effect
	for i := range short.Effects {
		switch short.Effects[i].Source {
		case "full-demo-intro":
			introFx = &short.Effects[i]
		case "full-demo-outro":
			outroFx = &short.Effects[i]
		}
	}
	if introFx == nil || outroFx == nil {
		t.Fatalf("effects = %#v, want intro and outro image overlays", short.Effects)
	}
	wantIntroStart, wantIntroEnd, wantOutroStart, wantOutroEnd := demooverlay.OverlayWindows(short.DurationSeconds)
	if introFx.Type != EffectImage || introFx.StartSeconds != wantIntroStart || introFx.EndSeconds != wantIntroEnd {
		t.Fatalf("intro effect = %#v, want %.1f-%.1f (after fade, before live)", introFx, wantIntroStart, wantIntroEnd)
	}
	if introFx.FadeInSeconds != demooverlay.IntroOverlaySlideSeconds {
		t.Fatalf("intro slide-in fade = %.2f, want %.2f", introFx.FadeInSeconds, demooverlay.IntroOverlaySlideSeconds)
	}
	if outroFx.Type != EffectImage || outroFx.StartSeconds != wantOutroStart || outroFx.EndSeconds != wantOutroEnd {
		t.Fatalf("outro effect = %#v, want %.1f-%.1f", outroFx, wantOutroStart, wantOutroEnd)
	}
	if wantIntroStart < demooverlay.FadeFromBlackSeconds+demooverlay.IntroOverlayAfterFadeSeconds-0.01 {
		t.Fatal("roster must wait until ~4s after the fade")
	}
	if wantIntroEnd >= float64(demooverlay.IntroFreezeSeconds) {
		t.Fatal("roster must leave before live action")
	}
	command := strings.Join(short.FFmpegCommand, " ")
	if !strings.Contains(command, intro) || !strings.Contains(command, outro) {
		t.Fatalf("ffmpeg missing overlay stills:\n%s", command)
	}
	if !strings.Contains(command, "fade=t=in:st=0") {
		t.Fatalf("ffmpeg missing fade from black:\n%s", command)
	}
	if short.MusicPath != "" {
		t.Fatalf("full demo mixed a music bed: %q", short.MusicPath)
	}
	for _, effect := range short.Effects {
		if effect.Type == EffectZoom {
			t.Fatalf("full demo gained punch-in zoom: %#v", effect)
		}
		if strings.Contains(strings.ToLower(effect.Value), "subscribe") || strings.Contains(strings.ToLower(effect.Value), "suscríb") {
			t.Fatalf("full demo gained a subscribe CTA: %#v", effect)
		}
		if effect.Source == "edit-request" && (effect.Type == EffectText) {
			t.Fatalf("full demo gained a Shorts bookend/hook: %#v", effect)
		}
	}
}

func TestBuildManifestShortsPathIgnoresFullDemoOverlay(t *testing.T) {
	dir := t.TempDir()
	result := testRecordingResult(dir)
	doc := demooverlay.Build(demooverlay.Roster{
		TargetSteamID64: "76561198148986856",
		Players:         []demooverlay.RosterPlayer{{SteamID64: "76561198148986856", Name: "donk666", Kills: 23, Deaths: 14}},
	}, nil)
	opts := testManifestOptions(dir, nil)
	opts.FullDemoOverlay = &doc
	opts.FullDemoIntroImagePath = filepath.Join(dir, "intro.png")
	opts.FullDemoOutroImagePath = filepath.Join(dir, "outro.png")

	manifest := mustBuildManifest(t, result, opts)
	if len(manifest.Shorts) == 0 {
		t.Fatal("no shorts")
	}
	for _, short := range manifest.Shorts {
		if strings.Contains(short.Title, "VOICECOMMS") {
			t.Fatalf("shorts title used Full Demo copy: %q", short.Title)
		}
		if short.OutputFormat != OutputFormatShort9x16 {
			t.Fatalf("shorts format = %q, want %q", short.OutputFormat, OutputFormatShort9x16)
		}
		command := strings.Join(short.FFmpegCommand, " ")
		if !strings.Contains(command, "crop=1080:1920") {
			t.Fatalf("shorts lost portrait crop:\n%s", command)
		}
		if strings.Contains(command, "fade=t=in:st=0") {
			t.Fatalf("shorts gained Full Demo fade-from-black:\n%s", command)
		}
		for _, effect := range short.Effects {
			if effect.Source == "full-demo-intro" || effect.Source == "full-demo-outro" {
				t.Fatalf("shorts gained a Full Demo overlay: %#v", effect)
			}
		}
	}
}
