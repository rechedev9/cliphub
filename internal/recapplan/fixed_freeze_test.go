package recapplan

import (
	"strings"
	"testing"
)

func TestFixedFreezeCannotBeExtendedByVoiceOrOldOptions(t *testing.T) {
	for _, rate := range []int{64, 100, 128} {
		for _, seconds := range []float64{0, 1, 2, 5, 20} {
			f := fixtureFacts()
			f.TickRate = rate
			o := fixtureOptions()
			o.Editorial.FreezeSeconds = seconds
			o.Editorial.MaxFreezeSeconds = 60
			o.Editorial.KeepFreezeVoice = true
			o.Editorial.VoiceContextSeconds = 3
			v := VoiceEvidence{Availability: "available", IndexHash: strings.Repeat("b", 64), IndexRef: "voice.json", ClockKind: ClockIngame, Activity: []TickRange{{Start: 0, End: f.EndTick}}}
			d, err := Plan(f, o, v, nil, "facts.json")
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Blockers) > 0 || !d.UsesFixedFreeze() {
				t.Fatalf("fixed policy not enforced at %d Hz: %+v", rate, d.Blockers)
			}
			for _, r := range d.Rounds {
				if r.LiveStartTick-r.RequestedStartTick != 2*rate || r.CaptureStartTick != r.RequestedStartTick {
					t.Fatalf("variable freeze: %+v", r)
				}
			}
			if o.Editorial.FreezeSeconds != seconds || !o.Editorial.KeepFreezeVoice {
				t.Fatal("planner mutated caller options")
			}
		}
	}
}

func TestManualRangesCannotChangeFixedFreeze(t *testing.T) {
	for _, delta := range []int{-100, 0, 100} {
		o := fixtureOptions()
		o.Editorial.ManualRanges = []ManualRange{{RoundID: "round-001", StartTick: 2300 + delta, EndTick: 8400}}
		d, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "no_packets"}, nil, "facts.json")
		if delta != 0 {
			if err == nil {
				t.Fatal("manual start bypassed fixed freeze")
			}
			continue
		}
		if err != nil || !d.UsesFixedFreeze() {
			t.Fatalf("fixed-start manual end rejected: %v", err)
		}
	}
}

func TestDefaultFreezeIsFixedAndIndependentOfVoice(t *testing.T) {
	o := DefaultOptions().Editorial
	if o.FreezeSeconds != 2 || o.MaxFreezeSeconds != 2 || o.KeepFreezeVoice || o.VoiceContextSeconds != 0 {
		t.Fatalf("variable defaults: %+v", o)
	}
}
