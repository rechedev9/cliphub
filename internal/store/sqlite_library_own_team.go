package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// touchTeam ends a team patch: it applies the Mi equipo choice (true moves the mark to the team,
// false clears it, nil leaves it) and stamps the update time.
func touchTeam(ctx context.Context, tx *sql.Tx, id uuid.UUID, own *bool) error {
	if own != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE library_teams SET is_own = 0 WHERE is_own = 1`); err != nil {
			return fmt.Errorf("clear own team: %w", err)
		}
		if *own {
			if _, err := tx.ExecContext(ctx, `UPDATE library_teams SET is_own = 1 WHERE id = ?`, id.String()); err != nil {
				return fmt.Errorf("set own team: %w", err)
			}
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE library_teams SET updated_at = ? WHERE id = ?`, millis(libraryNow()), id.String())
	return err
}
