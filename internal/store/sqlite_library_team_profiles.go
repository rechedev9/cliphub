package store

import (
	"context"
	"database/sql"
	"errors"
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
		// One own team: marking this one hands the flag over.
		if p.Own {
			if _, err := tx.ExecContext(ctx, `UPDATE library_team_profiles SET own = 0, updated_at = ? WHERE own = 1 AND team_id <> ?`,
				millis(now), p.TeamID.String()); err != nil {
				return err
			}
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

// mergeTeamProfiles moves from's profile to into, which keeps its own rank when it
// has a profile; into is the own team after the merge when either one was.
func mergeTeamProfiles(ctx context.Context, tx *sql.Tx, from, into uuid.UUID) error {
	var own int
	err := tx.QueryRowContext(ctx, `SELECT own FROM library_team_profiles WHERE team_id = ?`, from.String()).Scan(&own)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("merge team profile: %w", err)
	}
	for _, stmt := range []string{
		`UPDATE library_team_profiles SET own = 0 WHERE team_id = ?1`,
		`UPDATE OR IGNORE library_team_profiles SET team_id = ?2 WHERE team_id = ?1`,
		`DELETE FROM library_team_profiles WHERE team_id = ?1`,
	} {
		if _, err := tx.ExecContext(ctx, stmt, from.String(), into.String()); err != nil {
			return fmt.Errorf("merge team profile: %w", err)
		}
	}
	if own == 0 {
		return nil
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO library_team_profiles (team_id, own, hltv_rank, updated_at) VALUES (?, 1, 0, ?)
		ON CONFLICT(team_id) DO UPDATE SET own = 1`, into.String(), millis(libraryNow()))
	if err != nil {
		return fmt.Errorf("merge team profile: %w", err)
	}
	return nil
}
