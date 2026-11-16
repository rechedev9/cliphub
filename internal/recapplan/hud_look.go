package recapplan

import (
	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/hudthemes"
)

// HUDRenderer builds the custom HUD renderer of the approved overlays.
func (o OverlayOptions) HUDRenderer() (*customhud.Renderer, error) {
	renderer, err := customhud.NewRenderer(o.HUDTheme)
	if err != nil {
		return nil, err
	}
	// Field by field, so the option audit sees every leaf the render reads.
	if c := o.HUDCustom; c != nil {
		renderer.Theme = renderer.Theme.WithStyle(hudthemes.Style{Name: c.Name, CT: c.CT, T: c.T, Background: c.Background, Radius: c.Radius})
	}
	renderer.Portrait = o.HUDPortrait != nil
	h := o.HUDOptions
	renderer.Options = customhud.Options{CompactHeader: h.CompactHeader, HideLoadout: h.HideLoadout, RoundWinBanner: h.RoundWinBanner, WinProbability: h.WinProbability}
	renderer.Teams = customhud.Teams{CTStart: o.TeamNames.CTStart, TStart: o.TeamNames.TStart}
	return renderer, nil
}
