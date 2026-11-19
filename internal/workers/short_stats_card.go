package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/playercard"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/tasks"
)

const statsCardFACEITTimeout = 10 * time.Second

// writeShortStatsCard writes the closing card of a 9:16 Short and returns its path. A missing roster
// renders the Short without the card instead of failing it; a multi-POV Short has no single player.
func (w *RenderWorker) writeShortStatsCard(j job.Job, workDir string, edit renderplan.EditRequest) string {
	if !edit.StatsCard || edit.Format != renderplan.FormatShort9x16 || edit.FullDemo != nil || j.IsMultiPOV() {
		return ""
	}
	target := playercard.Target(j)
	roster, err := w.readRoster(j.ID)
	if err != nil {
		logWorkerError(j.ID, tasks.TypeRenderVariant, fmt.Errorf("stats card left out: %w", err))
		return ""
	}
	card, ok := shortStatsCard(roster, target)
	if !ok {
		logWorkerError(j.ID, tasks.TypeRenderVariant, fmt.Errorf("stats card left out: player %s is not in the roster", target))
		return ""
	}
	if en, found := w.statsCardEnrichment(j.ID, target); found {
		card.FACEIT = statsCardFACEIT(en)
	}
	path := filepath.Join(workDir, "stats-card.json")
	if err := writeJSONFile(path, card); err != nil {
		logWorkerError(j.ID, tasks.TypeRenderVariant, fmt.Errorf("stats card left out: %w", err))
		return ""
	}
	return path
}

func (w *RenderWorker) readRoster(id uuid.UUID) (parser.RosterResult, error) {
	var roster parser.RosterResult
	rc, err := w.storage.Open(artifacts.RosterKey(id))
	if err != nil {
		return roster, fmt.Errorf("open roster: %w", err)
	}
	defer rc.Close()
	if err := json.NewDecoder(rc).Decode(&roster); err != nil {
		return roster, fmt.Errorf("decode roster: %w", err)
	}
	return roster, nil
}

// shortStatsCard is the player's match line from the roster scan, with ClipHub's own rating.
func shortStatsCard(roster parser.RosterResult, steamID string) (editor.StatsCard, bool) {
	card, ok := playercard.RosterCard(roster, steamID)
	if !ok {
		return editor.StatsCard{}, false
	}
	return editor.StatsCard{Player: card.Name, Kills: card.Kills, Deaths: card.Deaths, HSPct: card.HSPct, ADR: card.ADR, Rating: card.Rating}, true
}

// statsCardEnrichment prefers the Full Demo FACEIT snapshot; without one it asks FACEIT for the
// player alone. Any failure leaves the card without FACEIT data.
func (w *RenderWorker) statsCardEnrichment(id uuid.UUID, steamID string) (demooverlay.Enrichment, bool) {
	if stored, found, err := storedOverlayEnrichment(w, id); err == nil && found {
		en, ok := stored[steamID]
		return en, ok
	}
	if w.cfg.Faceit == nil {
		return demooverlay.Enrichment{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), statsCardFACEITTimeout)
	defer cancel()
	players, _ := w.cfg.Faceit.OverlayPlayers(ctx, []string{steamID})
	player, ok := players[steamID]
	if !ok {
		return demooverlay.Enrichment{}, false
	}
	return demooverlay.Enrichment{SkillLevel: player.SkillLevel, Last20: last20FromFACEIT(player.Recent), BestMap: player.BestMap}, true
}

// statsCardFACEIT keeps the level, the recent win rate and the best map; nil when FACEIT has none of them.
func statsCardFACEIT(en demooverlay.Enrichment) *editor.StatsCardFACEIT {
	out := editor.StatsCardFACEIT{Level: max(en.SkillLevel, 0), BestMap: en.BestMap}
	if en.Last20 != nil && en.Last20.WinPct != nil {
		win := *en.Last20.WinPct
		out.WinPct = &win
	}
	if out.Level == 0 && out.WinPct == nil && out.BestMap == "" {
		return nil
	}
	return &out
}
