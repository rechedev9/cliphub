package recording

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

func TestFullDemoWithProSettingsRecordsAtTheProAspectRatio(t *testing.T) {
	facts := recapplan.Facts{
		SchemaVersion: recapplan.DocumentVersion, DemoSHA256: strings.Repeat("a", 64),
		TargetSteamID64: "76561198000000001", ClockKind: recapplan.ClockIngame,
		TickRate: 64, EndTick: 2400, Complete: true,
		Rounds: []recapplan.RoundFacts{{ID: "round-001", Number: 1, StartTick: 1000,
			FreezeEndTick: 1320, RoundEndTick: 2000, Evidence: "round-events"}},
	}
	options := recapplan.DefaultOptions()
	options.Audio.Voice.Enabled = false
	options.Capture.Crosshair.AllowCaptureDefault = true
	pro, err := recapplan.ResolveProSettings("donk")
	if err != nil {
		t.Fatal(err)
	}
	options.Capture.ProSettings = pro
	doc, err := recapplan.Plan(facts, options, recapplan.VoiceEvidence{Availability: "not_requested"}, nil, "facts.json")
	if err != nil {
		t.Fatal(err)
	}
	base := killplan.NewPlan()
	base.Demo = killplan.Demo{SHA256: facts.DemoSHA256, Tickrate: 64, DurationTicks: 2400}
	base.Target.SteamID64 = facts.TargetSteamID64
	adapted := doc.KillPlan(base)
	profile, err := NewPlanFromKillPlan(adapted, "source.dem", "profile", DefaultStreamConfig())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewPlanFromKillPlan(adapted, "source.dem", t.TempDir(), profile.Stream, &doc)
	if err != nil {
		t.Fatalf("Full Demo recording: %v", err)
	}
	if plan.Stream.Width != 1440 || plan.Stream.Height != 1080 {
		t.Fatalf("stream %dx%d, want 1440x1080", plan.Stream.Width, plan.Stream.Height)
	}
	plan.Stream.Width = 1920
	if err := plan.validateFullDemo(); err == nil {
		t.Fatal("a 16:9 stream was accepted for a 4:3 pro")
	}
	plan.Stream.Width = 1440
	required, err := fullDemoRequiredCaptureCvars(plan)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]float64{"viewmodel_fov": 68, "viewmodel_offset_x": 2.5, "viewmodel_offset_y": 0, "viewmodel_offset_z": -1.5} {
		if got, ok := required[name]; !ok || got != want {
			t.Errorf("required %s = %v, %v; want %v", name, got, ok, want)
		}
	}
	script, err := GenerateHLAEJavaScriptWithAttestation(plan, "pro-settings-test-only")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, `const fullDemoProCvars = {"viewmodel_fov":68,"viewmodel_offset_x":2.5,"viewmodel_offset_y":0,"viewmodel_offset_z":-1.5};`) {
		t.Fatal("HLAE script does not apply the pro viewmodel")
	}
}
