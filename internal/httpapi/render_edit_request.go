package httpapi

import (
	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// renderEditRequest preserves JSON field presence so review corrections can
// update one choice without resetting the rest of the approved edit contract.
type renderEditRequest struct {
	FullDemo            *recapplan.Snapshot `json:"full_demo"`
	Format              *string             `json:"format"`
	KillEffect          *string             `json:"killEffect"`
	Transition          *string             `json:"transition"`
	Intro               *bool               `json:"intro"`
	Outro               *bool               `json:"outro"`
	HookText            *bool               `json:"hook_text"`
	KillCounter         *bool               `json:"kill_counter"`
	MatchRecap          *bool               `json:"match_recap"`
	DeathReel           *bool               `json:"death_reel"`
	SeparateShorts      *bool               `json:"separate_shorts"`
	VoiceComms          *bool               `json:"voice_comms"`
	VoiceVolume         *float64            `json:"voice_volume"`
	NativeHUD           *bool               `json:"native_hud"`
	VictimReplay        *bool               `json:"victim_replay"`
	VictimReplaySpeed   *float64            `json:"victim_replay_speed"`
	PlayBadges          *bool               `json:"play_badges"`
	WinProbability      *bool               `json:"win_probability"`
	PlayerLabels        *bool               `json:"player_labels"`
	GrenadeTrails       *bool               `json:"grenade_trails"`
	StatsCard           *bool               `json:"stats_card"`
	OpeningCard         *string             `json:"opening_card"`
	EndCard             *string             `json:"end_card"`
	CoverCard           *string             `json:"cover_card"`
	CenteredKillfeed    *bool               `json:"centered_killfeed"`
	VerticalHUD         *bool               `json:"vertical_hud"`
	VerticalHUDTheme    *string             `json:"vertical_hud_theme"`
	Radar               *bool               `json:"radar"`
	RadarPosition       *string             `json:"radar_position"`
	RadarSize           *string             `json:"radar_size"`
	RadarOpacity        *float64            `json:"radar_opacity"`
	CoverStrategy       *string             `json:"cover_strategy"`
	CoverFirstFrame     *bool               `json:"cover_first_frame"`
	IntroText           *string             `json:"intro_text"`
	OutroText           *string             `json:"outro_text"`
	KeyDropFamily       *string             `json:"keydrop_family"`
	KeyDropStyle        *string             `json:"keydrop_style"`
	KeyDropCode         *string             `json:"keydrop_code"`
	KeyDropPositionY    *float64            `json:"keydrop_position_y"`
	KeyDropStartSeconds *float64            `json:"keydrop_start_seconds"`
	KeyDropEndSeconds   *float64            `json:"keydrop_end_seconds"`
	DemoSource          *string             `json:"demo_source"`
	OverlayTheme        *string             `json:"overlay_theme"`
	PreRollSeconds      *int                `json:"pre_roll_seconds"`
	CaptureHUD          *string             `json:"capture_hud"`
	// BrandWatermark and BrandBanner turn the saved brand kit on or off; applyBrandKit resolves them.
	BrandWatermark *brandChoice[brandkit.Watermark] `json:"brand_watermark"`
	BrandBanner    *brandChoice[brandkit.Banner]    `json:"brand_banner"`
}

func (r renderEditRequest) merge(base renderplan.EditRequest) renderplan.EditRequest {
	if r.FullDemo != nil {
		base = renderplan.FullDemoEditRequest(*r.FullDemo)
	}
	if r.Format != nil {
		base.Format = *r.Format
	}
	if r.KillEffect != nil {
		base.KillEffect = *r.KillEffect
	}
	if r.Transition != nil {
		base.Transition = *r.Transition
	}
	if r.Intro != nil {
		base.Intro = *r.Intro
	}
	if r.Outro != nil {
		base.Outro = *r.Outro
	}
	if r.HookText != nil {
		base.HookText = *r.HookText
	}
	if r.KillCounter != nil {
		base.KillCounter = *r.KillCounter
	}
	if r.MatchRecap != nil {
		base.MatchRecap = *r.MatchRecap
	}
	if r.DeathReel != nil {
		base.DeathReel = *r.DeathReel
	}
	if r.SeparateShorts != nil {
		base.SeparateShorts = *r.SeparateShorts
	}
	if r.VoiceComms != nil {
		base.VoiceComms = *r.VoiceComms
	}
	if r.VoiceVolume != nil {
		v := *r.VoiceVolume
		base.VoiceVolume = &v
	}
	if r.NativeHUD != nil {
		base.NativeHUD = *r.NativeHUD
	}
	if r.VictimReplay != nil {
		base.VictimReplay = *r.VictimReplay
	}
	if r.VictimReplaySpeed != nil {
		base.VictimReplaySpeed = *r.VictimReplaySpeed
	}
	if r.PlayBadges != nil {
		base.PlayBadges = *r.PlayBadges
	}
	if r.WinProbability != nil {
		base.WinProbability = *r.WinProbability
	}
	if r.PlayerLabels != nil {
		base.PlayerLabels = *r.PlayerLabels
	}
	if r.GrenadeTrails != nil {
		base.GrenadeTrails = *r.GrenadeTrails
	}
	if r.StatsCard != nil {
		base.StatsCard = *r.StatsCard
	}
	for _, slot := range []struct{ from, to *string }{{r.OpeningCard, &base.OpeningCard}, {r.EndCard, &base.EndCard}, {r.CoverCard, &base.CoverCard}} {
		if slot.from != nil {
			*slot.to = *slot.from
		}
	}
	if r.CenteredKillfeed != nil {
		base.CenteredKillfeed = *r.CenteredKillfeed
	}
	if r.VerticalHUD != nil {
		base.VerticalHUD = *r.VerticalHUD
	}
	if r.VerticalHUDTheme != nil {
		base.VerticalHUDTheme = *r.VerticalHUDTheme
	}
	if r.Radar != nil {
		base.Radar = *r.Radar
	}
	if r.RadarPosition != nil {
		base.RadarPosition = *r.RadarPosition
	}
	if r.RadarSize != nil {
		base.RadarSize = *r.RadarSize
	}
	if r.RadarOpacity != nil {
		base.RadarOpacity = *r.RadarOpacity
	}
	if r.CoverStrategy != nil {
		base.CoverStrategy = *r.CoverStrategy
	}
	if r.CoverFirstFrame != nil {
		base.CoverFirstFrame = *r.CoverFirstFrame
	}
	if r.IntroText != nil {
		base.IntroText = *r.IntroText
	}
	if r.OutroText != nil {
		base.OutroText = *r.OutroText
	}
	if r.KeyDropFamily != nil {
		base.KeyDropFamily = *r.KeyDropFamily
	}
	if r.KeyDropStyle != nil {
		base.KeyDropStyle = *r.KeyDropStyle
	}
	if r.KeyDropCode != nil {
		base.KeyDropCode = *r.KeyDropCode
	}
	if r.KeyDropPositionY != nil {
		y := *r.KeyDropPositionY
		base.KeyDropPositionY = &y
	}
	if r.KeyDropStartSeconds != nil {
		s := *r.KeyDropStartSeconds
		base.KeyDropStartSeconds = &s
	}
	if r.KeyDropEndSeconds != nil {
		e := *r.KeyDropEndSeconds
		base.KeyDropEndSeconds = &e
	}
	if r.DemoSource != nil {
		base.DemoSource = *r.DemoSource
	}
	if r.OverlayTheme != nil {
		base.OverlayTheme = *r.OverlayTheme
	}
	if r.PreRollSeconds != nil {
		base.PreRollSeconds = *r.PreRollSeconds
	}
	if r.CaptureHUD != nil {
		base.CaptureHUD = *r.CaptureHUD
	}
	return renderplan.NormalizeEditRequest(base)
}

func (r renderEditRequest) complete() bool {
	return r.Format != nil &&
		r.KillEffect != nil &&
		r.Transition != nil &&
		r.Intro != nil &&
		r.Outro != nil &&
		r.HookText != nil &&
		r.KillCounter != nil &&
		r.CoverStrategy != nil &&
		r.CoverFirstFrame != nil &&
		r.IntroText != nil &&
		r.OutroText != nil
}
