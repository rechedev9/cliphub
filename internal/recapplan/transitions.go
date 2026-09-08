package recapplan

import (
	"fmt"
	"math"
	"slices"
)

// Optional on the wire so historical approvals keep their original hash.
// When present every decision is required, including disabled effects.
type TransitionOptions struct {
	Enabled           bool    `json:"enabled"`
	DurationFrames    int     `json:"duration_frames"`
	Whip              bool    `json:"whip"`
	Direction         string  `json:"direction"`
	WhipStrength      float64 `json:"whip_strength"`
	BlurPixels        int     `json:"blur_pixels"`
	Zoom              bool    `json:"zoom"`
	ZoomPercent       float64 `json:"zoom_percent"`
	ZoomAnchor        string  `json:"zoom_anchor"`
	Flash             bool    `json:"flash"`
	FlashFrames       int     `json:"flash_frames"`
	FlashIntensity    float64 `json:"flash_intensity"`
	RGBSplit          bool    `json:"rgb_split"`
	RGBPixels         int     `json:"rgb_pixels"`
	Whoosh            bool    `json:"whoosh"`
	WhooshGainDB      float64 `json:"whoosh_gain_db"`
	Impact            bool    `json:"impact"`
	ImpactGainDB      float64 `json:"impact_gain_db"`
	ImpactDurationMS  int     `json:"impact_duration_ms"`
	ImpactFrequency   int     `json:"impact_frequency"`
	CommsTailSeconds  float64 `json:"comms_tail_seconds"`
	GameFadeMS        int     `json:"game_fade_ms"`
	GameTailLowpassHz int     `json:"game_tail_lowpass_hz"`
}

func DefaultTransitions() TransitionOptions {
	return TransitionOptions{
		DurationFrames: 8, Whip: true, Direction: "alternate", WhipStrength: .08, BlurPixels: 24,
		Zoom: true, ZoomPercent: 10, ZoomAnchor: "cut", FlashFrames: 2, FlashIntensity: .08,
		RGBPixels: 2, Whoosh: true, WhooshGainDB: -18, Impact: true, ImpactGainDB: -20,
		ImpactDurationMS: 180, ImpactFrequency: 55, CommsTailSeconds: .6, GameFadeMS: 40,
	}
}

func (o TransitionOptions) Validate() error {
	if !slices.Contains([]string{"left", "right", "up", "down", "alternate", "follow-motion"}, o.Direction) {
		return fmt.Errorf("invalid transitions.direction %q", o.Direction)
	}
	if !slices.Contains([]string{"cut", "last-kill", "round-end"}, o.ZoomAnchor) {
		return fmt.Errorf("invalid transitions.zoom_anchor %q", o.ZoomAnchor)
	}
	for _, n := range []struct {
		name             string
		value, low, high float64
	}{
		{"duration_frames", float64(o.DurationFrames), 6, 18},
		{"whip_strength", o.WhipStrength, .02, .2}, {"blur_pixels", float64(o.BlurPixels), 0, 64},
		{"zoom_percent", o.ZoomPercent, 5, 15}, {"flash_frames", float64(o.FlashFrames), 2, 4},
		{"flash_intensity", o.FlashIntensity, .02, .2}, {"rgb_pixels", float64(o.RGBPixels), 1, 6},
		{"whoosh_gain_db", o.WhooshGainDB, -36, -6}, {"impact_gain_db", o.ImpactGainDB, -36, -6},
		{"impact_duration_ms", float64(o.ImpactDurationMS), 80, 400}, {"impact_frequency", float64(o.ImpactFrequency), 35, 90},
		{"comms_tail_seconds", o.CommsTailSeconds, 0, 1.5}, {"game_fade_ms", float64(o.GameFadeMS), 0, 250},
		{"game_tail_lowpass_hz", float64(o.GameTailLowpassHz), 0, 12000},
	} {
		if math.IsNaN(n.value) || math.IsInf(n.value, 0) || n.value < n.low || n.value > n.high {
			return fmt.Errorf("transitions.%s must be finite and between %g and %g", n.name, n.low, n.high)
		}
	}
	if o.GameTailLowpassHz > 0 && o.GameTailLowpassHz < 200 {
		return fmt.Errorf("transitions.game_tail_lowpass_hz must be 0 or at least 200")
	}
	return nil
}

type TransitionBoundary struct {
	OutgoingIndex int   `json:"outgoing_index"`
	IncomingIndex int   `json:"incoming_index"`
	Frame         int64 `json:"frame"`
	BeforeFrames  int64 `json:"before_frames"`
	AfterFrames   int64 `json:"after_frames"`
}

// Transitions decorate existing frames. Ads, split continuations, the first
// frame and the end of the program are never treated as round transitions.
func (d Document) TransitionBoundaries() []TransitionBoundary {
	o := d.Options.Transitions
	if o == nil || !o.Enabled {
		return nil
	}
	var result []TransitionBoundary
	for i := 1; i < len(d.Timeline); i++ {
		a, b := d.Timeline[i-1], d.Timeline[i]
		if a.Role != "round" || b.Role != "round" || a.SourceRef == b.SourceRef {
			continue
		}
		before := min(int64(o.DurationFrames/2), (a.EndFrame-a.StartFrame)/2)
		after := min(int64(o.DurationFrames-o.DurationFrames/2), (b.EndFrame-b.StartFrame)/2)
		if before < 1 || after < 1 {
			continue
		}
		result = append(result, TransitionBoundary{i - 1, i, b.StartFrame, before, after})
	}
	return result
}

func (d Document) HasTransitionSFX() bool {
	o := d.Options.Transitions
	return o != nil && o.Enabled && (o.Whoosh || o.Impact) && len(d.TransitionBoundaries()) > 0
}
