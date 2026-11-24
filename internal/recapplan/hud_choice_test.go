package recapplan

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/customhud"
)

func TestChooseHUDLeavesValidOptionsForEveryDesign(t *testing.T) {
	portrait := AssetRef{ID: "22222222-2222-4222-8222-222222222222", SHA256: strings.Repeat("a", 64)}
	choices := []string{""}
	for _, theme := range customhud.Themes() {
		choices = append(choices, theme.ID)
	}
	for _, from := range choices {
		for _, to := range choices {
			o := DefaultOptions()
			if err := o.ChooseHUD(from); err != nil {
				t.Fatalf("choose %q: %v", from, err)
			}
			o.Overlays.HUDPortrait = &portrait
			if err := o.ChooseHUD(to); err != nil {
				t.Fatalf("choose %q after %q: %v", to, from, err)
			}
			if err := o.Validate(); err != nil {
				t.Fatalf("%q after %q does not validate: %v", to, from, err)
			}
			canonical, err := CanonicalNewOptions(o)
			if err != nil {
				t.Fatalf("%q after %q: %v", to, from, err)
			}
			if err := canonical.Validate(); err != nil {
				t.Fatalf("%q after %q does not validate: %v", to, from, err)
			}
			if canonical.Overlays.HUDTheme != to {
				t.Fatalf("%q after %q planned theme %q", to, from, canonical.Overlays.HUDTheme)
			}
			if (canonical.Overlays.HUDPortrait != nil) != (to == "focus") {
				t.Fatalf("%q after %q kept portrait %v", to, from, canonical.Overlays.HUDPortrait)
			}
		}
	}
}

func TestChooseHUDRefusesAnUnknownDesign(t *testing.T) {
	o := DefaultOptions()
	before := o
	if err := o.ChooseHUD("nope"); err == nil {
		t.Fatal("unknown HUD accepted")
	}
	if o.Capture.HUDProfile != before.Capture.HUDProfile || o.Overlays.HUDTheme != before.Overlays.HUDTheme {
		t.Fatal("refused choice changed the options")
	}
}
