package worldoverlay

import (
	"math"
	"testing"
)

func TestViewProjectsThroughTheCS2Camera(t *testing.T) {
	v := newView(Camera{Eye: Vec{0, 0, 64}, Side: "CT"}, 1920, 1080)
	near := func(t *testing.T, gotX, gotY, wantX, wantY float64) {
		t.Helper()
		if math.Abs(gotX-wantX) > 0.01 || math.Abs(gotY-wantY) > 0.01 {
			t.Fatalf("projected to (%.2f, %.2f), want (%.2f, %.2f)", gotX, gotY, wantX, wantY)
		}
	}
	x, y, ok := v.project(Vec{100, 0, 64})
	if !ok {
		t.Fatal("a point straight ahead did not project")
	}
	near(t, x, y, 960, 540)
	// Default FOV: the focal length is height / 1.5 = 720 px on every aspect ratio, and +Y is left.
	x, y, _ = v.project(Vec{100, -50, 64})
	near(t, x, y, 1320, 540)
	x, y, _ = v.project(Vec{100, 0, 114})
	near(t, x, y, 960, 180)
	if _, _, ok := v.project(Vec{-100, 0, 64}); ok {
		t.Error("a point behind the eye projected")
	}
	// Positive pitch looks down, so a point below and ahead sits at the centre.
	down := newView(Camera{Eye: Vec{0, 0, 64}, Pitch: 45}, 1920, 1080)
	x, y, _ = down.project(Vec{100, 0, -36})
	near(t, x, y, 960, 540)
	// Yaw 90 looks along +Y.
	left := newView(Camera{Eye: Vec{0, 0, 64}, Yaw: 90}, 1920, 1080)
	x, y, _ = left.project(Vec{0, 100, 64})
	near(t, x, y, 960, 540)
}

func TestClipNearKeepsOnlyThePartInFront(t *testing.T) {
	v := newView(Camera{Eye: Vec{0, 0, 0}}, 1920, 1080)
	a, b, ok := v.clipNear(Vec{-50, 10, 0}, Vec{50, 10, 0})
	if !ok || math.Abs(v.depth(a)-nearDepth) > 1e-9 || b != (Vec{50, 10, 0}) {
		t.Fatalf("clipNear = %v %v %v, want a moved to the near plane", a, b, ok)
	}
	if _, _, ok := v.clipNear(Vec{-50, 0, 0}, Vec{-10, 0, 0}); ok {
		t.Error("a segment wholly behind the eye was kept")
	}
}

func TestSamplerInterpolatesAndHidesZoom(t *testing.T) {
	frames := []Frame{
		{Tick: 100, Camera: &Camera{Eye: Vec{0, 0, 0}, Yaw: 359}},
		{Tick: 101, Camera: &Camera{Eye: Vec{10, 0, 0}, Yaw: 1}},
	}
	for tick := 102; tick < 140; tick++ {
		c := Camera{FOV: 90}
		if tick >= 110 && tick < 115 {
			c = Camera{FOV: 40, Scoped: true}
		}
		frames = append(frames, Frame{Tick: tick, Camera: &c})
	}
	frames = append(frames, Frame{Tick: 150, Camera: &Camera{}})
	s := newSampler(Segment{Frames: frames})
	c, ok := s.camera(100.5)
	if !ok || c.Eye != (Vec{5, 0, 0}) || math.Abs(c.Yaw-360) > 1e-9 {
		t.Fatalf("camera(100.5) = %+v %v, want the midpoint across the yaw wrap", c, ok)
	}
	if _, ok := s.camera(99); ok {
		t.Error("a tick before the first sample has a camera")
	}
	if _, ok := s.camera(112); ok {
		t.Error("a scoped tick has a camera")
	}
	// CS2 animates the FOV back after the scope, so the overlay waits zoomSettleTicks.
	if _, ok := s.camera(115 + zoomSettleTicks - 1); ok {
		t.Error("the camera came back before the zoom settled")
	}
	if _, ok := s.camera(115 + zoomSettleTicks); !ok {
		t.Error("the camera stayed hidden after the zoom settled")
	}
	if _, ok := s.camera(145); ok {
		t.Error("a gap longer than maxTickGap was interpolated")
	}
}

func TestSamplerDropsPlayersGoneAtTheNextSample(t *testing.T) {
	s := newSampler(Segment{Frames: []Frame{
		{Tick: 10, Players: []Player{{Slot: 0, Eye: Vec{0, 0, 0}}, {Slot: 1}}},
		{Tick: 11, Players: []Player{{Slot: 0, Eye: Vec{4, 0, 0}}}},
	}})
	got := s.players(10.25)
	if len(got) != 1 || got[0].Slot != 0 || got[0].Eye != (Vec{1, 0, 0}) {
		t.Fatalf("players(10.25) = %+v, want slot 0 interpolated and slot 1 dropped", got)
	}
	if got := s.players(10); len(got) != 2 {
		t.Fatalf("players(10) = %+v, want both players of the exact sample", got)
	}
}
