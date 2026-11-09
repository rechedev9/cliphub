package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

const libraryPlaylistColumns = `SELECT p.id, p.name, p.folder, p.favorite, p.author, p.created_at, p.updated_at FROM library_playlists p`

// libraryPlaylistRounds resolves each round's match by id, else by its demo's hash.
const libraryPlaylistRounds = `SELECT pr.playlist_id, pr.match_id, pr.round, pr.map, pr.demo_sha256, pr.comment, pr.notes,
	COALESCE(m.id, m2.id, ''), COALESCE(m.map, m2.map, '')
	FROM library_playlist_rounds pr
	LEFT JOIN library_matches m ON m.id = pr.match_id
	LEFT JOIN library_demos d ON pr.demo_sha256 <> '' AND d.sha256 = pr.demo_sha256
	LEFT JOIN library_matches m2 ON m2.demo_id = d.id`

func scanLibraryPlaylist(row sqlScanner) (library.Playlist, error) {
	var p library.Playlist
	var id string
	var favorite int
	var created, updated int64
	if err := row.Scan(&id, &p.Name, &p.Folder, &favorite, &p.Author, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Playlist{}, library.ErrNotFound
		}
		return library.Playlist{}, err
	}
	var err error
	if p.ID, err = uuid.Parse(id); err != nil {
		return library.Playlist{}, err
	}
	p.Favorite = favorite != 0
	p.CreatedAt, p.UpdatedAt = fromMillis(created), fromMillis(updated)
	p.Rounds = []library.PlaylistRound{}
	return p, nil
}

// withPlaylistRounds reads the rounds of the playlists, in order, and derives their maps.
func withPlaylistRounds(ctx context.Context, q libraryQuerier, playlists []library.Playlist) error {
	if len(playlists) == 0 {
		return nil
	}
	ids := make([]any, len(playlists))
	at := map[string]int{}
	for i, p := range playlists {
		ids[i] = p.ID.String()
		at[p.ID.String()] = i
	}
	rows, err := q.QueryContext(ctx, libraryPlaylistRounds+` WHERE pr.playlist_id IN (`+placeholders(len(ids))+`) ORDER BY pr.playlist_id, pr.position`, ids...)
	if err != nil {
		return fmt.Errorf("list playlist rounds: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var playlistID, matchID, notes, resolvedID, resolvedMap string
		var r library.PlaylistRound
		if err := rows.Scan(&playlistID, &matchID, &r.Round, &r.Map, &r.DemoSHA256, &r.Comment, &notes, &resolvedID, &resolvedMap); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(notes), &r.Notes); err != nil {
			return fmt.Errorf("decode playlist %s round notes: %w", playlistID, err)
		}
		if r.Notes == nil {
			r.Notes = []library.PlaylistNote{}
		}
		if resolvedID == "" {
			r.Missing = true
		} else {
			matchID, r.Map = resolvedID, resolvedMap
		}
		if r.MatchID, err = uuid.Parse(matchID); err != nil {
			return fmt.Errorf("decode playlist %s round: %w", playlistID, err)
		}
		i := at[playlistID]
		playlists[i].Rounds = append(playlists[i].Rounds, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range playlists {
		playlists[i].Maps = library.PlaylistMaps(playlists[i].Rounds)
	}
	return nil
}

func getLibraryPlaylist(ctx context.Context, q libraryQuerier, id uuid.UUID) (library.Playlist, error) {
	p, err := scanLibraryPlaylist(q.QueryRowContext(ctx, libraryPlaylistColumns+` WHERE p.id = ?`, id.String()))
	if err != nil {
		return library.Playlist{}, err
	}
	list := []library.Playlist{p}
	if err := withPlaylistRounds(ctx, q, list); err != nil {
		return library.Playlist{}, err
	}
	return list[0], nil
}

func insertPlaylistRounds(ctx context.Context, tx *sql.Tx, id uuid.UUID, rounds []library.PlaylistRound) error {
	for i, r := range rounds {
		if r.Notes == nil {
			r.Notes = []library.PlaylistNote{}
		}
		notes, err := json.Marshal(r.Notes)
		if err != nil {
			return fmt.Errorf("encode playlist round notes: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO library_playlist_rounds (playlist_id, position, match_id, round, map, demo_sha256, comment, notes)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id.String(), i, r.MatchID.String(), r.Round, r.Map, r.DemoSHA256, r.Comment, string(notes)); err != nil {
			return fmt.Errorf("insert playlist round: %w", err)
		}
	}
	return nil
}

func favoriteInt(favorite bool) int {
	if favorite {
		return 1
	}
	return 0
}

func (r *SQLiteLibraryRepository) CreatePlaylist(ctx context.Context, p *library.Playlist) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := millis(libraryNow())
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_playlists WHERE id = ?`, p.ID.String()).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return library.ErrConflict
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO library_playlists (id, name, folder, favorite, author, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, p.ID.String(), p.Name, p.Folder, favoriteInt(p.Favorite), p.Author, now, now); err != nil {
			return err
		}
		return insertPlaylistRounds(ctx, tx, p.ID, p.Rounds)
	})
	if err != nil {
		return err
	}
	stored, err := r.GetPlaylist(ctx, p.ID)
	if err != nil {
		return err
	}
	*p = stored
	return nil
}

func (r *SQLiteLibraryRepository) GetPlaylist(ctx context.Context, id uuid.UUID) (library.Playlist, error) {
	return getLibraryPlaylist(ctx, r.db, id)
}

func (r *SQLiteLibraryRepository) ListPlaylists(ctx context.Context, filter library.PlaylistFilter) ([]library.Playlist, error) {
	query := libraryPlaylistColumns + ` WHERE 1 = 1`
	args := []any{}
	if filter.Favorite {
		query += ` AND p.favorite = 1`
	}
	if filter.NoFolder {
		query += ` AND p.folder = ''`
	} else if filter.Folder != "" {
		query += ` AND ` + sqlLower + `(p.folder) = ?`
		args = append(args, strings.ToLower(filter.Folder))
	}
	if filter.Map != "" {
		query += ` AND EXISTS (SELECT 1 FROM library_playlist_rounds pr WHERE pr.playlist_id = p.id AND pr.map = ?)`
		args = append(args, filter.Map)
	}
	if q := strings.ToLower(library.CleanName(filter.Query)); q != "" {
		query += ` AND instr(` + sqlLower + `(p.name || char(10) || p.author || char(10) || p.folder), ?) > 0`
		args = append(args, q)
	}
	query += ` ORDER BY p.updated_at DESC, p.id ASC LIMIT ? OFFSET ?`
	args = append(args, clampPlaylistLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list playlists: %w", err)
	}
	out := []library.Playlist{}
	for rows.Next() {
		p, err := scanLibraryPlaylist(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := withPlaylistRounds(ctx, r.db, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *SQLiteLibraryRepository) UpdatePlaylist(ctx context.Context, id uuid.UUID, patch library.PlaylistPatch) (library.Playlist, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		current, err := getLibraryPlaylist(ctx, tx, id)
		if err != nil {
			return err
		}
		next := library.ApplyPlaylistPatch(current, patch)
		updated := millis(current.UpdatedAt)
		if patch.Edits() {
			updated = millis(libraryNow())
		}
		if _, err := tx.ExecContext(ctx, `UPDATE library_playlists SET name = ?, folder = ?, favorite = ?, author = ?, updated_at = ? WHERE id = ?`,
			next.Name, next.Folder, favoriteInt(next.Favorite), next.Author, updated, id.String()); err != nil {
			return err
		}
		if patch.Rounds == nil {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM library_playlist_rounds WHERE playlist_id = ?`, id.String()); err != nil {
			return err
		}
		return insertPlaylistRounds(ctx, tx, id, next.Rounds)
	})
	if err != nil {
		return library.Playlist{}, err
	}
	return r.GetPlaylist(ctx, id)
}

func (r *SQLiteLibraryRepository) DeletePlaylist(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_playlists WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete playlist: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}
