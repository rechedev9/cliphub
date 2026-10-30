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

const libraryAnnotationColumns = `SELECT id, match_id, round, tick, text, strokes, created_at, updated_at FROM library_annotations`

const libraryAnnotationOrder = ` ORDER BY match_id ASC, round ASC, tick ASC, created_at ASC, id ASC`

func scanLibraryAnnotation(row sqlScanner) (library.Annotation, error) {
	var a library.Annotation
	var id, matchID, strokes string
	var created, updated int64
	if err := row.Scan(&id, &matchID, &a.Round, &a.Tick, &a.Text, &strokes, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Annotation{}, library.ErrNotFound
		}
		return library.Annotation{}, err
	}
	var err error
	if a.ID, err = uuid.Parse(id); err != nil {
		return library.Annotation{}, err
	}
	if a.MatchID, err = uuid.Parse(matchID); err != nil {
		return library.Annotation{}, err
	}
	if err := json.Unmarshal([]byte(strokes), &a.Strokes); err != nil {
		return library.Annotation{}, fmt.Errorf("decode annotation strokes: %w", err)
	}
	if a.Strokes == nil {
		a.Strokes = []library.Stroke{}
	}
	a.CreatedAt, a.UpdatedAt = fromMillis(created), fromMillis(updated)
	return a, nil
}

func encodeStrokes(strokes []library.Stroke) (string, error) {
	if strokes == nil {
		strokes = []library.Stroke{}
	}
	raw, err := json.Marshal(strokes)
	if err != nil {
		return "", fmt.Errorf("encode annotation strokes: %w", err)
	}
	return string(raw), nil
}

func (r *SQLiteLibraryRepository) CreateAnnotation(ctx context.Context, a *library.Annotation) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	strokes, err := encodeStrokes(a.Strokes)
	if err != nil {
		return err
	}
	now := millis(libraryNow())
	err = r.inTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_matches WHERE id = ?`, a.MatchID.String()).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return library.ErrNotFound
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO library_annotations (id, match_id, round, tick, text, strokes, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			a.ID.String(), a.MatchID.String(), a.Round, a.Tick, a.Text, strokes, now, now)
		return err
	})
	if err != nil {
		return err
	}
	stored, err := r.GetAnnotation(ctx, a.ID)
	if err != nil {
		return err
	}
	*a = stored
	return nil
}

func (r *SQLiteLibraryRepository) GetAnnotation(ctx context.Context, id uuid.UUID) (library.Annotation, error) {
	return scanLibraryAnnotation(r.db.QueryRowContext(ctx, libraryAnnotationColumns+` WHERE id = ?`, id.String()))
}

func (r *SQLiteLibraryRepository) ListAnnotations(ctx context.Context, filter library.AnnotationFilter) ([]library.Annotation, error) {
	out := []library.Annotation{}
	ids := uniqueMatchIDs(filter.MatchIDs)
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids)+2)
	for _, id := range ids {
		args = append(args, id.String())
	}
	query := libraryAnnotationColumns + ` WHERE match_id IN (` + placeholders(len(ids)) + `)`
	if filter.Round > 0 {
		query += ` AND round = ?`
		args = append(args, filter.Round)
	}
	query += libraryAnnotationOrder + ` LIMIT ?`
	args = append(args, library.MaxAnnotationList)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list annotations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanLibraryAnnotation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) UpdateAnnotation(ctx context.Context, id uuid.UUID, patch library.AnnotationPatch) (library.Annotation, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		current, err := scanLibraryAnnotation(tx.QueryRowContext(ctx, libraryAnnotationColumns+` WHERE id = ?`, id.String()))
		if err != nil {
			return err
		}
		next := applyAnnotationPatch(current, patch)
		strokes, err := encodeStrokes(next.Strokes)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE library_annotations SET tick = ?, text = ?, strokes = ?, updated_at = ? WHERE id = ?`,
			next.Tick, next.Text, strokes, millis(libraryNow()), id.String())
		return err
	})
	if err != nil {
		return library.Annotation{}, err
	}
	return r.GetAnnotation(ctx, id)
}

func (r *SQLiteLibraryRepository) DeleteAnnotation(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_annotations WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete annotation: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}
