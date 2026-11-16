package renderplan

import "testing"

func TestEditRequestRadarRules(t *testing.T) {
	short := func(edit EditRequest) EditRequest {
		edit.Format, edit.KillEffect, edit.Transition = FormatShort9x16, KillEffectClean, TransitionCut
		return NormalizeEditRequest(edit)
	}
	radar := short(EditRequest{Radar: true, RadarPosition: " Top-Right ", RadarSize: "medium", RadarOpacity: 0.9})
	if radar.RadarPosition != "top-right" || radar.RadarSize != "" || radar.RadarOpacity != 0 {
		t.Fatalf("normalized = %+v, want a clean corner and the default size and opacity stored as unset", radar)
	}
	if err := radar.Validate(); err != nil {
		t.Fatalf("a radar Short was rejected: %v", err)
	}
	if off := short(EditRequest{RadarPosition: "top-left", RadarSize: "large", RadarOpacity: 0.5}); off.RadarPosition != "" || off.RadarSize != "" || off.RadarOpacity != 0 {
		t.Fatalf("radar options without the radar were kept: %+v", off)
	}
	if death := short(EditRequest{DeathReel: true, Radar: true}); !death.Radar || death.Validate() != nil {
		t.Fatal("a death reel should keep the radar: it shows where the player died")
	}
	for name, edit := range map[string]EditRequest{
		"native HUD":    func() EditRequest { e := radar; e.NativeHUD = true; return e }(),
		"match recap":   func() EditRequest { e := radar; e.MatchRecap = true; return e }(),
		"bad corner":    func() EditRequest { e := radar; e.RadarPosition = "center"; return e }(),
		"bad size":      func() EditRequest { e := radar; e.RadarSize = "huge"; return e }(),
		"faint":         func() EditRequest { e := radar; e.RadarOpacity = 0.1; return e }(),
		"options alone": {Format: FormatShort9x16, KillEffect: KillEffectClean, Transition: TransitionCut, RadarSize: "large"},
	} {
		if err := edit.Validate(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	for variant, ok := range map[string]bool{"viral-60-clean": true, "clean-pov-60": true, "full-hud-60": false, "gameplay-pov-60": false} {
		err := GenerateIntent{Variant: variant, Edit: radar}.Validate()
		if (err == nil) != ok {
			t.Errorf("%s: err=%v, want ok=%v", variant, err, ok)
		}
	}
}
