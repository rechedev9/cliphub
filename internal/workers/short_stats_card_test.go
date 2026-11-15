package workers

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const statsCardTarget = "76561198000000001"

func statsCardRoster() parser.RosterResult {
	return parser.RosterResult{Players: []parser.PlayerStat{
		{SteamID64: "76561198000000002", Name: "other", Kills: 1},
		{SteamID64: statsCardTarget, Name: "ZywOo", Kills: 24, Deaths: 15, Assists: 5, Rounds: 20, ADR: 92.5, HSPct: 50, KAST: 75},
	}}
}

func TestShortStatsCardUsesClipHubRating(t *testing.T) {
	t.Parallel()
	card, ok := shortStatsCard(statsCardRoster(), statsCardTarget)
	if !ok || card.Player != "ZywOo" || card.Kills != 24 || card.Deaths != 15 {
		t.Fatalf("card = %+v, %v", card, ok)
	}
	want := tacticalplan.Rating(tacticalplan.RatingLine{Rounds: 20, Kills: 24, Deaths: 15, Assists: 5, Damage: 1850, KASTRounds: 15})
	if card.Rating == nil || *card.Rating != float64(int(want*100+0.5))/100 {
		t.Fatalf("rating = %v, want ClipHub's %.2f", card.Rating, want)
	}
	if card.ADR == nil || *card.ADR != 92.5 || card.HSPct == nil || *card.HSPct != 50 {
		t.Fatalf("rates = %v %v", card.ADR, card.HSPct)
	}
	if _, ok := shortStatsCard(statsCardRoster(), "76561198000000009"); ok {
		t.Fatal("a player outside the roster got a card")
	}
}

func TestWriteShortStatsCardAddsStoredFACEITData(t *testing.T) {
	t.Parallel()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	put := func(key string, value any) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Put(key, bytes.NewReader(raw)); err != nil {
			t.Fatal(err)
		}
	}
	put(artifacts.RosterKey(id), statsCardRoster())
	win := 60.0
	put(artifacts.FullDemoFaceitKey(id), map[string]demooverlay.Enrichment{statsCardTarget: {SkillLevel: 9, Last20: &demooverlay.Last20{WinPct: &win}, BestMap: "de_nuke"}})
	w := &RenderWorker{storage: store}
	edit := renderplan.EditRequest{Format: renderplan.FormatShort9x16, StatsCard: true}
	j := job.Job{ID: id, TargetSteamID: statsCardTarget}
	path := w.writeShortStatsCard(j, t.TempDir(), edit)
	if path == "" {
		t.Fatal("no stats card written")
	}
	card, err := editor.LoadStatsCard(path)
	if err != nil {
		t.Fatal(err)
	}
	if card.FACEIT == nil || card.FACEIT.Level != 9 || *card.FACEIT.WinPct != 60 || card.FACEIT.BestMap != "de_nuke" {
		t.Fatalf("FACEIT data = %+v", card.FACEIT)
	}
	edit.StatsCard = false
	if w.writeShortStatsCard(j, t.TempDir(), edit) != "" {
		t.Fatal("a card was written without stats_card")
	}
	j.ID = uuid.New()
	edit.StatsCard = true
	if w.writeShortStatsCard(j, filepath.Join(t.TempDir(), "x"), edit) != "" {
		t.Fatal("a job without a roster got a card")
	}
}

func TestStatsCardFACEITNeedsData(t *testing.T) {
	t.Parallel()
	if got := statsCardFACEIT(demooverlay.Enrichment{Nickname: "x"}); got != nil {
		t.Fatalf("empty FACEIT data = %+v, want nil", got)
	}
}

func TestShortOverlayArgs(t *testing.T) {
	t.Parallel()
	got := shortOverlayArgs(renderplan.EditRequest{VictimReplay: true, VictimReplaySpeed: 0.25, PlayBadges: true, WinProbability: true})
	want := []string{"--victim-replay", "--victim-replay-speed", "0.25", "--play-badges", "--win-probability"}
	if !slices.Equal(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
	if got := shortOverlayArgs(renderplan.EditRequest{VictimReplay: true}); !slices.Equal(got, []string{"--victim-replay"}) {
		t.Fatalf("default speed args = %q", got)
	}
}
