// Package job defines the orchestrator's core domain type and helpers.
package job

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

// ErrNotFound is returned by Get when no job has the requested id.
var ErrNotFound = errors.New("job not found")

// ErrConflict is returned when an operation is rejected because the job is not
// in a state that allows it (e.g. a parse request for a job that was never
// scanned). It maps to HTTP 409 at the API boundary.
var ErrConflict = errors.New("job state conflict")

// Status is the lifecycle state of a Job.
type Status int

const (
	StatusQueued Status = iota
	StatusParsing
	StatusParsed
	StatusRecording
	StatusRecorded
	StatusComposing
	StatusComposed
	StatusDone
	StatusFailed
	// Appended after StatusFailed so the existing integer values are unchanged.
	StatusScanning // queued→scanning while the roster scan runs
	StatusScanned  // roster ready; awaiting the user's target pick
	// Appended to preserve every existing durable integer value. The media is
	// materialized but QA warnings must be reviewed before it is publishable.
	StatusReviewRequired
)

var statusNames = [...]string{
	"queued",
	"parsing",
	"parsed",
	"recording",
	"recorded",
	"composing",
	"composed",
	"done",
	"failed",
	"scanning",
	"scanned",
	"review_required",
}

// String returns the canonical lowercase representation used in JSON and DB.
func (s Status) String() string {
	if int(s) < 0 || int(s) >= len(statusNames) {
		return "unknown"
	}
	return statusNames[s]
}

// MarshalJSON renders Status as the canonical string.
func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON parses a JSON string into a Status.
func (s *Status) UnmarshalJSON(b []byte) error {
	var raw string
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	parsed, err := ParseStatus(raw)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

// ParseStatus converts a canonical name into a Status.
func ParseStatus(name string) (Status, error) {
	for i, n := range statusNames {
		if n == name {
			return Status(i), nil
		}
	}
	return 0, fmt.Errorf("unknown job status %q", name)
}

// CanHaveRenderState is true once a capture has finished (or the job failed
// after one). Studio hides leftover render documents before this point so a
// recapture cannot adopt a previous ready, failed, or queued variant.
func (s Status) CanHaveRenderState() bool {
	switch s {
	case StatusRecorded, StatusComposing, StatusComposed, StatusReviewRequired, StatusDone, StatusFailed:
		return true
	default:
		return false
	}
}

// CaptureActive is true while a capture or its edit is running, so a new
// generate would race the worker instead of starting one.
func (s Status) CaptureActive() bool {
	return s == StatusRecording || s == StatusComposing
}

// Statuses returns every defined Status in declaration order. Exhaustive
// walks (startup sweeps, contract tests) must range over this instead of a
// hand-written list, which is how review_required was silently skipped.
func Statuses() []Status {
	out := make([]Status, len(statusNames))
	for i := range statusNames {
		out[i] = Status(i)
	}
	return out
}

// Job is the canonical domain model used by the API, the worker, and the DB.
type Job struct {
	ID            uuid.UUID `json:"id"`
	Status        Status    `json:"status"`
	FailureReason string    `json:"failure_reason,omitempty"`
	// FailureCode is the stable obs class for a terminal failure (missing_plate,
	// capture_flake, demo_incompatible, …). FailureReason stays human text.
	FailureCode string `json:"failure_code,omitempty"`
	// SeriesID groups the jobs of one uploaded bo3/bo5 series. It is a
	// client-minted UUID shared by every demo in the series; empty for a
	// standalone single-demo upload.
	SeriesID string `json:"series_id,omitempty"`
	// DemoFileName is the sanitized original file name of the uploaded demo,
	// kept only for display; empty when the upload carried no usable name.
	DemoFileName  string `json:"demo_file_name,omitempty"`
	DemoPath      string `json:"demo_path"`
	DemoSHA256    string `json:"demo_sha256"`
	TargetSteamID string `json:"target_steamid"`
	// MultiPOV lists every player of a multi-POV job; TargetSteamID is its first.
	MultiPOV  *MultiPOV      `json:"multi_pov,omitempty"`
	Rules     rules.Rules    `json:"rules"`
	KillPlan  *killplan.Plan `json:"kill_plan,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// MultiPOV is the player list of a job whose clips come from several POVs of
// one demo. The first SteamID is the anchor: recap, deaths and voice use it.
type MultiPOV struct {
	SteamIDs []string `json:"steam_ids"`
}

// MultiPOV player limits: one team, and at least two players.
const (
	MinMultiPOVPlayers = 2
	MaxMultiPOVPlayers = 5
)

// POVSteamIDs returns the job's players in pick order.
func (j Job) POVSteamIDs() []string {
	if j.MultiPOV != nil && len(j.MultiPOV.SteamIDs) > 0 {
		return j.MultiPOV.SteamIDs
	}
	if j.TargetSteamID == "" {
		return nil
	}
	return []string{j.TargetSteamID}
}

// IsMultiPOV reports whether the job mixes several players.
func (j Job) IsMultiPOV() bool {
	return j.MultiPOV != nil && len(j.MultiPOV.SteamIDs) > 1
}

// ParseInputs is what a parse claim writes onto a scanned or parsed job.
type ParseInputs struct {
	TargetSteamID string
	MultiPOV      *MultiPOV
	Rules         rules.Rules
}
