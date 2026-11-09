package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func (r *SQLiteLibraryRepository) SetTeamProfile(ctx context.Context, p *library.TeamProfile) error {
	now := libraryNow()
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		if err := r.teamExists(ctx, tx, p.TeamID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO library_team_profiles (team_id, own, hltv_rank, updated_at) VALUES (?, ?, ?, ?)
			ON CONFLICT(team_id) DO UPDATE SET own = excluded.own, hltv_rank = excluded.hltv_rank, updated_at = excluded.updated_at`,
			p.TeamID.String(), boolInt(p.Own), p.HLTVRank, millis(now))
		return err
	})
	if err != nil {
		return err
	}
	p.UpdatedAt = fromMillis(millis(now))
	return nil
}

func (r *SQLiteLibraryRepository) ListTeamProfiles(ctx context.Context) ([]library.TeamProfile, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT team_id, own, hltv_rank, updated_at FROM library_team_profiles ORDER BY team_id`)
	if err != nil {
		return nil, fmt.Errorf("list team profiles: %w", err)
	}
	defer rows.Close()
	out := []library.TeamProfile{}
	for rows.Next() {
		var p library.TeamProfile
		var teamID string
		var own int
		var updated int64
		if err := rows.Scan(&teamID, &own, &p.HLTVRank, &updated); err != nil {
			return nil, err
		}
		if p.TeamID, err = uuid.Parse(teamID); err != nil {
			return nil, err
		}
		p.Own, p.UpdatedAt = own != 0, fromMillis(updated)
		out = append(out, p)
	}
	return out, rows.Err()
}
