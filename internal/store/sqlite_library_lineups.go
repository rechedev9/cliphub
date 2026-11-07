package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// lineupDetail is the lineup's JSON column: what is read whole and never filtered on.
type lineupDetail struct {
	Origin             [3]float64             `json:"origin"`
	Pitch              float64                `json:"pitch"`
	Yaw                float64                `json:"yaw"`
	Landing            *[3]float64            `json:"landing,omitempty"`
	ThrowCallout       string                 `json:"throw_callout,omitempty"`
	LandingCallout     string                 `json:"landing_callout,omitempty"`
	Movement           string                 `json:"movement"`
	Jump               bool                   `json:"jump,omitempty"`
	Click              string                 `json:"click"`
	TechniqueEstimated bool                   `json:"technique_estimated,omitempty"`
	Source             *tacticalplan.Evidence `json:"source,omitempty"`
}

const libraryLineupColumns = `SELECT id, title, map, side, grenade, status, tags, notes, detail, created_at, updated_at FROM library_lineups`

func scanLibraryLineup(row sqlScanner) (library.Lineup, error) {
	var l library.Lineup
	var id, tags, detail string
	var created, updated int64
	if err := row.Scan(&id, &l.Title, &l.Map, &l.Side, &l.Grenade, &l.Status, &tags, &l.Notes, &detail, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Lineup{}, library.ErrNotFound
		}
		return library.Lineup{}, err
	}
	var err error
	if l.ID, err = uuid.Parse(id); err != nil {
		return library.Lineup{}, err
	}
	var d lineupDetail
	if err := json.Unmarshal([]byte(tags), &l.Tags); err != nil {
		return library.Lineup{}, fmt.Errorf("decode lineup %s tags: %w", id, err)
	}
	if err := json.Unmarshal([]byte(detail), &d); err != nil {
		return library.Lineup{}, fmt.Errorf("decode lineup %s: %w", id, err)
	}
	l.Origin, l.Pitch, l.Yaw, l.Landing = d.Origin, d.Pitch, d.Yaw, d.Landing
	l.ThrowCallout, l.LandingCallout = d.ThrowCallout, d.LandingCallout
	l.Movement, l.Jump, l.Click, l.TechniqueEstimated, l.Source = d.Movement, d.Jump, d.Click, d.TechniqueEstimated, d.Source
	l.CreatedAt, l.UpdatedAt = fromMillis(created), fromMillis(updated)
	return lineupCopy(l), nil
}

func encodeLineup(l library.Lineup) (tags, detail string, err error) {
	l = lineupCopy(l)
	rawTags, err := json.Marshal(l.Tags)
	if err != nil {
		return "", "", fmt.Errorf("encode lineup tags: %w", err)
	}
	rawDetail, err := json.Marshal(lineupDetail{
		Origin: l.Origin, Pitch: l.Pitch, Yaw: l.Yaw, Landing: l.Landing, ThrowCallout: l.ThrowCallout, LandingCallout: l.LandingCallout,
		Movement: l.Movement, Jump: l.Jump, Click: l.Click, TechniqueEstimated: l.TechniqueEstimated, Source: l.Source,
	})
	if err != nil {
		return "", "", fmt.Errorf("encode lineup: %w", err)
	}
	return string(rawTags), string(rawDetail), nil
}

func (r *SQLiteLibraryRepository) CreateLineup(ctx context.Context, l *library.Lineup) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	tags, detail, err := encodeLineup(*l)
	if err != nil {
		return err
	}
	now := millis(libraryNow())
	err = r.inTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_lineups WHERE id = ?`, l.ID.String()).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return library.ErrConflict
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO library_lineups (id, title, map, side, grenade, status, tags, notes, detail, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			l.ID.String(), l.Title, l.Map, l.Side, l.Grenade, lineupCopy(*l).Status, tags, l.Notes, detail, now, now)
		return err
	})
	if err != nil {
		return err
	}
	stored, err := r.GetLineup(ctx, l.ID)
	if err != nil {
		return err
	}
	*l = stored
	return nil
}

func (r *SQLiteLibraryRepository) GetLineup(ctx context.Context, id uuid.UUID) (library.Lineup, error) {
	return scanLibraryLineup(r.db.QueryRowContext(ctx, libraryLineupColumns+` WHERE id = ?`, id.String()))
}

func (r *SQLiteLibraryRepository) ListLineups(ctx context.Context, filter library.LineupFilter) ([]library.Lineup, error) {
	query := libraryLineupColumns + ` WHERE 1 = 1`
	args := []any{}
	for _, f := range []struct{ column, value string }{
		{"map", filter.Map}, {"side", filter.Side}, {"grenade", filter.Grenade}, {"status", filter.Status},
	} {
		if f.value != "" {
			query += ` AND ` + f.column + ` = ?`
			args = append(args, f.value)
		}
	}
	if filter.ExcludeArchived {
		query += ` AND status <> ?`
		args = append(args, library.StatusArchived)
	}
	if filter.Tag != "" {
		query += tagFilterSQL("tags")
		args = append(args, filter.Tag)
	}
	query += ` ORDER BY updated_at DESC, id ASC LIMIT ? OFFSET ?`
	args = append(args, clampLineupLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list lineups: %w", err)
	}
	defer rows.Close()
	out := []library.Lineup{}
	for rows.Next() {
		l, err := scanLibraryLineup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) UpdateLineup(ctx context.Context, id uuid.UUID, patch library.LineupPatch) (library.Lineup, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		current, err := scanLibraryLineup(tx.QueryRowContext(ctx, libraryLineupColumns+` WHERE id = ?`, id.String()))
		if err != nil {
			return err
		}
		next := library.ApplyLineupPatch(current, patch)
		tags, detail, err := encodeLineup(next)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE library_lineups SET title = ?, status = ?, tags = ?, notes = ?, detail = ?, updated_at = ? WHERE id = ?`,
			next.Title, next.Status, tags, next.Notes, detail, millis(libraryNow()), id.String())
		return err
	})
	if err != nil {
		return library.Lineup{}, err
	}
	return r.GetLineup(ctx, id)
}

func (r *SQLiteLibraryRepository) DeleteLineup(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_lineups WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete lineup: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}
