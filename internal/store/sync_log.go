package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"
	"sync"
)

const syncLogBatchCap = 1000

// SyncChange is one recorded mutation. Key holds the key column values in the
// order syncTables lists them; Row is the current row (nil for a delete).
type SyncChange struct {
	Seq       int64          `json:"seq"`
	Table     string         `json:"table"`
	Key       []any          `json:"key"`
	Op        string         `json:"op"`
	ChangedAt int64          `json:"changed_at"`
	Row       map[string]any `json:"row"`
}

// SyncBatch is one page of changes; Next is the highest seq read.
type SyncBatch struct {
	Changes []SyncChange `json:"changes"`
	Next    int64        `json:"next"`
}

// SyncInputError reports a change that names an unknown table or column, so an
// HTTP caller can answer 400 instead of 500.
type SyncInputError struct {
	Table  string
	Column string
}

func (e *SyncInputError) Error() string {
	if e.Column != "" {
		return fmt.Sprintf("unknown column %q for %s", e.Column, e.Table)
	}
	return fmt.Sprintf("unknown sync table %q", e.Table)
}

// SyncLog reads and applies the sync_changes log of one jobs.db.
type SyncLog struct {
	db *sql.DB

	contractOnce sync.Once
	contract     SyncContract
	contractErr  error
}

func NewSyncLog(db *sql.DB) *SyncLog { return &SyncLog{db: db} }

// Changes returns log rows with seq > after, oldest first, at most limit rows.
// Rows for the same (tbl, row_key) collapse to the last one.
func (s *SyncLog) Changes(ctx context.Context, after int64, limit int) (SyncBatch, error) {
	return s.ChangesExcluding(ctx, after, limit, "")
}

// ChangesExcluding is Changes without the rows a hub logged while applying the
// named device's own push, so a device does not receive its own edits back.
// The cursor still moves past them.
func (s *SyncLog) ChangesExcluding(ctx context.Context, after int64, limit int, origin string) (SyncBatch, error) {
	if limit <= 0 {
		return SyncBatch{Changes: []SyncChange{}, Next: after}, nil
	}
	if limit > syncLogBatchCap {
		limit = syncLogBatchCap
	}
	// One read transaction gives the log and the rows one snapshot.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SyncBatch{}, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx,
		`SELECT seq, tbl, row_key, op, changed_at, origin FROM sync_changes WHERE seq > ? ORDER BY seq ASC LIMIT ?`,
		after, limit)
	if err != nil {
		return SyncBatch{}, err
	}
	defer rows.Close()

	type logRow struct {
		seq       int64
		tbl       string
		rowKey    string
		op        string
		changedAt int64
		origin    string
	}
	byKey := map[string]logRow{}
	next := after
	for rows.Next() {
		var r logRow
		if err := rows.Scan(&r.seq, &r.tbl, &r.rowKey, &r.op, &r.changedAt, &r.origin); err != nil {
			return SyncBatch{}, err
		}
		byKey[r.tbl+"\x00"+r.rowKey] = r
		if r.seq > next {
			next = r.seq
		}
	}
	if err := rows.Err(); err != nil {
		return SyncBatch{}, err
	}
	rows.Close()
	collapsed := make([]logRow, 0, len(byKey))
	for _, r := range byKey {
		if origin == "" || r.origin != origin {
			collapsed = append(collapsed, r)
		}
	}
	sort.Slice(collapsed, func(i, j int) bool { return collapsed[i].seq < collapsed[j].seq })

	changes := make([]SyncChange, 0, len(collapsed))
	for _, r := range collapsed {
		key, err := parseSyncKey(r.rowKey)
		if err != nil {
			return SyncBatch{}, fmt.Errorf("parse sync key for %s: %w", r.tbl, err)
		}
		change := SyncChange{Seq: r.seq, Table: r.tbl, Key: key, Op: r.op, ChangedAt: r.changedAt}
		if r.op == "upsert" {
			row, found, err := currentSyncRow(ctx, tx, r.tbl, key)
			if err != nil {
				return SyncBatch{}, err
			}
			if !found {
				change.Op = "delete"
			} else {
				change.Row = row
			}
		}
		changes = append(changes, change)
	}
	return SyncBatch{Changes: changes, Next: next}, nil
}

func parseSyncKey(raw string) ([]any, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var key []any
	if err := dec.Decode(&key); err != nil {
		return nil, err
	}
	for i, v := range key {
		if n, ok := v.(json.Number); ok {
			if iv, err := n.Int64(); err == nil {
				key[i] = iv
			} else {
				key[i], _ = n.Float64()
			}
		}
	}
	return key, nil
}

// currentSyncRow reads the current non-local columns of one row.
func currentSyncRow(ctx context.Context, tx *sql.Tx, table string, key []any) (map[string]any, bool, error) {
	t, ok := syncTableByName[table]
	if !ok {
		return nil, false, &SyncInputError{Table: table}
	}
	where := make([]string, len(t.keyColumns))
	for i, col := range t.keyColumns {
		where[i] = col + " = ?"
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT * FROM `+table+` WHERE `+strings.Join(where, " AND "), key...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, false, rows.Err()
	}
	cols, err := rows.Columns()
	if err != nil {
		return nil, false, err
	}
	values := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, false, err
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	row := make(map[string]any, len(cols))
	for i, col := range cols {
		if slices.Contains(t.localColumns, col) {
			continue
		}
		row[col] = values[i]
	}
	return row, true, nil
}

// Apply replays changes without logging them again. It runs in one transaction
// and sets sync_meta.applying so the triggers stay silent.
func (s *SyncLog) Apply(ctx context.Context, changes []SyncChange) error {
	return s.apply(ctx, syncApply{changes: changes})
}

// ApplyAdopting is Apply on a device pulling from its hub: a demo or match that
// is here under another id takes the hub's id, and adopt runs inside the
// transaction for each one, so the caller can copy the files kept by id.
func (s *SyncLog) ApplyAdopting(ctx context.Context, changes []SyncChange, adopt func(SyncAdoption) error) error {
	return s.apply(ctx, syncApply{changes: changes, adopt: adopt})
}

type syncApply struct {
	origin  string
	changes []SyncChange
	adopt   func(SyncAdoption) error
}

// ApplyFrom is what a hub runs for a device's push: the changes are applied
// and logged, tagged with the device, so every other device receives them.
func (s *SyncLog) ApplyFrom(ctx context.Context, origin string, changes []SyncChange) error {
	if origin == "" {
		return fmt.Errorf("sync origin is required")
	}
	return s.apply(ctx, syncApply{origin: origin, changes: changes})
}

func (s *SyncLog) apply(ctx context.Context, in syncApply) error {
	origin := in.origin
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	marker, value := "applying", "1"
	if origin != "" {
		marker, value = "origin", origin
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sync_meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, marker, value); err != nil {
		return err
	}
	described := map[string]map[string]syncColumn{}
	skipped := 0
	for _, change := range orderSyncChanges(in.changes) {
		err := applySyncChange(ctx, tx, change, described)
		if isSyncClash(err) && in.adopt != nil {
			adoption, adopted, adoptErr := adoptSyncTwin(ctx, tx, change)
			if adoptErr != nil {
				return adoptErr
			}
			if adopted {
				if err := in.adopt(adoption); err != nil {
					return err
				}
				err = applySyncChange(ctx, tx, change, described)
			}
		}
		if isSyncClash(err) {
			// A hub keeps its own copy of a demo that two devices imported under two ids;
			// the devices adopt the hub's id on their next pull and push their rows again.
			skipped++
			continue
		}
		if err != nil {
			return err
		}
	}
	if skipped > 0 {
		log.Printf("sync: kept %d local rows that clash with received changes", skipped)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sync_meta WHERE key = ?`, marker); err != nil {
		return err
	}
	return tx.Commit()
}

// isSyncClash reports a received row that a unique key or a missing parent in
// this library refuses. SQLite undoes only that statement.
func isSyncClash(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "FOREIGN KEY constraint failed")
}

// orderSyncChanges keeps the last change per row, then puts deletes first,
// children before parents, and upserts after them, parents before children.
func orderSyncChanges(changes []SyncChange) []SyncChange {
	last := make(map[string]int, len(changes))
	for i, c := range changes {
		last[syncRowID(c)] = i
	}
	ordered := make([]SyncChange, 0, len(last))
	for i, c := range changes {
		if last[syncRowID(c)] == i {
			ordered = append(ordered, c)
		}
	}
	rank := func(c SyncChange) int {
		if c.Op == "delete" {
			return -1 - syncTableIndex[c.Table]
		}
		return syncTableIndex[c.Table]
	}
	sort.SliceStable(ordered, func(i, j int) bool { return rank(ordered[i]) < rank(ordered[j]) })
	return ordered
}

// syncRowID names a change's row; JSON keeps "a","bc" apart from "ab","c".
func syncRowID(c SyncChange) string {
	key, err := json.Marshal(c.Key)
	if err != nil {
		return c.Table + "\x00" + fmt.Sprintf("%#v", c.Key)
	}
	return c.Table + "\x00" + string(key)
}

// applySyncChange applies one change; described caches each table's columns for the batch.
func applySyncChange(ctx context.Context, tx *sql.Tx, change SyncChange, described map[string]map[string]syncColumn) error {
	table, ok := syncTableByName[change.Table]
	if !ok {
		return &SyncInputError{Table: change.Table}
	}
	columns, ok := described[table.name]
	if !ok {
		var err error
		if columns, err = describeSyncTable(ctx, tx, table); err != nil {
			return err
		}
		described[table.name] = columns
	}
	switch change.Op {
	case "delete":
		return applySyncDelete(ctx, tx, table, change, columns)
	case "upsert":
		return applySyncUpsert(ctx, tx, table, change, columns)
	default:
		return fmt.Errorf("unknown sync op %q", change.Op)
	}
}

type syncColumn struct {
	declared   string
	notNull    bool
	hasDefault bool
}

func describeSyncTable(ctx context.Context, tx *sql.Tx, table syncTable) (map[string]syncColumn, error) {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(`+table.name+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]syncColumn{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, declared string
		var dflt any
		if err := rows.Scan(&cid, &name, &declared, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		columns[name] = syncColumn{declared: declared, notNull: notNull != 0, hasDefault: dflt != nil}
	}
	return columns, rows.Err()
}

func applySyncDelete(ctx context.Context, tx *sql.Tx, table syncTable, change SyncChange, columns map[string]syncColumn) error {
	if len(change.Key) != len(table.keyColumns) {
		return fmt.Errorf("sync key for %s has %d values, want %d", table.name, len(change.Key), len(table.keyColumns))
	}
	where := make([]string, len(table.keyColumns))
	args := make([]any, len(table.keyColumns))
	for i, col := range table.keyColumns {
		where[i] = col + " = ?"
		v, err := normalizeSyncValue(change.Key[i], columns[col].declared)
		if err != nil {
			return fmt.Errorf("key %s of %s: %w", col, table.name, err)
		}
		args[i] = v
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+table.name+` WHERE `+strings.Join(where, " AND "), args...); err != nil {
		return fmt.Errorf("delete from %s: %w", table.name, err)
	}
	return nil
}

func applySyncUpsert(ctx context.Context, tx *sql.Tx, table syncTable, change SyncChange, columns map[string]syncColumn) error {
	if len(change.Key) != len(table.keyColumns) {
		return fmt.Errorf("sync key for %s has %d values, want %d", table.name, len(change.Key), len(table.keyColumns))
	}
	keyVals := make([]any, len(table.keyColumns))
	for i, col := range table.keyColumns {
		v, err := normalizeSyncValue(change.Key[i], columns[col].declared)
		if err != nil {
			return fmt.Errorf("key %s of %s: %w", col, table.name, err)
		}
		keyVals[i] = v
	}
	rowCols := make([]string, 0, len(change.Row))
	for name := range change.Row {
		if slices.Contains(table.keyColumns, name) || slices.Contains(table.localColumns, name) {
			continue
		}
		if _, ok := columns[name]; !ok {
			return &SyncInputError{Table: table.name, Column: name}
		}
		rowCols = append(rowCols, name)
	}
	sort.Strings(rowCols)

	insertCols := append([]string(nil), table.keyColumns...)
	insertVals := append([]any(nil), keyVals...)
	for _, name := range rowCols {
		v, err := normalizeSyncValue(change.Row[name], columns[name].declared)
		if err != nil {
			return fmt.Errorf("column %s of %s: %w", name, table.name, err)
		}
		insertCols = append(insertCols, name)
		insertVals = append(insertVals, v)
	}
	// A device-local column without a default still needs a value on insert.
	for _, name := range table.localColumns {
		col, ok := columns[name]
		if !ok {
			continue
		}
		if col.notNull && !col.hasDefault {
			insertCols = append(insertCols, name)
			insertVals = append(insertVals, "")
		}
	}

	stmt := `INSERT INTO ` + table.name + ` (` + strings.Join(insertCols, ", ") + `) VALUES (` + placeholders(len(insertCols)) + `)`
	if len(rowCols) > 0 {
		sets := make([]string, len(rowCols))
		for i, name := range rowCols {
			sets[i] = name + " = excluded." + name
		}
		stmt += ` ON CONFLICT (` + strings.Join(table.keyColumns, ", ") + `) DO UPDATE SET ` + strings.Join(sets, ", ")
	} else {
		stmt += ` ON CONFLICT (` + strings.Join(table.keyColumns, ", ") + `) DO NOTHING`
	}
	if _, err := tx.ExecContext(ctx, stmt, insertVals...); err != nil {
		return fmt.Errorf("upsert into %s: %w", table.name, err)
	}
	return nil
}

func normalizeSyncValue(v any, declared string) (any, error) {
	n, ok := v.(json.Number)
	if !ok {
		return v, nil
	}
	switch strings.ToUpper(declared) {
	case "INTEGER", "INT", "BIGINT":
		return n.Int64()
	case "REAL", "FLOAT", "DOUBLE", "NUMERIC":
		return n.Float64()
	default:
		return n.String(), nil
	}
}

// Backfill logs every existing row of every syncable table as an upsert. It
// is for the first connection to a hub: migration 26 only logs later edits.
func (s *SyncLog) Backfill(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, table := range syncTables {
		var present int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table.name).Scan(&present); err != nil {
			return err
		}
		if present == 0 {
			continue
		}
		keys := make([]string, len(table.keyColumns))
		for i, col := range table.keyColumns {
			keys[i] = col
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sync_changes (tbl, row_key, op, changed_at) SELECT ?, json_array(`+strings.Join(keys, ", ")+`), 'upsert', CAST(unixepoch('subsec') * 1000 AS INTEGER) FROM `+table.name,
			table.name); err != nil {
			return fmt.Errorf("backfill %s: %w", table.name, err)
		}
	}
	return tx.Commit()
}

// Pending counts log rows after the cursor, for a "changes waiting" figure.
func (s *SyncLog) Pending(ctx context.Context, after int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_changes WHERE seq > ?`, after).Scan(&n)
	return n, err
}
