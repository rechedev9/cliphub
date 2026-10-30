package recapplan

import "testing"

func TestRoundSelectionLimitsThePlanAndItsHash(t *testing.T) {
	all := fixtureDocument(t)
	o := fixtureOptions()
	o.Rounds = []int{2, 25}
	d, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "no_packets"}, nil, "facts")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Rounds) != 2 || d.Rounds[0].Number != 2 || d.Rounds[1].Number != 25 || len(d.Blockers) != 0 {
		t.Fatalf("selected rounds = %+v, blockers %+v", d.Rounds, d.Blockers)
	}
	if d.PlanHash == all.PlanHash {
		t.Fatal("a round selection must change the plan hash")
	}
	// No selection keeps the wire format, so approved documents keep their hash.
	if again, _ := Plan(fixtureFacts(), fixtureOptions(), VoiceEvidence{Availability: "no_packets"}, nil, "facts"); again.PlanHash != all.PlanHash {
		t.Fatal("an empty selection changed the hash")
	}
	o.Rounds = []int{7}
	if none, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "no_packets"}, nil, "facts"); err != nil || len(none.Blockers) == 0 {
		t.Fatalf("a selection with no round of the player = %+v, %v; want a blocker", none.Blockers, err)
	}
	for _, rounds := range [][]int{{2, 2}, {3, 1}, {0}} {
		o.Rounds = rounds
		if err := o.Validate(); err == nil {
			t.Fatalf("Validate accepted rounds %v", rounds)
		}
	}
	current := DefaultOptions()
	current.Rounds = []int{25, 2, 2}
	canonical, err := CanonicalNewOptions(current)
	if err != nil || len(canonical.Rounds) != 2 || canonical.Rounds[0] != 2 {
		t.Fatalf("canonical rounds = %v, %v", canonical.Rounds, err)
	}
}
