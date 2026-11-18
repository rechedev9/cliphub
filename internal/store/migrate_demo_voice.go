package store

import (
	"database/sql"
	"fmt"
)

// migrateV31DemoVoice stores the transcript of the voices a demo recorded. Studio only: the
// tables are not in syncTables, so ClipMind never receives them.
func migrateV31DemoVoice(tx *sql.Tx) error {
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS library_demo_voice (
			match_id      TEXT    PRIMARY KEY REFERENCES library_matches(id) ON DELETE CASCADE,
			state         TEXT    NOT NULL,
			model         TEXT    NOT NULL DEFAULT '',
			error         TEXT    NOT NULL DEFAULT '',
			english       TEXT    NOT NULL DEFAULT 'none',
			english_error TEXT    NOT NULL DEFAULT '',
			updated_at    INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS library_demo_voice_lines (
			id        TEXT    PRIMARY KEY,
			match_id  TEXT    NOT NULL REFERENCES library_demo_voice(match_id) ON DELETE CASCADE,
			steamid64 TEXT    NOT NULL DEFAULT '',
			name      TEXT    NOT NULL DEFAULT '',
			team_key  TEXT    NOT NULL DEFAULT '',
			start_ms  INTEGER NOT NULL,
			end_ms    INTEGER NOT NULL,
			tick      INTEGER NOT NULL DEFAULT 0,
			round     INTEGER NOT NULL DEFAULT 0,
			text      TEXT    NOT NULL,
			text_key  TEXT    NOT NULL,
			corrected INTEGER NOT NULL DEFAULT 0,
			english   TEXT    NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS library_demo_voice_lines_time ON library_demo_voice_lines(match_id, start_ms, steamid64)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("demo voice: %w", err)
		}
	}
	return nil
}
