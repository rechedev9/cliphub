package recording

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

func TestFullDemoRecordingAcceptsApprovedTailBeyondLegacyDuration(t *testing.T) {
	for _, tc := range []struct {
		name           string
		legacyDuration int
		sourceEnd      int
		deathTick      int
		wantEnd        int
		wantDuration   int
	}{
		{"round tail", 2000, 2400, 0, 2128, 2128},
		{"death tail", 2000, 2400, 1980, 2172, 2172},
		{"tail bounded by source EOF", 2000, 2040, 0, 2040, 2040},
		{"existing duration already covers tail", 2400, 2400, 0, 2128, 2400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := recapplan.Facts{
				SchemaVersion: recapplan.DocumentVersion, DemoSHA256: strings.Repeat("a", 64),
				TargetSteamID64: "76561198000000001", ClockKind: recapplan.ClockIngame,
				TickRate: 64, EndTick: tc.sourceEnd, Complete: true,
				Rounds: []recapplan.RoundFacts{{ID: "round-001", Number: 1, StartTick: 1000,
					FreezeEndTick: 1320, RoundEndTick: 2000, Evidence: "round-events"}},
			}
			if tc.deathTick > 0 {
				facts.Rounds[0].DeathTick = &tc.deathTick
			}
			options := recapplan.DefaultOptions()
			options.Audio.Voice.Enabled, options.Audio.Music.Enabled, options.Sponsor.Enabled = false, false, false
			options.Capture.Crosshair.AllowCaptureDefault = true
			doc, err := recapplan.Plan(facts, options, recapplan.VoiceEvidence{Availability: "not_requested"}, nil, "facts.json")
			if err != nil {
				t.Fatal(err)
			}
			// Both new plans and plans persisted by earlier releases use this adapter.
			encoded, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &doc); err != nil {
				t.Fatal(err)
			}
			base := killplan.NewPlan()
			base.Demo = killplan.Demo{SHA256: facts.DemoSHA256, Tickrate: 64, DurationTicks: tc.legacyDuration}
			base.Target.SteamID64 = facts.TargetSteamID64
			adapted := doc.KillPlan(base)
			if adapted.Demo.DurationTicks != tc.wantDuration || base.Demo.DurationTicks != tc.legacyDuration {
				t.Fatalf("adapted duration=%d base=%d; want %d / %d", adapted.Demo.DurationTicks, base.Demo.DurationTicks, tc.wantDuration, tc.legacyDuration)
			}
			after, err := json.Marshal(doc)
			if err != nil || !reflect.DeepEqual(encoded, after) {
				t.Fatal("adapting the recording changed the approved document")
			}
			for _, encoder := range []string{"", EncoderNVENC} {
				stream := DefaultStreamConfig()
				stream.Encoder = encoder
				// The worker first normalizes its stream without the Full Demo document.
				profile, err := NewPlanFromKillPlan(adapted, "source.dem", "profile", stream)
				if err != nil {
					t.Fatalf("worker profile: %v", err)
				}
				plan, err := NewPlanFromKillPlan(adapted, "source.dem", t.TempDir(), profile.Stream, &doc)
				if err != nil {
					t.Fatalf("Full Demo recording: %v", err)
				}
				if len(plan.Segments) != 1 || EffectiveRecordEndTick(plan.Segments[0], plan) != tc.wantEnd {
					t.Fatalf("approved tail changed: %+v", plan.Segments)
				}
				if _, err := GenerateHLAEJavaScriptWithAttestation(plan, "duration-test-only"); err != nil {
					t.Fatalf("HLAE script: %v", err)
				}
			}
		})
	}
}

func TestLegacyRecordingStillRejectsSegmentBeyondDuration(t *testing.T) {
	base := killplan.NewPlan()
	base.Demo = killplan.Demo{SHA256: strings.Repeat("a", 64), Tickrate: 64, DurationTicks: 2000}
	base.Target.SteamID64 = "76561198000000001"
	base.Segments = []killplan.Segment{{ID: "seg-001", Round: 1, TickStart: 1200, TickEnd: 2128}}
	if _, err := NewPlanFromKillPlan(base, "source.dem", t.TempDir(), DefaultStreamConfig()); err == nil || !strings.Contains(err.Error(), "exceeds demo duration") {
		t.Fatalf("expected legacy duration rejection, got %v", err)
	}
}
