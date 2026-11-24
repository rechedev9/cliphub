package recapplan

import "github.com/rechedev9/cliphub/internal/customhud"

func (o *Options) ChooseHUD(themeID string) error {
	if themeID == "" {
		o.Capture.HUDProfile = NativeHUDProfile
		o.Overlays.HUDTheme = ""
		o.Overlays.HUDPortrait = nil
		return nil
	}
	if err := customhud.ValidateTheme(themeID); err != nil {
		return err
	}
	o.Capture.HUDProfile = customhud.CaptureProfile
	o.Overlays.HUDTheme = themeID
	if themeID != "focus" {
		o.Overlays.HUDPortrait = nil
	}
	return nil
}
