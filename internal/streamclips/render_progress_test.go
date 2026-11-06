package streamclips

import (
	"math"
	"testing"

	"github.com/google/uuid"
)

func TestNewRenderProgress(t *testing.T) {
	tests := []struct {
		name        string
		done, clips int
		fraction    float64
		wantPercent float64
		wantClip    int
	}{
		{name: "first clip starting", done: 0, clips: 4, fraction: 0, wantPercent: 0, wantClip: 1},
		{name: "halfway through the second clip", done: 1, clips: 4, fraction: 0.5, wantPercent: 37.5, wantClip: 2},
		{name: "last clip nearly done", done: 3, clips: 4, fraction: 0.99, wantPercent: 99, wantClip: 4},
		{name: "every clip done waits for publish", done: 4, clips: 4, fraction: 0, wantPercent: 99, wantClip: 4},
		{name: "fraction out of range is clamped", done: 0, clips: 2, fraction: 3, wantPercent: 50, wantClip: 1},
		{name: "no clips", done: 0, clips: 0, fraction: 0.5, wantPercent: 0, wantClip: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewRenderProgress(VariantStreamerFullframeNoCam, tt.done, tt.clips, tt.fraction)
			if math.Abs(got.Percent-tt.wantPercent) > 1e-9 || got.Clip != tt.wantClip {
				t.Fatalf("got %+v, want percent %v clip %d", got, tt.wantPercent, tt.wantClip)
			}
			if got.Variant != VariantStreamerFullframeNoCam || (tt.clips > 0 && got.Clips != tt.clips) {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestRenderProgressBoardNilIsSafe(t *testing.T) {
	var board *RenderProgressBoard
	board.Set(uuid.New(), RenderProgress{Percent: 10})
	if _, ok := board.Get(uuid.New()); ok {
		t.Fatal("nil board reported progress")
	}
	board.Clear(uuid.New())
}
