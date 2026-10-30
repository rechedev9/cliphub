package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vodfetch"
)

// sqlExecer is the slice of *sql.DB and *sql.Tx the schema helpers need, so a
// step can run inside the migration transaction and the same helper can
// inspect a live connection in tests.
type sqlExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
}

// migrations is the ordered schema history of jobs.db. PRAGMA user_version
// records how many steps have run; a step never runs twice and a newer binary
// never edits an older step. Step 1 reproduces the schema every pre-versioned
// database already has, including the ad hoc repairs that used to run on every
// open, so a database with user_version 0 and populated tables converges
// without data movement.
var migrations = []func(tx *sql.Tx) error{
	migrateV1Baseline,
	migrateV2Constraints,
	migrateV3Library,
	migrateV4LibraryJobs,
	migrateV5LibraryReports,
	migrateV6LibraryAnnotations,
	migrateV7LibraryPlays,
	migrateV8LibrarySeriesVeto,
}

// sqliteMaxOpenConns is the connection pool size. WAL lets readers run while
// one writer commits, so Studio's status polls no longer queue behind a worker
// transaction; SQLite itself still serializes writers.
const sqliteMaxOpenConns = 4

// openSQLite opens (creating if needed) the SQLite database at path and brings
// its schema to the current version. Every transaction begins IMMEDIATE, so a
// read-modify-write transaction holds the write lock from its first statement
// and two writers wait on busy_timeout instead of deadlocking on lock upgrade.
func openSQLite(path string) (*sql.DB, error) {
	// Pragmas travel in the DSN so the driver re-applies them on every
	// connection: database/sql replaces a connection the driver marks bad
	// (an interrupted statement does that), and foreign_keys / busy_timeout
	// are per-connection state that a db.Exec on open would not carry over.
	// Without foreign_keys the job_kill_plans cascade is decorative.
	db, err := sql.Open("sqlite", path+"?"+strings.Join([]string{
		"_pragma=busy_timeout(5000)",
		"_pragma=journal_mode(WAL)",
		"_pragma=synchronous(NORMAL)",
		"_pragma=foreign_keys(1)",
		"_txlock=immediate",
	}, "&"))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// An in-memory database is private to its connection: a pool would open
	// several empty databases, so it keeps the single connection.
	if strings.Contains(path, ":memory:") || strings.Contains(path, "mode=memory") {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(sqliteMaxOpenConns)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func schemaVersion(q sqlExecer) (int, error) {
	rows, err := q.Query(`PRAGMA user_version`)
	if err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	defer rows.Close()
	version := 0
	if rows.Next() {
		if err := rows.Scan(&version); err != nil {
			return 0, fmt.Errorf("scan schema version: %w", err)
		}
	}
	return version, rows.Err()
}

// migrate applies every step past the recorded version, each in its own
// transaction together with the version bump, so a crash mid-step leaves the
// database at the previous version rather than half-migrated.
func migrate(db *sql.DB) error {
	version, err := schemaVersion(db)
	if err != nil {
		return err
	}
	if version > len(migrations) {
		return fmt.Errorf("jobs.db schema version %d is newer than this build supports (%d)", version, len(migrations))
	}
	for step := version; step < len(migrations); step++ {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin schema migration %d: %w", step+1, err)
		}
		if err := migrations[step](tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("schema migration %d: %w", step+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", step+1)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record schema version %d: %w", step+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit schema migration %d: %w", step+1, err)
		}
	}
	return nil
}

func columnExists(q sqlExecer, table, column string) (bool, error) {
	rows, err := q.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, fmt.Errorf("scan %s column: %w", table, err)
		}
		found = found || name == column
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate %s columns: %w", table, err)
	}
	return found, nil
}

// migrateV1Baseline is the pre-versioned schema plus the repairs that every
// open used to perform: the jobs row without an embedded kill plan, the
// stream_jobs columns added over time, and the split of legacy stream source
// URLs into private/public halves.
func migrateV1Baseline(tx *sql.Tx) error {
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS jobs (
			id         TEXT PRIMARY KEY,
			data       BLOB    NOT NULL,
			status     TEXT    NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS job_kill_plans (
			job_id TEXT PRIMARY KEY,
			plan   BLOB NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS stream_jobs (
			id             TEXT    PRIMARY KEY,
			status         TEXT    NOT NULL,
			failure_reason TEXT,
			failure_code   TEXT,
			source_path    TEXT    NOT NULL,
			source_sha256  TEXT    NOT NULL,
			source_url     TEXT,
			public_source_url TEXT,
			title          TEXT,
			probe          TEXT    NOT NULL,
			edit_plan      TEXT,
			created_at     INTEGER NOT NULL,
			updated_at     INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS editor_assets (
			id TEXT PRIMARY KEY,
			sha256 TEXT NOT NULL,
			file_name TEXT NOT NULL,
			origin TEXT NOT NULL,
			origin_job_id TEXT,
			origin_variant TEXT,
			origin_name TEXT,
			probe TEXT NOT NULL,
			media_key TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS editor_projects (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			failure_reason TEXT NOT NULL DEFAULT '',
			plan_json TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("create baseline table: %w", err)
		}
	}
	if err := migrateKillPlansOffJobsRow(tx); err != nil {
		return err
	}
	for _, column := range []string{"public_source_url", "failure_code"} {
		found, err := columnExists(tx, "stream_jobs", column)
		if err != nil {
			return err
		}
		if !found {
			if _, err := tx.Exec(`ALTER TABLE stream_jobs ADD COLUMN ` + column + ` TEXT`); err != nil {
				return fmt.Errorf("add stream_jobs %s: %w", column, err)
			}
		}
	}
	return migrateStreamSourceURLs(tx)
}

// migrateKillPlansOffJobsRow moves a kill plan out of the jobs row — from a
// leftover `kill_plan` column or an embedded `data.kill_plan` — into
// job_kill_plans, then strips both sources. Idempotent: a second run is a
// no-op once the sibling table holds the plan and `data` has no `$.kill_plan`.
func migrateKillPlansOffJobsRow(tx *sql.Tx) error {
	hasColumn, err := columnExists(tx, "jobs", "kill_plan")
	if err != nil {
		return err
	}
	if hasColumn {
		if _, err := tx.Exec(`
			INSERT INTO job_kill_plans (job_id, plan)
			SELECT id, kill_plan FROM jobs
			WHERE kill_plan IS NOT NULL
			  AND id NOT IN (SELECT job_id FROM job_kill_plans)`); err != nil {
			return fmt.Errorf("move kill_plan column: %w", err)
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO job_kill_plans (job_id, plan)
		SELECT id, json_extract(data, '$.kill_plan')
		FROM jobs
		WHERE json_extract(data, '$.kill_plan') IS NOT NULL
		  AND id NOT IN (SELECT job_id FROM job_kill_plans)`); err != nil {
		return fmt.Errorf("extract embedded kill plan: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE jobs
		SET data = json_remove(data, '$.kill_plan')
		WHERE json_type(data, '$.kill_plan') IS NOT NULL`); err != nil {
		return fmt.Errorf("strip embedded kill plan: %w", err)
	}
	if hasColumn {
		if _, err := tx.Exec(`UPDATE jobs SET kill_plan = NULL WHERE kill_plan IS NOT NULL`); err != nil {
			return fmt.Errorf("clear jobs kill_plan column: %w", err)
		}
	}
	return nil
}

// migrateStreamSourceURLs splits a legacy single `source_url` into the private
// acquisition URL (kept only while acquiring) and the credential-free public
// URL. Rows whose URL fails the current source policy lose it; an acquiring
// row cannot proceed without one and is failed with a stable reason.
func migrateStreamSourceURLs(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id, status, COALESCE(source_url,'') FROM stream_jobs WHERE COALESCE(source_url,'') <> ''`)
	if err != nil {
		return fmt.Errorf("query legacy stream source urls: %w", err)
	}
	type legacySource struct {
		id, status, privateURL string
	}
	var sources []legacySource
	for rows.Next() {
		var source legacySource
		if err := rows.Scan(&source.id, &source.status, &source.privateURL); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan legacy stream source url: %w", err)
		}
		sources = append(sources, source)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close legacy stream source rows: %w", err)
	}

	for _, legacy := range sources {
		source, validationErr := vodfetch.ValidateSource(legacy.privateURL)
		if validationErr != nil {
			_, err = tx.Exec(
				`UPDATE stream_jobs SET source_url = NULL, public_source_url = NULL,
				 status = CASE WHEN status = ? THEN ? ELSE status END,
				 failure_reason = CASE WHEN status = ? THEN ? ELSE failure_reason END
				 WHERE id = ?`,
				string(streamclips.StatusAcquiring), string(streamclips.StatusFailed),
				string(streamclips.StatusAcquiring), "legacy source URL rejected by current security policy",
				legacy.id,
			)
		} else {
			privateURL := any(nil)
			if legacy.status == string(streamclips.StatusAcquiring) {
				privateURL = source.AcquisitionURL
			}
			_, err = tx.Exec(
				`UPDATE stream_jobs SET source_url = ?, public_source_url = ? WHERE id = ?`,
				privateURL, nullableText(source.PublicURL), legacy.id,
			)
		}
		if err != nil {
			return fmt.Errorf("migrate stream source url for %s: %w", legacy.id, err)
		}
	}
	return nil
}

// migrateV2Constraints adds what the baseline never had: a real foreign key
// from kill plans to jobs (SQLite cannot add one in place, so the small table
// is rebuilt and orphans dropped), the leftover jobs.kill_plan column removed,
// and indexes for every ordering and filter the repositories issue.
func migrateV2Constraints(tx *sql.Tx) error {
	for _, stmt := range []string{
		`CREATE TABLE job_kill_plans_v2 (
			job_id TEXT PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
			plan   BLOB NOT NULL
		)`,
		`INSERT INTO job_kill_plans_v2 (job_id, plan)
		 SELECT job_id, plan FROM job_kill_plans
		 WHERE job_id IN (SELECT id FROM jobs)`,
		`DROP TABLE job_kill_plans`,
		`ALTER TABLE job_kill_plans_v2 RENAME TO job_kill_plans`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("rebuild job_kill_plans with foreign key: %w", err)
		}
	}
	hasLegacyColumn, err := columnExists(tx, "jobs", "kill_plan")
	if err != nil {
		return err
	}
	if hasLegacyColumn {
		if _, err := tx.Exec(`ALTER TABLE jobs DROP COLUMN kill_plan`); err != nil {
			return fmt.Errorf("drop legacy jobs.kill_plan column: %w", err)
		}
	}
	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS jobs_recent ON jobs(updated_at DESC, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS jobs_status_recent ON jobs(status, updated_at DESC, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS jobs_series ON jobs(json_extract(data, '$.series_id'), created_at ASC, id ASC)`,
		`CREATE INDEX IF NOT EXISTS stream_jobs_recent ON stream_jobs(updated_at DESC, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS stream_jobs_status_recent ON stream_jobs(status, updated_at DESC, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS editor_assets_recent ON editor_assets(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS editor_assets_sha256 ON editor_assets(sha256, created_at ASC)`,
		`CREATE INDEX IF NOT EXISTS editor_projects_recent ON editor_projects(updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS editor_projects_status_recent ON editor_projects(status, updated_at DESC, id ASC)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}
	return nil
}

// migrateV3Library adds the shared demo library: demos keyed by file hash, the
// match each one holds, series, teams with their aliases, players, and the
// match to team and match to player links. Rosters, nick history and team
// history are derived from the links, so they never drift from the matches.
func migrateV3Library(tx *sql.Tx) error {
	for _, stmt := range []string{
		`CREATE TABLE library_demos (
			id            TEXT    PRIMARY KEY,
			sha256        TEXT    NOT NULL UNIQUE,
			file_name     TEXT    NOT NULL,
			path          TEXT    NOT NULL,
			size_bytes    INTEGER NOT NULL,
			source        TEXT    NOT NULL,
			source_manual INTEGER NOT NULL DEFAULT 0,
			cs2_build     INTEGER NOT NULL DEFAULT 0,
			patch_version INTEGER NOT NULL DEFAULT 0,
			state         TEXT    NOT NULL,
			error_code    TEXT    NOT NULL DEFAULT '',
			error         TEXT    NOT NULL DEFAULT '',
			played_at     INTEGER,
			imported_at   INTEGER NOT NULL,
			updated_at    INTEGER NOT NULL
		)`,
		`CREATE TABLE library_demo_tags (
			demo_id TEXT NOT NULL REFERENCES library_demos(id) ON DELETE CASCADE,
			tag_key TEXT NOT NULL,
			tag     TEXT NOT NULL,
			PRIMARY KEY (demo_id, tag_key)
		)`,
		`CREATE TABLE library_series (
			id         TEXT    PRIMARY KEY,
			format     TEXT    NOT NULL,
			event      TEXT    NOT NULL DEFAULT '',
			veto       TEXT    NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE library_matches (
			id             TEXT    PRIMARY KEY,
			demo_id        TEXT    NOT NULL UNIQUE REFERENCES library_demos(id) ON DELETE CASCADE,
			series_id      TEXT    REFERENCES library_series(id) ON DELETE SET NULL,
			map            TEXT    NOT NULL,
			played_at      INTEGER NOT NULL,
			tickrate       REAL    NOT NULL,
			duration_ticks INTEGER NOT NULL,
			round_count    INTEGER NOT NULL,
			created_at     INTEGER NOT NULL,
			updated_at     INTEGER NOT NULL
		)`,
		`CREATE TABLE library_teams (
			id         TEXT    PRIMARY KEY,
			name       TEXT    NOT NULL,
			name_key   TEXT    NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE library_team_aliases (
			alias_key TEXT PRIMARY KEY,
			alias     TEXT NOT NULL,
			team_id   TEXT NOT NULL REFERENCES library_teams(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE library_match_teams (
			match_id   TEXT    NOT NULL REFERENCES library_matches(id) ON DELETE CASCADE,
			team_key   TEXT    NOT NULL,
			start_side TEXT    NOT NULL,
			clan_name  TEXT    NOT NULL,
			clan_key   TEXT    NOT NULL,
			team_id    TEXT    REFERENCES library_teams(id) ON DELETE SET NULL,
			assignment TEXT    NOT NULL,
			score      INTEGER NOT NULL,
			PRIMARY KEY (match_id, team_key)
		)`,
		`CREATE TABLE library_players (
			steam_id         TEXT    PRIMARY KEY,
			nick             TEXT    NOT NULL,
			nick_key         TEXT    NOT NULL,
			faceit_player_id TEXT    NOT NULL DEFAULT '',
			faceit_nickname  TEXT    NOT NULL DEFAULT '',
			created_at       INTEGER NOT NULL,
			updated_at       INTEGER NOT NULL
		)`,
		`CREATE TABLE library_match_players (
			match_id TEXT    NOT NULL REFERENCES library_matches(id) ON DELETE CASCADE,
			steam_id TEXT    NOT NULL REFERENCES library_players(steam_id),
			team_key TEXT    NOT NULL,
			slot     INTEGER NOT NULL,
			name     TEXT    NOT NULL,
			rounds   INTEGER NOT NULL,
			kills    INTEGER NOT NULL,
			deaths   INTEGER NOT NULL,
			assists  INTEGER NOT NULL,
			damage   INTEGER NOT NULL,
			PRIMARY KEY (match_id, steam_id)
		)`,
		`CREATE INDEX library_demos_recent ON library_demos(imported_at DESC, id ASC)`,
		`CREATE INDEX library_demos_state ON library_demos(state, imported_at DESC)`,
		`CREATE INDEX library_demo_tags_key ON library_demo_tags(tag_key, demo_id)`,
		`CREATE INDEX library_matches_recent ON library_matches(played_at DESC, id ASC)`,
		`CREATE INDEX library_matches_map ON library_matches(map, played_at DESC)`,
		`CREATE INDEX library_matches_series ON library_matches(series_id, played_at ASC)`,
		`CREATE INDEX library_teams_name ON library_teams(name_key, id)`,
		`CREATE INDEX library_team_aliases_team ON library_team_aliases(team_id)`,
		`CREATE INDEX library_match_teams_team ON library_match_teams(team_id, match_id)`,
		`CREATE INDEX library_match_teams_clan ON library_match_teams(clan_key)`,
		`CREATE INDEX library_players_nick ON library_players(nick_key, steam_id)`,
		`CREATE INDEX library_match_players_steam ON library_match_players(steam_id, match_id)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("create library schema: %w", err)
		}
	}
	return nil
}

// migrateV4LibraryJobs links Crear jobs to the library demo they were made
// from, so a match lists its videos and a demo used in Crear shows up in the
// library. A job has at most one demo; deleting the demo drops its links.
func migrateV4LibraryJobs(tx *sql.Tx) error {
	for _, stmt := range []string{
		`CREATE TABLE library_demo_jobs (
			job_id    TEXT    PRIMARY KEY,
			demo_id   TEXT    NOT NULL REFERENCES library_demos(id) ON DELETE CASCADE,
			linked_at INTEGER NOT NULL
		)`,
		`CREATE INDEX library_demo_jobs_demo ON library_demo_jobs(demo_id, linked_at, job_id)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("create library job links: %w", err)
		}
	}
	return nil
}

// migrateV5LibraryReports stores the scouting reports: a rival team, the
// matches the report reads (a JSON array of match ids) and the side it looks
// at. The sections are not stored; they are computed from the documents.
func migrateV5LibraryReports(tx *sql.Tx) error {
	for _, stmt := range []string{
		`CREATE TABLE library_reports (
			id         TEXT    PRIMARY KEY,
			title      TEXT    NOT NULL,
			team_id    TEXT    NOT NULL REFERENCES library_teams(id) ON DELETE CASCADE,
			match_ids  TEXT    NOT NULL,
			side       TEXT    NOT NULL DEFAULT '',
			notes      TEXT    NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE INDEX library_reports_team ON library_reports(team_id, updated_at)`,
		`CREATE INDEX library_reports_updated ON library_reports(updated_at, id)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("create library reports: %w", err)
		}
	}
	return nil
}

// migrateV6LibraryAnnotations adds the replay notes: a text and a drawing in
// world units, anchored to a tick of a round of a match. A re-analysis keeps
// the match id, so its notes survive; deleting the demo deletes them.
func migrateV6LibraryAnnotations(tx *sql.Tx) error {
	for _, stmt := range []string{
		`CREATE TABLE library_annotations (
			id         TEXT    PRIMARY KEY,
			match_id   TEXT    NOT NULL REFERENCES library_matches(id) ON DELETE CASCADE,
			round      INTEGER NOT NULL CHECK (round > 0),
			tick       INTEGER NOT NULL CHECK (tick >= 0),
			text       TEXT    NOT NULL DEFAULT '',
			strokes    TEXT    NOT NULL DEFAULT '[]',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE INDEX library_annotations_match ON library_annotations(match_id, round, tick)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("create library annotations: %w", err)
		}
	}
	return nil
}

// migrateV7LibraryPlays stores the stratbook: a play's map, side and kind,
// its grenades, positions and example rounds as JSON, and the team that runs
// it. A play outlives its team (the link is cleared) and its example matches.
func migrateV7LibraryPlays(tx *sql.Tx) error {
	for _, stmt := range []string{
		`CREATE TABLE library_plays (
			id                TEXT    PRIMARY KEY,
			title             TEXT    NOT NULL,
			team_id           TEXT    REFERENCES library_teams(id) ON DELETE SET NULL,
			map               TEXT    NOT NULL,
			side              TEXT    NOT NULL CHECK (side IN ('CT', 'T')),
			kind              TEXT    NOT NULL,
			site              TEXT    NOT NULL DEFAULT 'none',
			origin            TEXT    NOT NULL,
			positions_seconds REAL    NOT NULL DEFAULT 0,
			grenades          TEXT    NOT NULL DEFAULT '[]',
			positions         TEXT    NOT NULL DEFAULT '[]',
			examples          TEXT    NOT NULL DEFAULT '[]',
			notes             TEXT    NOT NULL DEFAULT '',
			created_at        INTEGER NOT NULL,
			updated_at        INTEGER NOT NULL
		)`,
		`CREATE INDEX library_plays_updated ON library_plays(updated_at, id)`,
		`CREATE INDEX library_plays_team ON library_plays(team_id, updated_at)`,
		`CREATE INDEX library_plays_map ON library_plays(map, side)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("create library plays: %w", err)
		}
	}
	return nil
}

// migrateV8LibrarySeriesVeto gives a series its two teams, by slot, and the
// known veto order as JSON. Deleting a team clears its slot; the steps keep
// the slot letter, so a merge only has to move the slot.
func migrateV8LibrarySeriesVeto(tx *sql.Tx) error {
	for _, stmt := range []string{
		`ALTER TABLE library_series ADD COLUMN team_a TEXT REFERENCES library_teams(id) ON DELETE SET NULL`,
		`ALTER TABLE library_series ADD COLUMN team_b TEXT REFERENCES library_teams(id) ON DELETE SET NULL`,
		`ALTER TABLE library_series ADD COLUMN veto_steps TEXT NOT NULL DEFAULT '[]'`,
		`CREATE INDEX library_series_team_a ON library_series(team_a)`,
		`CREATE INDEX library_series_team_b ON library_series(team_b)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("add series veto: %w", err)
		}
	}
	return nil
}
