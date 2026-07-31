package recording

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/tickcut/internal/artifacts"
)

const CaptureProgressSchemaVersion = "1.0"

// CaptureProgress is UI-only evidence for one in-flight attempt. Completed IDs
// describe local recorder output observed so far; they are not durable clips
// and never participate in readiness or retry decisions.
type CaptureProgress struct {
	SchemaVersion       string    `json:"schema_version"`
	AttemptID           uuid.UUID `json:"attempt_id"`
	SegmentIDs          []string  `json:"segment_ids"`
	CompletedSegmentIDs []string  `json:"completed_segment_ids"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func NewCaptureProgress(attemptID uuid.UUID, segmentIDs, completed []string, now time.Time) (CaptureProgress, error) {
	if attemptID == uuid.Nil {
		return CaptureProgress{}, fmt.Errorf("capture progress attempt id is required")
	}
	if len(segmentIDs) == 0 {
		return CaptureProgress{}, fmt.Errorf("capture progress segment ids are required")
	}
	allowed := make(map[string]bool, len(segmentIDs))
	for _, segmentID := range segmentIDs {
		if err := artifacts.ValidateArtifactToken("segment id", segmentID); err != nil {
			return CaptureProgress{}, err
		}
		if allowed[segmentID] {
			return CaptureProgress{}, fmt.Errorf("capture progress contains duplicate segment %q", segmentID)
		}
		allowed[segmentID] = true
	}
	seenCompleted := make(map[string]bool, len(completed))
	for _, segmentID := range completed {
		if !allowed[segmentID] {
			return CaptureProgress{}, fmt.Errorf("completed segment %q is outside the capture selection", segmentID)
		}
		if seenCompleted[segmentID] {
			return CaptureProgress{}, fmt.Errorf("capture progress contains duplicate completion %q", segmentID)
		}
		seenCompleted[segmentID] = true
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return CaptureProgress{
		SchemaVersion:       CaptureProgressSchemaVersion,
		AttemptID:           attemptID,
		SegmentIDs:          slices.Clone(segmentIDs),
		CompletedSegmentIDs: slices.Clone(completed),
		UpdatedAt:           now.UTC(),
	}, nil
}

func (p CaptureProgress) Validate() error {
	if p.SchemaVersion != CaptureProgressSchemaVersion {
		return fmt.Errorf("unsupported capture progress schema %q", p.SchemaVersion)
	}
	if p.UpdatedAt.IsZero() {
		return fmt.Errorf("capture progress updated at is required")
	}
	_, err := NewCaptureProgress(p.AttemptID, p.SegmentIDs, p.CompletedSegmentIDs, p.UpdatedAt)
	return err
}
