package store

import (
	"database/sql"
	"fmt"
)

func migrateV35StreamVoice(tx *sql.Tx) error {
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS stream_voice (
			job_id     TEXT    PRIMARY KEY REFERENCES stream_jobs(id) ON DELETE CASCADE,
			state      TEXT    NOT NULL,
			model      TEXT    NOT NULL DEFAULT '',
			error      TEXT    NOT NULL DEFAULT '',
			spans      TEXT    NOT NULL DEFAULT '[]',
			pending    TEXT    NOT NULL DEFAULT '[]',
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS stream_voice_lines (
			id            TEXT    PRIMARY KEY,
			job_id        TEXT    NOT NULL REFERENCES stream_jobs(id) ON DELETE CASCADE,
			start_ms      INTEGER NOT NULL,
			end_ms        INTEGER NOT NULL,
			text          TEXT    NOT NULL,
			corrected     INTEGER NOT NULL DEFAULT 0,
			original_text TEXT    NOT NULL DEFAULT '',
			discarded     INTEGER NOT NULL DEFAULT 0,
			words         TEXT    NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS stream_voice_lines_time ON stream_voice_lines(job_id, start_ms)`,
		`CREATE TABLE IF NOT EXISTS stream_subtitles (
			job_id  TEXT    PRIMARY KEY REFERENCES stream_jobs(id) ON DELETE CASCADE,
			shorts  INTEGER NOT NULL DEFAULT 0,
			summary INTEGER NOT NULL DEFAULT 0,
			hidden  TEXT    NOT NULL DEFAULT '{}'
		)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("stream voice: %w", err)
		}
	}
	return nil
}
