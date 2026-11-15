package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// list_moments must pass on whether a clutch was won, a lost one included, and the win-chance swing.
func TestDescribeMomentCarriesClutchResultAndWinSwing(t *testing.T) {
	var m tuiclient.Moment
	if err := json.Unmarshal([]byte(`{"segment_id":"seg-001","clutch_vs":3,"clutch_won":false,"win_prob_swing":0.42,"reason_codes":["clutch","fast_triple"]}`), &m); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(describeMoment(m))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"clutch_won":false`, `"win_prob_swing":0.42`, `"fast_triple"`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("moment = %s, missing %s", out, want)
		}
	}
}
