package store

import (
	"database/sql"
	"fmt"
)

// migrateV32DropDemoVoiceEnglish drops the Whisper English translation of the demo voices that Studio 5.25.0 stored.
func migrateV32DropDemoVoiceEnglish(tx *sql.Tx) error {
	for _, c := range []struct{ table, column string }{
		{"library_demo_voice", "english"}, {"library_demo_voice", "english_error"}, {"library_demo_voice_lines", "english"},
	} {
		found, err := columnExists(tx, c.table, c.column)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		if _, err := tx.Exec(`ALTER TABLE ` + c.table + ` DROP COLUMN ` + c.column); err != nil {
			return fmt.Errorf("drop %s.%s: %w", c.table, c.column, err)
		}
	}
	return nil
}
