package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

func TestApplyFullDemoChoicesStoryOverlays(t *testing.T) {
	on, off := true, false
	plain, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{})
	if err != nil || plain.Overlays.Halftime || plain.Overlays.Subscribe {
		t.Fatalf("the story overlays must stay off unless asked: %+v %v", plain.Overlays, err)
	}
	chosen, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{storyOverlaysIn: storyOverlaysIn{HalftimeScoreboard: &on, SubscribeButton: &on}})
	if err != nil || !chosen.Overlays.Halftime || !chosen.Overlays.Subscribe {
		t.Fatalf("halftime_scoreboard and subscribe_button were dropped: %+v %v", chosen.Overlays, err)
	}
	if canonical, err := recapplan.CanonicalNewOptions(chosen); err != nil || !canonical.Overlays.Halftime || !canonical.Overlays.Subscribe {
		t.Fatalf("the planner dropped the story overlays: %+v %v", canonical.Overlays, err)
	}
	removed, err := applyFullDemoChoices(chosen, planFullDemoIn{storyOverlaysIn: storyOverlaysIn{HalftimeScoreboard: &off}})
	if err != nil || removed.Overlays.Halftime || !removed.Overlays.Subscribe {
		t.Fatalf("false must remove only its own overlay: %+v %v", removed.Overlays, err)
	}

	doc := recapplan.Document{Options: recapplan.DefaultOptions(), Clock: recapplan.Clock{FPS: 60}}
	if out := describeFullDemoPlan("job-1", doc); out.HalftimeScoreboard || out.SubscribeButton || strings.Contains(out.Summary, "descanso") || strings.Contains(out.Summary, "suscripción") {
		t.Fatalf("a default plan reports story overlays: %+v", out)
	}
	doc.Options.Overlays.Halftime = true
	if out := describeFullDemoPlan("job-1", doc); !out.HalftimeScoreboard || out.SubscribeButton || !strings.Contains(out.Summary, "Con marcador del descanso.") {
		t.Fatalf("halftime plan = %+v", out)
	}
	doc.Options.Overlays.Subscribe = true
	if out := describeFullDemoPlan("job-1", doc); !out.HalftimeScoreboard || !out.SubscribeButton || !strings.Contains(out.Summary, "marcador del descanso y botón de suscripción") {
		t.Fatalf("plan with both = %+v", out)
	}
	if !strings.Contains(serverInstructions, "halftime_scoreboard") || !strings.Contains(serverInstructions, "subscribe_button") {
		t.Fatal("the server instructions do not explain the story overlays")
	}
}
