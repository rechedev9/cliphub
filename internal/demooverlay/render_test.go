package demooverlay

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestRenderPNGsFillsTenImagineSlotsWithDemoAndFACEITFacts(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH; overlay compositor cannot run on this host")
	}
	font, err := mediafont.Materialize()
	if err != nil {
		t.Fatalf("materialize font: %v", err)
	}
	i := func(v int) *int { return &v }
	f := func(v float64) *float64 { return &v }
	faceit := map[string]Enrichment{
		"1":  {Nickname: "mezii--", Country: "gb", ELO: 3625, SkillLevel: 10, Last20: &Last20{Matches: i(1577), WinPct: f(80), ADR: f(96.7), KD: f(1.66), KR: f(0.93)}},
		"2":  {Nickname: "ZywOo", Country: "fr", ELO: 3500, SkillLevel: 10, Last20: &Last20{Matches: i(4752), WinPct: f(70), ADR: f(106.3), KD: f(2.02), KR: f(1.04)}},
		"3":  {Nickname: "-ZubZ", Country: "se", ELO: 2715, SkillLevel: 10, Last20: &Last20{Matches: i(3611), WinPct: f(20), ADR: f(75.2), KD: f(0.99), KR: f(0.7)}},
		"4":  {Nickname: "KarlssonxP", Country: "se", ELO: 2701, SkillLevel: 10, Last20: &Last20{Matches: i(4920), WinPct: f(45), ADR: f(78.2), KD: f(1.02), KR: f(0.7)}},
		"5":  {Nickname: "ZwooshY", Country: "fi", ELO: 2818, SkillLevel: 10, Last20: &Last20{Matches: i(2551), WinPct: f(35), ADR: f(83.5), KD: f(1.22), KR: f(0.8)}},
		"6":  {Nickname: "doghamster", Country: "se", ELO: 2908, SkillLevel: 10, Last20: &Last20{Matches: i(6106), WinPct: f(65), ADR: f(83.9), KD: f(0.97), KR: f(0.73)}},
		"7":  {Nickname: "DJTruecel", Country: "dk", ELO: 3242, SkillLevel: 10, Last20: &Last20{Matches: i(3822), WinPct: f(60), ADR: f(75.7), KD: f(1.01), KR: f(0.67)}},
		"8":  {Nickname: "sil2nthill", Country: "at", ELO: 3057, SkillLevel: 10, Last20: &Last20{Matches: i(3360), WinPct: f(60), ADR: f(82.0), KD: f(1.18), KR: f(0.8)}},
		"9":  {Nickname: "bananek147", Country: "pl", ELO: 2752, SkillLevel: 10, Last20: &Last20{Matches: i(5167), WinPct: f(60), ADR: f(71.0), KD: f(1.03), KR: f(0.66)}},
		"10": {Nickname: "Sil3nT189", Country: "es", ELO: 2860, SkillLevel: 10, Last20: &Last20{Matches: i(3380), WinPct: f(40), ADR: f(84.0), KD: f(1.12), KR: f(0.78)}},
	}
	doc := Build(Roster{
		TargetSteamID64: "2",
		Map:             "de_mirage",
		ScoreCT:         13,
		ScoreT:          8,
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "mezii--", Team: "CT", Kills: 18, Deaths: 12},
			{SteamID64: "2", Name: "ZywOo", Team: "CT", Kills: 23, Deaths: 10},
			{SteamID64: "3", Name: "-ZubZ", Team: "CT", Kills: 11, Deaths: 16},
			{SteamID64: "4", Name: "KarlssonxP", Team: "CT", Kills: 14, Deaths: 15},
			{SteamID64: "5", Name: "ZwooshY", Team: "CT", Kills: 9, Deaths: 17},
			{SteamID64: "6", Name: "doghamster", Team: "T", Kills: 16, Deaths: 14},
			{SteamID64: "7", Name: "DJTruecel", Team: "T", Kills: 12, Deaths: 15},
			{SteamID64: "8", Name: "sil2nthill", Team: "T", Kills: 15, Deaths: 13},
			{SteamID64: "9", Name: "bananek147", Team: "T", Kills: 10, Deaths: 16},
			{SteamID64: "10", Name: "Sil3nT189", Team: "T", Kills: 13, Deaths: 14},
		},
	}, faceit)
	dir := t.TempDir()
	intro := filepath.Join(dir, "full-demo-intro.png")
	outro := filepath.Join(dir, "full-demo-outro.png")
	if err := RenderPNGs(ffmpeg, font, doc, intro, outro); err != nil {
		t.Fatalf("RenderPNGs: %v", err)
	}
	raw, err := os.ReadFile(intro)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("intro is not a PNG")
	}
	got := introFilter(doc, font)
	for _, name := range []string{"mezii--", "ZywOo", "doghamster", "Sil3nT189"} {
		if !strings.Contains(got, name) {
			t.Fatalf("intro filter missing %q", name)
		}
	}
	if dest := os.Getenv("FULL_DEMO_OVERLAY_OUT"); dest != "" {
		if err := os.MkdirAll(dest, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dest, "intro-filled.png"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
