package editor

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func TestPlayerNameListsTheFilmedPlayers(t *testing.T) {
	plan := recording.RecordingPlan{TargetSteamID64: "1", TargetNameInDemo: "ropz"}
	if got := playerName(plan); got != "ropz" {
		t.Fatalf("single POV name = %q, want ropz", got)
	}
	for _, name := range []string{"b1t", "", "b1t", "jL"} {
		s := recording.RecordingSegment{}
		if name != "" {
			s.Observe = &killplan.Target{SteamID64: "id-" + name, NameInDemo: name}
		}
		plan.Segments = append(plan.Segments, s)
	}
	if got := playerName(plan); got != "b1t, ropz y 1 más" {
		t.Fatalf("multi-POV name = %q, want the first two players and a count", got)
	}
}
