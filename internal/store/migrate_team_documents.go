package store

import (
	"database/sql"
	"fmt"
)

// migrateV27TeamDocuments stores a team's documents, such as an HTML anti-strat. They stay on each device:
// the sync batches rows by count under an 8 MiB body cap, and one document can reach 5 MiB.
func migrateV27TeamDocuments(tx *sql.Tx) error {
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS library_team_documents (
			id           TEXT    PRIMARY KEY,
			team_id      TEXT    NOT NULL REFERENCES library_teams(id) ON DELETE CASCADE,
			title        TEXT    NOT NULL,
			file_name    TEXT    NOT NULL DEFAULT '',
			content_type TEXT    NOT NULL,
			size_bytes   INTEGER NOT NULL,
			sha256       TEXT    NOT NULL,
			content      BLOB    NOT NULL,
			created_at   INTEGER NOT NULL,
			updated_at   INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS library_team_documents_team ON library_team_documents(team_id, updated_at)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("library team documents: %w", err)
		}
	}
	return nil
}
