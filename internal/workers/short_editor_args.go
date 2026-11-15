package workers

import (
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/renderplan"
)

// shortOverlayArgs are the editor flags of the victim replay, the centered kill feed, the play badges,
// the win chance, the vertical HUD and the radar.
// The player labels and grenade trails go with their world track (shortDataArgs).
func shortOverlayArgs(edit renderplan.EditRequest) []string {
	var args []string
	if edit.VictimReplay {
		args = append(args, "--victim-replay")
		if edit.VictimReplaySpeed != 0 {
			args = append(args, "--victim-replay-speed", strconv.FormatFloat(edit.VictimReplaySpeed, 'f', -1, 64))
		}
	}
	if edit.CenteredKillfeed {
		args = append(args, "--centered-killfeed")
	}
	if edit.PlayBadges {
		args = append(args, "--play-badges")
	}
	if edit.WinProbability {
		args = append(args, "--win-probability")
	}
	if edit.VerticalHUD {
		args = append(args, "--vertical-hud")
		if edit.VerticalHUDTheme != "" {
			args = append(args, "--vertical-hud-theme", edit.VerticalHUDTheme)
		}
	}
	if edit.Radar {
		args = append(args, "--radar")
		if edit.RadarPosition != "" {
			args = append(args, "--radar-position", edit.RadarPosition)
		}
		if edit.RadarSize != "" {
			args = append(args, "--radar-size", edit.RadarSize)
		}
		if edit.RadarOpacity != 0 {
			args = append(args, "--radar-opacity", strconv.FormatFloat(edit.RadarOpacity, 'f', -1, 64))
		}
	}
	return args
}

// keyDropArgs are the editor flags of the affiliate banner; none without a style.
func keyDropArgs(edit renderplan.EditRequest) []string {
	style := strings.TrimSpace(edit.KeyDropStyle)
	if style == "" {
		return nil
	}
	var args []string
	if family := strings.TrimSpace(edit.KeyDropFamily); family != "" {
		args = append(args, "--keydrop-family", family)
	}
	args = append(args, "--keydrop-style", style)
	if code := strings.TrimSpace(edit.KeyDropCode); code != "" {
		args = append(args, "--keydrop-code", code)
	}
	if y := edit.KeyDropPositionY; y != nil {
		args = append(args, "--keydrop-position-y", strconv.FormatFloat(*y, 'f', 6, 64))
	}
	if s := edit.KeyDropStartSeconds; s != nil {
		args = append(args, "--keydrop-start", strconv.FormatFloat(*s, 'f', 6, 64))
	}
	if e := edit.KeyDropEndSeconds; e != nil {
		args = append(args, "--keydrop-end", strconv.FormatFloat(*e, 'f', 6, 64))
	}
	return args
}

// compileSegmentsArgs returns the zv-editor flags for a multi-segment render: one
// concatenated Short, or with SeparateShorts one Short per segment of the same capture.
// A single segment keeps the per-segment Short.
func compileSegmentsArgs(edit renderplan.EditRequest, segmentIDs []string) []string {
	if len(segmentIDs) < 2 {
		return nil
	}
	if edit.SeparateShorts {
		return []string{"--segments", strings.Join(segmentIDs, ",")}
	}
	return []string{"--compile-segments", "--segments", strings.Join(segmentIDs, ",")}
}
