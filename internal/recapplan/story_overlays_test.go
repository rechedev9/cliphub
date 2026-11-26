package recapplan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStoryOverlaysAreOptionalRenderOnlyChoices(t *testing.T) {
	plain := fixtureOptions()
	raw, err := json.Marshal(plain.Overlays)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "halftime") || strings.Contains(string(raw), "subscribe") {
		t.Fatalf("a plan without the story overlays changed its wire shape: %s", raw)
	}
	if DefaultOptions().Overlays.Halftime || DefaultOptions().Overlays.Subscribe {
		t.Fatal("the story overlays must be off by default")
	}
	before, err := Plan(fixtureFacts(), plain, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Options){
		"halftime":  func(o *Options) { o.Overlays.Halftime = true },
		"subscribe": func(o *Options) { o.Overlays.Subscribe = true },
	} {
		chosen := fixtureOptions()
		change(&chosen)
		current := DefaultOptions()
		change(&current)
		canonical, err := CanonicalNewOptions(current)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if canonical.Overlays.Halftime != current.Overlays.Halftime || canonical.Overlays.Subscribe != current.Overlays.Subscribe {
			t.Fatalf("%s was dropped by the canonical options", name)
		}
		if err := canonical.Validate(); err != nil {
			t.Fatalf("%s plan is invalid: %v", name, err)
		}
		if err := canonical.ValidateCurrentFullDemoPolicy(); err != nil {
			t.Fatalf("%s plan must be admitted: %v", name, err)
		}
		after, err := Plan(fixtureFacts(), chosen, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if after.PlanHash == before.PlanHash {
			t.Fatalf("%s must change the plan hash", name)
		}
		a, errA := before.CaptureHash()
		b, errB := after.CaptureHash()
		if errA != nil || errB != nil || a != b {
			t.Fatalf("%s must not change the capture", name)
		}
		encoded, err := json.Marshal(after)
		if err != nil {
			t.Fatal(err)
		}
		var restored Document
		if err := decodeStrict(encoded, &restored); err != nil || restored.Options.Overlays != after.Options.Overlays {
			t.Fatalf("%s round trip: %v", name, err)
		}
	}
}

func TestSavedPlansWithoutStoryOverlaysKeepTheirHash(t *testing.T) {
	d, err := Plan(fixtureFacts(), fixtureOptions(), VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	overlays := generic["options"].(map[string]any)["overlays"].(map[string]any)
	if _, ok := overlays["halftime"]; ok {
		t.Fatal("halftime reached a plan that did not choose it")
	}
	if _, ok := overlays["subscribe"]; ok {
		t.Fatal("subscribe reached a plan that did not choose it")
	}
	var restored Document
	if err := decodeStrict(raw, &restored); err != nil {
		t.Fatal(err)
	}
	hash, err := restored.Hash()
	if err != nil || hash != d.PlanHash {
		t.Fatalf("a saved plan changed its hash: %s != %s (%v)", hash, d.PlanHash, err)
	}
}
