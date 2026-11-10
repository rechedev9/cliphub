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

const libraryPatternColumns = `SELECT id, name, map, team_id, category, play_id, detail, created_at, updated_at FROM library_patterns`

// patternDetail is the JSON column: the blocks and the scope, which no query filters on.
type patternDetail struct {
	Pattern json.RawMessage      `json:"pattern"`
	Scope   library.PatternScope `json:"scope"`
}

func scanLibraryPattern(row sqlScanner) (library.Pattern, error) {
	var p library.Pattern
	var id, detail string
	var team, play sql.NullString
	var created, updated int64
	if err := row.Scan(&id, &p.Name, &p.Map, &team, &p.Category, &play, &detail, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Pattern{}, library.ErrNotFound
		}
		return library.Pattern{}, err
	}
	var err error
	if p.ID, err = uuid.Parse(id); err != nil {
		return library.Pattern{}, err
	}
	if team.Valid {
		teamID, err := uuid.Parse(team.String)
		if err != nil {
			return library.Pattern{}, err
		}
		p.TeamID = &teamID
	}
	if play.Valid {
		playID, err := uuid.Parse(play.String)
		if err != nil {
			return library.Pattern{}, err
		}
		p.PlayID = &playID
	}
	var d patternDetail
	if err := json.Unmarshal([]byte(detail), &d); err != nil {
		return library.Pattern{}, fmt.Errorf("decode pattern %s: %w", id, err)
	}
	if err := json.Unmarshal(d.Pattern, &p.Pattern); err != nil {
		return library.Pattern{}, fmt.Errorf("decode pattern %s blocks: %w", id, err)
	}
	p.Scope = d.Scope
	p.CreatedAt, p.UpdatedAt = fromMillis(created), fromMillis(updated)
	return library.PatternCopy(p), nil
}

// patternRow is a pattern's columns as SQLite stores them.
type patternRow struct {
	detail     string
	team, play sql.NullString
}

func nullUUID(id *uuid.UUID) sql.NullString {
	if id == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: id.String(), Valid: true}
}

func encodePatternDetail(p library.Pattern) (patternRow, error) {
	blocks, err := json.Marshal(p.Pattern)
	if err != nil {
		return patternRow{}, fmt.Errorf("encode pattern blocks: %w", err)
	}
	detail, err := json.Marshal(patternDetail{Pattern: blocks, Scope: p.Scope})
	if err != nil {
		return patternRow{}, fmt.Errorf("encode pattern: %w", err)
	}
	return patternRow{detail: string(detail), team: nullUUID(p.TeamID), play: nullUUID(p.PlayID)}, nil
}

func (r *SQLiteLibraryRepository) CreatePattern(ctx context.Context, p *library.Pattern) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	row, err := encodePatternDetail(*p)
	if err != nil {
		return err
	}
	now := millis(libraryNow())
	err = r.inTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_patterns WHERE id = ?`, p.ID.String()).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return library.ErrConflict
		}
		if err := checkPatternPlay(ctx, tx, p.PlayID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO library_patterns (id, name, map, team_id, category, play_id, detail, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, p.ID.String(), p.Name, p.Map, row.team, p.Category, row.play, row.detail, now, now)
		return err
	})
	if err != nil {
		return err
	}
	got, err := r.GetPattern(ctx, p.ID)
	if err != nil {
		return err
	}
	*p = got
	return nil
}

func (r *SQLiteLibraryRepository) GetPattern(ctx context.Context, id uuid.UUID) (library.Pattern, error) {
	return scanLibraryPattern(r.db.QueryRowContext(ctx, libraryPatternColumns+` WHERE id = ?`, id.String()))
}

func (r *SQLiteLibraryRepository) ListPatterns(ctx context.Context, filter library.PatternFilter) ([]library.Pattern, error) {
	query := libraryPatternColumns + ` WHERE 1 = 1`
	args := []any{}
	if filter.Map != "" {
		query += ` AND map = ?`
		args = append(args, filter.Map)
	}
	if filter.TeamID != nil {
		query += ` AND (team_id = ? OR team_id IS NULL)`
		args = append(args, filter.TeamID.String())
	}
	if filter.Category != "" {
		query += ` AND category = ?`
		args = append(args, filter.Category)
	}
	if filter.PlayID != nil {
		query += ` AND play_id = ?`
		args = append(args, filter.PlayID.String())
	}
	query += ` ORDER BY ` + sqlLower + `(name) ASC, id ASC LIMIT ? OFFSET ?`
	args = append(args, clampPatternLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list patterns: %w", err)
	}
	defer rows.Close()
	out := []library.Pattern{}
	for rows.Next() {
		p, err := scanLibraryPattern(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// checkPatternPlay turns a link to a play that does not exist into ErrNotFound, not a foreign key error.
func checkPatternPlay(ctx context.Context, tx *sql.Tx, playID *uuid.UUID) error {
	if playID == nil {
		return nil
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_plays WHERE id = ?`, playID.String()).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return library.ErrNotFound
	}
	return nil
}

func (r *SQLiteLibraryRepository) UpdatePattern(ctx context.Context, id uuid.UUID, patch library.PatternPatch) (library.Pattern, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		current, err := scanLibraryPattern(tx.QueryRowContext(ctx, libraryPatternColumns+` WHERE id = ?`, id.String()))
		if err != nil {
			return err
		}
		next := library.ApplyPatternPatch(current, patch)
		if err := checkPatternPlay(ctx, tx, patch.PlayID); err != nil {
			return err
		}
		row, err := encodePatternDetail(next)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE library_patterns SET name = ?, map = ?, team_id = ?, category = ?, play_id = ?, detail = ?, updated_at = ? WHERE id = ?`,
			next.Name, next.Map, row.team, next.Category, row.play, row.detail, millis(libraryNow()), id.String())
		return err
	})
	if err != nil {
		return library.Pattern{}, err
	}
	return r.GetPattern(ctx, id)
}

func (r *SQLiteLibraryRepository) DeletePattern(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_patterns WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete pattern: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}
