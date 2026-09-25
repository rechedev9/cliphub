package recapplan

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/killplan"
)

func fixtureFacts() Facts {
	death := 14000
	return Facts{SchemaVersion: DocumentVersion, DemoSHA256: strings.Repeat("a", 64), TargetSteamID64: "76561198000000001", ClockKind: ClockIngame, TickRate: 100, EndTick: 21000, Complete: true, Warnings: []Notice{}, Rounds: []RoundFacts{
		{ID: "round-001", Number: 1, StartTick: 1000, FreezeEndTick: 2500, RoundEndTick: 8500, NextStartTick: 9000, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}},
		{ID: "round-002", Number: 2, StartTick: 9000, FreezeEndTick: 10500, RoundEndTick: 15000, NextStartTick: 15500, DeathTick: &death, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}},
		{ID: "round-025", Number: 25, StartTick: 15500, FreezeEndTick: 17000, RoundEndTick: 19000, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}},
	}}
}

func fixtureOptions() Options {
	o := DefaultOptions()
	// Fixtures model already-approved historical documents. New drafts use the
	// current defaults; old documents must remain readable without acquiring
	// newly introduced HUD or transition fields.
	o.Capture.HUDProfile = "native-clean-spectator"
	o.Overlays = OverlayOptions{Theme: "faceit-orange", Source: "demo"}
	o.Transitions = nil
	o.Capture.Crosshair.AllowCaptureDefault = true
	o.Audio.Voice.Enabled = false
	o.Audio.Music.Enabled = false
	return o
}

func fixtureDocument(t *testing.T) Document {
	t.Helper()
	d, err := Plan(fixtureFacts(), fixtureOptions(), VoiceEvidence{Availability: "no_packets"}, nil, "jobs/facts.json")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDocumentStrictRoundTrip(t *testing.T) {
	d := fixtureDocument(t)
	d.Options.Audio.Game.Gain = 0
	d.Options.Audio.Voice.Gain = 0
	d.PlanHash, _ = d.Hash()
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var got Document
	if err := decodeStrict(b, &got); err != nil {
		t.Fatal(err)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
	if got.Options.Audio.Game.Gain != 0 || got.Options.Audio.Voice.Enabled || got.Options.HasBumpers() || got.Input.TargetSteamID64 != "76561198000000001" {
		t.Fatalf("decisions or identity changed: %+v", got)
	}
	store := &memStore{}
	id := uuid.New()
	if err := SaveDocument(store, id, got); err != nil {
		t.Fatal(err)
	}
	loaded, found, err := LoadCurrentDocument(store, id)
	if err != nil || !found || loaded.PlanHash != d.PlanHash {
		t.Fatalf("durable roundtrip: %v %v %+v", found, err, loaded)
	}
	got.Options.Audio.Game.Gain = 0.5
	got.PlanHash, _ = got.Hash()
	if err := SaveDocument(store, id, got); err == nil {
		t.Fatal("overwrote immutable plan")
	}
}

func TestOptionsRejectAmbiguousJSON(t *testing.T) {
	b, _ := json.Marshal(fixtureOptions())
	valid := string(b)
	withBumpers := fixtureOptions()
	withBumpers.Bumpers = &BumperOptions{}
	b, _ = json.Marshal(withBumpers)
	bumpers := string(b)
	for _, tc := range []struct{ name, input string }{
		{"missing false", strings.Replace(valid, `"xray":false,`, "", 1)},
		{"null false", strings.Replace(valid, `"xray":false`, `"xray":null`, 1)},
		{"unknown", strings.Replace(valid, `"xray":false`, `"xray":false,"surprise":1`, 1)},
		{"duplicate", strings.Replace(valid, `"xray":false`, `"xray":true,"xray":false`, 1)},
		{"future profile", strings.Replace(valid, ProfileChill, "full-demo-pov-chill-v2", 1)},
		{"missing nullable asset", strings.Replace(bumpers, `,"video":null`, "", 1)},
		{"partial nested object", strings.Replace(valid, `"gain":1,"voice_priority":false`, `"gain":1`, 1)},
		{"trailing", valid + `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var o Options
			if err := json.Unmarshal([]byte(tc.input), &o); err == nil {
				t.Fatal("accepted ambiguous or unsupported decisions")
			}
		})
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), -1, 3} {
		t.Run("gain", func(t *testing.T) {
			o := fixtureOptions()
			o.Audio.Game.Gain = value
			if err := o.Validate(); err == nil {
				t.Fatalf("accepted gain %v", value)
			}
		})
	}
}

func TestPlanRoundEvidence(t *testing.T) {
	for _, tc := range []struct {
		name              string
		edit              func(*Facts, *Options, *VoiceEvidence)
		start, end, count int
		reason            string
	}{
		{"survival zero kills", func(*Facts, *Options, *VoiceEvidence) {}, 2300, 8700, 3, "fixed-freeze-2s"},
		{"next round caps tail", func(f *Facts, _ *Options, _ *VoiceEvidence) { f.Rounds[0].NextStartTick = 8600 }, 2300, 8600, 3, "fixed-freeze-2s"},
		{"file caps tail", func(f *Facts, _ *Options, _ *VoiceEvidence) {
			f.Rounds = f.Rounds[:1]
			f.Rounds[0].NextStartTick = 0
			f.EndTick = 8550
		}, 2300, 8550, 1, "fixed-freeze-2s"},
		{"death first live tick", func(f *Facts, _ *Options, _ *VoiceEvidence) { tick := 2500; f.Rounds[0].DeathTick = &tick }, 2300, 2800, 3, "fixed-freeze-2s"},
		{"death at round end", func(f *Facts, _ *Options, _ *VoiceEvidence) { tick := 8500; f.Rounds[0].DeathTick = &tick }, 2300, 8800, 3, "fixed-freeze-2s"},
		{"freeze death excluded", func(f *Facts, _ *Options, _ *VoiceEvidence) { tick := 2400; f.Rounds[0].DeathTick = &tick }, 10300, 14300, 2, "fixed-freeze-2s"},
		{"missing freeze excluded", func(f *Facts, _ *Options, _ *VoiceEvidence) { f.Rounds[0].FreezeEndTick = 0 }, 10300, 14300, 2, "fixed-freeze-2s"},
		{"zero margins preserved", func(_ *Facts, o *Options, _ *VoiceEvidence) {
			o.Editorial.FreezeSeconds = 0
			o.Editorial.RoundTailSeconds = 0
		}, 2300, 8501, 3, "fixed-freeze-2s"},
		{"voice cannot extend freeze", func(_ *Facts, _ *Options, v *VoiceEvidence) {
			v.Availability = "available"
			v.IndexHash = strings.Repeat("b", 64)
			v.IndexRef = "voice/index.json"
			v.ClockKind = ClockIngame
			v.Activity = []TickRange{{1200, 1350}}
		}, 2300, 8700, 3, "fixed-freeze-2s"},
		{"manual range", func(_ *Facts, o *Options, _ *VoiceEvidence) {
			o.Editorial.ManualRanges = []ManualRange{{"round-001", 2300, 8400}}
		}, 2300, 8400, 3, "manual-approved-range"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, o, v := fixtureFacts(), fixtureOptions(), VoiceEvidence{Availability: "no_packets"}
			tc.edit(&f, &o, &v)
			d, err := Plan(f, o, v, nil, "facts")
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Rounds) != tc.count || d.Rounds[0].RequestedStartTick != tc.start || d.Rounds[0].RequestedEndTick != tc.end || d.Rounds[0].StartReason != tc.reason {
				t.Fatalf("rounds=%+v", d.Rounds)
			}
			if d.Rounds[len(d.Rounds)-1].Number != f.Rounds[len(f.Rounds)-1].Number {
				t.Fatal("source round number was renumbered")
			}
		})
	}
}

func TestContentHashesAndCaptureReuse(t *testing.T) {
	base := fixtureDocument(t)
	captureHash, _ := base.CaptureHash()
	for _, tc := range []struct {
		name                        string
		edit                        func(*Document)
		planChanges, captureChanges bool
	}{
		{"volatile metadata", func(d *Document) { d.PlanID = uuid.NewString(); d.Revision++; d.Input.FactsRef = "another/path" }, false, false},
		{"same round IDs new ticks", func(d *Document) { d.Rounds[0].RequestedStartTick++; d.Rounds[0].CaptureStartTick++ }, true, true},
		{"mix only", func(d *Document) { d.Options.Audio.Game.Gain = 0 }, true, false},
		{"sponsor only", func(d *Document) { d.Options.Bumpers = &BumperOptions{Sponsor: &BumperSlot{Enabled: true}} }, true, false},
		{"crosshair", func(d *Document) { d.Options.Capture.Crosshair.AllowCaptureDefault = false }, true, true},
		{"hud", func(d *Document) { d.Options.Capture.HUDProfile = "native" }, true, true},
		{"demo same name new bytes", func(d *Document) { d.Input.DemoSHA256 = strings.Repeat("c", 64) }, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := fixtureDocument(t)
			tc.edit(&d)
			ph, _ := d.Hash()
			ch, _ := d.CaptureHash()
			if (ph != base.PlanHash) != tc.planChanges || (ch != captureHash) != tc.captureChanges {
				t.Fatalf("plan changed=%v capture changed=%v", ph != base.PlanHash, ch != captureHash)
			}
		})
	}
	narrow := fixtureDocument(t)
	narrow.Rounds[0].RequestedStartTick += 100
	narrow.Rounds[0].RequestedEndTick -= 100
	if !CaptureCovers(base, narrow) {
		t.Fatal("narrower render required recapture")
	}
	narrow.Rounds[0].RequestedStartTick = base.Rounds[0].CaptureStartTick - 1
	if CaptureCovers(base, narrow) {
		t.Fatal("expanded coverage reused uncertified frames")
	}
}

func TestCertifiedEndsKeepTheApprovedDocument(t *testing.T) {
	d := fixtureDocument(t)
	snapshot := Snapshot{Document: d, Approval: Approval{d.PlanHash, true, time.Now().UTC()}}
	for _, tc := range []struct {
		name        string
		end         int
		allow, fail bool
	}{
		{"complete", 14300, true, false}, {"safe tail", 14001, true, false}, {"interior loss", 13999, true, true}, {"tail rule unapproved", 14001, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := snapshot
			s.Document.Options.Editorial.AllowSafeTailTrim = tc.allow
			s.Document.PlanHash, _ = s.Document.Hash()
			s.Approval.PlanHash = s.Document.PlanHash
			s.Approval.AllowSafeTailTrim = tc.allow
			got, err := ApplyCertifiedEnds(s, map[string]int{"round-001": 8700, "round-002": tc.end, "round-025": 19200})
			if (err != nil) != tc.fail {
				t.Fatalf("effective=%+v error=%v", got, err)
			}
			if snapshot.Document.Rounds[1].EffectiveEndTick != 14300 {
				t.Fatal("approved document was mutated")
			}
		})
	}
}

func TestUnavailableAssetsAndVoiceRemainEnabled(t *testing.T) {
	for _, availability := range []string{"no_packets", "no_team_packets", "silent", "unsupported_codec", "invalid_timeline", "failed"} {
		t.Run(availability, func(t *testing.T) {
			o := fixtureOptions()
			o.Audio.Voice.Enabled = true
			o.Audio.Music.Enabled = true
			o.Bumpers = &BumperOptions{Sponsor: &BumperSlot{Enabled: true}}
			d, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: availability}, nil, "facts")
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Blockers) < 3 || !d.Options.Audio.Music.Enabled || !d.Options.HasBumpers() || !d.Options.Audio.Voice.Enabled {
				t.Fatalf("silently degraded: %+v", d)
			}
			err = (Snapshot{Document: d, Approval: Approval{d.PlanHash, true, time.Now().UTC()}}).Validate()
			var coded *Error
			if !errors.As(err, &coded) {
				t.Fatalf("generation should fail with typed error: %v", err)
			}
		})
	}
}

func TestRationalFrameClock(t *testing.T) {
	for _, tc := range []struct {
		ticks, rate int
		frames      int64
	}{{64, 64, 60}, {100, 100, 60}, {1, 100, 1}, {100 * 60 * 40, 100, 60 * 60 * 40}, {128 * 60 * 40, 128, 60 * 60 * 40}} {
		got, err := TickFrames(tc.ticks, tc.rate)
		if err != nil || got != tc.frames {
			t.Fatalf("%+v: frames=%d err=%v", tc, got, err)
		}
		if got*SamplesPerFrame != tc.frames*800 {
			t.Fatal("sample clock drift")
		}
	}
}

func FuzzOptionsDecode(f *testing.F) {
	b, _ := json.Marshal(DefaultOptions())
	f.Add(b)
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, b []byte) {
		var options Options
		if err := json.Unmarshal(b, &options); err == nil {
			if err := options.Validate(); err != nil {
				t.Fatal(err)
			}
		}
	})
}
