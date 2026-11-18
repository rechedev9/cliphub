package workers

import (
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// fullDemoOverlayPalette is the HUD design's palette from the approved plan; nil for a Short or another design.
func fullDemoOverlayPalette(edit renderplan.EditRequest) *demooverlay.Palette {
	if edit.FullDemo == nil {
		return nil
	}
	return demooverlay.DesignPalette(edit.FullDemo.Document.Options.Overlays)
}
