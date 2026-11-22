package store

import (
	"database/sql"
	"fmt"
)

func migrateV36DemoVoiceVocab(tx *sql.Tx) error {
	found, err := columnExists(tx, "library_demo_voice_lines", "vocab_text")
	if err != nil || found {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE library_demo_voice_lines ADD COLUMN vocab_text TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("demo voice vocab text: %w", err)
	}
	return nil
}
