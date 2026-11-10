package store

import (
	"database/sql"
	"fmt"
)

// migrateV23LibraryVoiceRecordings stores the TeamSpeak plugin's recordings, with the match they
// were added to, and each team's speakers: who a TeamSpeak identity is in CS2.
func migrateV23LibraryVoiceRecordings(tx *sql.Tx) error {
	for _, stmt := range []string{
		`CREATE TABLE library_voice_recordings (
			id            TEXT    PRIMARY KEY,
			started_at    INTEGER NOT NULL,
			ended_at      INTEGER NOT NULL,
			map           TEXT    NOT NULL DEFAULT '',
			server        TEXT    NOT NULL DEFAULT '',
			channel       TEXT    NOT NULL DEFAULT '',
			trigger       TEXT    NOT NULL DEFAULT '',
			self_steamid  TEXT    NOT NULL DEFAULT '',
			score_ct      INTEGER NOT NULL DEFAULT -1,
			score_t       INTEGER NOT NULL DEFAULT -1,
			interrupted   INTEGER NOT NULL DEFAULT 0,
			tracks        TEXT    NOT NULL DEFAULT '[]',
			rounds        TEXT    NOT NULL DEFAULT '[]',
			match_id      TEXT    REFERENCES library_matches(id) ON DELETE SET NULL,
			note          TEXT    NOT NULL DEFAULT '',
			created_at    INTEGER NOT NULL,
			updated_at    INTEGER NOT NULL
		)`,
		`CREATE INDEX library_voice_recordings_match ON library_voice_recordings(match_id)`,
		`CREATE TABLE library_voice_speakers (
			team_id     TEXT    NOT NULL REFERENCES library_teams(id) ON DELETE CASCADE,
			speaker_key TEXT    NOT NULL,
			nickname    TEXT    NOT NULL DEFAULT '',
			steamid64   TEXT    NOT NULL,
			updated_at  INTEGER NOT NULL,
			PRIMARY KEY (team_id, speaker_key)
		)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("library voice recordings: %w", err)
		}
	}
	return nil
}
