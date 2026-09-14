package recapplan

import (
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

func TestObservedCrosshairBlockerDoesNotOfferRetiredFallback(t *testing.T) {
	o := DefaultOptions()
	o.Audio.Voice.Enabled = false
	d, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil {
		t.Fatal(err)
	}
	for _, blocker := range d.Blockers {
		if blocker.Code == ErrPOVContract && strings.Contains(blocker.Message, "Observed player crosshair") {
			if strings.Contains(blocker.Message, "provide a code") || strings.Contains(blocker.Message, "capture default") {
				t.Fatalf("retired fallback leaked into blocker: %q", blocker.Message)
			}
			return
		}
	}
	t.Fatalf("missing observed-crosshair blocker: %+v", d.Blockers)
}
