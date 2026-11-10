package tacticalplan

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

func TestRoundInstantsFollowTheTimeNotTheCount(t *testing.T) {
	a := roundSpan{match: scoutMatchOne, round: 3, from: 10, to: 20}.instants()
	if want := int(10 / HeatmapSampleSeconds); len(a) != want {
		t.Fatalf("a 10 s span gave %d instants, want %d", len(a), want)
	}
	for i, s := range a {
		// Each instant stays inside its own slot of the span, so they never cross.
		if lo, hi := 10+float64(i)*HeatmapSampleSeconds, 10+float64(i+1)*HeatmapSampleSeconds; s < lo || s > hi {
			t.Fatalf("instant %d = %v, want it in [%v, %v]", i, s, lo, hi)
		}
	}
	// A span twice as long holds twice the instants, so a round that lasts longer weighs more.
	if long := (roundSpan{match: scoutMatchOne, round: 3, from: 0, to: 20}).instants(); len(long) != 2*len(a) {
		t.Fatalf("a 20 s span gave %d instants, want %d", len(long), 2*len(a))
	}
	if odd := (roundSpan{match: scoutMatchOne, round: 3, from: 0, to: 10.3}).instants(); len(odd) != 41 || odd[0] < 0 || odd[40] > 10.3 {
		t.Fatalf("a 10.3 s span = %d instants, want 41 inside it", len(odd))
	}
	if short := (roundSpan{match: scoutMatchOne, round: 3, from: 4, to: 4.2}).instants(); len(short) != 0 {
		t.Fatalf("a span shorter than one step gave %d instants, want none", len(short))
	}
	if b := (roundSpan{match: scoutMatchOne, round: 3, from: 10, to: 20}).instants(); b[7] != a[7] {
		t.Fatal("the same round gave other instants")
	}
	if c := (roundSpan{match: scoutMatchOne, round: 4, from: 10, to: 20}).instants(); c[7] == a[7] {
		t.Fatal("another round should get its own jitter")
	}
}

func TestFramesAtInterpolatesBetweenFrames(t *testing.T) {
	frames := []Frame{{Tick: 100}, {Tick: 108}, {Tick: 116}}
	if _, _, _, _, ok := framesAt(frames, 99); ok {
		t.Fatal("before the first frame there is nothing to read")
	}
	if a, b, k, i, ok := framesAt(frames, 110); !ok || a.Tick != 108 || b.Tick != 116 || k != 0.25 || i != 1 {
		t.Fatalf("tick 110 = %d, %d, %v, %d, %v", a.Tick, b.Tick, k, i, ok)
	}
	if a, b, k, i, ok := framesAt(frames, 116); !ok || a.Tick != 116 || b.Tick != 116 || k != 0 || i != 2 {
		t.Fatalf("an exact frame = %d, %d, %v, %d", a.Tick, b.Tick, k, i)
	}
	if _, _, _, _, ok := framesAt(frames, 117); ok {
		t.Fatal("after the last frame there is nothing to read")
	}
}

func TestWindowSpanClipsTheRound(t *testing.T) {
	r := Round{TickFreezeEnd: 640, TickEnd: 640 + 64*90, Bomb: &Bomb{PlantTick: 640 + 64*40}}
	cases := []struct {
		w        SpatialWindow
		from, to float64
		ok       bool
	}{
		{SpatialWindow{}, 0, 90, true},
		{SpatialWindow{From: 55, To: 75}, 55, 75, true},
		{SpatialWindow{To: 200}, 0, 90, true},
		{SpatialWindow{FromPlant: true}, 40, 90, true},
		{SpatialWindow{From: 95}, 0, 0, false},
	}
	for _, c := range cases {
		from, to, ok := c.w.span(r, 64)
		if ok != c.ok || (ok && (from != c.from || to != c.to)) {
			t.Fatalf("%+v = %v, %v, %v; want %v, %v, %v", c.w, from, to, ok, c.from, c.to, c.ok)
		}
	}
	if _, _, ok := (SpatialWindow{FromPlant: true}).span(Round{TickFreezeEnd: 640, TickEnd: 6400}, 64); ok {
		t.Fatal("a round with no plant has nothing after the plant")
	}
}

func TestHeatmapDensityBinsTheMarksOnTheRadarSquare(t *testing.T) {
	m := spatialMatch(t, Filter{TeamKey: "ct-start", RoundTo: 1})
	radar := radarmap.Calibration{Map: "de_test", PosX: -1000, PosY: 1000, Scale: 4, Size: 500}
	h, err := BuildHeatmap([]SpatialMatch{m}, HeatmapOptions{Kind: HeatPosition, Window: SpatialWindow{To: 2}, Radar: radar})
	if err != nil {
		t.Fatal(err)
	}
	if h.DensityGrid != HeatDensityGrid || len(h.Density) != 1 || h.Density[0].Side != SideCT || h.Density[0].Seconds != nil {
		t.Fatalf("density = %+v, want one CT layer without seconds", h.Density)
	}
	// The square spans 2000 units, so one bin is 13.3 units: y 10 is row 74 and x 0 to 128 starts at column 75.
	total := 0
	for i, bin := range h.Density[0].Bins {
		if bin[1] != 74 || bin[0] < 75 || bin[0] > 85 || (i > 0 && bin[0] <= h.Density[0].Bins[i-1][0]) {
			t.Fatalf("bin %d = %v, want row 74 and columns 75 to 85 in order", i, bin)
		}
		total += bin[2]
	}
	if want := int(2 / HeatmapSampleSeconds); total != want {
		t.Fatalf("bins hold %d marks, want every one of the %d samples", total, want)
	}
	timing, _ := BuildHeatmap([]SpatialMatch{m}, HeatmapOptions{Kind: HeatTiming, Radar: radar})
	if len(timing.Density) != 1 || len(timing.Density[0].Seconds) != len(timing.Density[0].Bins) {
		t.Fatalf("timing density = %+v, want a mean second per bin", timing.Density)
	}
}
