package recapplan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/customhud"
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
		if ch == nativeHash || ch != fch || !CaptureCovers(first, d) {
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
	if legacyHash == currentHash || CaptureCovers(legacy, first) || CaptureCovers(first, legacy) {
		t.Fatal("radar profile revision reused a capture with different native settings")
	}
}
