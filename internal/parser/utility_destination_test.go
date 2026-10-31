package parser

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Positions are place centroids measured in a real Inferno demo; the old boxes
// called CT spawn "B site" and T spawn's side of mid "Apartments".
func TestInferUtilityDestinationNamesInfernoPlaces(t *testing.T) {
	none := tacticalplan.MapGeometry{}
	tests := []struct {
		name string
		pos  [3]float64
		want string
	}{
		{"CT spawn", [3]float64{2057, 2236, 150}, "CT Spawn"},
		{"B site", [3]float64{596, 2835, 160}, "Bombsite B"},
		{"banana", [3]float64{452, 1798, 110}, "Banana"},
		{"apartments", [3]float64{1343, -229, 150}, "Apartments"},
		{"A site", [3]float64{2048, 360, 160}, "Bombsite A"},
		{"top of mid", [3]float64{1410, 683, 150}, "Top of Mid"},
		{"lower mid", [3]float64{-644, 574, 0}, "Lower Mid"},
	}
	for _, tt := range tests {
		if got := inferUtilityDestination("de_inferno", SmokeGrenadeType, tt.pos, none); got != tt.want {
			t.Errorf("%s: inferUtilityDestination(%v) = %q, want %q", tt.name, tt.pos, got, tt.want)
		}
	}
	if got := inferUtilityDestination("de_inferno", FlashbangType, [3]float64{-3000, -3000, 0}, none); got != "contested area" {
		t.Errorf("flash outside every area = %q, want contested area", got)
	}
	if got := inferUtilityDestination("de_mirage", SmokeGrenadeType, [3]float64{596, 2835, 160}, none); got != "" {
		t.Errorf("other maps = %q, want no destination", got)
	}
}

func TestInferUtilityDestinationPrefersThePlacePlayersStoodAt(t *testing.T) {
	b := tacticalplan.NewOccupancyBuilder(radarmap.Calibration{}, placeCellSize)
	for i := 0; i < 3; i++ {
		b.Add(-110, 267, 195, "Kitchen")
	}
	places := b.Build("", radarmap.Calibration{}, 1)
	// No area box covers this spot; the players seen there were in Kitchen.
	if got := inferUtilityDestination("de_inferno", MolotovType, [3]float64{-100, 270, 196}, places); got != "Kitchen" {
		t.Fatalf("destination = %q, want Kitchen", got)
	}
}
