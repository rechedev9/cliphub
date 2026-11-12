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
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("create sync log: %w", err)
		}
	}
	for _, table := range syncTables {
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
	}
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
