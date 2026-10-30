package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// SQLiteLibraryRepository is the library over the library_* tables of jobs.db.
type SQLiteLibraryRepository struct {
	db *sql.DB
}

// NewSQLiteLibraryRepository returns the library repository over db, which
// openSQLite has already migrated to the current schema.
func NewSQLiteLibraryRepository(db *sql.DB) *SQLiteLibraryRepository {
	return &SQLiteLibraryRepository{db: db}
}

// libraryQuerier is the slice of *sql.DB and *sql.Tx the library reads use,
// so a write transaction can read its own changes through the same helpers.
type libraryQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func millis(t time.Time) int64 { return t.UTC().UnixMilli() }

func fromMillis(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func likeContains(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(q) + "%"
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func parseNullUUID(s sql.NullString) (*uuid.UUID, error) {
	if !s.Valid || s.String == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s.String)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (r *SQLiteLibraryRepository) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

const libraryDemoColumns = `SELECT d.id, d.sha256, d.file_name, d.path, d.size_bytes, d.source, d.source_manual,
	d.cs2_build, d.patch_version, d.state, d.error_code, d.error, d.played_at, d.imported_at, d.updated_at, m.id
	FROM library_demos d LEFT JOIN library_matches m ON m.demo_id = d.id`

func scanLibraryDemo(row sqlScanner) (library.Demo, error) {
	var (
		d                 library.Demo
		id, source, state string
		manual            int
		playedAt          sql.NullInt64
		imported, updated int64
		matchID           sql.NullString
	)
	if err := row.Scan(&id, &d.SHA256, &d.FileName, &d.Path, &d.SizeBytes, &source, &manual, &d.CS2Build, &d.PatchVersion,
		&state, &d.ErrorCode, &d.Error, &playedAt, &imported, &updated, &matchID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Demo{}, library.ErrNotFound
		}
		return library.Demo{}, err
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return library.Demo{}, err
	}
	d.ID = parsed
	d.Source = library.Source(source)
	d.SourceManual = manual != 0
	d.State = library.ParseState(state)
	d.ImportedAt = fromMillis(imported)
	d.UpdatedAt = fromMillis(updated)
	if playedAt.Valid {
		t := fromMillis(playedAt.Int64)
		d.PlayedAt = &t
	}
	if d.MatchID, err = parseNullUUID(matchID); err != nil {
		return library.Demo{}, err
	}
	d.Tags = []string{}
	d.JobIDs = []uuid.UUID{}
	return d, nil
}

func (r *SQLiteLibraryRepository) attachDemoTags(ctx context.Context, q libraryQuerier, demos []library.Demo) error {
	if len(demos) == 0 {
		return nil
	}
	args := make([]any, len(demos))
	index := make(map[string]int, len(demos))
	for i, d := range demos {
		args[i] = d.ID.String()
		index[d.ID.String()] = i
	}
	rows, err := q.QueryContext(ctx, `SELECT demo_id, tag FROM library_demo_tags WHERE demo_id IN (`+placeholders(len(args))+`) ORDER BY demo_id, tag_key`, args...)
	if err != nil {
		return fmt.Errorf("query demo tags: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var demoID, tag string
		if err := rows.Scan(&demoID, &tag); err != nil {
			return err
		}
		if i, ok := index[demoID]; ok {
			demos[i].Tags = append(demos[i].Tags, tag)
		}
	}
	return rows.Err()
}

func (r *SQLiteLibraryRepository) attachDemoJobs(ctx context.Context, q libraryQuerier, demos []library.Demo) error {
	if len(demos) == 0 {
		return nil
	}
	args := make([]any, len(demos))
	index := make(map[string]int, len(demos))
	for i, d := range demos {
		args[i] = d.ID.String()
		index[d.ID.String()] = i
	}
	rows, err := q.QueryContext(ctx, `SELECT demo_id, job_id FROM library_demo_jobs WHERE demo_id IN (`+placeholders(len(args))+`)
		ORDER BY demo_id, linked_at, job_id`, args...)
	if err != nil {
		return fmt.Errorf("query demo jobs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var demoID, jobID string
		if err := rows.Scan(&demoID, &jobID); err != nil {
			return err
		}
		parsed, err := uuid.Parse(jobID)
		if err != nil {
			return err
		}
		if i, ok := index[demoID]; ok {
			demos[i].JobIDs = append(demos[i].JobIDs, parsed)
		}
	}
	return rows.Err()
}

// attachDemoLinks fills the tags and the job links of demos read from the demo table.
func (r *SQLiteLibraryRepository) attachDemoLinks(ctx context.Context, q libraryQuerier, demos []library.Demo) error {
	if err := r.attachDemoTags(ctx, q, demos); err != nil {
		return err
	}
	return r.attachDemoJobs(ctx, q, demos)
}

func (r *SQLiteLibraryRepository) getDemo(ctx context.Context, q libraryQuerier, where string, arg any) (library.Demo, error) {
	d, err := scanLibraryDemo(q.QueryRowContext(ctx, libraryDemoColumns+` WHERE `+where, arg))
	if err != nil {
		return library.Demo{}, err
	}
	demos := []library.Demo{d}
	if err := r.attachDemoLinks(ctx, q, demos); err != nil {
		return library.Demo{}, err
	}
	return demos[0], nil
}

func insertDemoTags(ctx context.Context, q libraryQuerier, demoID uuid.UUID, tags []string) error {
	for _, tag := range tags {
		if _, err := q.ExecContext(ctx, `INSERT INTO library_demo_tags (demo_id, tag_key, tag) VALUES (?, ?, ?)`,
			demoID.String(), libraryTagKey(tag), tag); err != nil {
			return fmt.Errorf("insert demo tag: %w", err)
		}
	}
	return nil
}

func (r *SQLiteLibraryRepository) CreateDemo(ctx context.Context, d *library.Demo) (bool, error) {
	prepareNewDemo(d)
	created := false
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		var playedAt any
		if d.PlayedAt != nil {
			playedAt = millis(*d.PlayedAt)
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO library_demos
			(id, sha256, file_name, path, size_bytes, source, source_manual, cs2_build, patch_version, state, error_code, error, played_at, imported_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(sha256) DO NOTHING`,
			d.ID.String(), d.SHA256, d.FileName, d.Path, d.SizeBytes, string(d.Source), boolInt(d.SourceManual), d.CS2Build, d.PatchVersion,
			string(d.State), d.ErrorCode, d.Error, playedAt, millis(d.ImportedAt), millis(d.UpdatedAt))
		if err != nil {
			return fmt.Errorf("insert demo: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			existing, err := r.getDemo(ctx, tx, `d.sha256 = ?`, d.SHA256)
			if err != nil {
				return err
			}
			*d = existing
			return nil
		}
		created = true
		return insertDemoTags(ctx, tx, d.ID, d.Tags)
	})
	return created, err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (r *SQLiteLibraryRepository) GetDemo(ctx context.Context, id uuid.UUID) (library.Demo, error) {
	return r.getDemo(ctx, r.db, `d.id = ?`, id.String())
}

func (r *SQLiteLibraryRepository) ListDemos(ctx context.Context, filter library.DemoFilter) ([]library.Demo, error) {
	var where []string
	var args []any
	if len(filter.States) > 0 {
		where = append(where, `d.state IN (`+placeholders(len(filter.States))+`)`)
		for _, s := range filter.States {
			args = append(args, string(s))
		}
	}
	if filter.Source != "" {
		where = append(where, `d.source = ?`)
		args = append(args, string(filter.Source))
	}
	if filter.Tag != "" {
		where = append(where, `EXISTS (SELECT 1 FROM library_demo_tags t WHERE t.demo_id = d.id AND t.tag_key = ?)`)
		args = append(args, libraryTagKey(library.CleanName(filter.Tag)))
	}
	query := libraryDemoColumns
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, ` AND `)
	}
	query += ` ORDER BY d.imported_at DESC, d.id ASC LIMIT ? OFFSET ?`
	args = append(args, library.ClampLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list demos: %w", err)
	}
	demos := []library.Demo{}
	for rows.Next() {
		d, err := scanLibraryDemo(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		demos = append(demos, d)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachDemoLinks(ctx, r.db, demos); err != nil {
		return nil, err
	}
	return demos, nil
}

func (r *SQLiteLibraryRepository) UpdateDemo(ctx context.Context, id uuid.UUID, patch library.DemoPatch) (library.Demo, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := r.getDemo(ctx, tx, `d.id = ?`, id.String()); err != nil {
			return err
		}
		if patch.Source != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE library_demos SET source = ?, source_manual = 1 WHERE id = ?`, string(*patch.Source), id.String()); err != nil {
				return fmt.Errorf("update demo source: %w", err)
			}
		}
		if patch.Tags != nil {
			if _, err := tx.ExecContext(ctx, `DELETE FROM library_demo_tags WHERE demo_id = ?`, id.String()); err != nil {
				return fmt.Errorf("clear demo tags: %w", err)
			}
			if err := insertDemoTags(ctx, tx, id, library.NormalizeTags(*patch.Tags)); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `UPDATE library_demos SET updated_at = ? WHERE id = ?`, millis(libraryNow()), id.String())
		return err
	})
	if err != nil {
		return library.Demo{}, err
	}
	return r.GetDemo(ctx, id)
}

func (r *SQLiteLibraryRepository) SetDemoState(ctx context.Context, id uuid.UUID, state library.ParseState, errorCode, message string) error {
	if state != library.ParseFailed {
		errorCode, message = "", ""
	}
	res, err := r.db.ExecContext(ctx, `UPDATE library_demos SET state = ?, error_code = ?, error = ?, updated_at = ? WHERE id = ?`,
		string(state), errorCode, message, millis(libraryNow()), id.String())
	if err != nil {
		return fmt.Errorf("set demo state: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}

func (r *SQLiteLibraryRepository) DeleteDemo(ctx context.Context, id uuid.UUID) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		steamIDs, err := queryStrings(ctx, tx, `SELECT mp.steam_id FROM library_match_players mp
			JOIN library_matches m ON m.id = mp.match_id WHERE m.demo_id = ? ORDER BY mp.steam_id`, id.String())
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM library_demos WHERE id = ?`, id.String()); err != nil {
			return fmt.Errorf("delete demo: %w", err)
		}
		return refreshPlayerNicks(ctx, tx, steamIDs)
	})
}

func (r *SQLiteLibraryRepository) LinkDemoJob(ctx context.Context, demoID, jobID uuid.UUID) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := r.getDemo(ctx, tx, `d.id = ?`, demoID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO library_demo_jobs (job_id, demo_id, linked_at) VALUES (?, ?, ?)
			ON CONFLICT(job_id) DO NOTHING`, jobID.String(), demoID.String(), millis(libraryNow())); err != nil {
			return fmt.Errorf("link demo job: %w", err)
		}
		return nil
	})
}

func (r *SQLiteLibraryRepository) DemoForJob(ctx context.Context, jobID uuid.UUID) (library.Demo, error) {
	return r.getDemo(ctx, r.db, `d.id = (SELECT demo_id FROM library_demo_jobs WHERE job_id = ?)`, jobID.String())
}

func queryStrings(ctx context.Context, q libraryQuerier, query string, args ...any) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// refreshPlayerNicks sets each player's current nick to the name they used in
// their most recent match, so a date change or a deleted demo cannot leave a
// stale nick behind. A player with no match left keeps the last one.
func refreshPlayerNicks(ctx context.Context, q libraryQuerier, steamIDs []string) error {
	for _, steamID := range steamIDs {
		var name string
		err := q.QueryRowContext(ctx, `SELECT mp.name FROM library_match_players mp
			JOIN library_matches m ON m.id = mp.match_id
			WHERE mp.steam_id = ? ORDER BY m.played_at DESC, m.id DESC LIMIT 1`, steamID).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read latest nick: %w", err)
		}
		if _, err := q.ExecContext(ctx, `UPDATE library_players SET nick = ?, nick_key = ? WHERE steam_id = ?`,
			name, strings.ToLower(name), steamID); err != nil {
			return fmt.Errorf("refresh nick: %w", err)
		}
	}
	return nil
}

type sqliteMatchTeamRow struct {
	key, startSide, clan, clanKey, assignment string
	teamID                                    *uuid.UUID
	score                                     int
}

func (r *SQLiteLibraryRepository) SaveAnalysis(ctx context.Context, analysis library.Analysis) (library.Match, error) {
	var matchID uuid.UUID
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		demo, err := r.getDemo(ctx, tx, `d.id = ?`, analysis.DemoID.String())
		if err != nil {
			return err
		}
		now := libraryNow()
		source := demo.Source
		if !demo.SourceManual && library.ValidSource(analysis.Source) {
			source = analysis.Source
		}
		if _, err := tx.ExecContext(ctx, `UPDATE library_demos SET source = ?, cs2_build = ?, patch_version = ?, state = ?,
			error_code = '', error = '', updated_at = ? WHERE id = ?`,
			string(source), analysis.CS2Build, analysis.PatchVersion, string(library.ParseReady), millis(now), demo.ID.String()); err != nil {
			return fmt.Errorf("mark demo ready: %w", err)
		}

		if demo.MatchID == nil {
			matchID = uuid.New()
			playedAt := demo.ImportedAt
			if demo.PlayedAt != nil {
				playedAt = *demo.PlayedAt
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO library_matches
				(id, demo_id, map, played_at, tickrate, duration_ticks, round_count, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				matchID.String(), demo.ID.String(), analysis.Map, millis(playedAt), analysis.Tickrate, analysis.DurationTicks,
				analysis.RoundCount, millis(now), millis(now)); err != nil {
				return fmt.Errorf("insert match: %w", err)
			}
		} else {
			matchID = *demo.MatchID
			if _, err := tx.ExecContext(ctx, `UPDATE library_matches SET map = ?, tickrate = ?, duration_ticks = ?, round_count = ?, updated_at = ?
				WHERE id = ?`, analysis.Map, analysis.Tickrate, analysis.DurationTicks, analysis.RoundCount, millis(now), matchID.String()); err != nil {
				return fmt.Errorf("update match: %w", err)
			}
		}

		previous, err := r.matchTeamRows(ctx, tx, matchID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM library_match_teams WHERE match_id = ?`, matchID.String()); err != nil {
			return fmt.Errorf("clear match teams: %w", err)
		}
		var taken []uuid.UUID
		for _, team := range analysis.Teams {
			row := sqliteMatchTeamRow{
				key: team.Key, startSide: team.StartSide, clan: team.ClanName,
				clanKey: library.NormalizeName(team.ClanName), score: team.Score,
				assignment: library.AssignmentNone,
			}
			if prev, ok := previous[team.Key]; ok && prev.assignment == library.AssignmentManual {
				row.assignment, row.teamID = library.AssignmentManual, prev.teamID
			} else if row.clanKey != "" {
				teamID, err := r.resolveClanTeam(ctx, tx, team.ClanName, now)
				if err != nil {
					return err
				}
				if !containsUUID(taken, teamID) {
					row.assignment, row.teamID = library.AssignmentAuto, &teamID
				}
			}
			if row.teamID != nil {
				taken = append(taken, *row.teamID)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO library_match_teams
				(match_id, team_key, start_side, clan_name, clan_key, team_id, assignment, score) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				matchID.String(), row.key, row.startSide, row.clan, row.clanKey, nullableUUID(row.teamID), row.assignment, row.score); err != nil {
				return fmt.Errorf("insert match team: %w", err)
			}
		}

		affected, err := queryStrings(ctx, tx, `SELECT steam_id FROM library_match_players WHERE match_id = ?`, matchID.String())
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM library_match_players WHERE match_id = ?`, matchID.String()); err != nil {
			return fmt.Errorf("clear match players: %w", err)
		}
		for _, p := range analysis.Players {
			if _, err := tx.ExecContext(ctx, `INSERT INTO library_players (steam_id, nick, nick_key, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?) ON CONFLICT(steam_id) DO NOTHING`,
				p.SteamID64, p.Name, strings.ToLower(p.Name), millis(now), millis(now)); err != nil {
				return fmt.Errorf("upsert player: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO library_match_players
				(match_id, steam_id, team_key, slot, name, rounds, kills, deaths, assists, damage) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				matchID.String(), p.SteamID64, p.TeamKey, int(p.Slot), p.Name, p.Rounds, p.Kills, p.Deaths, p.Assists, p.Damage); err != nil {
				return fmt.Errorf("insert match player: %w", err)
			}
			affected = append(affected, p.SteamID64)
		}
		return refreshPlayerNicks(ctx, tx, affected)
	})
	if err != nil {
		return library.Match{}, err
	}
	return r.GetMatch(ctx, matchID)
}

func containsUUID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// resolveClanTeam returns the team a clan name is an alias of, creating the
// team (named after the clan) when no team claims it yet.
func (r *SQLiteLibraryRepository) resolveClanTeam(ctx context.Context, tx *sql.Tx, clan string, now time.Time) (uuid.UUID, error) {
	key := library.NormalizeName(clan)
	var raw string
	err := tx.QueryRowContext(ctx, `SELECT team_id FROM library_team_aliases WHERE alias_key = ?`, key).Scan(&raw)
	if err == nil {
		return uuid.Parse(raw)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("resolve clan alias: %w", err)
	}
	id := uuid.New()
	name := library.CleanName(clan)
	if _, err := tx.ExecContext(ctx, `INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id.String(), name, library.NormalizeName(name), millis(now), millis(now)); err != nil {
		return uuid.Nil, fmt.Errorf("create team for clan: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO library_team_aliases (alias_key, alias, team_id) VALUES (?, ?, ?)`, key, name, id.String()); err != nil {
		return uuid.Nil, fmt.Errorf("create clan alias: %w", err)
	}
	return id, nil
}

func (r *SQLiteLibraryRepository) matchTeamRows(ctx context.Context, q libraryQuerier, matchID uuid.UUID) (map[string]sqliteMatchTeamRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT team_key, start_side, clan_name, clan_key, team_id, assignment, score
		FROM library_match_teams WHERE match_id = ?`, matchID.String())
	if err != nil {
		return nil, fmt.Errorf("query match teams: %w", err)
	}
	defer rows.Close()
	out := map[string]sqliteMatchTeamRow{}
	for rows.Next() {
		var row sqliteMatchTeamRow
		var teamID sql.NullString
		if err := rows.Scan(&row.key, &row.startSide, &row.clan, &row.clanKey, &teamID, &row.assignment, &row.score); err != nil {
			return nil, err
		}
		if row.teamID, err = parseNullUUID(teamID); err != nil {
			return nil, err
		}
		out[row.key] = row
	}
	return out, rows.Err()
}

const libraryMatchColumns = `SELECT m.id, m.demo_id, m.series_id, m.map, m.played_at, d.source, m.tickrate, m.duration_ticks,
	m.round_count, m.created_at, m.updated_at FROM library_matches m JOIN library_demos d ON d.id = m.demo_id`

func scanLibraryMatch(row sqlScanner) (library.Match, error) {
	var (
		m                          library.Match
		id, demoID, source         string
		seriesID                   sql.NullString
		playedAt, created, updated int64
	)
	if err := row.Scan(&id, &demoID, &seriesID, &m.Map, &playedAt, &source, &m.Tickrate, &m.DurationTicks,
		&m.RoundCount, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Match{}, library.ErrNotFound
		}
		return library.Match{}, err
	}
	var err error
	if m.ID, err = uuid.Parse(id); err != nil {
		return library.Match{}, err
	}
	if m.DemoID, err = uuid.Parse(demoID); err != nil {
		return library.Match{}, err
	}
	if m.SeriesID, err = parseNullUUID(seriesID); err != nil {
		return library.Match{}, err
	}
	m.Source = library.Source(source)
	m.PlayedAt = fromMillis(playedAt)
	m.CreatedAt = fromMillis(created)
	m.UpdatedAt = fromMillis(updated)
	m.Teams = []library.MatchTeam{}
	return m, nil
}

func (r *SQLiteLibraryRepository) attachMatchTeams(ctx context.Context, q libraryQuerier, matches []library.Match) error {
	if len(matches) == 0 {
		return nil
	}
	args := make([]any, len(matches))
	index := make(map[string]int, len(matches))
	for i, m := range matches {
		args[i] = m.ID.String()
		index[m.ID.String()] = i
	}
	rows, err := q.QueryContext(ctx, `SELECT mt.match_id, mt.team_key, mt.start_side, mt.clan_name, mt.team_id, COALESCE(t.name, ''), mt.assignment, mt.score
		FROM library_match_teams mt LEFT JOIN library_teams t ON t.id = mt.team_id
		WHERE mt.match_id IN (`+placeholders(len(args))+`) ORDER BY mt.match_id, mt.team_key`, args...)
	if err != nil {
		return fmt.Errorf("query match teams: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var matchID string
		var team library.MatchTeam
		var teamID sql.NullString
		if err := rows.Scan(&matchID, &team.Key, &team.StartSide, &team.ClanName, &teamID, &team.TeamName, &team.Assignment, &team.Score); err != nil {
			return err
		}
		if team.TeamID, err = parseNullUUID(teamID); err != nil {
			return err
		}
		if i, ok := index[matchID]; ok {
			matches[i].Teams = append(matches[i].Teams, team)
		}
	}
	return rows.Err()
}

func (r *SQLiteLibraryRepository) GetMatch(ctx context.Context, id uuid.UUID) (library.Match, error) {
	m, err := scanLibraryMatch(r.db.QueryRowContext(ctx, libraryMatchColumns+` WHERE m.id = ?`, id.String()))
	if err != nil {
		return library.Match{}, err
	}
	matches := []library.Match{m}
	if err := r.attachMatchTeams(ctx, r.db, matches); err != nil {
		return library.Match{}, err
	}
	m = matches[0]
	rows, err := r.db.QueryContext(ctx, `SELECT steam_id, name, team_key, slot, rounds, kills, deaths, assists, damage
		FROM library_match_players WHERE match_id = ? ORDER BY slot, steam_id`, id.String())
	if err != nil {
		return library.Match{}, fmt.Errorf("query match players: %w", err)
	}
	defer rows.Close()
	m.Players = []library.MatchPlayer{}
	for rows.Next() {
		var p library.MatchPlayer
		var slot int
		if err := rows.Scan(&p.SteamID64, &p.Name, &p.TeamKey, &slot, &p.Rounds, &p.Kills, &p.Deaths, &p.Assists, &p.Damage); err != nil {
			return library.Match{}, err
		}
		p.Slot = uint8(slot)
		m.Players = append(m.Players, p)
	}
	return m, rows.Err()
}

func (r *SQLiteLibraryRepository) ListMatches(ctx context.Context, filter library.MatchFilter) ([]library.Match, error) {
	var where []string
	var args []any
	if filter.Map != "" {
		where = append(where, `m.map = ?`)
		args = append(args, filter.Map)
	}
	if filter.TeamID != nil {
		where = append(where, `EXISTS (SELECT 1 FROM library_match_teams mt WHERE mt.match_id = m.id AND mt.team_id = ?)`)
		args = append(args, filter.TeamID.String())
	}
	if filter.SteamID != "" {
		where = append(where, `EXISTS (SELECT 1 FROM library_match_players mp WHERE mp.match_id = m.id AND mp.steam_id = ?)`)
		args = append(args, filter.SteamID)
	}
	if filter.SeriesID != nil {
		where = append(where, `m.series_id = ?`)
		args = append(args, filter.SeriesID.String())
	}
	if filter.Source != "" {
		where = append(where, `d.source = ?`)
		args = append(args, string(filter.Source))
	}
	if filter.Tag != "" {
		where = append(where, `EXISTS (SELECT 1 FROM library_demo_tags t WHERE t.demo_id = m.demo_id AND t.tag_key = ?)`)
		args = append(args, libraryTagKey(library.CleanName(filter.Tag)))
	}
	if filter.From != nil {
		where = append(where, `m.played_at >= ?`)
		args = append(args, millis(*filter.From))
	}
	if filter.To != nil {
		where = append(where, `m.played_at <= ?`)
		args = append(args, millis(*filter.To))
	}
	query := libraryMatchColumns
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, ` AND `)
	}
	query += ` ORDER BY m.played_at DESC, m.id ASC LIMIT ? OFFSET ?`
	args = append(args, library.ClampLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list matches: %w", err)
	}
	matches := []library.Match{}
	for rows.Next() {
		m, err := scanLibraryMatch(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		matches = append(matches, m)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachMatchTeams(ctx, r.db, matches); err != nil {
		return nil, err
	}
	return matches, nil
}

func (r *SQLiteLibraryRepository) UpdateMatch(ctx context.Context, id uuid.UUID, patch library.MatchPatch) (library.Match, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := scanLibraryMatch(tx.QueryRowContext(ctx, libraryMatchColumns+` WHERE m.id = ?`, id.String())); err != nil {
			return err
		}
		now := millis(libraryNow())
		switch {
		case patch.ClearSeries:
			if _, err := tx.ExecContext(ctx, `UPDATE library_matches SET series_id = NULL WHERE id = ?`, id.String()); err != nil {
				return err
			}
		case patch.SeriesID != nil:
			var exists int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_series WHERE id = ?`, patch.SeriesID.String()).Scan(&exists); err != nil {
				return err
			}
			if exists == 0 {
				return library.ErrNotFound
			}
			if _, err := tx.ExecContext(ctx, `UPDATE library_matches SET series_id = ? WHERE id = ?`, patch.SeriesID.String(), id.String()); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE library_series SET updated_at = ? WHERE id = ?`, now, patch.SeriesID.String()); err != nil {
				return err
			}
		}
		if patch.PlayedAt != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE library_matches SET played_at = ? WHERE id = ?`, millis(*patch.PlayedAt), id.String()); err != nil {
				return err
			}
			steamIDs, err := queryStrings(ctx, tx, `SELECT steam_id FROM library_match_players WHERE match_id = ?`, id.String())
			if err != nil {
				return err
			}
			if err := refreshPlayerNicks(ctx, tx, steamIDs); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `UPDATE library_matches SET updated_at = ? WHERE id = ?`, now, id.String())
		return err
	})
	if err != nil {
		return library.Match{}, err
	}
	return r.GetMatch(ctx, id)
}

func (r *SQLiteLibraryRepository) teamExists(ctx context.Context, q libraryQuerier, id uuid.UUID) error {
	var n int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_teams WHERE id = ?`, id.String()).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return library.ErrNotFound
	}
	return nil
}

func (r *SQLiteLibraryRepository) AssignMatchTeam(ctx context.Context, a library.TeamAssignment) (library.Match, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		rows, err := r.matchTeamRows(ctx, tx, a.MatchID)
		if err != nil {
			return err
		}
		side, ok := rows[a.TeamKey]
		if !ok {
			return library.ErrNotFound
		}
		if a.TeamID != nil {
			if err := r.teamExists(ctx, tx, *a.TeamID); err != nil {
				return err
			}
			for key, other := range rows {
				if key != a.TeamKey && sameUUID(other.teamID, a.TeamID) {
					return library.ErrConflict
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE library_match_teams SET team_id = ?, assignment = ? WHERE match_id = ? AND team_key = ?`,
			nullableUUID(a.TeamID), library.AssignmentManual, a.MatchID.String(), a.TeamKey); err != nil {
			return fmt.Errorf("assign match team: %w", err)
		}
		if a.RememberAlias && a.TeamID != nil && side.clanKey != "" {
			var previousOwner sql.NullString
			if err := tx.QueryRowContext(ctx, `SELECT team_id FROM library_team_aliases WHERE alias_key = ?`, side.clanKey).Scan(&previousOwner); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO library_team_aliases (alias_key, alias, team_id) VALUES (?, ?, ?)
				ON CONFLICT(alias_key) DO UPDATE SET team_id = excluded.team_id`, side.clanKey, side.clan, a.TeamID.String()); err != nil {
				return fmt.Errorf("remember alias: %w", err)
			}
			if err := adoptAutoMatches(ctx, tx, *a.TeamID, []string{side.clanKey}); err != nil {
				return err
			}
			// The team the import created for this clan name is left with no
			// alias and no match once the name moves; it would only clutter lists.
			if previousOwner.Valid && previousOwner.String != a.TeamID.String() {
				if _, err := tx.ExecContext(ctx, `DELETE FROM library_teams WHERE id = ?
					AND NOT EXISTS (SELECT 1 FROM library_team_aliases WHERE team_id = library_teams.id)
					AND NOT EXISTS (SELECT 1 FROM library_match_teams WHERE team_id = library_teams.id)
					AND NOT EXISTS (SELECT 1 FROM library_reports WHERE team_id = library_teams.id)
					AND NOT EXISTS (SELECT 1 FROM library_plays WHERE team_id = library_teams.id)
					AND NOT EXISTS (SELECT 1 FROM library_series WHERE team_a = library_teams.id OR team_b = library_teams.id)`, previousOwner.String); err != nil {
					return fmt.Errorf("prune emptied team: %w", err)
				}
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE library_matches SET updated_at = ? WHERE id = ?`, millis(libraryNow()), a.MatchID.String())
		return err
	})
	if err != nil {
		return library.Match{}, err
	}
	return r.GetMatch(ctx, a.MatchID)
}

// adoptAutoMatches points every automatically assigned side whose clan name
// is one of aliasKeys at teamID, unless the other side of that match already
// is teamID. Manual assignments are never touched.
func adoptAutoMatches(ctx context.Context, q libraryQuerier, teamID uuid.UUID, aliasKeys []string) error {
	if len(aliasKeys) == 0 {
		return nil
	}
	args := []any{teamID.String(), library.AssignmentAuto}
	for _, key := range aliasKeys {
		args = append(args, key)
	}
	args = append(args, teamID.String())
	if _, err := q.ExecContext(ctx, `UPDATE library_match_teams SET team_id = ?
		WHERE assignment = ? AND clan_key IN (`+placeholders(len(aliasKeys))+`)
		AND NOT EXISTS (SELECT 1 FROM library_match_teams o WHERE o.match_id = library_match_teams.match_id
			AND o.team_key <> library_match_teams.team_key AND o.team_id = ?)`, args...); err != nil {
		return fmt.Errorf("adopt matches by alias: %w", err)
	}
	return nil
}

// claimAliases checks that no team other than owner holds any of aliases.
func claimAliases(ctx context.Context, q libraryQuerier, owner uuid.UUID, aliases []libraryAlias) error {
	for _, a := range aliases {
		var teamID string
		err := q.QueryRowContext(ctx, `SELECT team_id FROM library_team_aliases WHERE alias_key = ?`, a.key).Scan(&teamID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if teamID != owner.String() {
			return library.ErrConflict
		}
	}
	return nil
}

func insertAliases(ctx context.Context, q libraryQuerier, teamID uuid.UUID, aliases []libraryAlias) error {
	for _, a := range aliases {
		if _, err := q.ExecContext(ctx, `INSERT INTO library_team_aliases (alias_key, alias, team_id) VALUES (?, ?, ?)`,
			a.key, a.alias, teamID.String()); err != nil {
			return fmt.Errorf("insert alias: %w", err)
		}
	}
	return nil
}

func aliasKeys(aliases []libraryAlias) []string {
	out := make([]string, len(aliases))
	for i, a := range aliases {
		out[i] = a.key
	}
	return out
}

func (r *SQLiteLibraryRepository) CreateTeam(ctx context.Context, t *library.Team) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.Name = library.CleanName(t.Name)
	now := libraryNow()
	t.CreatedAt, t.UpdatedAt = now, now
	aliases := normalizeLibraryAliases(append([]string{t.Name}, t.Aliases...))
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		if err := claimAliases(ctx, tx, t.ID, aliases); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			t.ID.String(), t.Name, library.NormalizeName(t.Name), millis(now), millis(now)); err != nil {
			return fmt.Errorf("insert team: %w", err)
		}
		if err := insertAliases(ctx, tx, t.ID, aliases); err != nil {
			return err
		}
		return adoptAutoMatches(ctx, tx, t.ID, aliasKeys(aliases))
	})
	if err != nil {
		return err
	}
	t.Aliases = libraryAliasNames(aliases)
	t.MatchCount, err = r.teamMatchCount(ctx, t.ID)
	t.Roster = nil
	return err
}

func (r *SQLiteLibraryRepository) teamMatchCount(ctx context.Context, id uuid.UUID) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_match_teams WHERE team_id = ?`, id.String()).Scan(&n)
	return n, err
}

const libraryTeamColumns = `SELECT t.id, t.name, t.created_at, t.updated_at,
	(SELECT COUNT(*) FROM library_match_teams mt WHERE mt.team_id = t.id) FROM library_teams t`

func scanLibraryTeam(row sqlScanner) (library.Team, error) {
	var t library.Team
	var id string
	var created, updated int64
	if err := row.Scan(&id, &t.Name, &created, &updated, &t.MatchCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Team{}, library.ErrNotFound
		}
		return library.Team{}, err
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return library.Team{}, err
	}
	t.ID = parsed
	t.CreatedAt = fromMillis(created)
	t.UpdatedAt = fromMillis(updated)
	t.Aliases = []string{}
	return t, nil
}

func (r *SQLiteLibraryRepository) attachAliases(ctx context.Context, teams []library.Team) error {
	if len(teams) == 0 {
		return nil
	}
	args := make([]any, len(teams))
	index := make(map[string]int, len(teams))
	for i, t := range teams {
		args[i] = t.ID.String()
		index[t.ID.String()] = i
	}
	rows, err := r.db.QueryContext(ctx, `SELECT team_id, alias FROM library_team_aliases WHERE team_id IN (`+placeholders(len(args))+`)
		ORDER BY team_id, alias_key`, args...)
	if err != nil {
		return fmt.Errorf("query aliases: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var teamID, alias string
		if err := rows.Scan(&teamID, &alias); err != nil {
			return err
		}
		if i, ok := index[teamID]; ok {
			teams[i].Aliases = append(teams[i].Aliases, alias)
		}
	}
	return rows.Err()
}

func (r *SQLiteLibraryRepository) GetTeam(ctx context.Context, id uuid.UUID) (library.Team, error) {
	t, err := scanLibraryTeam(r.db.QueryRowContext(ctx, libraryTeamColumns+` WHERE t.id = ?`, id.String()))
	if err != nil {
		return library.Team{}, err
	}
	teams := []library.Team{t}
	if err := r.attachAliases(ctx, teams); err != nil {
		return library.Team{}, err
	}
	t = teams[0]
	rows, err := r.db.QueryContext(ctx, `SELECT mp.steam_id, p.nick, MIN(m.played_at), MAX(m.played_at), COUNT(*)
		FROM library_match_players mp
		JOIN library_match_teams mt ON mt.match_id = mp.match_id AND mt.team_key = mp.team_key
		JOIN library_matches m ON m.id = mp.match_id
		JOIN library_players p ON p.steam_id = mp.steam_id
		WHERE mt.team_id = ?
		GROUP BY mp.steam_id
		ORDER BY MAX(m.played_at) DESC, mp.steam_id ASC`, id.String())
	if err != nil {
		return library.Team{}, fmt.Errorf("query roster: %w", err)
	}
	defer rows.Close()
	t.Roster = []library.RosterEntry{}
	for rows.Next() {
		var e library.RosterEntry
		var first, last int64
		if err := rows.Scan(&e.SteamID64, &e.Nick, &first, &last, &e.Matches); err != nil {
			return library.Team{}, err
		}
		e.FirstPlayedAt, e.LastPlayedAt = fromMillis(first), fromMillis(last)
		t.Roster = append(t.Roster, e)
	}
	return t, rows.Err()
}

func (r *SQLiteLibraryRepository) ListTeams(ctx context.Context, filter library.TeamFilter) ([]library.Team, error) {
	query := libraryTeamColumns
	var args []any
	if q := library.NormalizeName(filter.Query); q != "" {
		query += ` WHERE t.name_key LIKE ? ESCAPE '\' OR EXISTS (SELECT 1 FROM library_team_aliases a WHERE a.team_id = t.id AND a.alias_key LIKE ? ESCAPE '\')`
		args = append(args, likeContains(q), likeContains(q))
	}
	query += ` ORDER BY t.name_key, t.id LIMIT ? OFFSET ?`
	args = append(args, library.ClampLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	teams := []library.Team{}
	for rows.Next() {
		t, err := scanLibraryTeam(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		teams = append(teams, t)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachAliases(ctx, teams); err != nil {
		return nil, err
	}
	return teams, nil
}

func (r *SQLiteLibraryRepository) UpdateTeam(ctx context.Context, id uuid.UUID, patch library.TeamPatch) (library.Team, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		if err := r.teamExists(ctx, tx, id); err != nil {
			return err
		}
		if patch.Name != nil {
			name := library.CleanName(*patch.Name)
			if _, err := tx.ExecContext(ctx, `UPDATE library_teams SET name = ?, name_key = ? WHERE id = ?`,
				name, library.NormalizeName(name), id.String()); err != nil {
				return fmt.Errorf("rename team: %w", err)
			}
		}
		if patch.Aliases != nil {
			aliases := normalizeLibraryAliases(*patch.Aliases)
			if err := claimAliases(ctx, tx, id, aliases); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM library_team_aliases WHERE team_id = ?`, id.String()); err != nil {
				return fmt.Errorf("clear aliases: %w", err)
			}
			if err := insertAliases(ctx, tx, id, aliases); err != nil {
				return err
			}
			if err := adoptAutoMatches(ctx, tx, id, aliasKeys(aliases)); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `UPDATE library_teams SET updated_at = ? WHERE id = ?`, millis(libraryNow()), id.String())
		return err
	})
	if err != nil {
		return library.Team{}, err
	}
	return r.GetTeam(ctx, id)
}

func (r *SQLiteLibraryRepository) MergeTeams(ctx context.Context, from, into uuid.UUID) (library.Team, error) {
	if from == into {
		return library.Team{}, library.ErrConflict
	}
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		if err := r.teamExists(ctx, tx, from); err != nil {
			return err
		}
		if err := r.teamExists(ctx, tx, into); err != nil {
			return err
		}
		// A match between the two teams cannot keep both sides on one team.
		if _, err := tx.ExecContext(ctx, `UPDATE library_match_teams SET team_id = NULL, assignment = ?
			WHERE team_id = ? AND EXISTS (SELECT 1 FROM library_match_teams o WHERE o.match_id = library_match_teams.match_id
				AND o.team_key <> library_match_teams.team_key AND o.team_id = ?)`,
			library.AssignmentNone, from.String(), into.String()); err != nil {
			return fmt.Errorf("clear head to head sides: %w", err)
		}
		for _, stmt := range []string{
			`UPDATE library_match_teams SET team_id = ? WHERE team_id = ?`,
			`UPDATE library_team_aliases SET team_id = ? WHERE team_id = ?`,
			`UPDATE library_reports SET team_id = ? WHERE team_id = ?`,
			`UPDATE library_plays SET team_id = ? WHERE team_id = ?`,
			`UPDATE library_series SET team_a = ? WHERE team_a = ?`,
			`UPDATE library_series SET team_b = ? WHERE team_b = ?`,
		} {
			if _, err := tx.ExecContext(ctx, stmt, into.String(), from.String()); err != nil {
				return fmt.Errorf("merge team: %w", err)
			}
		}
		// A series between the two teams keeps one slot of the merged team.
		if _, err := tx.ExecContext(ctx, `UPDATE library_series SET team_b = NULL WHERE team_a = team_b`); err != nil {
			return fmt.Errorf("merge series slots: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM library_teams WHERE id = ?`, from.String()); err != nil {
			return fmt.Errorf("delete merged team: %w", err)
		}
		_, err := tx.ExecContext(ctx, `UPDATE library_teams SET updated_at = ? WHERE id = ?`, millis(libraryNow()), into.String())
		return err
	})
	if err != nil {
		return library.Team{}, err
	}
	return r.GetTeam(ctx, into)
}

const libraryPlayerColumns = `SELECT p.steam_id, p.nick, p.faceit_player_id, p.faceit_nickname, p.created_at, p.updated_at,
	(SELECT COUNT(*) FROM library_match_players mp WHERE mp.steam_id = p.steam_id) FROM library_players p`

func scanLibraryPlayer(row sqlScanner) (library.Player, error) {
	var p library.Player
	var created, updated int64
	if err := row.Scan(&p.SteamID64, &p.Nick, &p.FaceitPlayerID, &p.FaceitNickname, &created, &updated, &p.MatchCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Player{}, library.ErrNotFound
		}
		return library.Player{}, err
	}
	p.CreatedAt, p.UpdatedAt = fromMillis(created), fromMillis(updated)
	return p, nil
}

func (r *SQLiteLibraryRepository) GetPlayer(ctx context.Context, steamID string) (library.Player, error) {
	p, err := scanLibraryPlayer(r.db.QueryRowContext(ctx, libraryPlayerColumns+` WHERE p.steam_id = ?`, steamID))
	if err != nil {
		return library.Player{}, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT mp.name, MIN(m.played_at), MAX(m.played_at)
		FROM library_match_players mp JOIN library_matches m ON m.id = mp.match_id
		WHERE mp.steam_id = ? GROUP BY mp.name ORDER BY MAX(m.played_at) DESC, mp.name ASC`, steamID)
	if err != nil {
		return library.Player{}, fmt.Errorf("query nicks: %w", err)
	}
	p.Nicks = []library.PlayerNick{}
	for rows.Next() {
		var n library.PlayerNick
		var first, last int64
		if err := rows.Scan(&n.Nick, &first, &last); err != nil {
			_ = rows.Close()
			return library.Player{}, err
		}
		n.FirstSeenAt, n.LastSeenAt = fromMillis(first), fromMillis(last)
		p.Nicks = append(p.Nicks, n)
	}
	if err := rows.Close(); err != nil {
		return library.Player{}, err
	}
	rows, err = r.db.QueryContext(ctx, `SELECT mt.team_id, t.name, MIN(m.played_at), MAX(m.played_at), COUNT(*)
		FROM library_match_players mp
		JOIN library_match_teams mt ON mt.match_id = mp.match_id AND mt.team_key = mp.team_key
		JOIN library_matches m ON m.id = mp.match_id
		JOIN library_teams t ON t.id = mt.team_id
		WHERE mp.steam_id = ?
		GROUP BY mt.team_id ORDER BY MAX(m.played_at) DESC, mt.team_id ASC`, steamID)
	if err != nil {
		return library.Player{}, fmt.Errorf("query player teams: %w", err)
	}
	defer rows.Close()
	p.Teams = []library.PlayerTeam{}
	for rows.Next() {
		var pt library.PlayerTeam
		var teamID string
		var first, last int64
		if err := rows.Scan(&teamID, &pt.TeamName, &first, &last, &pt.Matches); err != nil {
			return library.Player{}, err
		}
		if pt.TeamID, err = uuid.Parse(teamID); err != nil {
			return library.Player{}, err
		}
		pt.FirstPlayedAt, pt.LastPlayedAt = fromMillis(first), fromMillis(last)
		p.Teams = append(p.Teams, pt)
	}
	return p, rows.Err()
}

func (r *SQLiteLibraryRepository) ListPlayers(ctx context.Context, filter library.PlayerFilter) ([]library.Player, error) {
	var where []string
	var args []any
	if q := strings.ToLower(strings.TrimSpace(filter.Query)); q != "" {
		where = append(where, `(p.nick_key LIKE ? ESCAPE '\' OR p.steam_id = ?)`)
		args = append(args, likeContains(q), q)
	}
	if filter.TeamID != nil {
		where = append(where, `EXISTS (SELECT 1 FROM library_match_players mp
			JOIN library_match_teams mt ON mt.match_id = mp.match_id AND mt.team_key = mp.team_key
			WHERE mp.steam_id = p.steam_id AND mt.team_id = ?)`)
		args = append(args, filter.TeamID.String())
	}
	query := libraryPlayerColumns
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, ` AND `)
	}
	query += ` ORDER BY p.nick_key, p.steam_id LIMIT ? OFFSET ?`
	args = append(args, library.ClampLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list players: %w", err)
	}
	defer rows.Close()
	players := []library.Player{}
	for rows.Next() {
		p, err := scanLibraryPlayer(rows)
		if err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

func (r *SQLiteLibraryRepository) SetPlayerFaceit(ctx context.Context, steamID, faceitPlayerID, faceitNickname string) (library.Player, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE library_players SET faceit_player_id = ?, faceit_nickname = ?, updated_at = ? WHERE steam_id = ?`,
		faceitPlayerID, faceitNickname, millis(libraryNow()), steamID)
	if err != nil {
		return library.Player{}, fmt.Errorf("set player faceit: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return library.Player{}, err
	} else if n == 0 {
		return library.Player{}, library.ErrNotFound
	}
	return r.GetPlayer(ctx, steamID)
}
