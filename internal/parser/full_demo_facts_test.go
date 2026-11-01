package parser

import (
	"fmt"
	"slices"
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
			// Leave realistic respawn acquisition time in freeze at 64 Hz.
			c.RecordRoundLiveStart(RoundLiveStart{Round: tc.number, Tick: 400})
			if !tc.truncated {
				c.RecordRoundEnd(RoundEnd{Round: tc.number, Tick: 800})
			}
			var deaths []TargetDeath
			if tc.freezeDeath {
				deaths = []TargetDeath{{Round: tc.number, Tick: 150}}
			}
			facts := c.fullDemoFacts(PlanMeta{SHA256: strings.Repeat("a", 64), Tickrate: 64, DurationTicks: 850}, deaths, tc.reset)
			if err := facts.Validate(); err != nil {
				t.Fatal(err)
			}
			if facts.Complete == tc.truncated || len(facts.Rounds) != 1 || facts.Rounds[0].Number != tc.number || len(facts.Rounds[0].Kills) != 0 {
				t.Fatalf("facts: %+v", facts)
			}
			o := recapplan.DefaultOptions()
			o.Audio.Voice.Enabled, o.Audio.Music.Enabled, o.Editorial.KeepFreezeVoice = false, false, false
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

func TestFullDemoFactsSkipBoundedIncompleteRound(t *testing.T) {
	for _, tc := range []struct {
		name               string
		missingLive, first bool
	}{
		{name: "recording starts after round 1 went live", missingLive: true, first: true},
		{name: "round end lost mid match", first: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewCollector(targetID, defaultTestRules())
			for round, start := range map[int]int{1: 1, 2: 1000, 3: 2000} {
				c.RecordRoundStart(RoundStart{Round: round, Tick: start})
				broken := round == 2
				if tc.first {
					broken = round == 1
				}
				if !broken || !tc.missingLive {
					c.RecordRoundLiveStart(RoundLiveStart{Round: round, Tick: start + 300})
				}
				if !broken || tc.missingLive {
					c.RecordRoundEnd(RoundEnd{Round: round, Tick: start + 700})
				}
			}
			facts := c.fullDemoFacts(PlanMeta{SHA256: strings.Repeat("a", 64), Tickrate: 64, DurationTicks: 2750}, nil, false)
			if err := facts.Validate(); err != nil {
				t.Fatal(err)
			}
			if !facts.Complete {
				t.Fatalf("a bounded incomplete round marked the source incomplete: %+v", facts)
			}
			o := recapplan.DefaultOptions()
			o.Audio.Voice.Enabled, o.Audio.Music.Enabled, o.Editorial.KeepFreezeVoice = false, false, false
			o.Capture.Crosshair.AllowCaptureDefault = true
			d, err := recapplan.Plan(facts, o, recapplan.VoiceEvidence{Availability: "not_requested"}, nil, "facts.json")
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Blockers) != 0 || len(d.Rounds) != 2 {
				t.Fatalf("want the two complete rounds and no blocker: %+v", d)
			}
			skipped := "round-002"
			if tc.first {
				skipped = "round-001"
			}
			for _, r := range d.Rounds {
				if r.ID == skipped {
					t.Fatalf("incomplete round %s was planned", skipped)
				}
			}
			if !slices.ContainsFunc(d.Warnings, func(n recapplan.Notice) bool { return n.RoundID == skipped }) {
				t.Fatalf("no warning names the skipped round: %+v", d.Warnings)
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
