package anticheat

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
)

// pushSamples fills a track's ring with one sample per tick, starting at
// startTick, using the yaw values supplied and a level pitch.
func pushSamples(t *track, startTick int, yaws []float64, ringSize int) {
	for i, yaw := range yaws {
		t.push(angleSample{tick: startTick + i, yaw: yaw}, ringSize)
	}
}

func newAimTrack(ringSize int) *track {
	return &track{
		ring:         make([]angleSample, ringSize),
		preaimTicks:  map[uint64]int{},
		spottedSince: map[uint64]int{},
	}
}

func TestViewVectorPointsWhereTheCrosshairPoints(t *testing.T) {
	// Yaw 0 looks down +X; yaw 90 looks down +Y; a positive pitch aims down.
	if v := viewVector(0, 0); math.Abs(v.X-1) > 1e-9 || math.Abs(v.Y) > 1e-9 || math.Abs(v.Z) > 1e-9 {
		t.Fatalf("viewVector(0,0) = %+v, want +X", v)
	}
	if v := viewVector(90, 0); math.Abs(v.Y-1) > 1e-9 {
		t.Fatalf("viewVector(90,0) = %+v, want +Y", v)
	}
	if v := viewVector(0, 90); v.Z >= 0 {
		t.Fatalf("viewVector(0,90) = %+v, want a downward Z for a positive pitch", v)
	}
}

func TestAngleBetween(t *testing.T) {
	if got := angleBetween(r3.Vector{X: 1}, r3.Vector{Y: 1}); math.Abs(got-90) > 1e-9 {
		t.Fatalf("angleBetween(+X, +Y) = %g, want 90", got)
	}
	if got := angleBetween(r3.Vector{X: 1}, r3.Vector{X: 5}); got > 1e-9 {
		t.Fatalf("angleBetween(+X, +5X) = %g, want 0", got)
	}
	if got := angleBetween(r3.Vector{}, r3.Vector{X: 1}); got != 180 {
		t.Fatalf("angleBetween(zero, +X) = %g, want 180 so it can never read as a lock", got)
	}
}

func TestShortestArcWrapsAroundZero(t *testing.T) {
	if got := shortestArc(359 - 1); math.Abs(got+2) > 1e-9 {
		t.Fatalf("shortestArc(358) = %g, want -2", got)
	}
	if got := shortestArc(-359); math.Abs(got-1) > 1e-9 {
		t.Fatalf("shortestArc(-359) = %g, want 1", got)
	}
	if got := shortestArc(45); got != 45 {
		t.Fatalf("shortestArc(45) = %g, want 45", got)
	}
}

func TestRingKeepsOnlyTheMostRecentWindow(t *testing.T) {
	tr := newAimTrack(4)
	pushSamples(tr, 100, []float64{0, 1, 2, 3, 4, 5}, 4)

	window := tr.window()
	if len(window) != 4 {
		t.Fatalf("window length = %d, want the ring size 4", len(window))
	}
	if window[0].tick != 102 || window[3].tick != 105 {
		t.Fatalf("window = ticks %d..%d, want 102..105", window[0].tick, window[3].tick)
	}
}

func TestPreShotAimFindsTheFlickAndItsSettleTime(t *testing.T) {
	tr := newAimTrack(16)
	// Still, then a 60° jump in one tick, then four ticks of holding still.
	pushSamples(tr, 100, []float64{0, 0, 0, 60, 60, 60, 60}, 16)

	peak, settle, jitter, ok := tr.preShotAim(106, 64)
	if !ok {
		t.Fatal("preShotAim reported no usable angles")
	}
	// 60° in one 64-tick frame is 3840 °/s.
	if math.Abs(peak-3840) > 1 {
		t.Fatalf("peak = %g °/s, want ~3840", peak)
	}
	// The peak landed on tick 103, three ticks (~47 ms) before the kill.
	if math.Abs(settle-3.0/64*1000) > 0.5 {
		t.Fatalf("settle = %g ms, want ~%g", settle, 3.0/64*1000)
	}
	if jitter != 0 {
		t.Fatalf("jitter = %g, want 0 for a single clean flick", jitter)
	}
}

func TestPreShotAimCountsDirectionReversals(t *testing.T) {
	tr := newAimTrack(16)
	pushSamples(tr, 100, []float64{0, 2, 0, 2, 0, 2}, 16)

	_, _, jitter, ok := tr.preShotAim(105, 64)
	if !ok {
		t.Fatal("preShotAim reported no usable angles")
	}
	if jitter <= 0.5 {
		t.Fatalf("jitter = %g, want a high reversal share for an alternating aim", jitter)
	}
}

func TestPreShotAimIgnoresSamplesAfterTheKill(t *testing.T) {
	tr := newAimTrack(16)
	// The huge jump happens after the kill tick and must not be counted.
	pushSamples(tr, 100, []float64{0, 1, 2, 3, 170}, 16)

	peak, _, _, ok := tr.preShotAim(102, 64)
	if !ok {
		t.Fatal("preShotAim reported no usable angles")
	}
	if peak > 100 {
		t.Fatalf("peak = %g °/s, want only the pre-kill motion", peak)
	}
}

func TestPreShotAimNeedsEnoughSamples(t *testing.T) {
	tr := newAimTrack(16)
	pushSamples(tr, 100, []float64{0, 1}, 16)

	if _, _, _, ok := tr.preShotAim(101, 64); ok {
		t.Fatal("preShotAim reported usable angles from two samples")
	}
}

func TestPreShotAimHandlesYawWrap(t *testing.T) {
	tr := newAimTrack(16)
	// Crossing 360 → 0 is a 4° move, not a 356° spin.
	pushSamples(tr, 100, []float64{358, 359, 0, 1, 2}, 16)

	peak, _, jitter, ok := tr.preShotAim(104, 64)
	if !ok {
		t.Fatal("preShotAim reported no usable angles")
	}
	if peak > 1.5*64 {
		t.Fatalf("peak = %g °/s, want ~64 for a steady 1°/tick pan across the wrap", peak)
	}
	if jitter != 0 {
		t.Fatalf("jitter = %g, want 0: a wrap is not a direction reversal", jitter)
	}
}

func TestViewAnglesNormalisePitch(t *testing.T) {
	// The demo reports pitch as 270..90; 270 means looking straight up.
	if got := normalisePitch(270); got != -90 {
		t.Fatalf("pitch 270 normalised to %g, want -90", got)
	}
	if got := normalisePitch(45); got != 45 {
		t.Fatalf("pitch 45 normalised to %g, want 45", got)
	}
}

// normalisePitch mirrors the conversion viewAngles applies, so the wrap can be
// tested without a demo-backed player.
func normalisePitch(pitch float64) float64 {
	if pitch >= 180 {
		pitch -= 360
	}
	return pitch
}
