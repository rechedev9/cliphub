package store

import (
	"database/sql"
	"fmt"
)

// migrateV30DemoTitle gives each library demo an optional title. The column is
// synced, so it changes the sync contract: Studio and ClipMind ship together.
func migrateV30DemoTitle(tx *sql.Tx) error {
	found, err := columnExists(tx, "library_demos", "title")
	if err != nil || found {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE library_demos ADD COLUMN title TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add demo title: %w", err)
	}
	return nil
}
