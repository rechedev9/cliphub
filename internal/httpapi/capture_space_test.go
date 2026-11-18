package httpapi

import (
	"errors"
	"testing"
)

func TestSpaceCheckWarnsWhenTheEstimateDoesNotFit(t *testing.T) {
	free := func(n uint64, err error) func() (uint64, error) {
		return func() (uint64, error) { return n, err }
	}
	// 60 s of capture at 40 Mb/s is 300 MB; 60 s of render at 20 Mb/s is 150 MB.
	work := spaceEstimate{CaptureSeconds: 60, RenderSeconds: 60}
	if got := checkSpace(work, free(449_000_000, nil)); got == nil || !got.Short || got.NeededBytes != 450_000_000 || got.FreeBytes != 449_000_000 {
		t.Fatalf("449 MB free = %+v, want a warning for 450 MB", got)
	}
	if got := checkSpace(work, free(451_000_000, nil)); got == nil || got.Short {
		t.Fatalf("451 MB free = %+v, want no warning", got)
	}
	if got := checkSpace(spaceEstimate{RenderSeconds: 60}, free(200_000_000, nil)); got == nil || got.Short || got.NeededBytes != 150_000_000 {
		t.Fatalf("a render only = %+v, want 150 MB that fits", got)
	}
	if got := checkSpace(work, free(0, errors.New("no disk"))); got != nil {
		t.Fatalf("unknown free space = %+v, want no check", got)
	}
}
