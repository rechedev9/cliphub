package recapplan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/hudthemes"
)

func TestFocusPortraitIsVerifiedPersistedAndOnlyChangesRender(t *testing.T) {
	o := fixtureOptions()
	o.Capture.HUDProfile, o.Overlays.HUDTheme = customhud.CaptureProfile, "focus"
	plain, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil {
		t.Fatal(err)
	}
	ref := AssetRef{ID: "22222222-2222-4222-8222-222222222222", SHA256: strings.Repeat("a", 64)}
	o.Overlays.HUDPortrait = &ref
	newOptions := DefaultOptions()
	newOptions.Overlays.HUDTheme, newOptions.Overlays.HUDPortrait = "focus", &ref
	canonical, err := CanonicalNewOptions(newOptions)
	if err != nil || !reflect.DeepEqual(canonical.Overlays.HUDPortrait, &ref) {
		t.Fatalf("canonical portrait lost: %v", err)
	}
	missing, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil || len(missing.Blockers) == 0 {
		t.Fatal("unverified portrait accepted")
	}
	with, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, []AssetEvidence{{Ref: ref, HasImage: true, Title: "portrait.png"}}, "facts")
	if err != nil || len(with.Blockers) != 0 {
		t.Fatalf("portrait plan: %v %+v", err, with.Blockers)
	}
	a, _ := plain.CaptureHash()
	b, _ := with.CaptureHash()
	if a != b || plain.PlanHash == with.PlanHash {
		t.Fatal("portrait must change approval but reuse capture")
	}
	if !o.IsOverlayImage(ref) || !reflect.DeepEqual(o.AssetReferences(), []AssetRef{ref}) {
		t.Fatal("portrait not materialized as an image")
	}
	body, _ := json.Marshal(with)
	var restored Document
	if err := decodeStrict(body, &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(); err != nil {
		t.Fatal(err)
	}
	o.Overlays.HUDTheme = "arena"
	if o.Validate() == nil {
		t.Fatal("portrait accepted by incompatible HUD")
	}
}

func TestCustomHUDRequiresCompatibleCaptureAndThemesReuseIt(t *testing.T) {
	native := fixtureDocument(t)
	oldJSON, _ := json.Marshal(native)
	if strings.Contains(string(oldJSON), "hud_theme") {
		t.Fatal("changed native wire contract")
	}
	nativeHash, _ := native.CaptureHash()
	var first Document
	for i, theme := range customhud.Themes() {
		o := fixtureOptions()
		o.Capture.HUDProfile = customhud.CaptureProfile
		o.Overlays.HUDTheme = theme.ID
		d, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "no_packets"}, nil, "facts")
		if err != nil {
			t.Fatal(err)
		}
		if len(d.Blockers) > 0 {
			t.Fatalf("custom theme unexpectedly blocked: %+v", d.Blockers)
		}
		if i == 0 {
			first = d
		}
		ch, _ := d.CaptureHash()
		fch, _ := first.CaptureHash()
		if ch == nativeHash || ch != fch {
			t.Fatal("theme changes must reuse a clean capture; native must not")
		}
		if i > 0 && d.PlanHash == first.PlanHash {
			t.Fatal("theme change did not invalidate approval")
		}
		body, _ := json.Marshal(d)
		var roundtrip Document
		if err := decodeStrict(body, &roundtrip); err != nil {
			t.Fatal(err)
		}
		if roundtrip.Options.Overlays.HUDTheme != theme.ID {
			t.Fatal("theme lost on roundtrip")
		}
	}
	for _, tc := range []struct{ profile, theme string }{{"native", "arena"}, {customhud.CaptureProfile, ""}, {customhud.CaptureProfile, "missing"}} {
		o := fixtureOptions()
		o.Capture.HUDProfile = tc.profile
		o.Overlays.HUDTheme = tc.theme
		if o.Validate() == nil {
			t.Fatalf("accepted incompatible options %+v", tc)
		}
	}
	legacy := first
	legacy.Options.Capture.HUDProfile = customhud.LegacyCaptureProfile
	if err := legacy.Options.Validate(); err != nil {
		t.Fatal(err)
	}
	legacyHash, _ := legacy.CaptureHash()
	currentHash, _ := first.CaptureHash()
	if legacyHash == currentHash {
		t.Fatal("radar profile revision reused a capture with different native settings")
	}
}

func TestHUDLookIsKeptValidatedAndOnlyChangesRender(t *testing.T) {
	o := fixtureOptions()
	o.Capture.HUDProfile, o.Overlays.HUDTheme = customhud.CaptureProfile, "arena"
	plain, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil {
		t.Fatal(err)
	}
	radius := 6
	o.Overlays.HUDCustom = &hudthemes.Style{Name: "Mío", CT: "112233", T: "445566", Background: "000000", Radius: &radius}
	o.Overlays.HUDOptions = customhud.Options{CompactHeader: true, RoundWinBanner: true}
	o.Overlays.TeamNames = customhud.Teams{CTStart: "Vitality", TStart: "Spirit"}
	styled, err := Plan(fixtureFacts(), o, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil || len(styled.Blockers) != 0 {
		t.Fatalf("styled plan: %v %+v", err, styled.Blockers)
	}
	a, _ := plain.CaptureHash()
	b, _ := styled.CaptureHash()
	if a != b || plain.PlanHash == styled.PlanHash {
		t.Fatal("the HUD look must change approval but reuse the capture")
	}
	fresh := DefaultOptions()
	fresh.Overlays.HUDCustom, fresh.Overlays.HUDOptions, fresh.Overlays.TeamNames = o.Overlays.HUDCustom, o.Overlays.HUDOptions, o.Overlays.TeamNames
	canonical, err := CanonicalNewOptions(fresh)
	if err != nil || !reflect.DeepEqual(canonical.Overlays.HUDCustom, o.Overlays.HUDCustom) || canonical.Overlays.HUDOptions != o.Overlays.HUDOptions || canonical.Overlays.TeamNames != o.Overlays.TeamNames {
		t.Fatalf("canonical options dropped the HUD look: %+v, %v", canonical.Overlays, err)
	}
	renderer, err := o.Overlays.HUDRenderer()
	if err != nil || renderer.Theme.CT != "112233" || renderer.Theme.Radius != 6 || !renderer.Options.CompactHeader || renderer.Teams.TStart != "Spirit" {
		t.Fatalf("renderer = %+v, %v", renderer, err)
	}
	for name, change := range map[string]func(*Options){
		"bad colour":     func(o *Options) { o.Overlays.HUDCustom.CT = "zz" },
		"long name":      func(o *Options) { o.Overlays.TeamNames.CTStart = strings.Repeat("a", MaxTeamNameRunes+1) },
		"native with it": func(o *Options) { o.Overlays.HUDTheme, o.Capture.HUDProfile = "", NativeHUDProfile },
	} {
		bad := o
		custom := *o.Overlays.HUDCustom
		bad.Overlays.HUDCustom = &custom
		change(&bad)
		if bad.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
	body, _ := json.Marshal(plain.Options.Overlays)
	if strings.Contains(string(body), "hud_options") || strings.Contains(string(body), "team_names") || strings.Contains(string(body), "hud_custom") {
		t.Fatalf("an unstyled plan changed its wire format: %s", body)
	}
}
