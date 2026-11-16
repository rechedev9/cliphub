package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/hudthemes"
)

type SQLiteHUDThemeRepository struct {
	db *sql.DB
}

func NewSQLiteHUDThemeRepository(db *sql.DB) *SQLiteHUDThemeRepository {
	return &SQLiteHUDThemeRepository{db: db}
}

const hudThemeColumns = `SELECT id, base, name, ct, t, background, radius, created_at, updated_at FROM hud_themes`

func scanHUDTheme(row interface{ Scan(...any) error }) (hudthemes.Theme, error) {
	var (
		out              hudthemes.Theme
		id               string
		radius           sql.NullInt64
		created, updated int64
	)
	if err := row.Scan(&id, &out.Base, &out.Name, &out.CT, &out.T, &out.Background, &radius, &created, &updated); err != nil {
		return hudthemes.Theme{}, err
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return hudthemes.Theme{}, fmt.Errorf("hud theme id: %w", err)
	}
	out.ID = parsed
	if radius.Valid {
		value := int(radius.Int64)
		out.Radius = &value
	}
	out.CreatedAt, out.UpdatedAt = fromMillis(created), fromMillis(updated)
	return out, nil
}

func nullRadius(radius *int) sql.NullInt64 {
	if radius == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*radius), Valid: true}
}

func (r *SQLiteHUDThemeRepository) ListHUDThemes(ctx context.Context) ([]hudthemes.Theme, error) {
	rows, err := r.db.QueryContext(ctx, hudThemeColumns+` ORDER BY created_at, rowid`)
	if err != nil {
		return nil, fmt.Errorf("list hud themes: %w", err)
	}
	defer rows.Close()
	out := []hudthemes.Theme{}
	for rows.Next() {
		t, err := scanHUDTheme(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *SQLiteHUDThemeRepository) GetHUDTheme(ctx context.Context, id uuid.UUID) (hudthemes.Theme, error) {
	t, err := scanHUDTheme(r.db.QueryRowContext(ctx, hudThemeColumns+` WHERE id = ?`, id.String()))
	if errors.Is(err, sql.ErrNoRows) {
		return hudthemes.Theme{}, hudthemes.ErrNotFound
	}
	return t, err
}

// CreateHUDTheme counts and inserts in one transaction, so two creates cannot pass the limit together.
func (r *SQLiteHUDThemeRepository) CreateHUDTheme(ctx context.Context, t *hudthemes.Theme) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hud_themes`).Scan(&count); err != nil {
		return fmt.Errorf("count hud themes: %w", err)
	}
	if count >= hudthemes.MaxThemes {
		return hudthemes.ErrLimit
	}
	stored := t.Copy()
	newHUDTheme(&stored)
	if _, err := tx.ExecContext(ctx, `INSERT INTO hud_themes (id, base, name, ct, t, background, radius, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		stored.ID.String(), stored.Base, stored.Name, stored.CT, stored.T, stored.Background, nullRadius(stored.Radius), millis(stored.CreatedAt), millis(stored.UpdatedAt)); err != nil {
		return fmt.Errorf("create hud theme: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	*t = stored
	return nil
}

func (r *SQLiteHUDThemeRepository) UpdateHUDTheme(ctx context.Context, id uuid.UUID, style hudthemes.Style) (hudthemes.Theme, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE hud_themes SET name = ?, ct = ?, t = ?, background = ?, radius = ?, updated_at = ? WHERE id = ?`,
		style.Name, style.CT, style.T, style.Background, nullRadius(style.Radius), millis(time.Now()), id.String())
	if err != nil {
		return hudthemes.Theme{}, fmt.Errorf("update hud theme: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return hudthemes.Theme{}, err
	}
	if n == 0 {
		return hudthemes.Theme{}, hudthemes.ErrNotFound
	}
	return r.GetHUDTheme(ctx, id)
}

func (r *SQLiteHUDThemeRepository) DeleteHUDTheme(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM hud_themes WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete hud theme: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return hudthemes.ErrNotFound
	}
	return nil
}
