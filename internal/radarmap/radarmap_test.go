package radarmap

import (
	"math"
	"testing"
)

func TestLookupKnownMap(t *testing.T) {
	c, ok := Lookup("de_mirage")
	if !ok {
		t.Fatal("de_mirage must be calibrated")
	}
	if c.Source != SourceOverview {
		t.Fatalf("source = %q, want %q", c.Source, SourceOverview)
	}
	if c.PosX != -3230 || c.PosY != 1713 || c.Scale != 5.0 || c.Size != 1024 {
		t.Fatalf("unexpected mirage calibration: %+v", c)
	}
}

func TestLookupUnknownMap(t *testing.T) {
	if _, ok := Lookup("de_workshop_thing"); ok {
		t.Fatal("unknown map must not report a calibration")
	}
}

// The expected pixels come from CS2's own overview values for each map, worked
// through (world-pos)/scale by hand. They are the anchor that keeps the
// transform honest: a scale or sign regression moves these by whole pixels.
func TestWorldToPixel(t *testing.T) {
	tests := []struct {
		name           string
		mapName        string
		worldX, worldY float64
		wantX, wantY   float64
	}{
		{"mirage mid", "de_mirage", -1500, -600, 346.0, 462.6},
		{"mirage upper-left corner", "de_mirage", -3230, 1713, 0, 0},
		{"inferno banana", "de_inferno", 500, 1200, 527.9591836734694, 544.8979591836735},
		{"dust2 origin", "de_dust2", -2476, 3239, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, ok := Lookup(tt.mapName)
			if !ok {
				t.Fatalf("%s must be calibrated", tt.mapName)
			}
			gotX, gotY := c.WorldToPixel(tt.worldX, tt.worldY)
			if math.Abs(gotX-tt.wantX) > 1e-9 || math.Abs(gotY-tt.wantY) > 1e-9 {
				t.Fatalf("WorldToPixel(%v, %v) = (%v, %v), want (%v, %v)",
					tt.worldX, tt.worldY, gotX, gotY, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestPixelToWorldRoundTrip(t *testing.T) {
	for name := range calibrations {
		c, ok := Lookup(name)
		if !ok {
			t.Fatalf("%s must be calibrated", name)
		}
		for _, pt := range [][2]float64{{0, 0}, {123.5, 987.25}, {1023, 1023}} {
			wx, wy := c.PixelToWorld(pt[0], pt[1])
			gotX, gotY := c.WorldToPixel(wx, wy)
			if math.Abs(gotX-pt[0]) > 1e-6 || math.Abs(gotY-pt[1]) > 1e-6 {
				t.Fatalf("%s round trip of (%v, %v) = (%v, %v)", name, pt[0], pt[1], gotX, gotY)
			}
		}
	}
}

func TestLevelSplitsOnAltitude(t *testing.T) {
	nuke, ok := Lookup("de_nuke")
	if !ok {
		t.Fatal("de_nuke must be calibrated")
	}
	if !nuke.MultiLevel() {
		t.Fatal("de_nuke is a split-level map")
	}
	if got := nuke.Level(-600); got != LevelLower {
		t.Fatalf("Level(-600) = %q, want %q", got, LevelLower)
	}
	if got := nuke.Level(-495); got != LevelLower {
		t.Fatalf("Level at the exact threshold = %q, want %q", got, LevelLower)
	}
	if got := nuke.Level(-100); got != LevelDefault {
		t.Fatalf("Level(-100) = %q, want %q", got, LevelDefault)
	}

	mirage, _ := Lookup("de_mirage")
	if mirage.MultiLevel() {
		t.Fatal("de_mirage has one level")
	}
	if got := mirage.Level(-99999); got != LevelDefault {
		t.Fatalf("single-level map must always answer %q, got %q", LevelDefault, got)
	}
}

func TestDeriveCalibrationFramesBounds(t *testing.T) {
	b := Bounds{MinX: -1000, MinY: -500, MaxX: 1000, MaxY: 500}
	c, err := DeriveCalibration("de_unknown", b, 1024, 100)
	if err != nil {
		t.Fatalf("DeriveCalibration: %v", err)
	}
	if c.Source != SourceDerived {
		t.Fatalf("source = %q, want %q", c.Source, SourceDerived)
	}
	// Every corner of the padded bounds must land inside the radar square.
	for _, pt := range [][2]float64{
		{b.MinX, b.MinY}, {b.MinX, b.MaxY}, {b.MaxX, b.MinY}, {b.MaxX, b.MaxY},
	} {
		px, py := c.WorldToPixel(pt[0], pt[1])
		if px < 0 || px > float64(c.Size) || py < 0 || py > float64(c.Size) {
			t.Fatalf("world %v maps to (%v, %v), outside the %d px radar", pt, px, py, c.Size)
		}
	}
	// A square framing keeps the world centre at the radar centre.
	cx, cy := c.WorldToPixel(0, 0)
	if math.Abs(cx-512) > 1e-6 || math.Abs(cy-512) > 1e-6 {
		t.Fatalf("world centre maps to (%v, %v), want the radar centre", cx, cy)
	}
}

func TestDeriveCalibrationRejectsBadInput(t *testing.T) {
	valid := Bounds{MinX: -1, MinY: -1, MaxX: 1, MaxY: 1}
	tests := []struct {
		name   string
		bounds Bounds
		size   int
		margin float64
	}{
		{"empty bounds", Bounds{}, 1024, 0},
		{"inverted bounds", Bounds{MinX: 1, MinY: 1, MaxX: -1, MaxY: -1}, 1024, 0},
		{"zero size", valid, 0, 0},
		{"negative margin", valid, 1024, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DeriveCalibration("de_unknown", tt.bounds, tt.size, tt.margin); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestCalibrationValid(t *testing.T) {
	if (Calibration{}).Valid() {
		t.Fatal("zero calibration must not be valid")
	}
	c, _ := Lookup("de_ancient")
	if !c.Valid() {
		t.Fatal("shipped calibration must be valid")
	}
}
