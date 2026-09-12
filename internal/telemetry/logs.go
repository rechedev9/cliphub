package telemetry

import (
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	LogSchemaVersion   = 1
	MaxLogMessageBytes = 16 << 10
	maxLogBatchRecords = 64
	maxLogRequestBytes = 256 << 10
)

var logLabelPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:-]{0,95}$`)

// LogRecord is a technical log entry, not a file or an arbitrary attributes
// bag. Sequence orders one Studio session even when the client clock jumps.
// AttemptID connects a task's lifecycle and its subprocess output.
type LogRecord struct {
	SchemaVersion int       `json:"schema_version"`
	ID            string    `json:"id"`
	SupportCode   string    `json:"support_code"`
	SessionID     string    `json:"session_id"`
	Sequence      int64     `json:"sequence"`
	OccurredAt    time.Time `json:"occurred_at"`
	Release       string    `json:"release"`
	Source        string    `json:"source"`
	Level         string    `json:"level"`
	Event         string    `json:"event"`
	Message       string    `json:"message"`
	JobID         string    `json:"job_id,omitempty"`
	AttemptID     string    `json:"attempt_id,omitempty"`
	Operation     string    `json:"operation,omitempty"`
	Attempt       int       `json:"attempt,omitempty"`
	Outcome       string    `json:"outcome,omitempty"`
	DurationMS    int64     `json:"duration_ms,omitempty"`
	ExitCode      *int64    `json:"exit_code,omitempty"`
	LostRecords   int64     `json:"lost_records,omitempty"`
}

type LogBatch struct {
	Records []LogRecord `json:"records"`
}

func validateLogBatch(batch LogBatch, _ time.Time) ([]LogRecord, error) {
	if len(batch.Records) == 0 || len(batch.Records) > maxLogBatchRecords {
		return nil, errors.New("invalid log batch size")
	}
	records := make([]LogRecord, len(batch.Records))
	seen := make(map[string]bool, len(records))
	for i, record := range batch.Records {
		if record.SchemaVersion != LogSchemaVersion || !validLogUUID(record.ID) || !validLogUUID(record.SessionID) ||
			!supportCodePattern.MatchString(record.SupportCode) || !releasePattern.MatchString(record.Release) {
			return nil, fmt.Errorf("record %d: invalid identity", i)
		}
		if seen[record.ID] || record.Sequence < 1 || record.Sequence > 9_007_199_254_740_991 {
			return nil, fmt.Errorf("record %d: invalid sequence or duplicate id", i)
		}
		seen[record.ID] = true
		// Retention uses collector receipt time. Keep a client's original clock
		// even when it is wrong, rather than rejecting the evidence of a crash.
		if record.OccurredAt.IsZero() || record.OccurredAt.Year() < 2000 || record.OccurredAt.Year() > 2100 {
			return nil, fmt.Errorf("record %d: invalid time", i)
		}
		if !stringSet("studio", "orchestrator", "web", "recorder", "renderer", "telemetry")[record.Source] ||
			!stringSet("debug", "info", "warn", "error")[record.Level] || !logLabelPattern.MatchString(record.Event) {
			return nil, fmt.Errorf("record %d: invalid log labels", i)
		}
		if record.JobID != "" && !validLogUUID(record.JobID) || record.AttemptID != "" && !validLogUUID(record.AttemptID) ||
			record.Operation != "" && !logLabelPattern.MatchString(record.Operation) {
			return nil, fmt.Errorf("record %d: invalid correlation", i)
		}
		if record.Attempt < 0 || record.Attempt > 10000 || record.DurationMS < 0 || record.DurationMS > int64(7*24*time.Hour/time.Millisecond) ||
			record.LostRecords < 0 || record.LostRecords > 9_007_199_254_740_991 ||
			record.Outcome != "" && !stringSet("ok", "error", "timeout", "cancelled", "interrupted")[record.Outcome] {
			return nil, fmt.Errorf("record %d: invalid outcome", i)
		}
		if record.ExitCode != nil && (*record.ExitCode < -4_294_967_296 || *record.ExitCode > 4_294_967_295) {
			return nil, fmt.Errorf("record %d: invalid exit code", i)
		}
		if !utf8.ValidString(record.Message) || len(record.Message) > MaxLogMessageBytes {
			return nil, fmt.Errorf("record %d: invalid message size", i)
		}
		record.Message = filterDiagnosticMessage(record.Message, MaxLogMessageBytes)
		record.OccurredAt = record.OccurredAt.UTC()
		records[i] = record
	}
	return records, nil
}

func validLogUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && len(value) == 36
}
