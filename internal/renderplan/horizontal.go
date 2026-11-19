package renderplan

import "fmt"

// ShortCapture is how a Short's plays were recorded: the recorder HUD and whether CS2's kill feed sits in the 9:16 crop.
type ShortCapture struct {
	HUDMode              string
	PortraitSafeKillfeed bool
}

// The generate_short ids of the options HorizontalEdit leaves out of the 16:9 summary.
const (
	DroppedVerticalHUD      = "vertical_hud"
	DroppedCenteredKillfeed = "centered_killfeed"
	DroppedStatsCard        = "stats_card"
	DroppedRadar            = "radar"
	DroppedKillCounter      = "kill_counter"
	DroppedPlayerCards      = "player_cards"
)

const captureHUDGameplay = "gameplay"

// HorizontalEdit is the 16:9 summary of a Short's plays (Crear horizontal): the Short's capture, pre-roll,
// effects, transitions, texts and team comms, and the names of the 9:16 options it drops.
func HorizontalEdit(short EditRequest, capture ShortCapture) (EditRequest, []string) {
	short = NormalizeEditRequest(short)
	var dropped []string
	switch {
	case short.VerticalHUD:
		dropped = append(dropped, DroppedVerticalHUD)
	case short.CenteredKillfeed:
		dropped = append(dropped, DroppedCenteredKillfeed)
	}
	for _, option := range []struct {
		id string
		on bool
	}{{DroppedStatsCard, short.StatsCard}, {DroppedRadar, short.Radar}, {DroppedKillCounter, short.KillCounter},
		{DroppedPlayerCards, short.OpeningCard != "" || short.EndCard != "" || short.CoverCard != ""}} {
		if option.on {
			dropped = append(dropped, option.id)
		}
	}
	next := short
	next.Format, next.MatchRecap, next.DeathReel, next.SeparateShorts = FormatLandscape16x9, false, false, false
	next.VerticalHUD, next.VerticalHUDTheme, next.CenteredKillfeed, next.StatsCard, next.KillCounter = false, "", false, false, false
	next.Radar, next.RadarPosition, next.RadarSize, next.RadarOpacity = false, "", "", 0
	next.OpeningCard, next.EndCard, next.CoverCard = "", "", ""
	next.DemoSource, next.OverlayTheme = "", ""
	next.NativeHUD = capture.HUDMode == captureHUDGameplay
	next.CaptureHUD = ""
	if !next.NativeHUD {
		next.CaptureHUD = capture.HUDMode
	}
	next.PortraitSafeKillfeed = capture.PortraitSafeKillfeed
	return NormalizeEditRequest(next), dropped
}

// normalizePortraitSafeKillfeed keeps the Short's kill feed framing only on a 16:9 video of plays whose capture shows it.
func normalizePortraitSafeKillfeed(req EditRequest) EditRequest {
	if req.MatchRecap || req.FullDemo != nil || req.DeathReel || req.CaptureHUD == "clean" {
		req.PortraitSafeKillfeed = false
	}
	return req
}

func (r EditRequest) validatePortraitSafeKillfeed() error {
	if r.PortraitSafeKillfeed && r.Format != FormatLandscape16x9 {
		return fmt.Errorf("portrait_safe_killfeed is for a %s video made from a Short; a Short frames its kill feed from its preset", FormatLandscape16x9)
	}
	return nil
}
