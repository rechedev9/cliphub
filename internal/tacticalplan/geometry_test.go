package tacticalplan

import (
	"math"
	"testing"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

func TestOccupancyBuilderPacksCells(t *testing.T) {
	cal, ok := radarmap.Lookup("de_mirage")
	if !ok {
		t.Fatal("de_mirage must be calibrated")
	}
	b := NewOccupancyBuilder(cal, 64)
	// Two samples in the same cell, one in the next cell over.
	b.Add(10, 10, 0, "Middle")
	b.Add(20, 20, 0, "Middle")
	b.Add(100, 10, 0, "BombsiteA")

	geo := b.Build("de_mirage", cal, 1)
	if geo.Source != GeometrySourceOccupancy {
		t.Fatalf("source = %q, want %q", geo.Source, GeometrySourceOccupancy)
	}
	if geo.SampleCount != 3 {
		t.Fatalf("sample count = %d, want 3", geo.SampleCount)
	}
	if len(geo.Levels) != 1 || geo.Levels[0].Name != radarmap.LevelDefault {
		t.Fatalf("levels = %+v, want one default level", geo.Levels)
	}
	cells := geo.Levels[0].Cells
	if len(cells) != 2 {
		t.Fatalf("cells = %+v, want 2", cells)
	}
	if cells[0] != [3]int{0, 0, 2} {
		t.Fatalf("first cell = %v, want [0 0 2]", cells[0])
	}
	if cells[1] != [3]int{1, 0, 1} {
		t.Fatalf("second cell = %v, want [1 0 1]", cells[1])
	}
}

func TestOccupancyBuilderCallouts(t *testing.T) {
	cal, _ := radarmap.Lookup("de_mirage")
	b := NewOccupancyBuilder(cal, 64)
	b.Add(0, 0, 0, "Middle")
	b.Add(100, 200, 0, "Middle")
	b.Add(-500, -500, 0, "Apartments")
	b.Add(0, 0, 0, "") // no place name: counted in the grid, not as a callout

	geo := b.Build("de_mirage", cal, 1)
	if len(geo.Callouts) != 2 {
		t.Fatalf("callouts = %+v, want 2", geo.Callouts)
	}
	// Callouts are ordered by sample count, so the most-played one leads.
	if geo.Callouts[0].Name != "Middle" || geo.Callouts[0].Samples != 2 {
		t.Fatalf("first callout = %+v, want Middle with 2 samples", geo.Callouts[0])
	}
	if geo.Callouts[0].Center != [2]float64{50, 100} {
		t.Fatalf("Middle centre = %v, want the centre of mass (50, 100)", geo.Callouts[0].Center)
	}
	if geo.SampleCount != 4 {
		t.Fatalf("sample count = %d, want 4", geo.SampleCount)
	}
}

// Nuke's lower level must land on its own drawable layer, or the two floors
// overlap into an unreadable smear.
func TestOccupancyBuilderSplitsLevels(t *testing.T) {
	cal, ok := radarmap.Lookup("de_nuke")
	if !ok {
		t.Fatal("de_nuke must be calibrated")
	}
	b := NewOccupancyBuilder(cal, 64)
	b.Add(0, 0, 0, "Ramp")      // upper
	b.Add(0, 0, -900, "Vents")  // lower
	b.Add(64, 0, -900, "Vents") // lower

	geo := b.Build("de_nuke", cal, 1)
	if len(geo.Levels) != 2 {
		t.Fatalf("levels = %+v, want default and lower", geo.Levels)
	}
	byName := map[string]GeometryLevel{}
	for _, l := range geo.Levels {
		byName[l.Name] = l
	}
	if len(byName[radarmap.LevelDefault].Cells) != 1 {
		t.Fatalf("default level cells = %+v, want 1", byName[radarmap.LevelDefault].Cells)
	}
	if len(byName[radarmap.LevelLower].Cells) != 2 {
		t.Fatalf("lower level cells = %+v, want 2", byName[radarmap.LevelLower].Cells)
	}
	for _, c := range geo.Callouts {
		if c.Name == "Vents" && c.Level != radarmap.LevelLower {
			t.Fatalf("Vents callout is on level %q, want %q", c.Level, radarmap.LevelLower)
		}
	}
}

func TestOccupancyBuilderDropsSparseCells(t *testing.T) {
	cal, _ := radarmap.Lookup("de_mirage")
	b := NewOccupancyBuilder(cal, 64)
	b.Add(0, 0, 0, "Middle")
	b.Add(0, 0, 0, "Middle")
	b.Add(1000, 1000, 0, "Ladder") // seen once

	geo := b.Build("de_mirage", cal, 2)
	if len(geo.Levels) != 1 || len(geo.Levels[0].Cells) != 1 {
		t.Fatalf("cells = %+v, want only the twice-visited cell", geo.Levels)
	}
	for _, c := range geo.Callouts {
		if c.Name == "Ladder" {
			t.Fatal("a callout below the sample floor must be dropped")
		}
	}
}

func TestOccupancyBuilderTracksBounds(t *testing.T) {
	cal, _ := radarmap.Lookup("de_mirage")
	b := NewOccupancyBuilder(cal, 64)
	if !b.Bounds().Empty() {
		t.Fatal("a builder with no samples has empty bounds")
	}
	b.Add(-100, -200, 0, "")
	b.Add(300, 400, 0, "")
	got := b.Bounds()
	want := radarmap.Bounds{MinX: -100, MinY: -200, MaxX: 300, MaxY: 400}
	if got != want {
		t.Fatalf("bounds = %+v, want %+v", got, want)
	}
	if b.samples != 2 {
		t.Fatalf("samples = %d, want 2", b.samples)
	}
}

func TestOccupancyBuilderIgnoresNaN(t *testing.T) {
	cal, _ := radarmap.Lookup("de_mirage")
	b := NewOccupancyBuilder(cal, 64)
	b.Add(math.NaN(), 0, 0, "Middle")
	b.Add(0, math.NaN(), 0, "Middle")
	if b.samples != 0 {
		t.Fatalf("samples = %d, want 0", b.samples)
	}
}

func TestNearestCalloutUsesTheCellsEachPlaceOwns(t *testing.T) {
	cal, _ := radarmap.Lookup("de_mirage")
	b := NewOccupancyBuilder(cal, 64)
	// A long site whose far end lies closer to the stairs' centroid than its own.
	for cx := 0; cx < 10; cx++ {
		b.Add(float64(cx)*64+32, 32, 0, "BombsiteA")
	}
	b.Add(5*64+10, 10, 0, "BombsiteA")
	b.Add(5*64+20, 20, 0, "BombsiteA")
	b.Add(5*64+30, 30, 0, "Stairs") // outvoted in its cell
	b.Add(700, 100, 0, "Stairs")
	geo := b.Build("de_mirage", cal, 1)

	var site, stairs Callout
	for _, c := range geo.Callouts {
		switch c.Name {
		case "BombsiteA":
			site = c
		case "Stairs":
			stairs = c
		}
	}
	if len(site.Cells) != 10 || site.Cells[0] != [2]int{0, 0} || site.Cells[9] != [2]int{9, 0} {
		t.Fatalf("BombsiteA cells = %v, want cells 0..9 of row 0 in order", site.Cells)
	}
	if len(stairs.Cells) != 1 || stairs.Cells[0] != [2]int{10, 1} {
		t.Fatalf("Stairs cells = %v, want only [10 1]", stairs.Cells)
	}

	cases := []struct {
		name string
		pos  [3]float64
		want string
	}{
		{"an owned cell beats a closer centroid", [3]float64{600, 40, 0}, "BombsiteA"},
		{"an unowned cell takes its closest owned neighbour", [3]float64{650, -30, 0}, "BombsiteA"},
		{"off the grid falls back to the nearest centroid", [3]float64{2000, 2000, 0}, "Stairs"},
	}
	for _, tc := range cases {
		if got := NearestCallout(geo, tc.pos); got != tc.want {
			t.Errorf("%s: NearestCallout(%v) = %q, want %q", tc.name, tc.pos, got, tc.want)
		}
	}
}

func TestNearestCalloutSplitsACellSharedByTwoFloors(t *testing.T) {
	cal, _ := radarmap.Lookup("de_mirage")
	b := NewOccupancyBuilder(cal, 64)
	// Catwalk at -168 over Underpass at -368, in the same 64-unit cell; the
	// lagging place name also tags a few Catwalk positions as Underpass.
	for i := 0; i < 6; i++ {
		b.Add(10, 10, -168, "Catwalk")
	}
	b.Add(12, 12, -168, "Underpass")
	for i := 0; i < 3; i++ {
		b.Add(20, 20, -368, "Underpass")
	}
	// A cell where both names sit on one floor is not split.
	for i := 0; i < 4; i++ {
		b.Add(100, 10, -168, "Catwalk")
	}
	b.Add(100, 12, -160, "Middle")
	b.Add(100, 14, -160, "Middle")
	geo := b.Build("de_mirage", cal, 1)

	cases := []struct {
		pos  [3]float64
		want string
	}{
		{[3]float64{30, 30, -170}, "Catwalk"},
		{[3]float64{30, 30, -360}, "Underpass"},
		{[3]float64{110, 30, -168}, "Catwalk"},
		{[3]float64{110, 30, -380}, "Catwalk"},
	}
	for _, tc := range cases {
		if got := NearestCallout(geo, tc.pos); got != tc.want {
			t.Errorf("NearestCallout(%v) = %q, want %q", tc.pos, got, tc.want)
		}
	}
	// Without the altitude data, as in an older document, the cell's majority wins.
	for i := range geo.Callouts {
		geo.Callouts[i].Stacked = nil
	}
	if got := NearestCallout(geo, [3]float64{30, 30, -360}); got != "Catwalk" {
		t.Errorf("NearestCallout without stacked cells = %q, want Catwalk", got)
	}
}
