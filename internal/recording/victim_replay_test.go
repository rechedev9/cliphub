package recording

import (
	"slices"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

const (
	replayTarget = "76561198148986856"
	victimA      = "76561198000000001"
	victimB      = "76561198000000002"
)

func victimReplayKillPlan() killplan.Plan {
	kp := killplan.NewPlan()
	kp.Demo.Map = "de_mirage"
	kp.Demo.Tickrate = 64
	kp.Demo.SHA256 = strings.Repeat("a", 64)
	kp.Demo.DurationTicks = 60000
	kp.Target.SteamID64 = replayTarget
	kill := func(tick int, victim string) killplan.Kill {
		return killplan.Kill{Tick: tick, Victim: killplan.Player{SteamID64: victim}}
	}
	kp.Segments = []killplan.Segment{
		// Later in the demo but first in editorial order.
		{ID: "seg-002", TickStart: 30000, TickEnd: 31000, Kills: []killplan.Kill{kill(30500, victimB)}},
		// The last kill is a bot, so the replay falls back to the last real player.
		{ID: "seg-001", TickStart: 10000, TickEnd: 11000, Kills: []killplan.Kill{kill(10300, victimA), kill(10600, "BOT"), kill(10100, victimB)}},
		{ID: "seg-003", TickStart: 40000, TickEnd: 41000, Kills: []killplan.Kill{kill(40500, "0")}},
	}
	return kp
}

func TestNewPlanFromKillPlanAddsVictimReplaysInASecondPass(t *testing.T) {
	stream := DefaultStreamConfig()
	stream.VictimReplay = true
	plan, err := NewPlanFromKillPlan(victimReplayKillPlan(), `C:\demos\x.dem`, `C:\out`, stream)
	if err != nil {
		t.Fatalf("NewPlanFromKillPlan error = %v", err)
	}
	want := map[string]*VictimReplay{
		"seg-001": {SteamID64: victimA, KillTick: 10300, TickStart: 10300 - 3*64, TickEnd: 10300 + 32},
		"seg-002": {SteamID64: victimB, KillTick: 30500, TickStart: 30500 - 3*64, TickEnd: 30500 + 32},
		"seg-003": nil,
	}
	for _, segment := range plan.Segments {
		got, wantReplay := segment.VictimReplay, want[segment.ID]
		if (got == nil) != (wantReplay == nil) || (got != nil && *got != *wantReplay) {
			t.Errorf("%s VictimReplay = %+v, want %+v", segment.ID, got, wantReplay)
		}
	}
	var ids, povs []string
	for _, capture := range plan.CaptureSegments() {
		ids = append(ids, capture.ID)
		povs = append(povs, capture.POVSteamID64)
	}
	if wantIDs := []string{"seg-001", "seg-002", "seg-003", "seg-001-victim", "seg-002-victim"}; !slices.Equal(ids, wantIDs) {
		t.Errorf("capture order = %v, want %v", ids, wantIDs)
	}
	if wantPOVs := []string{"", "", "", victimA, victimB}; !slices.Equal(povs, wantPOVs) {
		t.Errorf("capture POVs = %v, want %v", povs, wantPOVs)
	}
	replay := plan.CaptureSegments()[3]
	if EffectiveRecordStartTick(replay, plan.Tickrate) != replay.TickStart || EffectiveRecordEndTick(replay, plan) != replay.TickEnd {
		t.Errorf("replay window is not recorded exactly: %+v", replay)
	}
}

func TestNewPlanFromKillPlanOmitsVictimReplaysUnlessAsked(t *testing.T) {
	plan, err := NewPlanFromKillPlan(victimReplayKillPlan(), `C:\demos\x.dem`, `C:\out`, DefaultStreamConfig())
	if err != nil {
		t.Fatalf("NewPlanFromKillPlan error = %v", err)
	}
	if got := len(plan.CaptureSegments()); got != len(plan.Segments) {
		t.Fatalf("capture windows = %d, want only the %d plan segments", got, len(plan.Segments))
	}
}

func TestValidateRejectsInconsistentVictimReplays(t *testing.T) {
	base := func() RecordingPlan {
		plan := testPlan()
		plan.Stream.VictimReplay = true
		plan.Segments = []RecordingSegment{{
			ID: "seg-001", TickStart: 22086, TickEnd: 22406,
			Kills:        []killplan.Kill{{Tick: 22300, Victim: killplan.Player{SteamID64: victimA}}},
			VictimReplay: &VictimReplay{SteamID64: victimA, KillTick: 22300, TickStart: 22108, TickEnd: 22332},
		}}
		return plan
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("valid replay plan: %v", err)
	}
	for name, mutate := range map[string]func(*RecordingPlan){
		"stream flag off":     func(p *RecordingPlan) { p.Stream.VictimReplay = false },
		"victim is target":    func(p *RecordingPlan) { p.Segments[0].VictimReplay.SteamID64 = p.TargetSteamID64 },
		"window outside":      func(p *RecordingPlan) { p.Segments[0].VictimReplay.TickEnd = 22500 },
		"kill not in segment": func(p *RecordingPlan) { p.Segments[0].VictimReplay.KillTick = 22200 },
		"pov on plan segment": func(p *RecordingPlan) { p.Segments[0].POVSteamID64 = victimA },
		"id collision": func(p *RecordingPlan) {
			p.Segments = append(p.Segments, RecordingSegment{ID: "seg-001-victim", TickStart: 30000, TickEnd: 30100})
		},
	} {
		plan := base()
		mutate(&plan)
		if err := plan.Validate(); err == nil {
			t.Errorf("%s: Validate accepted the plan", name)
		}
	}
}

func TestFilterResultSegmentsKeepsTheSelectedVictimReplay(t *testing.T) {
	plan := testPlan()
	plan.Stream.VictimReplay = true
	plan.Segments[0].Kills = []killplan.Kill{{Tick: 22300, Victim: killplan.Player{SteamID64: victimA}}}
	plan.Segments[0].VictimReplay = &VictimReplay{SteamID64: victimA, KillTick: 22300, TickStart: 22108, TickEnd: 22332}
	clip := func(id string) RecordingArtifact {
		return RecordingArtifact{SegmentID: id, Role: "segment", Type: "video", Path: id + ".mp4"}
	}
	result := RecordingResult{Plan: plan, Artifacts: []RecordingArtifact{clip("seg-001"), clip("seg-001-victim"), clip("seg-002")}}
	filtered, err := FilterResultSegments(result, []string{"seg-001"})
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, artifact := range filtered.Artifacts {
		kept = append(kept, artifact.SegmentID)
	}
	if want := []string{"seg-001", "seg-001-victim"}; !slices.Equal(kept, want) {
		t.Fatalf("kept artifacts = %v, want %v", kept, want)
	}
	if replay, ok := ResolveVictimReplayClip(filtered, "seg-001"); !ok || replay.Path != "seg-001-victim.mp4" {
		t.Fatalf("ResolveVictimReplayClip = %+v, %v", replay, ok)
	}
}
