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
	if introFx.Type != EffectImage || introFx.StartSeconds != 0 || introFx.EndSeconds != 8 {
		t.Fatalf("intro effect = %#v", introFx)
	}
	if outroFx.Type != EffectImage || outroFx.EndSeconds != short.DurationSeconds {
		t.Fatalf("outro effect = %#v", outroFx)
	}
	command := strings.Join(short.FFmpegCommand, " ")
	if !strings.Contains(command, intro) || !strings.Contains(command, outro) {
		t.Fatalf("ffmpeg missing overlay stills:\n%s", command)
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
		for _, effect := range short.Effects {
			if effect.Source == "full-demo-intro" || effect.Source == "full-demo-outro" {
				t.Fatalf("shorts gained a Full Demo overlay: %#v", effect)
			}
		}
	}
}
