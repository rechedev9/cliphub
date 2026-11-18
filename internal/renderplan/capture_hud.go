package renderplan

import (
	"fmt"
	"slices"
	"strings"
)

// CaptureHUDs are the Short capture profiles a 16:9 video of plays may record in instead of its preset's HUD:
// CS2's whole HUD, its kill feed alone, or no HUD (the recorder's --hud modes).
var CaptureHUDs = []string{"gameplay", "deathnotices", "clean"}

// normalizeCaptureHUD drops the profile where the native HUD, a match recap or a Full Demo already decides the capture.
func normalizeCaptureHUD(req EditRequest) EditRequest {
	req.CaptureHUD = strings.ToLower(strings.TrimSpace(req.CaptureHUD))
	if req.NativeHUD || req.MatchRecap || req.FullDemo != nil {
		req.CaptureHUD = ""
	}
	return req
}

func (r EditRequest) validateCaptureHUD() error {
	switch {
	case r.CaptureHUD == "":
		return nil
	case !slices.Contains(CaptureHUDs, r.CaptureHUD):
		return fmt.Errorf("capture_hud must be gameplay, deathnotices or clean")
	case r.Format != FormatLandscape16x9:
		return fmt.Errorf("capture_hud is for a %s video; a Short records the HUD of its preset", FormatLandscape16x9)
	}
	return nil
}
