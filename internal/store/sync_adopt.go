package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// SyncAdoption is a local demo or match that took the id its twin has on the
// hub: the same demo file imported on two devices before they synced.
type SyncAdoption struct {
	Table string
	From  string
	To    string
}

// adoptSyncTwin answers a received demo or match that clashes with the same demo
// (same sha256) or that demo's match stored here under another id. It moves every
// local reference to the received id, so notes and plays on both sides meet.
func adoptSyncTwin(ctx context.Context, tx *sql.Tx, change SyncChange) (SyncAdoption, bool, error) {
	if change.Op != "upsert" || len(change.Key) != 1 {
		return SyncAdoption{}, false, nil
	}
	to, ok := change.Key[0].(string)
	if !ok || to == "" {
		return SyncAdoption{}, false, nil
	}
	var query, column string
	switch change.Table {
	case "library_demos":
		query, column = `SELECT id FROM library_demos WHERE sha256 = ?`, "sha256"
	case "library_matches":
		query, column = `SELECT id FROM library_matches WHERE demo_id = ?`, "demo_id"
	default:
		return SyncAdoption{}, false, nil
	}
	match, ok := change.Row[column].(string)
	if !ok || match == "" {
		return SyncAdoption{}, false, nil
	}
	var from string
	err := tx.QueryRowContext(ctx, query, match).Scan(&from)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && from == to) {
		return SyncAdoption{}, false, nil
	}
	if err != nil {
		return SyncAdoption{}, false, err
	}
	if err := rekeySyncRows(ctx, tx, from, to); err != nil {
		return SyncAdoption{}, false, fmt.Errorf("adopt %s %s as %s: %w", change.Table, from, to, err)
	}
	return SyncAdoption{Table: change.Table, From: from, To: to}, true, nil
}

// rekeySyncRows replaces id from with to in every text column of the library,
// which covers keys, foreign keys, file keys and ids inside JSON. Ids are UUIDs,
// so a substring match is that id. The triggers log the moved rows for the hub.
func rekeySyncRows(ctx context.Context, tx *sql.Tx, from, to string) error {
	if _, err := tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sync_meta WHERE key = 'applying'`); err != nil {
		return err
	}
	columns, err := librarySyncTextColumns(ctx, tx)
	if err != nil {
		return err
	}
	for _, c := range columns {
		stmt := `UPDATE ` + c.table + ` SET ` + c.column + ` = replace(` + c.column + `, ?, ?) WHERE instr(` + c.column + `, ?) > 0`
		if _, err := tx.ExecContext(ctx, stmt, from, to, from); err != nil {
			return fmt.Errorf("rekey %s.%s: %w", c.table, c.column, err)
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sync_meta (key, value) VALUES ('applying', '1') ON CONFLICT(key) DO UPDATE SET value = excluded.value`)
	return err
}

type textColumn struct{ table, column string }

func librarySyncTextColumns(ctx context.Context, tx *sql.Tx) ([]textColumn, error) {
	rows, err := tx.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE 'library\_%' ESCAPE '\' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		tables = append(tables, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []textColumn
	for _, table := range tables {
		info, err := tx.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
		if err != nil {
			return nil, err
		}
		for info.Next() {
			var cid, notNull, pk int
			var name, declared string
			var dflt any
			if err := info.Scan(&cid, &name, &declared, &notNull, &dflt, &pk); err != nil {
				info.Close()
				return nil, err
			}
			if strings.Contains(strings.ToUpper(declared), "TEXT") || declared == "" {
				out = append(out, textColumn{table: table, column: name})
			}
		}
		info.Close()
		if err := info.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}
