package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

func TestApplyFullDemoChoicesHook(t *testing.T) {
	on, off := true, false
	plain, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{})
	if err != nil || plain.Hook != nil {
		t.Fatalf("the hook must stay off unless asked: %+v %v", plain.Hook, err)
	}
	chosen, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{storyOverlaysIn: storyOverlaysIn{Hook: &on}})
	if err != nil || chosen.Hook == nil || !chosen.Hook.Enabled {
		t.Fatalf("hook was dropped: %+v %v", chosen.Hook, err)
	}
	if canonical, err := recapplan.CanonicalNewOptions(chosen); err != nil || canonical.Hook == nil || !canonical.Hook.Enabled {
		t.Fatalf("the planner dropped the hook: %+v %v", canonical.Hook, err)
	}
	removed, err := applyFullDemoChoices(chosen, planFullDemoIn{storyOverlaysIn: storyOverlaysIn{Hook: &off}})
	if err != nil || removed.Hook != nil {
		t.Fatalf("false must remove the hook from the options: %+v %v", removed.Hook, err)
	}

	doc := recapplan.Document{Options: recapplan.DefaultOptions(), Clock: recapplan.Clock{FPS: 60}, Rounds: []recapplan.Round{{ID: "round-001", Number: 1}, {ID: "round-007", Number: 7}}}
	if out := describeFullDemoPlan("job-1", doc); out.Hook || out.HookRound != 0 || strings.Contains(out.Summary, "gancho") {
		t.Fatalf("a default plan reports a hook: %+v", out)
	}
	doc.Options.Hook = &recapplan.HookOptions{Enabled: true}
	if out := describeFullDemoPlan("job-1", doc); !out.Hook || out.HookRound != 0 || strings.Contains(out.Summary, "gancho") || out.Rounds != 2 {
		t.Fatalf("a skipped hook must be reported as asked for and absent: %+v", out)
	}
	doc.Clock.TickRate = 64
	for _, tick := range []int{100, 500, 900, 1300, 1700} {
		doc.Rounds[1].Kills = append(doc.Rounds[1].Kills, killplan.Kill{Tick: tick})
	}
	doc.Hook = &recapplan.Hook{RoundID: "round-007", FirstKillTick: 100, LastKillTick: 1700, Kills: 5}
	doc.Timeline = []recapplan.TimelineItem{{Role: "round", Reason: recapplan.HookReason, SourceRef: "round-007", SourceOffsetFrames: 600, StartFrame: 0, EndFrame: 1200}}
	if out := describeFullDemoPlan("job-1", doc); !out.Hook || out.HookRound != 7 || !strings.Contains(out.Summary, "gancho de la mejor jugada: 3 bajas en la ronda 7.") || out.Rounds != 2 {
		t.Fatalf("plan with a hook = %+v", out)
	}
	doc.Rounds[1].Kills = nil
	for _, tick := range []int{315, 700, 1100, 1500} {
		doc.Rounds[1].Kills = append(doc.Rounds[1].Kills, killplan.Kill{Tick: tick})
	}
	doc.Hook = &recapplan.Hook{RoundID: "round-007", FirstKillTick: 315, LastKillTick: 1500, Kills: 4}
	doc.Timeline[0].SourceOffsetFrames = 296
	if out := describeFullDemoPlan("job-1", doc); !strings.Contains(out.Summary, "3 bajas en la ronda 7.") {
		t.Fatalf("a kill one frame before the teaser starts is not shown: %s", out.Summary)
	}
	if !strings.Contains(serverInstructions, "plan_full_demo hook") || !strings.Contains(serverInstructions, "hook_round") {
		t.Fatal("the server instructions do not explain the hook")
	}
}
