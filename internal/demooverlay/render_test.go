package demooverlay

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/mediafont"
)

func TestRenderPNGsWritesIntroAndOutroStills(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH; overlay compositor cannot run on this host")
	}
	font, err := mediafont.Materialize()
	if err != nil {
		t.Fatalf("materialize font: %v", err)
	}
	doc := Build(Roster{
		TargetSteamID64: "76561198148986856",
		Map:             "de_mirage",
		ScoreCT:         13,
		ScoreT:          8,
		Players: []RosterPlayer{
			{SteamID64: "76561198148986856", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14, Assists: 4, ADR: 101.6, Rating: 1.35},
			{SteamID64: "9", Name: "KingwayO", Team: "T", Kills: 18, Deaths: 16},
		},
	}, nil)
	dir := t.TempDir()
	intro := filepath.Join(dir, "full-demo-intro.png")
	outro := filepath.Join(dir, "full-demo-outro.png")
	if err := RenderPNGs(ffmpeg, font, doc, intro, outro); err != nil {
		t.Fatalf("RenderPNGs: %v", err)
	}
	for _, path := range []string{intro, outro} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
			t.Fatalf("%s is not a PNG (%d bytes)", path, len(raw))
		}
	}
	if dest := os.Getenv("FULL_DEMO_OVERLAY_OUT"); dest != "" {
		if err := os.MkdirAll(dest, 0o750); err != nil {
			t.Fatalf("evidence dir: %v", err)
		}
		for src, name := range map[string]string{intro: "intro-roster.png", outro: "outro-scoreboard.png"} {
			raw, err := os.ReadFile(src)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dest, name), raw, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
