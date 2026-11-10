package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

const proMapColumns = `hltv_match_id, entry, demo_id, map, event, played_at, teams, analysis_version, sha256, imported_at`

func (r *SQLiteLibraryRepository) SaveProMap(ctx context.Context, m *library.ProMap) error {
	teams, err := json.Marshal(append([]library.ProTeam{}, m.Teams...))
	if err != nil {
		return err
	}
	var demo any
	if m.DemoID != nil {
		demo = m.DemoID.String()
	}
	m.ImportedAt = fromMillis(millis(libraryNow()))
	_, err = r.db.ExecContext(ctx, `INSERT INTO library_pro_maps (`+proMapColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hltv_match_id, entry) DO UPDATE SET demo_id = excluded.demo_id, map = excluded.map, event = excluded.event,
			played_at = excluded.played_at, teams = excluded.teams, analysis_version = excluded.analysis_version,
			sha256 = excluded.sha256, imported_at = excluded.imported_at`,
		m.MatchID, m.Entry, demo, m.Map, m.Event, millis(m.PlayedAt), string(teams), m.AnalysisVersion, m.SHA256, millis(m.ImportedAt))
	if err != nil {
		return fmt.Errorf("save pro map: %w", err)
	}
	return nil
}

func (r *SQLiteLibraryRepository) GetProMap(ctx context.Context, matchID int, entry string) (library.ProMap, error) {
	return r.oneProMap(ctx, `WHERE hltv_match_id = ? AND entry = ?`, matchID, entry)
}

func (r *SQLiteLibraryRepository) ProMapForDemo(ctx context.Context, demoID uuid.UUID) (library.ProMap, error) {
	return r.oneProMap(ctx, `WHERE demo_id = ?`, demoID.String())
}

func (r *SQLiteLibraryRepository) oneProMap(ctx context.Context, where string, args ...any) (library.ProMap, error) {
	maps, err := r.queryProMaps(ctx, where+` LIMIT 1`, args...)
	if err != nil {
		return library.ProMap{}, err
	}
	if len(maps) == 0 {
		return library.ProMap{}, library.ErrNotFound
	}
	return maps[0], nil
}

func (r *SQLiteLibraryRepository) ListProMaps(ctx context.Context, matchID int) ([]library.ProMap, error) {
	if matchID > 0 {
		return r.queryProMaps(ctx, `WHERE hltv_match_id = ? ORDER BY entry`, matchID)
	}
	return r.queryProMaps(ctx, `ORDER BY played_at DESC, hltv_match_id DESC, entry`)
}

func (r *SQLiteLibraryRepository) queryProMaps(ctx context.Context, tail string, args ...any) ([]library.ProMap, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+proMapColumns+` FROM library_pro_maps `+tail, args...)
	if err != nil {
		return nil, fmt.Errorf("list pro maps: %w", err)
	}
	defer rows.Close()
	out := []library.ProMap{}
	for rows.Next() {
		var m library.ProMap
		var demo sql.NullString
		var teams string
		var played, imported int64
		if err := rows.Scan(&m.MatchID, &m.Entry, &demo, &m.Map, &m.Event, &played, &teams, &m.AnalysisVersion, &m.SHA256, &imported); err != nil {
			return nil, err
		}
		if demo.Valid {
			if id, err := uuid.Parse(demo.String); err == nil {
				m.DemoID = &id
			}
		}
		if err := json.Unmarshal([]byte(teams), &m.Teams); err != nil {
			return nil, fmt.Errorf("pro map teams: %w", err)
		}
		m.PlayedAt, m.ImportedAt = fromMillis(played), fromMillis(imported)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) FollowProTeam(ctx context.Context, f *library.ProFollow) error {
	now := millis(libraryNow())
	if _, err := r.db.ExecContext(ctx, `INSERT INTO library_pro_follows (team_id, name, followed_at) VALUES (?, ?, ?)
		ON CONFLICT(team_id) DO UPDATE SET name = excluded.name`, f.TeamID, f.Name, now); err != nil {
		return fmt.Errorf("follow pro team: %w", err)
	}
	var followed int64
	var synced sql.NullInt64
	if err := r.db.QueryRowContext(ctx, `SELECT followed_at, synced_at FROM library_pro_follows WHERE team_id = ?`, f.TeamID).Scan(&followed, &synced); err != nil {
		return err
	}
	f.FollowedAt, f.SyncedAt = fromMillis(followed), nullMillis(synced)
	return nil
}

func nullMillis(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := fromMillis(v.Int64)
	return &t
}

func (r *SQLiteLibraryRepository) UnfollowProTeam(ctx context.Context, teamID int) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_pro_follows WHERE team_id = ?`, teamID)
	if err != nil {
		return fmt.Errorf("unfollow pro team: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}

func (r *SQLiteLibraryRepository) ListProFollows(ctx context.Context) ([]library.ProFollow, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT team_id, name, followed_at, synced_at FROM library_pro_follows ORDER BY name COLLATE NOCASE, team_id`)
	if err != nil {
		return nil, fmt.Errorf("list pro follows: %w", err)
	}
	defer rows.Close()
	out := []library.ProFollow{}
	for rows.Next() {
		var f library.ProFollow
		var followed int64
		var synced sql.NullInt64
		if err := rows.Scan(&f.TeamID, &f.Name, &followed, &synced); err != nil {
			return nil, err
		}
		f.FollowedAt, f.SyncedAt = fromMillis(followed), nullMillis(synced)
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) MarkProFollowSynced(ctx context.Context, teamID int, at time.Time) error {
	res, err := r.db.ExecContext(ctx, `UPDATE library_pro_follows SET synced_at = ? WHERE team_id = ?`, millis(at), teamID)
	if err != nil {
		return fmt.Errorf("mark pro follow synced: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}
