package library

import (
	"testing"

	"github.com/google/uuid"
)

func vetoCount(n int) VetoCount { return VetoCount{Count: n, SeriesIDs: make([]uuid.UUID, n)} }

func TestMapPreferenceFoldsVetoesPlayAndResults(t *testing.T) {
	records := []MapRecord{
		{Map: "de_nuke", Matches: 4, Wins: 3, Veto: VetoRecord{Picked: vetoCount(3)}},
		{Map: "de_vertigo", Veto: VetoRecord{Banned: vetoCount(4)}},
		{Map: "de_mirage", Matches: 2, Wins: 1, Ties: 1},
	}
	scorePreferences(records)

	nuke := records[0].Preference
	// veto 3/5, play 1, win 2·4/6 − 1: 100 × (0.3 + 0.25 + 0.0833).
	if nuke.Score != 63 || nuke.Veto != 0.6 || nuke.Play != 1 || nuke.Win != 0.3333 || nuke.Vetoes != 3 || nuke.Matches != 4 {
		t.Fatalf("nuke preference = %+v", nuke)
	}
	// Banned every time and never played: veto −4/6, play −1, win 0 with no matches.
	if vertigo := records[1].Preference; vertigo.Score != -58 || vertigo.Win != 0 || vertigo.Play != -1 {
		t.Fatalf("vertigo preference = %+v", vertigo)
	}
	// No vetoes: half the matches of the busiest map, a win and a tie.
	if mirage := records[2].Preference; mirage.Veto != 0 || mirage.Play != 0 || mirage.Win != 0.25 || mirage.Score != 6 {
		t.Fatalf("mirage preference = %+v", mirage)
	}
}

func TestMapPreferenceStaysInRange(t *testing.T) {
	best := mapPreference(MapRecord{Matches: 1000, Wins: 1000, Veto: VetoRecord{Picked: vetoCount(1000)}}, 1000)
	worst := mapPreference(MapRecord{Matches: 1000, Losses: 1000, Veto: VetoRecord{Banned: vetoCount(1000)}}, 100000)
	if best.Score > 100 || best.Score < 99 || worst.Score < -100 || worst.Score > -98 {
		t.Fatalf("extremes = %d and %d, want near 100 and -100", best.Score, worst.Score)
	}
	if empty := mapPreference(MapRecord{}, 0); empty.Score != 0 || empty.Play != 0 {
		t.Fatalf("a team with no history = %+v, want 0", empty)
	}
}
