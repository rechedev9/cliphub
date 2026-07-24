package anticheat

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// DocumentStatus is the lifecycle of one stored CheaterDetect analysis.
type DocumentStatus string

const (
	// StatusRunning means the analysis pass is in flight. The document is
	// written before the pass starts so the UI can poll from the first moment.
	StatusRunning DocumentStatus = "running"
	// StatusReady means Report is populated.
	StatusReady DocumentStatus = "ready"
	// StatusFailed means the pass ended without a report; FailureReason says why.
	StatusFailed DocumentStatus = "failed"
)

// Document is the durable artifact behind a job's CheaterDetect lane. The
// analysis deliberately does not touch the job's own status — a demo can be
// clipped and rendered while it is being screened, and a failed screening must
// never make a healthy job look broken — so the whole lifecycle lives here.
type Document struct {
	SchemaVersion int            `json:"schema_version"`
	JobID         string         `json:"job_id"`
	Status        DocumentStatus `json:"status"`
	StartedAt     time.Time      `json:"started_at"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	FailureReason string         `json:"failure_reason,omitempty"`
	Report        *Report        `json:"report,omitempty"`
}

// NewRunningDocument returns the document written when an analysis is queued.
func NewRunningDocument(jobID string, startedAt time.Time) Document {
	return Document{
		SchemaVersion: SchemaVersion,
		JobID:         jobID,
		Status:        StatusRunning,
		StartedAt:     startedAt.UTC(),
	}
}

// Complete returns the ready form of the document carrying report.
func (d Document) Complete(report Report, completedAt time.Time) Document {
	done := completedAt.UTC()
	d.Status = StatusReady
	d.CompletedAt = &done
	d.FailureReason = ""
	d.Report = &report
	return d
}

// Fail returns the failed form of the document. The reason is kept short and
// human-readable because it surfaces directly in the UI.
func (d Document) Fail(reason string, completedAt time.Time) Document {
	done := completedAt.UTC()
	d.Status = StatusFailed
	d.CompletedAt = &done
	d.FailureReason = reason
	d.Report = nil
	return d
}

// Encode writes the document as indented JSON.
func (d Document) Encode(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return fmt.Errorf("encode anticheat document: %w", err)
	}
	return nil
}

// DecodeDocument reads a stored analysis document.
func DecodeDocument(r io.Reader) (Document, error) {
	var d Document
	if err := json.NewDecoder(r).Decode(&d); err != nil {
		return Document{}, fmt.Errorf("decode anticheat document: %w", err)
	}
	return d, nil
}
