package store

import (
	"database/sql"
	"fmt"
)

func migrateV38VoiceCorrectedBy(tx *sql.Tx) error {
	for _, table := range []string{"library_demo_voice_lines", "stream_voice_lines"} {
		found, err := columnExists(tx, table, "corrected_by")
		if err != nil {
			return err
		}
		if found {
			continue
		}
		if _, err := tx.Exec(`ALTER TABLE ` + table + ` ADD COLUMN corrected_by TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add %s.corrected_by: %w", table, err)
		}
		if _, err := tx.Exec(`UPDATE ` + table + ` SET corrected_by = 'user' WHERE corrected = 1`); err != nil {
			return fmt.Errorf("mark the corrections of %s as the user's: %w", table, err)
		}
	}
	return nil
}
