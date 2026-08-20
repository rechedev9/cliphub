package recording

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCaptureWorkPercent(t *testing.T) {
	tests := []struct {
		name        string
		n           int
		finished    int
		weights     []int
		currentFrac float64
		want        int
	}{
		{name: "empty", n: 0, want: 0},
		{name: "none started", n: 4, finished: 0, want: 0},
		{name: "three of four equal", n: 4, finished: 3, want: 75},
		{name: "all done", n: 4, finished: 4, want: 100},
		{name: "finished past n clamps to 100", n: 2, finished: 9, want: 100},
		{name: "live take half of last equal", n: 4, finished: 3, currentFrac: 0.5, want: 88},
		{name: "live take never reports 100", n: 2, finished: 1, currentFrac: 1, want: 99},
		{name: "first take a third in", n: 4, finished: 0, currentFrac: 1.0 / 3.0, want: 8},
		{
			name:        "tick-weighted long last clip",
			n:           2,
			finished:    1,
			weights:     []int{10, 90},
			currentFrac: 0.5,
			want:        55,
		},
		{
			name:     "zero weights fall back to equal",
			n:        4,
			finished: 2,
			weights:  []int{0, 0, 0, 0},
			want:     50,
		},
		{name: "negative frac ignored", n: 4, finished: 1, currentFrac: -2, want: 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CaptureWorkPercent(tt.n, tt.finished, tt.weights, tt.currentFrac)
			if got != tt.want {
				t.Fatalf("CaptureWorkPercent(%d, %d, %v, %g) = %d, want %d", tt.n, tt.finished, tt.weights, tt.currentFrac, got, tt.want)
			}
		})
	}
}

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
