package store

import (
	"database/sql"
	"fmt"
)

func migrateV37StreamVoiceVocab(tx *sql.Tx) error {
	found, err := columnExists(tx, "stream_voice_lines", "vocab_text")
	if err != nil || found {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE stream_voice_lines ADD COLUMN vocab_text TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("stream voice vocab text: %w", err)
	}
	return nil
}
