package recapplan

import "testing"

func TestIntroDesignIsNeonOrHUDAndOnlyChangesRender(t *testing.T) {
	native := DefaultOptions()
	native.Overlays.HUDTheme, native.Capture.HUDProfile = "", NativeHUDProfile
	for _, base := range []Options{DefaultOptions(), native} {
		neon, err := CanonicalNewOptions(base)
		if err != nil || neon.Overlays.Theme != "neon-violet" {
			t.Fatalf("an untouched plan must keep the neon design: %q %v", neon.Overlays.Theme, err)
		}
		chosen := base
		chosen.Overlays.Theme = "hud"
		hud, err := CanonicalNewOptions(chosen)
		if err != nil || hud.Overlays.Theme != "hud" {
			t.Fatalf("the HUD design was dropped: %q %v", hud.Overlays.Theme, err)
		}
		if err := hud.Validate(); err != nil {
			t.Fatalf("HUD design plan is invalid: %v", err)
		}
		if err := hud.ValidateCurrentFullDemoPolicy(); err != nil {
			t.Fatalf("HUD design plan must be admitted: %v", err)
		}
		a, errA := Document{Options: neon}.CaptureHash()
		b, errB := Document{Options: hud}.CaptureHash()
		if errA != nil || errB != nil || a != b {
			t.Fatal("the intro design must not change the capture")
		}
	}
	for _, retired := range []string{"faceit-orange", "hud-changed"} {
		o := DefaultOptions()
		o.Overlays.Theme = retired
		if err := o.ValidateCurrentFullDemoPolicy(); err == nil {
			t.Fatalf("%q was admitted", retired)
		}
		if got, err := CanonicalNewOptions(o); err != nil || got.Overlays.Theme != "neon-violet" {
			t.Fatalf("%q must fall back to neon: %q %v", retired, got.Overlays.Theme, err)
		}
	}
}
