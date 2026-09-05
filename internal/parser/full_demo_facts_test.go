package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

func TestFullDemoFactsIndependentRoundEvents(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		number                        int
		reset, truncated, freezeDeath bool
	}{
		{name: "zero kill round", number: 1},
		{name: "warmup and knife reset", number: 1, reset: true},
		{name: "halftime keeps source numbering", number: 13},
		{name: "overtime keeps source numbering", number: 31},
		{name: "EOF in live round blocks approval", number: 2, truncated: true},
		{name: "freeze death excluded", number: 1, freezeDeath: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewCollector(targetID, defaultTestRules())
			if tc.reset {
				c.RecordRoundStart(RoundStart{Round: 1, Tick: 10})
				c.RecordRoundLiveStart(RoundLiveStart{Round: 1, Tick: 20})
				c.RecordRoundEnd(RoundEnd{Round: 1, Tick: 50})
				c.RecordKill(RawKill{Tick: 30, Round: 1, Weapon: "knife"})
				c.resetForMatchStart()
			}
			c.RecordRoundStart(RoundStart{Round: tc.number, Tick: 100})
			c.RecordRoundLiveStart(RoundLiveStart{Round: tc.number, Tick: 200})
			if !tc.truncated {
				c.RecordRoundEnd(RoundEnd{Round: tc.number, Tick: 400})
			}
			var deaths []TargetDeath
			if tc.freezeDeath {
				deaths = []TargetDeath{{Round: tc.number, Tick: 150}}
			}
			facts := c.fullDemoFacts(PlanMeta{SHA256: strings.Repeat("a", 64), Tickrate: 64, DurationTicks: 450}, deaths, tc.reset)
			if err := facts.Validate(); err != nil {
				t.Fatal(err)
			}
			if facts.Complete == tc.truncated || len(facts.Rounds) != 1 || facts.Rounds[0].Number != tc.number || len(facts.Rounds[0].Kills) != 0 {
				t.Fatalf("facts: %+v", facts)
			}
			o := recapplan.DefaultOptions()
			o.Audio.Voice.Enabled, o.Audio.Music.Enabled, o.Sponsor.Enabled, o.Editorial.KeepFreezeVoice = false, false, false, false
			o.Capture.Crosshair.AllowCaptureDefault = true
			d, err := recapplan.Plan(facts, o, recapplan.VoiceEvidence{Availability: "not_requested"}, nil, "facts.json")
			if err != nil {
				t.Fatal(err)
			}
			if tc.truncated || tc.freezeDeath {
				if len(d.Blockers) == 0 || len(d.Rounds) != 0 {
					t.Fatalf("unsafe round admitted: %+v", d)
				}
			} else if len(d.Blockers) != 0 || len(d.Rounds) != 1 {
				t.Fatalf("valid zero-kill round lost: %+v", d)
			}
		})
	}
}

func TestFullDemoCrosshairTimelineTracksGapsAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name     string
		codes    []string
		want     int
		overflow bool
	}{
		{"stable", []string{"code", "code", "code"}, 1, false},
		{"disconnect and return", []string{"code", "", "", "code"}, 3, false},
		{"oversized code becomes unknown", []string{"code", strings.Repeat("a", 65)}, 2, false},
		{"bounded changes", nil, 4096, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var timeline crosshairTimeline
			if tc.codes == nil {
				for i := 0; i < 5000; i++ {
					timeline.record(i, fmt.Sprint(i))
				}
			} else {
				for i, code := range tc.codes {
					timeline.record(i, code)
				}
			}
			if len(timeline.samples) != tc.want || timeline.overflow != tc.overflow {
				t.Fatalf("samples=%d overflow=%v", len(timeline.samples), timeline.overflow)
			}
			if tc.overflow && timeline.samples[len(timeline.samples)-1].Code != "" {
				t.Fatal("overflow preserved an unverified crosshair")
			}
		})
	}
}
