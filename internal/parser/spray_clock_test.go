package parser

import (
	"testing"
	"time"
)

func TestSprayClockKeepsTheEarliestSignedSpray(t *testing.T) {
	c := &SprayClock{}
	if _, ok := c.Earliest(); ok {
		t.Fatal("a demo without sprays has a date")
	}
	// 2026-10-17 16:34:18 UTC and 2026-10-17 17:03:39 UTC, as two FACEIT sprays carry them.
	for _, raw := range []any{uint32(1792256619), uint32(0), int32(-5), uint32(1600000000), float32(1792250000), uint32(1792254858)} {
		c.observe(raw)
	}
	got, ok := c.Earliest()
	if want := time.Date(2026, time.October, 17, 16, 34, 18, 0, time.UTC); !ok || !got.Equal(want) {
		t.Fatalf("Earliest = %v, %v; want %v (unsigned, pre-CS2 and non-integer values ignored)", got, ok, want)
	}
}
