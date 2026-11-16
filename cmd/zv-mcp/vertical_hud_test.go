package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestResolveEditVerticalHUD(t *testing.T) {
	on, off := true, false
	edit, err := resolveEdit(shortChoices{VerticalHUD: &on, VerticalHUDTheme: "Prism"})
	if err != nil {
		t.Fatal(err)
	}
	req := decodeEdit(t, edit)
	if !req.VerticalHUD || req.VerticalHUDTheme != "prism" || !req.CenteredKillfeed {
		t.Fatalf("vertical HUD not sent with its kill feed: %+v", req)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("renderplan rejected the vertical HUD: %v", err)
	}
	if summary := describeShortRequest(tuiclient.GenerateRequest{Edit: edit}); !strings.Contains(summary, "vertical_hud") || !strings.Contains(summary, "prism") {
		t.Errorf("summary misses the vertical HUD: %s", summary)
	}
	for name, c := range map[string]shortChoices{
		"landscape":         {Format: "landscape-16x9", VerticalHUD: &on},
		"native HUD":        {NativeHUD: &on, VerticalHUD: &on},
		"no kill feed":      {CenteredKillfeed: &off, VerticalHUD: &on},
		"unknown design":    {VerticalHUD: &on, VerticalHUDTheme: "missing"},
		"design without it": {VerticalHUDTheme: "prism"},
	} {
		if _, err := resolveEdit(c); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	themes := verticalHUDThemes()
	if len(themes) == 0 || !themes[0].Default || themes[0].ID != "arena" {
		t.Fatalf("designs = %+v, want the catalog with its first one as default", themes)
	}
}
