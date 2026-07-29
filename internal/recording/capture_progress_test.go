package recording

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCaptureProgressValidateRejectsZeroUpdatedAt(t *testing.T) {
	progress, err := NewCaptureProgress(uuid.New(), []string{"seg-001"}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	progress.UpdatedAt = time.Time{}

	err = progress.Validate()
	if err == nil || !strings.Contains(err.Error(), "updated at is required") {
		t.Fatalf("Validate error = %v, want missing updated_at error", err)
	}
}
