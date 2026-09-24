package workers

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/obs"
)

type reasonRecorder struct{ reason string }

func (r *reasonRecorder) UpdateStatus(_ context.Context, _ uuid.UUID, _ job.Status, reason string) error {
	r.reason = reason
	return nil
}

// Studio matches the stored reason by prefix (demo_incompatible:,
// unplayable_start:, ...), so the remote-only failure code must not lead it.
func TestMarkFailedStoresReasonWithoutFailureCode(t *testing.T) {
	err := obs.WithFailure(errors.New("demo_incompatible: the demo stops early"), obs.FailureCaptureIncomplete, obs.SubstageCapture)
	repo := &reasonRecorder{}
	if markErr := markFailed(repo, uuid.New(), err.Error()); markErr != nil {
		t.Fatal(markErr)
	}
	if repo.reason != "demo_incompatible: the demo stops early" {
		t.Fatalf("stored reason = %q", repo.reason)
	}
}
