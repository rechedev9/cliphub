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

const libraryPlayColumns = `SELECT p.id, p.title, p.team_id, COALESCE(t.name, ''), p.map, p.side, p.kind, p.site, p.origin,
	p.positions_seconds, p.grenades, p.positions, p.examples, p.notes, p.created_at, p.updated_at
	FROM library_plays p LEFT JOIN library_teams t ON t.id = p.team_id`

func scanLibraryPlay(row sqlScanner) (library.Play, error) {
	var p library.Play
	var id, grenades, positions, examples string
	var teamID sql.NullString
	var created, updated int64
	if err := row.Scan(&id, &p.Title, &teamID, &p.TeamName, &p.Map, &p.Side, &p.Kind, &p.Site, &p.Origin,
		&p.PositionsSeconds, &grenades, &positions, &examples, &p.Notes, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Play{}, library.ErrNotFound
		}
		return library.Play{}, err
	}
	var err error
	if p.ID, err = uuid.Parse(id); err != nil {
		return library.Play{}, err
	}
	if p.TeamID, err = parseNullUUID(teamID); err != nil {
		return library.Play{}, err
	}
	for _, field := range []struct {
		raw string
		dst any
	}{{grenades, &p.Grenades}, {positions, &p.Positions}, {examples, &p.Examples}} {
		if err := json.Unmarshal([]byte(field.raw), field.dst); err != nil {
			return library.Play{}, fmt.Errorf("decode play %s: %w", id, err)
		}
	}
	p.CreatedAt, p.UpdatedAt = fromMillis(created), fromMillis(updated)
	return fillPlayLists(p), nil
}

// encodedPlay is a play's JSON columns.
type encodedPlay struct {
	grenades, positions, examples string
}

func encodePlay(p library.Play) (encodedPlay, error) {
	p = fillPlayLists(p)
	var out encodedPlay
	for _, field := range []struct {
		src any
		dst *string
	}{{p.Grenades, &out.grenades}, {p.Positions, &out.positions}, {p.Examples, &out.examples}} {
		raw, err := json.Marshal(field.src)
		if err != nil {
			return encodedPlay{}, fmt.Errorf("encode play: %w", err)
		}
		*field.dst = string(raw)
	}
	return out, nil
}

func (r *SQLiteLibraryRepository) CreatePlay(ctx context.Context, p *library.Play) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	enc, err := encodePlay(*p)
	if err != nil {
		return err
	}
	now := millis(libraryNow())
	err = r.inTx(ctx, func(tx *sql.Tx) error {
		if p.TeamID != nil {
			if err := r.teamExists(ctx, tx, *p.TeamID); err != nil {
				return err
			}
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_plays WHERE id = ?`, p.ID.String()).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return library.ErrConflict
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO library_plays (id, title, team_id, map, side, kind, site, origin,
			positions_seconds, grenades, positions, examples, notes, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.ID.String(), p.Title, nullableUUID(p.TeamID), p.Map, p.Side, p.Kind, p.Site, p.Origin,
			p.PositionsSeconds, enc.grenades, enc.positions, enc.examples, p.Notes, now, now)
		return err
	})
	if err != nil {
		return err
	}
	stored, err := r.GetPlay(ctx, p.ID)
	if err != nil {
		return err
	}
	*p = stored
	return nil
}

func (r *SQLiteLibraryRepository) GetPlay(ctx context.Context, id uuid.UUID) (library.Play, error) {
	return scanLibraryPlay(r.db.QueryRowContext(ctx, libraryPlayColumns+` WHERE p.id = ?`, id.String()))
}

func (r *SQLiteLibraryRepository) ListPlays(ctx context.Context, filter library.PlayFilter) ([]library.Play, error) {
	query := libraryPlayColumns + ` WHERE 1 = 1`
	args := []any{}
	if filter.TeamID != nil {
		query += ` AND p.team_id = ?`
		args = append(args, filter.TeamID.String())
	}
	for _, f := range []struct{ column, value string }{{"p.map", filter.Map}, {"p.side", filter.Side}, {"p.kind", filter.Kind}} {
		if f.value != "" {
			query += ` AND ` + f.column + ` = ?`
			args = append(args, f.value)
		}
	}
	query += ` ORDER BY p.updated_at DESC, p.id ASC LIMIT ? OFFSET ?`
	args = append(args, clampPlayLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list plays: %w", err)
	}
	defer rows.Close()
	plays := []library.Play{}
	for rows.Next() {
		p, err := scanLibraryPlay(rows)
		if err != nil {
			return nil, err
		}
		plays = append(plays, p)
	}
	return plays, rows.Err()
}

func (r *SQLiteLibraryRepository) UpdatePlay(ctx context.Context, id uuid.UUID, patch library.PlayPatch) (library.Play, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		current, err := scanLibraryPlay(tx.QueryRowContext(ctx, libraryPlayColumns+` WHERE p.id = ?`, id.String()))
		if err != nil {
			return err
		}
		if patch.TeamID != nil {
			if err := r.teamExists(ctx, tx, *patch.TeamID); err != nil {
				return err
			}
		}
		next := applyPlayPatch(current, patch)
		enc, err := encodePlay(next)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE library_plays SET title = ?, team_id = ?, kind = ?, site = ?, notes = ?,
			grenades = ?, positions = ?, examples = ?, updated_at = ? WHERE id = ?`,
			next.Title, nullableUUID(next.TeamID), next.Kind, next.Site, next.Notes,
			enc.grenades, enc.positions, enc.examples, millis(libraryNow()), id.String())
		return err
	})
	if err != nil {
		return library.Play{}, err
	}
	return r.GetPlay(ctx, id)
}

func (r *SQLiteLibraryRepository) DeletePlay(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_plays WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete play: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}
