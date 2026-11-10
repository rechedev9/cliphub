package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

const libraryPatternMatchColumns = `SELECT pattern_id, match_id, team_key, revision, match_updated_at, played_at, detail FROM library_pattern_matches`

func scanPatternMatchRow(row sqlScanner) (library.PatternMatchRow, error) {
	var out library.PatternMatchRow
	var patternID, matchID, detail string
	var updated, played int64
	if err := row.Scan(&patternID, &matchID, &out.TeamKey, &out.Revision, &updated, &played, &detail); err != nil {
		return out, err
	}
	var err error
	if out.PatternID, err = uuid.Parse(patternID); err != nil {
		return out, err
	}
	if out.MatchID, err = uuid.Parse(matchID); err != nil {
		return out, err
	}
	if err := json.Unmarshal([]byte(detail), &out.Read); err != nil {
		return out, fmt.Errorf("decode pattern %s over match %s: %w", patternID, matchID, err)
	}
	out.MatchUpdatedAt, out.PlayedAt = fromMillis(updated), fromMillis(played)
	return out, nil
}

// SavePatternMatches replaces a pattern's rows over one match. A pattern or a match deleted
// meanwhile is ErrNotFound, and nothing is stored.
func (r *SQLiteLibraryRepository) SavePatternMatches(ctx context.Context, set library.PatternMatchSet) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM library_patterns WHERE id = ?) * (SELECT COUNT(*) FROM library_matches WHERE id = ?)`,
			set.PatternID.String(), set.MatchID.String()).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return library.ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM library_pattern_matches WHERE pattern_id = ? AND match_id = ?`,
			set.PatternID.String(), set.MatchID.String()); err != nil {
			return err
		}
		for _, row := range set.Rows {
			detail, err := json.Marshal(row.Read)
			if err != nil {
				return fmt.Errorf("encode pattern match: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO library_pattern_matches
				(pattern_id, match_id, team_key, revision, match_updated_at, played_at, matched, detail) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				set.PatternID.String(), set.MatchID.String(), row.TeamKey, row.Revision, millis(row.MatchUpdatedAt), millis(row.PlayedAt),
				row.Matched(), string(detail)); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListPatternMatches returns stored rows, newest match first.
func (r *SQLiteLibraryRepository) ListPatternMatches(ctx context.Context, filter library.PatternMatchFilter) ([]library.PatternMatchRow, error) {
	query := libraryPatternMatchColumns + ` WHERE 1 = 1`
	args := []any{}
	if filter.PatternID != nil {
		query += ` AND pattern_id = ?`
		args = append(args, filter.PatternID.String())
	}
	if filter.MatchID != nil {
		query += ` AND match_id = ?`
		args = append(args, filter.MatchID.String())
	}
	if filter.MatchedOnly {
		query += ` AND matched > 0`
	}
	query += ` ORDER BY played_at DESC, match_id ASC, team_key ASC, pattern_id ASC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list pattern matches: %w", err)
	}
	defer rows.Close()
	out := []library.PatternMatchRow{}
	for rows.Next() {
		row, err := scanPatternMatchRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
