package artifacts

import "time"

// Tactical analysis states. TacticalStateNone is never stored: it is what a
// reader reports for a job whose analysis has not been requested yet.
const (
	TacticalStateNone    = "none"
	TacticalStateQueued  = "queued"
	TacticalStateRunning = "running"
	TacticalStateReady   = "ready"
	TacticalStateFailed  = "failed"
)

// TacticalStatus is the readiness document written beside a job's tactical
// artifacts. Tactical analysis is optional per job, so its readiness is modelled
// by artifact presence plus this document rather than by a job.Status value.
// It lives next to its key because the worker writes it and the HTTP API serves
// it, while the tactical schema packages stay free of orchestration state.
//
// SchemaVersion is the tacticalplan schema the writer produces, so a reader can
// reject an analysis it does not understand without opening the document.
type TacticalStatus struct {
	State         string    `json:"state"`
	GeneratedAt   time.Time `json:"generated_at"`
	SchemaVersion string    `json:"schema_version"`
	SampleHZ      float64   `json:"sample_hz"`
	Error         string    `json:"error,omitempty"`
}
