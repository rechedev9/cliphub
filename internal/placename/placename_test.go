package placename

import "testing"

// The cases match web/lib/tactical-labels.test.ts, so the two stay in step.
func TestLabelReadsTheCalloutPlayersSay(t *testing.T) {
	tests := map[string]string{
		"BombsiteA":      "Bombsite A",
		"CTSpawn":        "CT Spawn",
		"TSpawn":         "T Spawn",
		"BDoors":         "B Doors",
		"ARamp":          "A Ramp",
		"CTSideUpper":    "CT Side Upper",
		"PalaceInterior": "Palace Interior",
		"TopofMid":       "Top of Mid",
		"BackofB":        "Back of B",
		"SnipersNest":    "Sniper's Nest",
		"Banana":         "Banana",
		"Floor50":        "50th Floor",
		"Mid2":           "Mid 2",
		"":               "",
	}
	for token, want := range tests {
		if got := Label(token); got != want {
			t.Errorf("Label(%q) = %q, want %q", token, got, want)
		}
	}
}
