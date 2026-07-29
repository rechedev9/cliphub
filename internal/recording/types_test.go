package recording

import (
	"strings"
	"testing"

	"github.com/rechedev9/fragforge/internal/killplan"
)

func TestAccountIDFromSteamID64(t *testing.T) {
	got, err := AccountIDFromSteamID64("76561198148986856")
	if err != nil {
		t.Fatalf("AccountIDFromSteamID64 error = %v", err)
	}
	if got != 188721128 {
		t.Errorf("account id = %d, want 188721128", got)
	}
}

func TestNewPlanFromKillPlan(t *testing.T) {
	kp := killplan.NewPlan()
	kp.Demo.Map = "de_ancient"
	kp.Demo.Tickrate = 64
	kp.Demo.SHA256 = strings.Repeat("a", 64)
	kp.Demo.DurationTicks = 30000
	kp.Target.SteamID64 = "76561198148986856"
	kp.Target.NameInDemo = "MartinezSa"
	kp.Segments = []killplan.Segment{
		{ID: "seg-001", TickStart: 22086, TickEnd: 22406},
	}

	plan, err := NewPlanFromKillPlan(kp, `C:\demos\x.dem`, `C:\out`, StreamConfig{})
	if err != nil {
		t.Fatalf("NewPlanFromKillPlan error = %v", err)
	}
	if plan.TargetAccountID != 188721128 {
		t.Errorf("TargetAccountID = %d, want 188721128", plan.TargetAccountID)
	}
	if plan.CaptureContract != CaptureContractVersion {
		t.Errorf("CaptureContract = %q, want %q", plan.CaptureContract, CaptureContractVersion)
	}
	if plan.DemoMap != "de_ancient" {
		t.Errorf("DemoMap = %q, want de_ancient", plan.DemoMap)
	}
	if plan.TargetNameInDemo != "MartinezSa" {
		t.Errorf("TargetNameInDemo = %q, want MartinezSa", plan.TargetNameInDemo)
	}
	if plan.Stream.Mode != StreamModeFFmpegDirect {
		t.Errorf("Stream.Mode = %q, want %q", plan.Stream.Mode, StreamModeFFmpegDirect)
	}
	if plan.Stream.HUDMode != HUDModeGameplay {
		t.Errorf("Stream.HUDMode = %q, want %q", plan.Stream.HUDMode, HUDModeGameplay)
	}
}

func TestNewPlanFromKillPlanSeparatesEditorialAndCaptureOrder(t *testing.T) {
	kp := killplan.NewPlan()
	kp.Demo.Map = "de_mirage"
	kp.Demo.Tickrate = 64
	kp.Demo.SHA256 = strings.Repeat("a", 64)
	kp.Demo.DurationTicks = 5000
	kp.Target.SteamID64 = "76561198148986856"
	// Best-moment selection may intentionally persist this editorial order.
	kp.Segments = []killplan.Segment{
		{ID: "best-late", TickStart: 3000, TickEnd: 3300},
		{ID: "second-early", TickStart: 1000, TickEnd: 1300},
	}

	plan, err := NewPlanFromKillPlan(kp, `C:\demos\x.dem`, `C:\out`, StreamConfig{})
	if err != nil {
		t.Fatalf("NewPlanFromKillPlan error = %v", err)
	}
	if got, want := []string{plan.Segments[0].ID, plan.Segments[1].ID}, []string{"second-early", "best-late"}; got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("capture order = %v, want %v", got, want)
	}
	if got, want := plan.EditorialSegmentIDs, []string{"best-late", "second-early"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("editorial order = %v, want %v", got, want)
	}
	resumed := plan.ToKillPlan()
	if got := []string{resumed.Segments[0].ID, resumed.Segments[1].ID}; got[0] != "best-late" || got[1] != "second-early" {
		t.Fatalf("resumed editorial order = %v", got)
	}
	if got := []string{kp.Segments[0].ID, kp.Segments[1].ID}; got[0] != "best-late" || got[1] != "second-early" {
		t.Fatalf("source editorial order was mutated: %v", got)
	}
}

func TestValidateRejectsBadSegment(t *testing.T) {
	p := RecordingPlan{
		DemoPath:        "x.dem",
		OutputDir:       "out",
		TargetAccountID: 1,
		Tickrate:        64,
		Stream:          DefaultStreamConfig(),
		Segments: []RecordingSegment{
			{ID: "seg-001", TickStart: 100, TickEnd: 100},
		},
	}
	if err := p.Validate(); err == nil {
		t.Fatal("Validate error = nil, want error")
	}
}

func TestValidateRejectsContradictorySteamAndAccountIDs(t *testing.T) {
	p := testPlan()
	p.TargetAccountID++
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "target_account_id") {
		t.Fatalf("Validate error = %v, want contradictory target_account_id error", err)
	}
}

func TestValidateRejectsUnknownHUDMode(t *testing.T) {
	p := testPlan()
	p.Stream.HUDMode = "weird"
	if err := p.Validate(); err == nil {
		t.Fatal("Validate error = nil, want error")
	}
}

func TestValidateRejectsInvalidCRF(t *testing.T) {
	p := testPlan()
	p.Stream.CRF = 52
	if err := p.Validate(); err == nil {
		t.Fatal("Validate error = nil, want error")
	}
}

func TestNewPlanPortraitSafeKillfeedDefaults(t *testing.T) {
	tests := []struct {
		hudMode      HUDMode
		wantSafeZone bool
	}{
		{hudMode: HUDModeDeathnotices, wantSafeZone: true},
		{hudMode: HUDModeGameplay},
	}
	for _, tc := range tests {
		t.Run(string(tc.hudMode), func(t *testing.T) {
			kp := killplan.NewPlan()
			kp.Demo.Tickrate = 64
			kp.Demo.SHA256 = strings.Repeat("a", 64)
			kp.Demo.DurationTicks = 1000
			kp.Target.SteamID64 = "76561198148986856"
			kp.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 100, TickEnd: 200}}
			stream := DefaultStreamConfig()
			stream.HUDMode = tc.hudMode
			stream.PortraitSafeKillfeed = true

			plan, err := NewPlanFromKillPlan(kp, "x.dem", "out", stream)
			if err != nil {
				t.Fatalf("NewPlanFromKillPlan error = %v", err)
			}
			wantX, wantY := 0.0, 0.0
			if tc.wantSafeZone {
				wantX = defaultDeathnoticeSafeZoneX
				wantY = defaultDeathnoticeSafeZoneY
			}
			if got := plan.Stream.DeathnoticeSafeZoneX; got != wantX {
				t.Fatalf("DeathnoticeSafeZoneX = %.2f, want %.2f", got, wantX)
			}
			if got := plan.Stream.DeathnoticeSafeZoneY; got != wantY {
				t.Fatalf("DeathnoticeSafeZoneY = %.2f, want %.2f", got, wantY)
			}
			if got, want := plan.Stream.DeathnoticeLifetime, defaultDeathnoticeLifetimeSeconds; got != want {
				t.Fatalf("DeathnoticeLifetime = %.2f, want %.2f", got, want)
			}
		})
	}
}

func TestValidateRejectsPortraitSafeKillfeedWithCleanHUD(t *testing.T) {
	p := testPlan()
	p.Stream.HUDMode = HUDModeClean
	p.Stream.PortraitSafeKillfeed = true

	err := p.Validate()
	if err == nil || !strings.Contains(err.Error(), "portrait_safe_killfeed") {
		t.Fatalf("Validate error = %v, want portrait_safe_killfeed HUD error", err)
	}
}

func TestValidateRejectsInvalidDeathnoticeLayout(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RecordingPlan)
	}{
		{name: "safe zone", mutate: func(p *RecordingPlan) { p.Stream.DeathnoticeSafeZoneX = 1.1 }},
		{name: "safe zone y", mutate: func(p *RecordingPlan) { p.Stream.DeathnoticeSafeZoneY = 1.1 }},
		{name: "lifetime", mutate: func(p *RecordingPlan) { p.Stream.DeathnoticeLifetime = 10.1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := testPlan()
			tt.mutate(&p)
			if err := p.Validate(); err == nil {
				t.Fatal("Validate error = nil, want error")
			}
		})
	}
}

func TestRecordingPlanValidateRejectsUnsafeSegmentIDs(t *testing.T) {
	for _, id := range []string{"..", "../outside", `..\..\outside`, "seg/001", `seg\001`, "-segment"} {
		t.Run(id, func(t *testing.T) {
			plan := testPlan()
			plan.Segments[0].ID = id
			if err := plan.Validate(); err == nil {
				t.Fatalf("Validate(%q) error = nil, want an unsafe artifact token error", id)
			}
		})
	}
}

func TestRecordingPlanValidateRejectsCaptureWindowOverlapAndOutOfOrder(t *testing.T) {
	for _, segments := range [][]RecordingSegment{
		{
			{ID: "seg-001", TickStart: 1000, TickEnd: 1200},
			{ID: "seg-002", TickStart: 1150, TickEnd: 1400},
		},
		{
			{ID: "seg-001", TickStart: 2000, TickEnd: 2200},
			{ID: "seg-002", TickStart: 1000, TickEnd: 1200},
		},
	} {
		plan := testPlan()
		plan.Segments = segments
		if err := plan.Validate(); err == nil || !strings.Contains(err.Error(), "overlaps or is out of order") {
			t.Fatalf("Validate error = %v, want capture-window ordering error", err)
		}
	}
}

func TestRecordingPlanValidateRejectsEventsOutsideSegmentAndDemo(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RecordingPlan)
	}{
		{
			name: "kill before segment",
			mutate: func(plan *RecordingPlan) {
				plan.Segments[0].Kills = []killplan.Kill{{Tick: plan.Segments[0].TickStart - 1}}
			},
		},
		{
			name: "utility after segment",
			mutate: func(plan *RecordingPlan) {
				plan.Segments[0].Utility = []killplan.UtilityThrow{{ThrowTick: plan.Segments[0].TickEnd + 1}}
			},
		},
		{
			name: "segment after demo",
			mutate: func(plan *RecordingPlan) {
				plan.DemoDurationTicks = plan.Segments[len(plan.Segments)-1].TickEnd - 1
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := testPlan()
			tt.mutate(&plan)
			if err := plan.Validate(); err == nil {
				t.Fatal("Validate error = nil, want bounded timeline error")
			}
		})
	}
}

func TestRecordingPlanToKillPlanPreservesScoringAndRhythmFacts(t *testing.T) {
	recordingPlan := RecordingPlan{
		DemoPath:          `C:\demos\match.dem`,
		DemoSHA256:        strings.Repeat("a", 64),
		DemoMap:           "de_mirage",
		DemoDurationTicks: 4096,
		TargetSteamID64:   "76561198000000001",
		TargetNameInDemo:  "Alpha",
		Tickrate:          128,
		Segments: []RecordingSegment{{
			ID:        "seg-002",
			Round:     8,
			TickStart: 128,
			TickEnd:   640,
			Kills:     []killplan.Kill{{Tick: 256, Headshot: true}},
			Utility:   []killplan.UtilityThrow{{Type: "smokegrenade"}},
		}},
	}

	got := recordingPlan.ToKillPlan()
	if got.Demo.Path != recordingPlan.DemoPath || got.Demo.SHA256 != recordingPlan.DemoSHA256 || got.Demo.Tickrate != 128 {
		t.Fatalf("demo identity = %#v", got.Demo)
	}
	if got.Target.SteamID64 != recordingPlan.TargetSteamID64 || got.Target.NameInDemo != "Alpha" {
		t.Fatalf("target = %#v", got.Target)
	}
	if len(got.Segments) != 1 || got.Segments[0].ID != "seg-002" || len(got.Segments[0].Kills) != 1 {
		t.Fatalf("segments = %#v", got.Segments)
	}
	if got.Stats.KillsAfterFilters != 1 || got.Stats.SmokesAfterFilters != 1 || got.Stats.DurationSecondsTotal != 4 {
		t.Fatalf("stats = %#v", got.Stats)
	}
}
