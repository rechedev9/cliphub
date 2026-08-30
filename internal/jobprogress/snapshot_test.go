package jobprogress

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestPercentOf(t *testing.T) {
	cases := []struct {
		done, total int64
		want        int
	}{
		{0, 0, 0},
		{0, 10, 0},
		{5, 10, 50},
		{1, 3, 33},
		{2, 3, 67},
		{10, 10, 100},
		{12, 10, 100},
		{-1, 10, 0},
		{5, -2, 0},
	}
	for _, tc := range cases {
		if got := PercentOf(tc.done, tc.total); got != tc.want {
			t.Fatalf("PercentOf(%d,%d) = %d, want %d", tc.done, tc.total, got, tc.want)
		}
	}
}

func TestNewSnapshotClampsAndLabels(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		done, total int64
		wantDone    int64
		wantPct     int
	}{
		{name: "mid", done: 64000, total: 172772, wantDone: 64000, wantPct: 37},
		{name: "over", done: 200, total: 100, wantDone: 100, wantPct: 100},
		{name: "neg", done: -4, total: 20, wantDone: 0, wantPct: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap, err := NewSnapshot(StageParse, UnitTicks, "ticks", tc.done, tc.total, now)
			if err != nil {
				t.Fatal(err)
			}
			if err := snap.Validate(); err != nil {
				t.Fatal(err)
			}
			if snap.Done != tc.wantDone || snap.Percent != tc.wantPct || snap.Label != "ticks" {
				t.Fatalf("snapshot = %+v, want done=%d percent=%d", snap, tc.wantDone, tc.wantPct)
			}
		})
	}
}

func TestNewSnapshotRequiresStageAndUnit(t *testing.T) {
	if _, err := NewSnapshot("", UnitTicks, "ticks", 1, 2, time.Now()); err == nil {
		t.Fatal("expected error for empty stage")
	}
	if _, err := NewSnapshot(StageParse, "", "ticks", 1, 2, time.Now()); err == nil {
		t.Fatal("expected error for empty unit")
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	snap, err := NewSnapshot(StageAnticheat, UnitTicks, "ticks", 8, 20, time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := snap.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	got, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stage != snap.Stage || got.Done != snap.Done || got.Total != snap.Total || got.Percent != snap.Percent {
		t.Fatalf("decoded %+v, want %+v", got, snap)
	}
	if !strings.Contains(got.Label, "ticks") {
		t.Fatalf("label = %q", got.Label)
	}
}
