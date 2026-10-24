package recapplan

import (
	"fmt"

	"github.com/rechedev9/cliphub/internal/prosettings"
)

// ResolveProSettings copies a catalog entry into the plan. An empty id keeps
// the standard 1920x1080 capture with the user's own viewmodel.
func ResolveProSettings(playerID string) (ProSettings, error) {
	if playerID == "" {
		return ProSettings{}, nil
	}
	p, ok := prosettings.Find(playerID)
	if !ok {
		return ProSettings{}, fmt.Errorf("unknown pro settings player %q", playerID)
	}
	width, height := p.CaptureSize()
	return ProSettings{
		PlayerID: p.ID, Name: p.Name, Width: p.Width, Height: p.Height, Scaling: p.Scaling,
		CaptureWidth: width, CaptureHeight: height,
		ViewmodelFOV: p.Viewmodel.FOV, OffsetX: p.Viewmodel.OffsetX, OffsetY: p.Viewmodel.OffsetY, OffsetZ: p.Viewmodel.OffsetZ,
	}, nil
}

// CaptureSize is the CS2 window and recorded stream size.
func (c CaptureOptions) CaptureSize() (int, int) {
	if c.ProSettings.PlayerID == "" {
		return 1920, 1080
	}
	return c.ProSettings.CaptureWidth, c.ProSettings.CaptureHeight
}

// StretchToFrame fills 16:9 the way a stretched pro sees it; every other
// capture keeps its aspect ratio inside the 1920x1080 frame.
func (c CaptureOptions) StretchToFrame() bool {
	return c.ProSettings.Scaling == prosettings.ScalingStretched
}

func (s ProSettings) Validate() error {
	if s == (ProSettings{}) {
		return nil
	}
	if s.PlayerID == "" || s.Name == "" || s.Width <= 0 || s.Height <= 0 {
		return fmt.Errorf("pro settings require a player and resolution")
	}
	if !prosettings.ValidScaling(s.Scaling) {
		return fmt.Errorf("invalid pro settings scaling %q", s.Scaling)
	}
	if s.CaptureHeight != prosettings.CaptureHeight || s.CaptureWidth != prosettings.CaptureWidth(s.Width, s.Height) || s.CaptureWidth < s.CaptureHeight || s.CaptureWidth > 1920 {
		return fmt.Errorf("pro settings capture size %dx%d does not match %dx%d", s.CaptureWidth, s.CaptureHeight, s.Width, s.Height)
	}
	return prosettings.Viewmodel{FOV: s.ViewmodelFOV, OffsetX: s.OffsetX, OffsetY: s.OffsetY, OffsetZ: s.OffsetZ}.Validate()
}

// Cvars are the CS2 settings the capture applies and must read back.
func (s ProSettings) Cvars() map[string]float64 {
	if s.PlayerID == "" {
		return map[string]float64{}
	}
	return map[string]float64{"viewmodel_fov": s.ViewmodelFOV, "viewmodel_offset_x": s.OffsetX, "viewmodel_offset_y": s.OffsetY, "viewmodel_offset_z": s.OffsetZ}
}
