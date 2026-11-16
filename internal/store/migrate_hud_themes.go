package store

import (
	"database/sql"
	"fmt"
)

// migrateV28HUDThemes stores the user's own Full Demo HUD themes. They stay on
// each device and are not in syncTables. A NULL radius keeps the base shape.
func migrateV28HUDThemes(tx *sql.Tx) error {
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS hud_themes (
		id         TEXT    PRIMARY KEY,
		base       TEXT    NOT NULL,
		name       TEXT    NOT NULL,
		ct         TEXT    NOT NULL,
		t          TEXT    NOT NULL,
		background TEXT    NOT NULL,
		radius     INTEGER,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`); err != nil {
		return fmt.Errorf("hud themes: %w", err)
	}
	return nil
}
