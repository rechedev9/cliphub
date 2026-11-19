package recapplan

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Studio 5.25.0 approved plans with subtitles.english: they still load with their hash and approval, and a new plan drops it.
func TestAPlanWithTheRetiredEnglishSubtitlesStillLoads(t *testing.T) {
	o := fixtureOptions()
	o.Audio.Voice.Enabled = true
	o.Subtitles = &SubtitleOptions{Enabled: true, LegacyEnglish: true}
	f := fixtureFacts()
	voice := VoiceEvidence{Availability: "available", IndexHash: strings.Repeat("b", 64), IndexRef: "voice.json", ClockKind: ClockIngame, Activity: []TickRange{{Start: 0, End: f.EndTick}}}
	d, err := Plan(f, o, voice, nil, "jobs/facts.json")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := json.Marshal(Snapshot{Document: d, Approval: Approval{PlanHash: d.PlanHash, AllowSafeTailTrim: d.Options.Editorial.AllowSafeTailTrim, Timestamp: time.Now().UTC()}})
	if err != nil || !strings.Contains(string(stored), `"subtitles":{"enabled":true,"english":true}`) {
		t.Fatalf("the fixture is not a 5.25.0 approval: %s, %v", stored, err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(stored, &snapshot); err != nil {
		t.Fatalf("a 5.25.0 approval does not decode: %v", err)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("a 5.25.0 approval no longer validates: %v", err)
	}
	store, id := &memStore{}, uuid.New()
	if err := SaveDocument(store, id, d); err != nil {
		t.Fatal(err)
	}
	if loaded, found, err := LoadCurrentDocument(store, id); err != nil || !found || loaded.PlanHash != d.PlanHash {
		t.Fatalf("a stored 5.25.0 plan = %v, %v; want it read with its hash", found, err)
	}
	fresh := DefaultOptions()
	fresh.Subtitles = o.Subtitles
	canonical, err := CanonicalNewOptions(fresh)
	if err != nil || canonical.Subtitles == nil || !canonical.Subtitles.Enabled || canonical.Subtitles.LegacyEnglish {
		t.Fatalf("a new plan from those options = %+v, %v; want spoken-language subtitles", canonical.Subtitles, err)
	}
}
