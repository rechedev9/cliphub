package demooverlay

import (
	"fmt"
	"strings"
	"testing"
)

func TestHookLabelFollowsTheDesign(t *testing.T) {
	palette := &Palette{CT: "112233", T: "AABBCC", Background: "000000", Text: "FFFFFF", Muted: "999999", Radius: 4}
	for name, design := range map[string]*Palette{"neon": nil, "hud": palette} {
		raw, err := HookHTML(design)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		page := string(raw)
		if !strings.Contains(page, ">MEJOR JUGADA<") {
			t.Fatalf("%s label lacks its text", name)
		}
		if !strings.Contains(page, fmt.Sprintf("left:%dpx;top:%dpx;height:%dpx", HookLabelLeft, HookLabelTop, HookLabelHeight)) {
			t.Fatalf("%s label is not in its top left place", name)
		}
		if strings.Contains(page, "http://") || strings.Contains(page, "https://") || strings.Contains(page, "<script") || strings.Contains(page, "<img") {
			t.Fatalf("%s page loads something from outside", name)
		}
		if strings.Contains(page, "#112233") != (design != nil) || strings.Contains(page, "#d99aff") != (design == nil) {
			t.Fatalf("%s page does not use its own colours", name)
		}
	}
	if HookLabelTop+HookLabelHeight > HalftimeCardTop || HookLabelLeft < FrameHeight*4/100+FrameHeight*26/100 {
		t.Fatal("the label must stay above the halftime card and clear of the top left radar")
	}
}
