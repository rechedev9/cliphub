package store

import (
	"database/sql"
	"fmt"
)

func migrateV34DemoVoiceWords(tx *sql.Tx) error {
	found, err := columnExists(tx, "library_demo_voice_lines", "words")
	if err != nil || found {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE library_demo_voice_lines ADD COLUMN words TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("demo voice words: %w", err)
	}
	return nil
}
