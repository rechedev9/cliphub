package renderplan

import (
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/radaroverlay"
)

// normalizeRadar clears the radar options when it is off and stores its defaults as unset, so equal
// choices give equal requests.
func normalizeRadar(req EditRequest) EditRequest {
	req.RadarPosition = strings.ToLower(strings.TrimSpace(req.RadarPosition))
	req.RadarSize = strings.ToLower(strings.TrimSpace(req.RadarSize))
	if !req.Radar {
		req.RadarPosition, req.RadarSize, req.RadarOpacity = "", "", 0
		return req
	}
	if req.RadarSize == radaroverlay.SizeMedium {
		req.RadarSize = ""
	}
	if req.RadarOpacity == radaroverlay.DefaultOpacity {
		req.RadarOpacity = 0
	}
	return req
}

// validateRadar keeps ClipHub's radar on a Short of plays recorded without the CS2 HUD, whose own
// radar would show twice; the capture is never changed to hide it.
func (r EditRequest) validateRadar() error {
	if !r.Radar {
		if r.RadarPosition != "" || r.RadarSize != "" || r.RadarOpacity != 0 {
			return fmt.Errorf("radar_position, radar_size and radar_opacity need radar")
		}
		return nil
	}
	switch {
	case r.MatchRecap || r.FullDemo != nil:
		return fmt.Errorf("the radar is for a Short of plays, not a match recap or a Full Demo")
	case r.NativeHUD:
		return fmt.Errorf("the radar draws over a capture without the CS2 HUD; turn native_hud off")
	}
	if err := radaroverlay.ValidatePosition(r.RadarPosition); err != nil {
		return err
	}
	if err := radaroverlay.ValidateSize(r.RadarSize); err != nil {
		return err
	}
	return radaroverlay.ValidateOpacity(r.RadarOpacity)
}
