package telemetry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrIngestBudget     = errors.New("telemetry ingest budget exhausted")
	ErrStorageHighWater = errors.New("telemetry storage high-water reached")
)

const storageHighWaterPages = 28_000

const schemaSQL = `
CREATE TABLE IF NOT EXISTS telemetry_events (
    id TEXT PRIMARY KEY,
    received_at INTEGER NOT NULL,
    occurred_at INTEGER NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('error', 'span')),
    support_code TEXT NOT NULL,
    session_id TEXT NOT NULL,
    release TEXT NOT NULL,
    component TEXT NOT NULL,
    name TEXT NOT NULL,
    stage TEXT NOT NULL,
    class TEXT NOT NULL,
    summary TEXT NOT NULL CHECK (summary = ''),
    fingerprint TEXT NOT NULL,
    os TEXT NOT NULL,
    arch TEXT NOT NULL,
    outcome TEXT NOT NULL,
    duration_ms INTEGER NOT NULL CHECK (duration_ms >= 0)
);
CREATE INDEX IF NOT EXISTS telemetry_events_support_time
    ON telemetry_events (support_code, occurred_at DESC);
CREATE INDEX IF NOT EXISTS telemetry_events_kind_time
    ON telemetry_events (kind, occurred_at DESC);
CREATE INDEX IF NOT EXISTS telemetry_events_release_time
    ON telemetry_events (release, occurred_at DESC);
CREATE INDEX IF NOT EXISTS telemetry_spans_grouping
    ON telemetry_events (kind, occurred_at, release, component, name, outcome, duration_ms);
`

// Store persists remote diagnostic events in one SQLite database. A single
// connection keeps connection-scoped PRAGMAs invariant and serializes reads
// with short ingest transactions; the HTTP layer bounds every admin query.
type Store struct {
	db             *sql.DB
	highWaterPages int64
}

func OpenStore(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("telemetry database path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create telemetry database directory: %w", err)
	}
	// #nosec G302 -- directories require execute permission; 0700 is owner-only.
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("restrict telemetry database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open telemetry database: %w", err)
	}
	// One long-lived connection makes every connection-scoped PRAGMA an actual
	// service invariant and serializes the single-writer SQLite ownership.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db, highWaterPages: storageHighWaterPages}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("restrict telemetry database: %w", err)
	}
	return store, nil
}

func (s *Store) initialize(ctx context.Context) error {
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA journal_size_limit=16777216",
		"PRAGMA max_page_count=32768",
	} {
		if _, err := s.db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("initialize telemetry database: %w", err)
		}
	}
	var pageSize int
	if err := s.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return fmt.Errorf("read telemetry page size: %w", err)
	}
	if pageSize != 4096 {
		return fmt.Errorf("telemetry page size %d is unsupported; expected 4096", pageSize)
	}
	return s.migrateSchema(ctx)
}

func (s *Store) migrateSchema(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read telemetry schema version: %w", err)
	}
	switch version {
	case 1:
		if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
			return fmt.Errorf("initialize telemetry schema: %w", err)
		}
		return nil
	case 0:
		// Version zero includes the pre-allowlist collector. Diagnostics are
		// replaceable, so purge rather than retaining any legacy free text or
		// content-derived fingerprints behind the hardened contract.
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin telemetry privacy migration: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS telemetry_events"); err != nil {
			return fmt.Errorf("purge legacy telemetry: %w", err)
		}
		if _, err := tx.ExecContext(ctx, schemaSQL); err != nil {
			return fmt.Errorf("recreate telemetry schema: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version=1"); err != nil {
			return fmt.Errorf("record telemetry schema version: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit telemetry privacy migration: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("telemetry schema version %d is newer than supported version 1", version)
	}
}

func (s *Store) Close() error { return s.db.Close() }

// Insert stores a validated batch idempotently. A client retry can resend the
// same event IDs without inflating incident counts.
func storagePagesAtHighWater(pageCount, freePages, highWaterPages int64) bool {
	return pageCount-freePages >= highWaterPages
}

func (s *Store) Insert(ctx context.Context, events []Event, receivedAt time.Time, admit func(int) (rollback func(), ok bool)) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	var pageCount, freePages int64
	if err := s.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); err != nil {
		return 0, fmt.Errorf("read telemetry page count: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&freePages); err != nil {
		return 0, fmt.Errorf("read telemetry free-page count: %w", err)
	}
	if storagePagesAtHighWater(pageCount, freePages, s.highWaterPages) {
		return 0, ErrStorageHighWater
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin telemetry insert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	statement, err := tx.PrepareContext(ctx, `
INSERT OR IGNORE INTO telemetry_events (
    id, received_at, occurred_at, kind, support_code, session_id,
    release, component, name, stage, class, summary, fingerprint,
    os, arch, outcome, duration_ms
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare telemetry insert: %w", err)
	}
	defer statement.Close()
	inserted := 0
	for _, event := range events {
		result, execErr := statement.ExecContext(
			ctx,
			event.ID,
			receivedAt.UTC().UnixMilli(),
			event.OccurredAt.UTC().UnixMilli(),
			event.Kind,
			event.SupportCode,
			event.SessionID,
			event.Release,
			event.Component,
			event.Name,
			event.Stage,
			event.Class,
			"",
			event.Fingerprint,
			event.OS,
			event.Arch,
			event.Outcome,
			event.DurationMS,
		)
		if execErr != nil {
			return 0, fmt.Errorf("insert telemetry event: %w", execErr)
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return 0, fmt.Errorf("count inserted telemetry event: %w", rowsErr)
		}
		inserted += int(rows)
	}
	var rollbackAdmission func()
	if admit != nil && inserted > 0 {
		var ok bool
		rollbackAdmission, ok = admit(inserted)
		if !ok {
			return 0, ErrIngestBudget
		}
	}
	if err := tx.Commit(); err != nil {
		if rollbackAdmission != nil {
			rollbackAdmission()
		}
		return 0, fmt.Errorf("commit telemetry insert: %w", err)
	}
	return inserted, nil
}

// IncidentQuery bounds the private agent query surface.
type IncidentQuery struct {
	SupportCode string
	Since       time.Time
	Limit       int
}

func (s *Store) Incidents(ctx context.Context, query IncidentQuery) ([]Event, error) {
	if !supportCodePattern.MatchString(query.SupportCode) {
		return nil, errors.New("support code is invalid")
	}
	if query.Limit < 1 || query.Limit > 200 {
		return nil, errors.New("limit must be between 1 and 200")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, occurred_at, kind, support_code, session_id, release, component,
       name, stage, class, fingerprint, os, arch, outcome, duration_ms
FROM telemetry_events
WHERE support_code = ? AND occurred_at >= ?
ORDER BY occurred_at DESC
LIMIT ?`, query.SupportCode, query.Since.UTC().UnixMilli(), query.Limit)
	if err != nil {
		return nil, fmt.Errorf("query telemetry incidents: %w", err)
	}
	defer rows.Close()
	var events []Event
	for rows.Next() {
		var event Event
		var occurredMS int64
		if err := rows.Scan(
			&event.ID,
			&occurredMS,
			&event.Kind,
			&event.SupportCode,
			&event.SessionID,
			&event.Release,
			&event.Component,
			&event.Name,
			&event.Stage,
			&event.Class,
			&event.Fingerprint,
			&event.OS,
			&event.Arch,
			&event.Outcome,
			&event.DurationMS,
		); err != nil {
			return nil, fmt.Errorf("scan telemetry incident: %w", err)
		}
		event.SchemaVersion = SchemaVersion
		event.OccurredAt = time.UnixMilli(occurredMS).UTC()
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate telemetry incidents: %w", err)
	}
	return events, nil
}

// ErrorGroup is an aggregate suitable for an agent's first triage pass.
type ErrorGroup struct {
	Release     string    `json:"release"`
	Component   string    `json:"component"`
	Stage       string    `json:"stage"`
	Class       string    `json:"class"`
	Name        string    `json:"name"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Count       int64     `json:"count"`
	LastSeen    time.Time `json:"last_seen"`
}

// SpanGroup summarizes the sampled durations for one operation.
type SpanGroup struct {
	Release   string `json:"release"`
	Component string `json:"component"`
	Name      string `json:"name"`
	Outcome   string `json:"outcome"`
	Count     int    `json:"count"`
	AverageMS int64  `json:"average_ms"`
	P95MS     int64  `json:"p95_ms"`
	MaximumMS int64  `json:"maximum_ms"`
}

// Summary is returned by the private stats API.
type Summary struct {
	Since       time.Time    `json:"since"`
	GeneratedAt time.Time    `json:"generated_at"`
	Storage     StorageUsage `json:"storage"`
	Errors      []ErrorGroup `json:"errors"`
	Spans       []SpanGroup  `json:"spans"`
}

// StorageUsage lets an agent detect abuse or capacity pressure without shell
// access to the VPS.
type StorageUsage struct {
	Events           int64 `json:"events"`
	DatabaseBytes    int64 `json:"database_bytes"`
	MaxDatabaseBytes int64 `json:"max_database_bytes"`
}

func (s *Store) Summary(ctx context.Context, since, now time.Time) (Summary, error) {
	errorsOut, err := s.errorGroups(ctx, since)
	if err != nil {
		return Summary{}, err
	}
	spansOut, err := s.spanGroups(ctx, since)
	if err != nil {
		return Summary{}, err
	}
	storage, err := s.storageUsage(ctx)
	if err != nil {
		return Summary{}, err
	}
	return Summary{
		Since:       since.UTC(),
		GeneratedAt: now.UTC(),
		Storage:     storage,
		Errors:      errorsOut,
		Spans:       spansOut,
	}, nil
}

func (s *Store) errorGroups(ctx context.Context, since time.Time) ([]ErrorGroup, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT release, component, stage, class, name, fingerprint, COUNT(*), MAX(occurred_at)
FROM telemetry_events
WHERE kind = 'error' AND occurred_at >= ?
GROUP BY release, component, stage, class, name, fingerprint
ORDER BY COUNT(*) DESC, MAX(occurred_at) DESC
LIMIT 200`, since.UTC().UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("query telemetry error groups: %w", err)
	}
	defer rows.Close()
	var groups []ErrorGroup
	for rows.Next() {
		var group ErrorGroup
		var lastSeenMS int64
		if err := rows.Scan(
			&group.Release,
			&group.Component,
			&group.Stage,
			&group.Class,
			&group.Name,
			&group.Fingerprint,
			&group.Count,
			&lastSeenMS,
		); err != nil {
			return nil, fmt.Errorf("scan telemetry error group: %w", err)
		}
		group.LastSeen = time.UnixMilli(lastSeenMS).UTC()
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate telemetry error groups: %w", err)
	}
	return groups, nil
}

func (s *Store) spanGroups(ctx context.Context, since time.Time) ([]SpanGroup, error) {
	rows, err := s.db.QueryContext(ctx, `
WITH ranked AS (
    SELECT release, component, name, outcome, duration_ms,
           ROW_NUMBER() OVER (
               PARTITION BY release, component, name, outcome
               ORDER BY duration_ms
           ) AS rank,
           COUNT(*) OVER (
               PARTITION BY release, component, name, outcome
           ) AS total
    FROM telemetry_events
    WHERE kind = 'span' AND occurred_at >= ?
)
SELECT release, component, name, outcome, total,
       CAST(AVG(duration_ms) AS INTEGER),
       MAX(CASE WHEN rank = ((total * 95 + 99) / 100) THEN duration_ms END),
       MAX(duration_ms)
FROM ranked
GROUP BY release, component, name, outcome, total
ORDER BY total DESC, release DESC, name
LIMIT 200`, since.UTC().UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("query telemetry span groups: %w", err)
	}
	defer rows.Close()
	var groups []SpanGroup
	for rows.Next() {
		var group SpanGroup
		if err := rows.Scan(
			&group.Release,
			&group.Component,
			&group.Name,
			&group.Outcome,
			&group.Count,
			&group.AverageMS,
			&group.P95MS,
			&group.MaximumMS,
		); err != nil {
			return nil, fmt.Errorf("scan telemetry span group: %w", err)
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate telemetry span groups: %w", err)
	}
	return groups, nil
}

func (s *Store) storageUsage(ctx context.Context) (StorageUsage, error) {
	var usage StorageUsage
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM telemetry_events`).Scan(&usage.Events); err != nil {
		return StorageUsage{}, fmt.Errorf("count telemetry storage: %w", err)
	}
	var pageCount, pageSize, maxPageCount int64
	for query, destination := range map[string]*int64{
		"PRAGMA page_count":     &pageCount,
		"PRAGMA page_size":      &pageSize,
		"PRAGMA max_page_count": &maxPageCount,
	} {
		if err := s.db.QueryRowContext(ctx, query).Scan(destination); err != nil {
			return StorageUsage{}, fmt.Errorf("read telemetry storage pragma: %w", err)
		}
	}
	usage.DatabaseBytes = pageCount * pageSize
	usage.MaxDatabaseBytes = maxPageCount * pageSize
	return usage, nil
}

func (s *Store) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM telemetry_events WHERE received_at < ?`, cutoff.UTC().UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("delete expired telemetry events: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count expired telemetry events: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA optimize"); err != nil {
		return deleted, fmt.Errorf("optimize telemetry database: %w", err)
	}
	return deleted, nil
}
