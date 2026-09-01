package editor

import (
	"encoding/json"
	"os"
	"testing"
)

func TestFFmpegEncodeFraction(t *testing.T) {
	tests := []struct {
		name     string
		outTime  int64
		expected float64
		wantFrac float64
		wantOK   bool
	}{
		{name: "zero duration skips", outTime: 1_000_000, expected: 0, wantOK: false},
		{name: "halfway", outTime: 30_000_000, expected: 60, wantFrac: 0.5, wantOK: true},
		{name: "caps at 99 percent of pass", outTime: 120_000_000, expected: 60, wantFrac: 0.99, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ffmpegEncodeFraction(tt.outTime, tt.expected)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got != tt.wantFrac {
				t.Fatalf("fraction = %v, want %v", got, tt.wantFrac)
			}
		})
	}
}

func TestParseFFmpegProgressOutTimeUs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int64
		wantOK  bool
	}{
		{
			name: "reads last out_time_us",
			content: "frame=1\nout_time_us=1000000\nprogress=continue\nout_time_us=2500000\nprogress=continue\n",
			want:   2_500_000,
			wantOK: true,
		},
		{name: "missing value", content: "progress=continue\n", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseFFmpegProgressOutTimeUs(tt.content)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got != tt.want {
				t.Fatalf("out_time_us = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEncodeProgressStateMonotonic(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/progress.json"
	tracker := NewProgressTracker(path)
	plan := buildEncodeProgressPlan([]ShortEdit{
		{DurationSeconds: 60, Parts: []ShortPart{{DurationSeconds: 60}}},
		{DurationSeconds: 40, Parts: []ShortPart{{DurationSeconds: 40}}},
	})
	state := newEncodeProgressState(plan, tracker, "Montando cortes y ritmo")
	last := 0
	record := func(pct int) {
		if pct < last {
			t.Fatalf("percent went backwards: %d after %d", pct, last)
		}
		last = pct
	}
	state.setFraction(0, 0.2)
	record(readProgressPercent(t, path))
	state.setFraction(1, 0.5)
	record(readProgressPercent(t, path))
	state.markDone(0)
	record(readProgressPercent(t, path))
	state.markDone(1)
	record(readProgressPercent(t, path))
	if last < progressFinalizeStart {
		t.Fatalf("final tracked percent = %d, want at least %d", last, progressFinalizeStart)
	}
}

func TestMapPassPercent(t *testing.T) {
	tests := []struct {
		start, end int
		frac       float64
		want       int
	}{
		{5, 92, 0, 5},
		{5, 92, 0.5, 49},
		{5, 92, 1, 92},
	}
	for _, tt := range tests {
		got := MapPassPercent(tt.start, tt.end, tt.frac)
		if got != tt.want {
			t.Fatalf("MapPassPercent(%d,%d,%v) = %d, want %d", tt.start, tt.end, tt.frac, got, tt.want)
		}
	}
}

func readProgressPercent(t *testing.T, path string) int {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var progress EditorProgress
	if err := json.Unmarshal(body, &progress); err != nil {
		t.Fatal(err)
	}
	return progress.Percent
}
