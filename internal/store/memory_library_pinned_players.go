package store

import (
	"context"

	"github.com/rechedev9/cliphub/internal/library"
)

func (r *MemoryLibraryRepository) ListPinnedPlayers(ctx context.Context) ([]library.PinnedPlayer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]library.PinnedPlayer{}, r.libreta.pinnedPlayers...), nil
}

func (r *MemoryLibraryRepository) SetPinnedPlayers(ctx context.Context, players []library.PinnedPlayer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := map[string]bool{}
	kept := make([]library.PinnedPlayer, 0, len(players))
	for _, p := range players {
		if !seen[p.SteamID64] {
			seen[p.SteamID64] = true
			kept = append(kept, p)
		}
	}
	r.libreta.pinnedPlayers = kept
	return nil
}
