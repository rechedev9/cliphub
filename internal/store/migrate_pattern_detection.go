package store

import (
	"database/sql"
	"fmt"
)

// migrateV24PatternDetection gives a saved pattern its category and a link to a Libreta play,
// and stores each pattern's detection per match and team, so a read never reruns the library.
// It runs again safely on a database rewound to an older version.
func migrateV24PatternDetection(tx *sql.Tx) error {
	for column, stmt := range map[string]string{
		"category": `ALTER TABLE library_patterns ADD COLUMN category TEXT NOT NULL DEFAULT ''`,
		"play_id":  `ALTER TABLE library_patterns ADD COLUMN play_id TEXT REFERENCES library_plays(id) ON DELETE SET NULL`,
	} {
		found, err := columnExists(tx, "library_patterns", column)
		if err != nil {
			return err
		}
		if found {
			continue
		}
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("pattern %s: %w", column, err)
		}
	}
	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS library_patterns_play ON library_patterns(play_id)`,
		`CREATE TABLE IF NOT EXISTS library_pattern_matches (
			pattern_id       TEXT    NOT NULL REFERENCES library_patterns(id) ON DELETE CASCADE,
			match_id         TEXT    NOT NULL REFERENCES library_matches(id) ON DELETE CASCADE,
			team_key         TEXT    NOT NULL,
			revision         TEXT    NOT NULL,
			match_updated_at INTEGER NOT NULL,
			played_at        INTEGER NOT NULL,
			matched          INTEGER NOT NULL,
			detail           TEXT    NOT NULL,
			PRIMARY KEY (pattern_id, match_id, team_key)
		)`,
		`CREATE INDEX IF NOT EXISTS library_pattern_matches_match ON library_pattern_matches(match_id, matched)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("pattern detection: %w", err)
		}
	}
	return nil
}
