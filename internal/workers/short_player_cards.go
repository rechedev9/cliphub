package workers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/playercard"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// writeShortPlayerCards writes the cards a one-player 9:16 Short opens, closes and is covered with and
// returns their editor flags. A failed read renders the Short without them instead of failing it.
func (w *RenderWorker) writeShortPlayerCards(ctx context.Context, j job.Job, workDir string, edit renderplan.EditRequest) []string {
	if edit.OpeningCard == "" && edit.EndCard == "" && edit.CoverCard == "" {
		return nil
	}
	if edit.Format != renderplan.FormatShort9x16 || edit.FullDemo != nil || edit.MatchRecap || playercard.Unavailable(j) != "" {
		return nil
	}
	fail := func(err error) []string {
		logWorkerError(j.ID, tasks.TypeRenderVariant, fmt.Errorf("player cards left out: %w", err))
		return nil
	}
	facts, err := playercard.Gather(ctx, w.playerCardSources(), j.ID, playercard.Target(j))
	if err != nil {
		return fail(err)
	}
	req := playercard.Request{Opening: edit.OpeningCard, End: edit.EndCard, Cover: edit.CoverCard, Card: facts.Card}
	if len(facts.Image) > 0 {
		req.ImagePath = filepath.Join(workDir, "player-card-image")
		if err := os.WriteFile(req.ImagePath, facts.Image, 0o600); err != nil {
			return fail(err)
		}
	}
	path := filepath.Join(workDir, "player-cards.json")
	if err := writeJSONFile(path, req); err != nil {
		return fail(err)
	}
	return []string{"--player-cards", path}
}

func (w *RenderWorker) playerCardSources() playercard.Sources {
	src := playercard.Sources{Files: w.storage, Library: w.cfg.Library, Avatar: w.fetchAvatar}
	if w.cfg.Faceit != nil {
		src.FACEIT = w.cfg.Faceit
	}
	return src
}

// fetchAvatar downloads an approved FACEIT avatar URL, through the test downloader when one is set.
func (w *RenderWorker) fetchAvatar(ctx context.Context, raw string) ([]byte, error) {
	if w.cfg.AvatarFetch != nil {
		return w.cfg.AvatarFetch(ctx, raw)
	}
	return faceit.FetchAvatar(ctx, nil, raw)
}
