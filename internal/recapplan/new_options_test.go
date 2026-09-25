package recapplan

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/customhud"
)

func TestDefaultOptionsUseSimplifiedFullDemoDefaults(t *testing.T) {
	o := DefaultOptions()
	if o.Audio.Music.Enabled || len(o.Audio.Music.Assets) != 0 || o.Sponsor.Enabled {
		t.Fatalf("new plan defaults activate optional media: %+v", o)
	}
	if o.Capture.HUDProfile != customhud.CaptureProfile || o.Overlays.HUDTheme == "" {
		t.Fatalf("custom HUD default is missing: %+v", o)
	}
	if !o.Overlays.Roster || !o.Overlays.Scoreboard || o.Overlays.Theme != "neon-violet" || o.Overlays.Mode != "generated" {
		t.Fatalf("generated neon overlay defaults are incomplete: %+v", o.Overlays)
	}
	if o.Transitions == nil || !o.Transitions.Enabled {
		t.Fatalf("Dinamico transitions are not enabled by default: %+v", o.Transitions)
	}
	if o.Audio.Game.Gain != 1 || o.Audio.Voice.Gain != 1.1 || !o.Audio.Voice.Enabled {
		t.Fatalf("new plans must start at game 100%% and team voices 110%%: %+v", o.Audio)
	}
	if err := o.Validate(); err != nil {
		t.Fatalf("default options fail validation: %v", err)
	}
}

// The Sonido card of a fresh plan renders these defaults; the web fixture must
// be the exact Go serialization so the two sides cannot drift.
func TestDefaultOptionsMatchWebFixture(t *testing.T) {
	b, err := os.ReadFile("../../web/lib/full-demo-go-defaults.fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("web/lib/full-demo-go-defaults.fixture.json drifted from DefaultOptions():\n got %s\nwant %s", encoded, bytes.TrimSpace(b))
	}
}

func TestCanonicalNewOptionsRetiresMusicAndCrosshairOverrides(t *testing.T) {
	for _, mutate := range []struct {
		name string
		fn   func(*Options)
	}{
		{"music", func(o *Options) { o.Audio.Music.Enabled = true }},
		{"music asset", func(o *Options) {
			o.Audio.Music.Assets = []AssetRef{{ID: "00000000-0000-4000-8000-000000000001", SHA256: strings.Repeat("a", 64)}}
		}},
		{"provided crosshair", func(o *Options) {
			o.Capture.Crosshair.Mode, o.Capture.Crosshair.Code = "provided-code", "CSGO-WsnnD-eHaMw-QNDf9-oxuDh-ydOUD"
		}},
		{"capture default", func(o *Options) { o.Capture.Crosshair.AllowCaptureDefault = true }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			o := DefaultOptions()
			mutate.fn(&o)
			if _, err := CanonicalNewOptions(o); err == nil {
				t.Fatal("retired new-flow setting was accepted")
			}
		})
	}
}

func TestCanonicalNewOptionsKeepsOnlySupportedChoices(t *testing.T) {
	o := DefaultOptions()
	o.Audio.Voice.Enabled = false
	o.Audio.Voice.Gain = 1.2
	o.Audio.Game.Gain = 0.6
	o.Audio.Game.VoicePriority = true
	o.Outputs.CoverPolicy = "custom-cover"
	o.Overlays.Roster, o.Overlays.Scoreboard, o.Overlays.Theme, o.Overlays.Mode = false, false, "faceit-orange", "screenshots"
	o.Overlays.Source = "faceit"
	o.Transitions.WhipStrength = .2
	o.Transitions.Enabled = false
	got, err := CanonicalNewOptions(o)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Overlays.Roster || !got.Overlays.Scoreboard || got.Overlays.Theme != "neon-violet" || got.Overlays.Mode != "generated" || got.Overlays.Source != "demo" {
		t.Fatalf("overlay settings were not canonicalized: %+v", got.Overlays)
	}
	want := DynamicTransitions()
	want.Enabled = false
	if got.Transitions == nil || *got.Transitions != want {
		t.Fatalf("transition toggle did not retain the Dinamico preset: %+v", got.Transitions)
	}
	if got.Audio.Voice.Enabled || got.Outputs != DefaultOptions().Outputs {
		t.Fatalf("voice toggle or fixed outputs were not canonicalized: %+v", got)
	}
	if got.Audio.Voice.Gain != 1.2 || got.Audio.Game.Gain != 0.6 || got.Audio.Game.VoicePriority {
		t.Fatalf("user mix levels must survive while voice priority stays automatic: %+v", got.Audio)
	}
}

func TestCurrentPolicyRejectsHistoricalRetiredGenerationChoices(t *testing.T) {
	for _, mutate := range []struct {
		name string
		fn   func(*Options)
	}{
		{"manual crosshair", func(o *Options) {
			o.Capture.Crosshair.Mode, o.Capture.Crosshair.Code = "provided-code", "CSGO-WsnnD-eHaMw-QNDf9-oxuDh-ydOUD"
		}},
		{"no roster", func(o *Options) { o.Overlays.Roster = false }},
		{"orange overlays", func(o *Options) { o.Overlays.Theme = "faceit-orange" }},
		{"legacy transitions", func(o *Options) { o.Transitions = nil }},
		{"silent voice fallback", func(o *Options) { o.Audio.Voice.ApprovedFallback = "without-voice" }},
		{"manual round interval", func(o *Options) {
			o.Editorial.ManualRanges = []ManualRange{{RoundID: "round-001", StartTick: 10, EndTick: 20}}
		}},
		{"short death tail", func(o *Options) { o.Editorial.DeathTailSeconds = 1 }},
		{"voice priority", func(o *Options) { o.Audio.Game.VoicePriority = true }},
		{"old cover", func(o *Options) { o.Outputs.CoverPolicy = "generated-gameplay" }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			o := DefaultOptions()
			mutate.fn(&o)
			if err := o.Validate(); err != nil {
				t.Fatalf("historical document must remain readable: %v", err)
			}
			if err := o.ValidateCurrentFullDemoPolicy(); err == nil {
				t.Fatal("retired generation configuration was admitted")
			}
		})
	}
}

func TestCurrentPolicyPreservesSupportedChoicesAndEmptyHistoricalSlices(t *testing.T) {
	o := DefaultOptions()
	o.SourceKind = "premier"
	o.Overlays.HUDTheme = "mono"
	o.Audio.Voice.Enabled = false
	o.Transitions.Enabled = false
	o.Sponsor.PlacementPolicy = "round-boundary"
	o.Sponsor.AfterRoundID = "round-002"
	o.Editorial.ManualRanges = nil
	o.Audio.Music.Assets = nil
	before, err := HashValue(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := o.ValidateCurrentFullDemoPolicy(); err != nil {
		t.Fatal(err)
	}
	after, err := HashValue(o)
	if err != nil || before != after {
		t.Fatalf("policy check mutated approved options: %s vs %s, %v", before, after, err)
	}
}

func TestObservedCrosshairBlockerDoesNotOfferRetiredFallback(t *testing.T) {
	o := DefaultOptions()
	o.Audio.Voice.Enabled = false
	d, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil {
		t.Fatal(err)
	}
	for _, blocker := range d.Blockers {
		if blocker.Code == ErrPOVContract && strings.Contains(blocker.Message, "mira del jugador") {
			if strings.Contains(blocker.Message, "provide a code") || strings.Contains(blocker.Message, "capture default") {
				t.Fatalf("retired fallback leaked into blocker: %q", blocker.Message)
			}
			return
		}
	}
	t.Fatalf("missing observed-crosshair blocker: %+v", d.Blockers)
}

func TestCustomHUDIsOptionalAndTrueViewIsACaptureChoice(t *testing.T) {
	native := DefaultOptions()
	native.Overlays.HUDTheme = ""
	native.Capture.HUDProfile = NativeHUDProfile
	got, err := CanonicalNewOptions(native)
	if err != nil {
		t.Fatal(err)
	}
	if got.Overlays.HUDTheme != "" || got.Capture.HUDProfile != NativeHUDProfile {
		t.Fatalf("native HUD was replaced by a broadcast HUD: %+v %+v", got.Capture, got.Overlays)
	}
	if err := got.ValidateCurrentFullDemoPolicy(); err != nil {
		t.Fatalf("native HUD plan must be admitted: %v", err)
	}

	spectator := native
	spectator.Capture.HUDProfile = "native"
	if err := spectator.ValidateCurrentFullDemoPolicy(); err == nil {
		t.Fatal("spectator panels of the plain native profile were admitted")
	}
	if got, err := CanonicalNewOptions(spectator); err != nil || got.Capture.HUDProfile != NativeHUDProfile {
		t.Fatalf("plain native must become the clean native profile: %+v %v", got.Capture, err)
	}

	orphan := DefaultOptions()
	orphan.Overlays.HUDTheme = ""
	if got, err := CanonicalNewOptions(orphan); err != nil || got.Overlays.HUDTheme != DefaultOptions().Overlays.HUDTheme {
		t.Fatalf("broadcast capture without a theme must get the default theme: %+v %v", got.Overlays, err)
	}

	for _, base := range []Options{DefaultOptions(), got} {
		off, err := json.Marshal(base.Capture)
		if err != nil || strings.Contains(string(off), "trueview") {
			t.Fatalf("TrueView off must keep the historical wire: %s %v", off, err)
		}
		trueView := base
		trueView.Capture.TrueView = true
		canonical, err := CanonicalNewOptions(trueView)
		if err != nil || !canonical.Capture.TrueView {
			t.Fatalf("TrueView choice was dropped: %+v %v", canonical.Capture, err)
		}
		if err := trueView.ValidateCurrentFullDemoPolicy(); err != nil {
			t.Fatalf("TrueView plan must be admitted: %v", err)
		}
		plain, trueViewDoc := Document{Options: base}, Document{Options: trueView}
		a, errA := plain.CaptureHash()
		b, errB := trueViewDoc.CaptureHash()
		if errA != nil || errB != nil || a == b {
			t.Fatal("TrueView must change the capture hash")
		}
	}
}
