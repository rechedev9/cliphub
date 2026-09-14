package workers

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

func TestWriteFullDemoOverlayUsesLocalSteamSnapshotWithoutFACEITFacts(t *testing.T) {
	const (
		publicID  = "76561198000000001"
		privateID = "76561198000000002"
		missingID = "76561198000000003"
	)
	store := newFakeStorage()
	id := uuid.New()
	putJSON(t, store, artifacts.RosterKey(id), parser.RosterResult{
		Players: []parser.PlayerStat{
			{SteamID64: publicID, Name: "demo-public", Team: "CT", Kills: 21, Deaths: 12, Assists: 4, Rounds: 20, ADR: 98.1, HSPct: 55},
			{SteamID64: privateID, Name: "demo-private", Team: "T", Kills: 14, Deaths: 18, Assists: 6, Rounds: 20, ADR: 71.4, HSPct: 43},
			{SteamID64: missingID, Name: "demo-missing", Team: "T", Kills: 12, Deaths: 19, Assists: 3, Rounds: 20},
		},
		Match: parser.MatchInfo{Map: "de_mirage", ScoreCT: 13, ScoreT: 7},
	})
	putJSON(t, store, artifacts.FullDemoSteamAvatarsKey(id), map[string]faceit.SteamAvatar{
		publicID:  {URL: "https://avatars.akamai.steamstatic.com/public.jpg"},
		privateID: {URL: "https://avatars.akamai.steamstatic.com/private.jpg", Private: true},
	})
	plan := minimalKillPlan()
	j := job.Job{ID: id, TargetSteamID: publicID, KillPlan: &plan, Rules: rules.Default()}
	var downloads []string
	w := NewRenderWorker(newFakeRepo(), store, RenderWorkerConfig{
		WorkDir: t.TempDir(),
		AvatarFetch: func(_ context.Context, raw string) ([]byte, error) {
			downloads = append(downloads, raw)
			return []byte("fake portrait"), nil
		},
	})
	edit := renderplan.RecapEditRequest()
	edit.OverlayTheme = renderplan.OverlayThemeNeonViolet
	path, err := w.writeFullDemoOverlay(j, t.TempDir(), "gameplay-pov-60", edit)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc demooverlay.Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	public := findOverlayCard(t, doc, publicID)
	if public.AvatarURL == "" || public.AvatarFile == "" || public.Name != "demo-public" || public.ELO != nil || public.Last20 != nil {
		t.Fatalf("local public card is not demo-only with a portrait: %+v", public)
	}
	private := findOverlayCard(t, doc, privateID)
	if private.AvatarURL != "" || private.AvatarFile != "" || private.ELO != nil || private.Last20 != nil {
		t.Fatalf("private local card leaked a profile avatar or FACEIT facts: %+v", private)
	}
	missing := findOverlayCard(t, doc, missingID)
	if missing.AvatarURL != "" || missing.AvatarFile != "" {
		t.Fatalf("missing local card invented an avatar: %+v", missing)
	}
	if len(downloads) != 1 || downloads[0] != public.AvatarURL {
		t.Fatalf("avatar downloads = %#v", downloads)
	}
	html, err := demooverlay.NeonIntroHTML(doc, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), "This match") || strings.Contains(string(html), "Last 20 Matches") || strings.Contains(string(html), ">Overall<") {
		t.Fatalf("local neon intro includes FACEIT-only content")
	}
}

func TestStoredLocalOverlayAvatarURLsIgnoresMalformedAndPrivateSnapshots(t *testing.T) {
	t.Parallel()
	store := newFakeStorage()
	id := uuid.New()
	putJSON(t, store, artifacts.FullDemoSteamAvatarsKey(id), map[string]faceit.SteamAvatar{
		"public":  {URL: "https://avatars.akamai.steamstatic.com/public.jpg"},
		"private": {URL: "https://avatars.akamai.steamstatic.com/private.jpg", Private: true},
		"http":    {URL: "http://avatars.akamai.steamstatic.com/http.jpg"},
	})
	w := NewRenderWorker(newFakeRepo(), store, RenderWorkerConfig{})
	got := storedLocalOverlayAvatarURLs(w, id)
	if len(got) != 1 || got["public"] == "" {
		t.Fatalf("stored local URLs = %#v", got)
	}
	store.files[artifacts.FullDemoSteamAvatarsKey(id)] = []byte("not-json")
	if got := storedLocalOverlayAvatarURLs(w, id); got != nil {
		t.Fatalf("malformed snapshot = %#v", got)
	}
}

func findOverlayCard(t *testing.T, doc demooverlay.Document, steamID string) demooverlay.PlayerCard {
	t.Helper()
	for _, side := range [][]demooverlay.PlayerCard{doc.Intro.Left, doc.Intro.Right} {
		for _, card := range side {
			if card.SteamID64 == steamID {
				return card
			}
		}
	}
	t.Fatalf("missing overlay card %s", steamID)
	return demooverlay.PlayerCard{}
}
