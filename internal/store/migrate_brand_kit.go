package store

import (
	"database/sql"
	"fmt"
)

// migrateV29BrandKit stores the one brand kit of this device (not in
// syncTables). Empty image columns mean the image is not set.
func migrateV29BrandKit(tx *sql.Tx) error {
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS brand_kit (
		id                   INTEGER PRIMARY KEY CHECK (id = 1),
		logo_id              TEXT    NOT NULL DEFAULT '',
		logo_sha256          TEXT    NOT NULL DEFAULT '',
		watermark_corner     TEXT    NOT NULL,
		watermark_opacity    REAL    NOT NULL,
		banner_id            TEXT    NOT NULL DEFAULT '',
		banner_sha256        TEXT    NOT NULL DEFAULT '',
		banner_start_seconds REAL    NOT NULL DEFAULT 0,
		banner_end_seconds   REAL    NOT NULL DEFAULT 0,
		updated_at           INTEGER NOT NULL
	)`); err != nil {
		return fmt.Errorf("brand kit: %w", err)
	}
	return nil
}

func migrateV39BrandKitChannelName(tx *sql.Tx) error {
	found, err := columnExists(tx, "brand_kit", "channel_name")
	if err != nil || found {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE brand_kit ADD COLUMN channel_name TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("brand kit channel name: %w", err)
	}
	return nil
}
