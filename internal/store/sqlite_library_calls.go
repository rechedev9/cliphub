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

const libraryCallColumns = `SELECT id, name, side, meaning, status, tags, created_at, updated_at FROM library_calls`

func scanLibraryCall(row sqlScanner) (library.Call, error) {
	var c library.Call
	var id, tags string
	var created, updated int64
	if err := row.Scan(&id, &c.Name, &c.Side, &c.Meaning, &c.Status, &tags, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Call{}, library.ErrNotFound
		}
		return library.Call{}, err
	}
	var err error
	if c.ID, err = uuid.Parse(id); err != nil {
		return library.Call{}, err
	}
	if err := json.Unmarshal([]byte(tags), &c.Tags); err != nil {
		return library.Call{}, fmt.Errorf("decode call %s tags: %w", id, err)
	}
	c.CreatedAt, c.UpdatedAt = fromMillis(created), fromMillis(updated)
	return callCopy(c), nil
}

func (r *SQLiteLibraryRepository) CreateCall(ctx context.Context, c *library.Call) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	stored := callCopy(*c)
	tags, err := json.Marshal(stored.Tags)
	if err != nil {
		return fmt.Errorf("encode call tags: %w", err)
	}
	now := millis(libraryNow())
	err = r.inTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_calls WHERE id = ?`, c.ID.String()).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return library.ErrConflict
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO library_calls (id, name, side, meaning, status, tags, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, c.ID.String(), stored.Name, stored.Side, stored.Meaning, stored.Status, string(tags), now, now)
		return err
	})
	if err != nil {
		return err
	}
	got, err := r.GetCall(ctx, c.ID)
	if err != nil {
		return err
	}
	*c = got
	return nil
}

func (r *SQLiteLibraryRepository) GetCall(ctx context.Context, id uuid.UUID) (library.Call, error) {
	return scanLibraryCall(r.db.QueryRowContext(ctx, libraryCallColumns+` WHERE id = ?`, id.String()))
}

func (r *SQLiteLibraryRepository) ListCalls(ctx context.Context, filter library.CallFilter) ([]library.Call, error) {
	query := libraryCallColumns + ` WHERE 1 = 1`
	args := []any{}
	if filter.Side != "" {
		query += ` AND side IN (?, ?)`
		args = append(args, filter.Side, library.CallSideBoth)
	}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if filter.ExcludeArchived {
		query += ` AND status <> ?`
		args = append(args, library.StatusArchived)
	}
	if filter.Tag != "" {
		query += ` AND EXISTS (SELECT 1 FROM json_each(tags) WHERE lower(json_each.value) = lower(?))`
		args = append(args, filter.Tag)
	}
	query += ` ORDER BY lower(name) ASC, id ASC LIMIT ? OFFSET ?`
	args = append(args, clampCallLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list calls: %w", err)
	}
	defer rows.Close()
	out := []library.Call{}
	for rows.Next() {
		c, err := scanLibraryCall(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) UpdateCall(ctx context.Context, id uuid.UUID, patch library.CallPatch) (library.Call, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		current, err := scanLibraryCall(tx.QueryRowContext(ctx, libraryCallColumns+` WHERE id = ?`, id.String()))
		if err != nil {
			return err
		}
		next := callCopy(library.ApplyCallPatch(current, patch))
		tags, err := json.Marshal(next.Tags)
		if err != nil {
			return fmt.Errorf("encode call tags: %w", err)
		}
		_, err = tx.ExecContext(ctx, `UPDATE library_calls SET name = ?, side = ?, meaning = ?, status = ?, tags = ?, updated_at = ? WHERE id = ?`,
			next.Name, next.Side, next.Meaning, next.Status, string(tags), millis(libraryNow()), id.String())
		return err
	})
	if err != nil {
		return library.Call{}, err
	}
	return r.GetCall(ctx, id)
}

func (r *SQLiteLibraryRepository) DeleteCall(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_calls WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete call: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}

func (r *SQLiteLibraryRepository) ListCalloutNames(ctx context.Context, mapName string) ([]library.CalloutName, error) {
	query, args := `SELECT map, token, name, updated_at FROM library_callout_names`, []any{}
	if mapName != "" {
		query += ` WHERE map = ?`
		args = append(args, mapName)
	}
	rows, err := r.db.QueryContext(ctx, query+` ORDER BY map, token`, args...)
	if err != nil {
		return nil, fmt.Errorf("list callout names: %w", err)
	}
	defer rows.Close()
	out := []library.CalloutName{}
	for rows.Next() {
		var c library.CalloutName
		var updated int64
		if err := rows.Scan(&c.Map, &c.Token, &c.Name, &updated); err != nil {
			return nil, err
		}
		c.UpdatedAt = fromMillis(updated)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) SetCalloutName(ctx context.Context, c *library.CalloutName) error {
	now := millis(libraryNow())
	if _, err := r.db.ExecContext(ctx, `INSERT INTO library_callout_names (map, token, name, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(map, token) DO UPDATE SET name = excluded.name, updated_at = excluded.updated_at`, c.Map, c.Token, c.Name, now); err != nil {
		return fmt.Errorf("set callout name: %w", err)
	}
	c.UpdatedAt = fromMillis(now)
	return nil
}

func (r *SQLiteLibraryRepository) DeleteCalloutName(ctx context.Context, mapName, token string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_callout_names WHERE map = ? AND token = ?`, mapName, token)
	if err != nil {
		return fmt.Errorf("delete callout name: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}
