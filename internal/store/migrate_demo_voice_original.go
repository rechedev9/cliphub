package store

import (
	"database/sql"
	"fmt"
)

func migrateV33DemoVoiceOriginal(tx *sql.Tx) error {
	for _, c := range []struct{ column, definition string }{
		{"original_text", `TEXT NOT NULL DEFAULT ''`},
		{"discarded", `INTEGER NOT NULL DEFAULT 0`},
	} {
		found, err := columnExists(tx, "library_demo_voice_lines", c.column)
		if err != nil {
			return err
		}
		if found {
			continue
		}
		if _, err := tx.Exec(`ALTER TABLE library_demo_voice_lines ADD COLUMN ` + c.column + ` ` + c.definition); err != nil {
			return fmt.Errorf("add library_demo_voice_lines.%s: %w", c.column, err)
		}
	}
	return nil
}
