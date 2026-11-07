package recording

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

const teammateA = "76561198000000077"

// multiPOVKillPlan films the target in seg-001 and a teammate in seg-002; the
// teammate's kill victim is the target's own victim, to check replay picking.
func multiPOVKillPlan(teammateStart int) killplan.Plan {
	kp := victimReplayKillPlan()
	mate := killplan.Target{SteamID64: teammateA, NameInDemo: "mate"}
	anchor := kp.Target
	kp.Segments = []killplan.Segment{
		{ID: "seg-001", TickStart: 10000, TickEnd: 11000, POV: &anchor, Kills: []killplan.Kill{{Tick: 10300, Victim: killplan.Player{SteamID64: victimA}}}},
		{ID: "seg-002", TickStart: teammateStart, TickEnd: teammateStart + 1000, POV: &mate, Kills: []killplan.Kill{
			{Tick: teammateStart + 300, Victim: killplan.Player{SteamID64: victimB}},
			{Tick: teammateStart + 600, Victim: killplan.Player{SteamID64: teammateA}},
		}},
	}
	return kp
}

func TestNewPlanFromKillPlanCarriesEachSegmentPOV(t *testing.T) {
	stream := DefaultStreamConfig()
	stream.VictimReplay = true
	plan, err := NewPlanFromKillPlan(multiPOVKillPlan(30000), `C:\demos\x.dem`, `C:\out`, stream)
	if err != nil {
		t.Fatalf("NewPlanFromKillPlan error = %v", err)
	}
	if plan.Segments[0].Observe != nil {
		t.Errorf("anchor segment Observe = %+v, want nil so single-POV captures stay reusable", plan.Segments[0].Observe)
	}
	if got := plan.ObservedSteamID(plan.Segments[1]); got != teammateA {
		t.Errorf("seg-002 films %q, want the teammate", got)
	}
	// The teammate's own death is never their replay: the replay shows victimB.
	if replay := plan.Segments[1].VictimReplay; replay == nil || replay.SteamID64 != victimB {
		t.Errorf("seg-002 victim replay = %+v, want victimB", replay)
	}
	if back := plan.ToKillPlan(); !back.IsMultiPOV() || back.SegmentPOV(back.Segments[1]).SteamID64 != teammateA {
		t.Errorf("ToKillPlan lost the segment POV: %+v", back.Segments)
	}
}

func TestValidateNeedsTimeToSwitchPOV(t *testing.T) {
	// seg-001 ends at 11000; the teammate's window must start half a second later.
	if _, err := NewPlanFromKillPlan(multiPOVKillPlan(11000+32), `C:\demos\x.dem`, `C:\out`, DefaultStreamConfig()); err != nil {
		t.Fatalf("half a second between POVs: %v", err)
	}
	// Settle padding moves the record start two seconds in, so start the window earlier.
	_, err := NewPlanFromKillPlan(multiPOVKillPlan(11000-2*64+31), `C:\demos\x.dem`, `C:\out`, DefaultStreamConfig())
	if err == nil || !strings.Contains(err.Error(), "too soon after seg-001 to switch POV") {
		t.Fatalf("error = %v, want a POV switch error", err)
	}
}

func TestScriptFiltersTheKillfeedPerPOV(t *testing.T) {
	stream := DefaultStreamConfig()
	stream.PortraitSafeKillfeed = true
	plan, err := NewPlanFromKillPlan(multiPOVKillPlan(30000), `C:\demos\x.dem`, `C:\out`, stream)
	if err != nil {
		t.Fatal(err)
	}
	script, err := GenerateHLAEJavaScript(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"targetSteamId": "` + teammateA + `"`, `killfeed-pov-seg-001`, `attackerMatch=!x` + teammateA} {
		if !strings.Contains(script, want) {
			t.Errorf("script lacks %s", want)
		}
	}
	single, err := NewPlanFromKillPlan(victimReplayKillPlan(), `C:\demos\x.dem`, `C:\out`, stream)
	if err != nil {
		t.Fatal(err)
	}
	if script, _ := GenerateHLAEJavaScript(single); strings.Contains(script, "killfeed-pov-") {
		t.Error("a single-POV script schedules per-window killfeed filters")
	}
}
