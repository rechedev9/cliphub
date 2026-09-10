package recapplan

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/customhud"
)

// A FACEIT demo rendered with a custom HUD kept the demo-facts-only overlay
// layout because overlays.source, not the demo origin, drove the layout. The
// origin owns the format; overlays.source only enriches a plain demo.
func TestOverlaySourceFollowsDemoOriginRegardlessOfHUD(t *testing.T) {
	cases := []struct {
		name, sourceKind, overlaySource, hud, want string
	}{
		{"faceit origin with custom HUD", "faceit", "demo", "circuit", "faceit"},
		{"faceit origin native HUD", "faceit", "demo", "", "faceit"},
		{"faceit origin and faceit data", "faceit", "faceit", "circuit", "faceit"},
		{"premier origin ignores faceit data", "premier", "faceit", "arena", "premier"},
		{"professional origin", "professional", "demo", "", "professional"},
		{"plain demo with faceit data (legacy documents)", "demo", "faceit", "", "faceit"},
		{"plain demo facts only", "demo", "demo", "circuit", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := DefaultOptions()
			o.SourceKind, o.Overlays.Source, o.Overlays.HUDTheme = tc.sourceKind, tc.overlaySource, tc.hud
			if tc.hud != "" {
				o.Capture.HUDProfile = customhud.CaptureProfile
			}
			if err := o.Validate(); err != nil {
				t.Fatal(err)
			}
			if got := o.OverlaySource(); got != tc.want {
				t.Fatalf("OverlaySource() = %q, want %q", got, tc.want)
			}
		})
	}
}
