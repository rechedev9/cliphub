package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// migrateV26SyncLog records every change to a syncable table so another device
// can replay it. It does not backfill existing rows: a later full sync sends them.
func migrateV26SyncLog(tx *sql.Tx) error {
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS sync_changes (
			seq        INTEGER PRIMARY KEY AUTOINCREMENT,
			tbl        TEXT    NOT NULL,
			row_key    TEXT    NOT NULL,
			op         TEXT    NOT NULL CHECK (op IN ('upsert','delete')),
			changed_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS sync_changes_key ON sync_changes(tbl, row_key)`,
		`CREATE TABLE IF NOT EXISTS sync_meta (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		// Voice lines had no key; a track never holds two lines that start on the same millisecond.
		`DELETE FROM library_voice_lines WHERE rowid NOT IN (
			SELECT MAX(rowid) FROM library_voice_lines GROUP BY match_id, track, start_ms)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS library_voice_lines_key ON library_voice_lines(match_id, track, start_ms)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("create sync log: %w", err)
		}
	}
	for _, table := range syncTables {
		// Only hand-built test databases lack a library table; a real one has them all.
		var present int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table.name).Scan(&present); err != nil {
			return fmt.Errorf("inspect %s: %w", table.name, err)
		}
		if present == 0 {
			continue
		}
		for _, stmt := range syncTableTriggers(table) {
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("create sync trigger for %s: %w", table.name, err)
			}
		}
	}
	return nil
}

// syncTableTriggers returns the insert, update and delete triggers for one table.
func syncTableTriggers(t syncTable) []string {
	return []string{
		syncTriggerSQL(t, "ai", "AFTER INSERT", "upsert", "NEW"),
		syncTriggerSQL(t, "au", "AFTER UPDATE", "upsert", "NEW"),
		syncTriggerSQL(t, "ad", "AFTER DELETE", "delete", "OLD"),
		syncKeyChangeTriggerSQL(t),
	}
}

// syncKeyChangeTriggerSQL logs a delete of the old key when an update moves a row
// to another key, so the other device does not keep the row under both keys.
func syncKeyChangeTriggerSQL(t syncTable) string {
	oldCols := make([]string, len(t.keyColumns))
	changed := make([]string, len(t.keyColumns))
	for i, col := range t.keyColumns {
		oldCols[i] = "OLD." + col
		changed[i] = "OLD." + col + " IS NOT NEW." + col
	}
	return fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS %s BEFORE UPDATE ON %s
WHEN (%s) AND NOT EXISTS (SELECT 1 FROM sync_meta WHERE key = 'applying' AND value = '1')
BEGIN
	INSERT INTO sync_changes (tbl, row_key, op, changed_at)
	VALUES ('%s', json_array(%s), 'delete', CAST(unixepoch('subsec') * 1000 AS INTEGER));
END`,
		syncTriggerName(t.name, "ak"), t.name, strings.Join(changed, " OR "), t.name, strings.Join(oldCols, ", "))
}

// syncTriggerSQL builds one trigger; op is the recorded mutation and row the
// NEW. or OLD. alias whose columns make up the key.
func syncTriggerSQL(t syncTable, event, timing, op, row string) string {
	cols := make([]string, len(t.keyColumns))
	for i, col := range t.keyColumns {
		cols[i] = row + "." + col
	}
	return fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS %s %s ON %s
WHEN NOT EXISTS (SELECT 1 FROM sync_meta WHERE key = 'applying' AND value = '1')
BEGIN
	INSERT INTO sync_changes (tbl, row_key, op, changed_at)
	VALUES ('%s', json_array(%s), '%s', CAST(unixepoch('subsec') * 1000 AS INTEGER));
END`,
		syncTriggerName(t.name, event), timing, t.name, t.name, strings.Join(cols, ", "), op)
}

func syncTriggerName(table, event string) string {
	return "sync_" + table + "_" + event
}
