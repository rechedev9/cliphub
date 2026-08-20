package recording

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
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
	// Percent is 0-100 of planned capture work, including the in-flight take.
	Percent int `json:"percent"`
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
		Percent:             CaptureWorkPercent(len(segmentIDs), len(completed), nil, 0),
	}, nil
}

func (p CaptureProgress) Validate() error {
	if p.SchemaVersion != CaptureProgressSchemaVersion {
		return fmt.Errorf("unsupported capture progress schema %q", p.SchemaVersion)
	}
	if p.UpdatedAt.IsZero() {
		return fmt.Errorf("capture progress updated at is required")
	}
	if p.Percent < 0 || p.Percent > 100 {
		return fmt.Errorf("capture progress percent %d is out of range", p.Percent)
	}
	_, err := NewCaptureProgress(p.AttemptID, p.SegmentIDs, p.CompletedSegmentIDs, p.UpdatedAt)
	return err
}

// CaptureWorkPercent is the 0-100 share of planned capture work already done.
// finished is how many leading segments are complete; currentFrac (0-1) is the
// in-flight take, from take elapsed vs that segment's tick duration. 100 is
// reserved for every segment completed; a live take caps at 99.
func CaptureWorkPercent(n, finished int, weights []int, currentFrac float64) int {
	if n <= 0 {
		return 0
	}
	if finished < 0 {
		finished = 0
	}
	if finished >= n {
		return 100
	}
	if currentFrac < 0 {
		currentFrac = 0
	}
	if currentFrac > 0.99 {
		currentFrac = 0.99
	}
	var total, recorded float64
	for i := 0; i < n; i++ {
		w := 1.0
		if i < len(weights) && weights[i] > 0 {
			w = float64(weights[i])
		}
		total += w
		switch {
		case i < finished:
			recorded += w
		case i == finished:
			recorded += w * currentFrac
		}
	}
	if total <= 0 {
		return 0
	}
	pct := int(math.Round(recorded / total * 100))
	if pct < 0 {
		return 0
	}
	if pct > 99 {
		return 99
	}
	return pct
}
