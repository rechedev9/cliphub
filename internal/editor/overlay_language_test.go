package editor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/overlaylang"
)

var spanishOverlayWords = []string{"GANADO", "ESTADÍSTICAS", "PARTIDA", "RATING DE", "NIVEL", "VICTORIA", "MEJOR MAPA", "PROB.", "RONDA", "COMPRAS", "BOMBA", "DESACTIVANDO", " CON ", " EN "}

func writeStatsCard(t *testing.T, dir string) string {
	t.Helper()
	adr, hs, rating, win := 92.44, 54.2, 1.18, 58.0
	body, err := json.Marshal(StatsCard{Player: "ZywOo", Kills: 24, Deaths: 15, ADR: &adr, HSPct: &hs, Rating: &rating,
		FACEIT: &StatsCardFACEIT{Level: 8, WinPct: &win, BestMap: "de_mirage"}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "stats-card.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func overlayLanguageRun(t *testing.T, language overlaylang.Language) (texts []string, scripts string) {
	t.Helper()
	dir := t.TempDir()
	fixture := verticalFixture(dir)
	fixture.Plan.Segments[1].Kills[0].ClutchVs, fixture.Plan.Segments[1].Kills[0].ClutchResult = 1, killplan.ClutchWon
	fixture.Plan.Segments[0].WinProb = []killplan.WinProbSpan{{TickStart: 14300, TickEnd: 14700, CTPct: 64}}
	result, err := verticalRun(t, dir, Config{
		RecordingResultPath:  writeRecordingResult(t, dir, fixture),
		VerticalHUDTrackPath: writeVerticalTrack(t, dir, fixture.Plan),
		StatsCardPath:        writeStatsCard(t, dir),
		PlayBadges:           true,
		WinProbability:       true,
		HookText:             true,
		OverlayLanguage:      language,
	})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	short := result.Shorts[0]
	for _, effect := range short.Effects {
		if effect.Type == EffectText {
			texts = append(texts, effect.Value)
		}
	}
	for _, path := range []string{short.Output + ".vertical-hud.ass", short.Output + ".killfeed.ass"} {
		script, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", filepath.Base(path), err)
		}
		scripts += string(script)
	}
	return texts, scripts
}

func TestEnglishShortBurnsNoSpanishText(t *testing.T) {
	t.Parallel()
	texts, scripts := overlayLanguageRun(t, overlaylang.English)
	for _, want := range []string{"CLUTCH 1V1 WON", "MATCH STATS", "CLIPHUB RATING 1.18", "FACEIT LEVEL 8 · 58% WIN RATE", "BEST MAP: MIRAGE", "ADR 92.4 · HS 54%", "WIN PROBABILITY", "CT 64%", "36% T"} {
		if !slices.Contains(texts, want) {
			t.Errorf("the English Short does not draw %q; it draws %q", want, texts)
		}
	}
	englishHook := func(text string) bool {
		return strings.HasPrefix(text, "3K WITH ") && strings.HasSuffix(text, " ON ANCIENT")
	}
	if !slices.ContainsFunc(texts, englishHook) {
		t.Errorf("the English hook title is missing: %q", texts)
	}
	if !strings.Contains(scripts, "}ROUND 17") {
		t.Error("the English vertical HUD does not name the round")
	}
	for _, text := range append(texts, scripts) {
		upper := strings.ToUpper(text)
		for _, word := range spanishOverlayWords {
			if strings.Contains(upper, word) {
				t.Errorf("the English Short still burns %q in %q", word, text)
			}
		}
	}
}

func TestSpanishStaysTheDefaultOverlayLanguage(t *testing.T) {
	t.Parallel()
	texts, scripts := overlayLanguageRun(t, overlaylang.Spanish)
	for _, want := range []string{"CLUTCH 1V1 GANADO", "ESTADÍSTICAS DE LA PARTIDA", "RATING DE CLIPHUB 1.18", "PROB. DE VICTORIA", "CT 64 %"} {
		if !slices.Contains(texts, want) {
			t.Errorf("the Spanish Short does not draw %q; it draws %q", want, texts)
		}
	}
	if !strings.Contains(scripts, "}RONDA 17") {
		t.Error("the Spanish vertical HUD lost its round label")
	}
}
