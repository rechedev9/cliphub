package tactical

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestAnAirKillMarksOnlyTheKillOfItsTick(t *testing.T) {
	events := []tacticalplan.Event{
		{Kind: tacticalplan.EventKill, Tick: 100},
		{Kind: tacticalplan.EventKill, Tick: 200},
		{Kind: tacticalplan.EventSmoke, Tick: 200},
	}
	markAirKill(events, 200)
	if events[0].AttackerInAir || !events[1].AttackerInAir {
		t.Fatalf("events = %+v, want only the kill at 200 from the air", events)
	}
	markAirKill(events, 300)
	if events[0].AttackerInAir {
		t.Fatal("a death with no kill on its tick marks nothing")
	}
}
