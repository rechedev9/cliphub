package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func (r *SQLiteLibraryRepository) GetTeamRoles(ctx context.Context, teamID uuid.UUID, mapName string) (library.TeamRoleSheet, error) {
	if err := r.teamExists(ctx, r.db, teamID); err != nil {
		return library.TeamRoleSheet{}, err
	}
	sheet := library.TeamRoleSheet{TeamID: teamID, Map: mapName}
	var t, ct string
	var updated int64
	err := r.db.QueryRowContext(ctx, `SELECT t, ct, updated_at FROM library_team_roles WHERE team_id = ? AND map = ?`,
		teamID.String(), mapName).Scan(&t, &ct, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return teamRolesCopy(sheet), nil
	}
	if err != nil {
		return library.TeamRoleSheet{}, fmt.Errorf("get team roles: %w", err)
	}
	if err := json.Unmarshal([]byte(t), &sheet.T); err != nil {
		return library.TeamRoleSheet{}, fmt.Errorf("decode team roles: %w", err)
	}
	if err := json.Unmarshal([]byte(ct), &sheet.CT); err != nil {
		return library.TeamRoleSheet{}, fmt.Errorf("decode team roles: %w", err)
	}
	sheet.UpdatedAt = fromMillis(updated)
	return teamRolesCopy(sheet), nil
}

func (r *SQLiteLibraryRepository) SetTeamRoles(ctx context.Context, sheet *library.TeamRoleSheet) error {
	stored := teamRolesCopy(*sheet)
	t, err := json.Marshal(stored.T)
	if err != nil {
		return fmt.Errorf("encode team roles: %w", err)
	}
	ct, err := json.Marshal(stored.CT)
	if err != nil {
		return fmt.Errorf("encode team roles: %w", err)
	}
	now := libraryNow()
	err = r.inTx(ctx, func(tx *sql.Tx) error {
		if err := r.teamExists(ctx, tx, stored.TeamID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO library_team_roles (team_id, map, t, ct, updated_at) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(team_id, map) DO UPDATE SET t = excluded.t, ct = excluded.ct, updated_at = excluded.updated_at`,
			stored.TeamID.String(), stored.Map, string(t), string(ct), millis(now))
		return err
	})
	if err != nil {
		return err
	}
	stored.UpdatedAt = fromMillis(millis(now))
	*sheet = stored
	return nil
}
