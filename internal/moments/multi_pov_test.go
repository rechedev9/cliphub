package moments

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func TestMultiPOVMomentsNameTheirPlayer(t *testing.T) {
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	a, b := killplan.Target{SteamID64: "1", NameInDemo: "ropz"}, killplan.Target{SteamID64: "2", NameInDemo: "b1t"}
	plan.Target = a
	plan.Segments = []killplan.Segment{
		{ID: "seg-001", TickStart: 1000, TickEnd: 1500, POV: &a},
		{ID: "seg-002", TickStart: 3000, TickEnd: 3500, POV: &b},
	}
	doc := Build(uuid.Nil, plan)
	if doc.Moments[1].Player != "b1t" || doc.Moments[1].SteamID64 != "2" {
		t.Fatalf("second moment = %q/%q, want b1t/2", doc.Moments[1].Player, doc.Moments[1].SteamID64)
	}
	for _, player := range []string{"2", "B1T"} {
		got := Apply(doc.Moments, Filter{Player: player})
		if len(got) != 1 || got[0].SegmentID != "seg-002" {
			t.Errorf("filter %q kept %+v, want only seg-002", player, got)
		}
	}
	single := plan
	single.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 1000, TickEnd: 1500}}
	if m := Build(uuid.Nil, single).Moments[0]; m.SteamID64 != "" || m.Player != "ropz" {
		t.Errorf("single-POV moment = %q/%q, want the target name and no SteamID", m.Player, m.SteamID64)
	}
}
