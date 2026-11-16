package renderplan

import "testing"

func TestEditRequestVerticalHUDRules(t *testing.T) {
	short := func(edit EditRequest) EditRequest {
		edit.Format, edit.KillEffect, edit.Transition = FormatShort9x16, KillEffectClean, TransitionCut
		return NormalizeEditRequest(edit)
	}
	hud := short(EditRequest{VerticalHUD: true, VerticalHUDTheme: " Prism "})
	if !hud.CenteredKillfeed || hud.VerticalHUDTheme != "prism" {
		t.Fatalf("normalized = %+v, want the centered kill feed and a clean design id", hud)
	}
	if err := hud.Validate(); err != nil {
		t.Fatalf("a vertical HUD Short was rejected: %v", err)
	}
	if off := short(EditRequest{VerticalHUDTheme: "prism"}); off.VerticalHUDTheme != "" || off.CenteredKillfeed {
		t.Fatalf("a design without the HUD was kept: %+v", off)
	}
	if death := short(EditRequest{DeathReel: true, VerticalHUD: true}); death.VerticalHUD || death.CenteredKillfeed {
		t.Fatal("a death reel kept the vertical HUD; its kill feed needs kill segments")
	}
	for name, edit := range map[string]EditRequest{
		"landscape":      func() EditRequest { e := hud; e.Format = FormatLandscape16x9; return e }(),
		"native HUD":     func() EditRequest { e := hud; e.NativeHUD = true; return e }(),
		"unknown design": func() EditRequest { e := hud; e.VerticalHUDTheme = "missing"; return e }(),
		"no kill feed":   func() EditRequest { e := hud; e.CenteredKillfeed = false; return e }(),
		"design alone":   {Format: FormatShort9x16, KillEffect: KillEffectClean, Transition: TransitionCut, VerticalHUDTheme: "prism"},
	} {
		if err := edit.Validate(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	for variant, ok := range map[string]bool{"viral-60-clean": true, "clean-pov-60": true, "full-hud-60": false} {
		err := GenerateIntent{Variant: variant, Edit: hud}.Validate()
		if (err == nil) != ok {
			t.Errorf("%s: err=%v, want ok=%v", variant, err, ok)
		}
	}
}
