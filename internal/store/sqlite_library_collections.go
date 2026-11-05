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

const libraryCollectionColumns = `SELECT id, name, items, created_at, updated_at FROM library_collections`

func scanLibraryCollection(row sqlScanner) (library.Collection, error) {
	var c library.Collection
	var id, items string
	var created, updated int64
	if err := row.Scan(&id, &c.Name, &items, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Collection{}, library.ErrNotFound
		}
		return library.Collection{}, err
	}
	var err error
	if c.ID, err = uuid.Parse(id); err != nil {
		return library.Collection{}, err
	}
	if err := json.Unmarshal([]byte(items), &c.Items); err != nil {
		return library.Collection{}, fmt.Errorf("decode collection %s: %w", id, err)
	}
	c.CreatedAt, c.UpdatedAt = fromMillis(created), fromMillis(updated)
	return collectionView(c), nil
}

func encodeCollectionItems(items []library.CollectionItem) (string, error) {
	raw, err := json.Marshal(append([]library.CollectionItem{}, items...))
	if err != nil {
		return "", fmt.Errorf("encode collection: %w", err)
	}
	return string(raw), nil
}

func (r *SQLiteLibraryRepository) CreateCollection(ctx context.Context, c *library.Collection) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	items, err := encodeCollectionItems(c.Items)
	if err != nil {
		return err
	}
	now := millis(libraryNow())
	err = r.inTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_collections WHERE id = ?`, c.ID.String()).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return library.ErrConflict
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO library_collections (id, name, items, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)`, c.ID.String(), c.Name, items, now, now)
		return err
	})
	if err != nil {
		return err
	}
	stored, err := r.GetCollection(ctx, c.ID)
	if err != nil {
		return err
	}
	*c = stored
	return nil
}

func (r *SQLiteLibraryRepository) GetCollection(ctx context.Context, id uuid.UUID) (library.Collection, error) {
	return scanLibraryCollection(r.db.QueryRowContext(ctx, libraryCollectionColumns+` WHERE id = ?`, id.String()))
}

func (r *SQLiteLibraryRepository) ListCollections(ctx context.Context) ([]library.Collection, error) {
	rows, err := r.db.QueryContext(ctx, libraryCollectionColumns+` ORDER BY updated_at DESC, id ASC LIMIT ?`, library.MaxCollectionList)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	defer rows.Close()
	out := []library.Collection{}
	for rows.Next() {
		c, err := scanLibraryCollection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) UpdateCollection(ctx context.Context, id uuid.UUID, patch library.CollectionPatch) (library.Collection, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		current, err := scanLibraryCollection(tx.QueryRowContext(ctx, libraryCollectionColumns+` WHERE id = ?`, id.String()))
		if err != nil {
			return err
		}
		next := applyCollectionPatch(current, patch)
		items, err := encodeCollectionItems(next.Items)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE library_collections SET name = ?, items = ?, updated_at = ? WHERE id = ?`,
			next.Name, items, millis(libraryNow()), id.String())
		return err
	})
	if err != nil {
		return library.Collection{}, err
	}
	return r.GetCollection(ctx, id)
}

func (r *SQLiteLibraryRepository) DeleteCollection(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_collections WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete collection: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}
