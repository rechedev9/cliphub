package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const logMaxPages = 524288 // 2 GiB, isolated from the error-event database.

var ErrLogConflict = errors.New("log identity already has different content")
var ErrInvalidLogQuery = errors.New("invalid log query")

type LogStore struct{ db *sql.DB }

// Logs have their own capacity budget so verbose output cannot displace the
// error index. FULL synchronous commits precede receipts sent to clients.
func openLogStore(path string) (*LogStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &LogStore{db: db}
	for _, statement := range []string{
		"PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA busy_timeout=5000",
		"PRAGMA journal_size_limit=16777216", fmt.Sprintf("PRAGMA max_page_count=%d", logMaxPages),
		`CREATE TABLE IF NOT EXISTS diagnostic_logs (
			cursor INTEGER PRIMARY KEY AUTOINCREMENT,
			id TEXT NOT NULL UNIQUE, support_code TEXT NOT NULL, session_id TEXT NOT NULL,
			sequence INTEGER NOT NULL, job_id TEXT NOT NULL, attempt_id TEXT NOT NULL,
			event TEXT NOT NULL, level TEXT NOT NULL, occurred_at INTEGER NOT NULL,
			received_at INTEGER NOT NULL, record_json TEXT NOT NULL CHECK (json_valid(record_json)),
			UNIQUE(support_code, session_id, sequence)
		)`,
		"CREATE INDEX IF NOT EXISTS logs_job_cursor ON diagnostic_logs(job_id,cursor)",
		"CREATE INDEX IF NOT EXISTS logs_support_cursor ON diagnostic_logs(support_code,cursor)",
		"CREATE INDEX IF NOT EXISTS logs_session_cursor ON diagnostic_logs(session_id,cursor)",
		"CREATE INDEX IF NOT EXISTS logs_received ON diagnostic_logs(received_at)",
		"CREATE INDEX IF NOT EXISTS logs_event ON diagnostic_logs(event)",
	} {
		if _, err = db.Exec(statement); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("initialize log store: %w", err)
		}
	}
	if err = os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *LogStore) Insert(ctx context.Context, records []LogRecord, received time.Time, admit func(int) (func(), bool)) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var inserted, bytes int
	for _, record := range records {
		// Repeat filtering at the final persistence boundary as for error events.
		record.Message = filterDiagnosticMessage(record.Message, MaxLogMessageBytes)
		encoded, err := json.Marshal(record)
		if err != nil {
			return 0, err
		}
		var prior string
		err = tx.QueryRowContext(ctx, "SELECT record_json FROM diagnostic_logs WHERE id=? OR (support_code=? AND session_id=? AND sequence=?)", record.ID, record.SupportCode, record.SessionID, record.Sequence).Scan(&prior)
		if err == nil {
			if prior != string(encoded) {
				return 0, ErrLogConflict
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO diagnostic_logs(id,support_code,session_id,sequence,job_id,attempt_id,event,level,occurred_at,received_at,record_json) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			record.ID, record.SupportCode, record.SessionID, record.Sequence, record.JobID, record.AttemptID, record.Event, record.Level, record.OccurredAt.UnixMilli(), received.UTC().UnixMilli(), string(encoded))
		if err != nil {
			return 0, err
		}
		inserted++
		bytes += len(encoded)
	}
	var rollback func()
	if admit != nil && bytes > 0 {
		var ok bool
		rollback, ok = admit(bytes)
		if !ok {
			return 0, ErrIngestBudget
		}
	}
	if err := tx.Commit(); err != nil {
		if rollback != nil {
			rollback()
		}
		return 0, err
	}
	return inserted, nil
}

type LogQuery struct {
	SupportCode string
	JobID       string
	SessionID   string
	Level       string
	Event       string
	After       int64
	Limit       int
	Since       time.Time
}

type StoredLog struct {
	Cursor     int64     `json:"cursor"`
	ReceivedAt time.Time `json:"received_at"`
	LogRecord
}

type LogPage struct {
	Records    []StoredLog `json:"records"`
	NextCursor int64       `json:"next_cursor"`
	HasMore    bool        `json:"has_more"`
}

func (s *LogStore) Query(ctx context.Context, query LogQuery) (LogPage, error) {
	page := LogPage{Records: []StoredLog{}, NextCursor: query.After}
	if query.Limit < 1 || query.Limit > 500 || query.After < 0 ||
		query.SupportCode != "" && !supportCodePattern.MatchString(query.SupportCode) ||
		query.JobID != "" && !validLogUUID(query.JobID) || query.SessionID != "" && !validLogUUID(query.SessionID) ||
		query.Level != "" && !stringSet("debug", "info", "warn", "error")[query.Level] {
		return page, ErrInvalidLogQuery
	}
	if query.Event != "" && !logLabelPattern.MatchString(query.Event) {
		return page, ErrInvalidLogQuery
	}
	clauses := []string{"cursor > ?", "received_at >= ?"}
	args := []any{query.After, query.Since.UTC().UnixMilli()}
	for _, filter := range []struct{ column, value string }{{"support_code", query.SupportCode}, {"job_id", query.JobID}, {"session_id", query.SessionID}, {"level", query.Level}, {"event", query.Event}} {
		if filter.value != "" {
			clauses = append(clauses, filter.column+" = ?")
			args = append(args, filter.value)
		}
	}
	args = append(args, query.Limit+1)
	rows, err := s.db.QueryContext(ctx, "SELECT cursor,received_at,record_json FROM diagnostic_logs WHERE "+strings.Join(clauses, " AND ")+" ORDER BY cursor LIMIT ?", args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var log StoredLog
		var received int64
		var encoded string
		if err := rows.Scan(&log.Cursor, &received, &encoded); err != nil {
			return page, err
		}
		if len(page.Records) == query.Limit {
			page.HasMore = true
			break
		}
		if err := json.Unmarshal([]byte(encoded), &log.LogRecord); err != nil {
			return page, err
		}
		log.ReceivedAt = time.UnixMilli(received).UTC()
		page.NextCursor = log.Cursor
		page.Records = append(page.Records, log)
	}
	return page, rows.Err()
}

type LogStorageUsage struct {
	Records             int64      `json:"records"`
	DatabaseBytes       int64      `json:"database_bytes"`
	MaxDatabaseBytes    int64      `json:"max_database_bytes"`
	ReportedLostRecords int64      `json:"reported_lost_records"`
	LastReceivedAt      *time.Time `json:"last_received_at"`
}

func (s *LogStore) Usage(ctx context.Context) (LogStorageUsage, error) {
	var usage LogStorageUsage
	var received sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), MAX(received_at) FROM diagnostic_logs`).Scan(&usage.Records, &received); err != nil {
		return usage, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(json_extract(record_json,'$.lost_records')),0) FROM diagnostic_logs WHERE event='delivery.gap'`).Scan(&usage.ReportedLostRecords); err != nil {
		return usage, err
	}
	var pages, pageSize int64
	for _, item := range []struct {
		pragma string
		value  *int64
	}{{"page_count", &pages}, {"page_size", &pageSize}} {
		if err := s.db.QueryRowContext(ctx, "PRAGMA "+item.pragma).Scan(item.value); err != nil {
			return usage, err
		}
	}
	usage.DatabaseBytes = pages * pageSize
	usage.MaxDatabaseBytes = logMaxPages * pageSize
	if received.Valid {
		value := time.UnixMilli(received.Int64).UTC()
		usage.LastReceivedAt = &value
	}
	return usage, nil
}

func (s *LogStore) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM diagnostic_logs WHERE received_at < ?", cutoff.UTC().UnixMilli())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
