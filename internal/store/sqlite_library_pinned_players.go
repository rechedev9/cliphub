package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rechedev9/cliphub/internal/library"
)

func (r *SQLiteLibraryRepository) ListPinnedPlayers(ctx context.Context) ([]library.PinnedPlayer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT steamid64, name FROM library_pinned_players ORDER BY position`)
	if err != nil {
		return nil, fmt.Errorf("list pinned players: %w", err)
	}
	defer rows.Close()
	out := []library.PinnedPlayer{}
	for rows.Next() {
		var p library.PinnedPlayer
		if err := rows.Scan(&p.SteamID64, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) SetPinnedPlayers(ctx context.Context, players []library.PinnedPlayer) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM library_pinned_players`); err != nil {
			return err
		}
		for i, p := range players {
			if _, err := tx.ExecContext(ctx, `INSERT INTO library_pinned_players (steamid64, name, position) VALUES (?, ?, ?)`,
				p.SteamID64, p.Name, i); err != nil {
				return err
			}
		}
		return nil
	})
}
