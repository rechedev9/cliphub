package recapplan

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/hudthemes"
	"github.com/rechedev9/cliphub/internal/killplan"
)

// The editing flow promises "solo vuelve a montar" from this hash: every render-only option keeps it, every capture option moves it.
func TestCaptureIdentityByOption(t *testing.T) {
	facts := fixtureFacts()
	facts.Rounds[2].Kills = []killplan.Kill{{Tick: 18900}}
	custom := func() Options {
		o := fixtureOptions()
		o.Capture.HUDProfile, o.Overlays.HUDTheme = customhud.CaptureProfile, customhud.Themes()[0].ID
		o.Editorial.AllowSafeTailTrim = true
		return o
	}
	captureHash := func(t *testing.T, o Options) string {
		t.Helper()
		d, err := Plan(facts, o, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
		if err != nil {
			t.Fatal(err)
		}
		h, err := d.CaptureHash()
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	base := captureHash(t, custom())
	radius := 4
	renderOnly := map[string]func(*Options){
		"diseño del HUD": func(o *Options) { o.Overlays.HUDTheme = customhud.Themes()[1].ID },
		"estilo propio": func(o *Options) {
			o.Overlays.HUDCustom = &hudthemes.Style{Name: "Mío", CT: "112233", T: "445566", Background: "000000", Radius: &radius}
		},
		"interruptores":     func(o *Options) { o.Overlays.HUDOptions = customhud.Options{CompactHeader: true, RoundWinBanner: true} },
		"nombres de equipo": func(o *Options) { o.Overlays.TeamNames = customhud.Teams{CTStart: "Vitality", TStart: "Spirit"} },
		"kit de marca": func(o *Options) {
			o.Overlays.Brand.Banner = &brandkit.Banner{Image: brandkit.ImageRef{ID: "22222222-2222-4222-8222-222222222222", SHA256: strings.Repeat("b", 64)}, StartSeconds: 5}
		},
		"transiciones":           func(o *Options) { transitions := DynamicTransitions(); o.Transitions = &transitions },
		"presentación":           func(o *Options) { o.Overlays.Roster = !o.Overlays.Roster },
		"sonido":                 func(o *Options) { o.Audio.Game.Gain, o.Audio.Voice.Gain = 0.5, 1.5 },
		"bumpers":                func(o *Options) { o.Bumpers = &BumperOptions{Intro: BumperSlot{Enabled: true}} },
		"origen de los overlays": func(o *Options) { o.SourceKind = "faceit" },
		"subtítulos":             func(o *Options) { o.Audio.Voice.Enabled, o.Subtitles = true, &SubtitleOptions{Enabled: true} },
	}
	for name, change := range renderOnly {
		o := custom()
		change(&o)
		if got := captureHash(t, o); got != base {
			t.Errorf("%s changed the capture identity", name)
		}
	}
	affectsCapture := map[string]func(*Options){
		"HUD original": func(o *Options) { o.Capture.HUDProfile, o.Overlays.HUDTheme = NativeHUDProfile, "" },
		"POV 1:1":      func(o *Options) { o.Capture.TrueView = true },
		"pro":          func(o *Options) { o.Capture.ProSettings = mustProSettings(t) },
		"rondas":       func(o *Options) { o.Rounds = []int{1, 2} },
		"marcador":     func(o *Options) { o.Overlays.Scoreboard = !o.Overlays.Scoreboard },
	}
	for name, change := range affectsCapture {
		o := custom()
		change(&o)
		if got := captureHash(t, o); got == base {
			t.Errorf("%s kept the capture identity", name)
		}
	}
}

func mustProSettings(t *testing.T) ProSettings {
	t.Helper()
	for _, id := range []string{"s1mple", "zywoo", "donk", "m0nesy", "niko"} {
		if settings, err := ResolveProSettings(id); err == nil {
			return settings
		}
	}
	t.Fatal("no catalog pro resolves")
	return ProSettings{}
}
