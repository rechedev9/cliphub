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
	// A real FACEIT demo carries 2026-10-17 17:03:39 and 16:34:18 UTC, decoded as uint64.
	for _, raw := range []any{uint64(1792256619), uint64(0), uint64(1600000000), uint64(1) << 40, int32(1792250000), float32(1792250000), uint64(1792254858)} {
		c.observe(raw)
	}
	got, ok := c.Earliest()
	if want := time.Date(2026, time.October, 17, 16, 34, 18, 0, time.UTC); !ok || !got.Equal(want) {
		t.Fatalf("Earliest = %v, %v; want %v (unsigned, pre-CS2, out-of-range and other types ignored)", got, ok, want)
	}
	c.observe(uint32(1792254000))
	if got, _ := c.Earliest(); !got.Equal(time.Date(2026, time.October, 17, 16, 20, 0, 0, time.UTC)) {
		t.Fatalf("Earliest = %v after an earlier uint32 spray", got)
	}
}
