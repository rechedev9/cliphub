package keydropbanner

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// OverlayParams drives the FFmpeg filter chain that paints a KeyDrop plate
// onto an already-laid-out content label and emits a new label.
//
// KeyDropImagePath must be a plate that already carries the live sponsor
// code (CompositeWithCode). This filter only scales, loops, and overlays —
// it never draws the code itself, so a wrong or stale code cannot hide
// inside an in-filtergraph drawtext that the caller never re-ran.
type OverlayParams struct {
	Style        Style
	OutputWidth  int
	OutputHeight int
	// PositionY is the vertical center of the plate as a fraction of output
	// height in [0.025, 0.975]. Zero means DefaultPositionY.
	PositionY float64
	// SlideEnabled animates the plate in from the left and out on exit.
	SlideEnabled bool
	// DurationSeconds is the clip length for plate loop and clamp of the
	// visibility window.
	DurationSeconds float64
	// StartSeconds / EndSeconds bound when the plate is visible on the clip
	// timeline (t=0 is clip start). Negative or zero End means clip end.
	// Start defaults to 0 when negative.
	StartSeconds float64
	EndSeconds   float64
	// ContentLabel is the filtergraph label of the video under the plate
	// (without brackets), e.g. "content" or "bannered".
	ContentLabel string
	// OutputLabel is the label written for the composited stream.
	OutputLabel string
	// InputIndex is the FFmpeg input index of the plate PNG (e.g. 1).
	InputIndex int
}

// BuildOverlayFilter returns the filtergraph fragment that scales the
// pre-composited plate and overlays it on ContentLabel.
//
// The fragment assumes the plate PNG is already present as input InputIndex
// and that ContentLabel exists. It ends by defining OutputLabel.
func BuildOverlayFilter(p OverlayParams) (string, error) {
	if p.Style.ID == "" {
		return "", fmt.Errorf("keydrop overlay: style is required")
	}
	if p.OutputWidth <= 0 || p.OutputHeight <= 0 {
		return "", fmt.Errorf("keydrop overlay: output size is required")
	}
	if p.DurationSeconds <= 0 || math.IsNaN(p.DurationSeconds) || math.IsInf(p.DurationSeconds, 0) {
		return "", fmt.Errorf("keydrop overlay: duration must be finite and > 0")
	}
	content := strings.TrimSpace(p.ContentLabel)
	if content == "" {
		return "", fmt.Errorf("keydrop overlay: content label is required")
	}
	out := strings.TrimSpace(p.OutputLabel)
	if out == "" {
		return "", fmt.Errorf("keydrop overlay: output label is required")
	}
	if p.InputIndex < 0 {
		return "", fmt.Errorf("keydrop overlay: input index must be >= 0")
	}

	positionY := p.PositionY
	if positionY == 0 {
		positionY = DefaultPositionY
	}
	if positionY < 0.025 || positionY > 0.975 {
		return "", fmt.Errorf("keydrop overlay: position_y must be between 0.025 and 0.975")
	}

	targetW := TargetWidth(p.OutputWidth)
	// Keep aspect from the plate.
	targetH := int(math.Round(float64(targetW) * float64(p.Style.Height) / float64(p.Style.Width)))
	if targetH < 1 {
		targetH = 1
	}
	centerY := int(math.Round(positionY * float64(p.OutputHeight)))
	top := centerY - targetH/2
	if top < 0 {
		top = 0
	}
	if top+targetH > p.OutputHeight {
		top = p.OutputHeight - targetH
	}

	start, end := resolveVisibleWindow(p.StartSeconds, p.EndSeconds, p.DurationSeconds)
	window := end - start
	xExpr := "(main_w-overlay_w)/2"
	if p.SlideEnabled {
		phase := math.Min(0.35, window/2)
		enterEnd := start + phase
		exitStart := end - phase
		// Slide in at window start, hold, slide out before window end.
		// t is clip-absolute (same clock as enable=between).
		xExpr = fmt.Sprintf(
			`if(lt(t\,%s)\,-overlay_w+(main_w-overlay_w)/2*((t-%s)/%s)\,if(lt(t\,%s)\,(main_w-overlay_w)/2\,(main_w-overlay_w)/2-overlay_w*((t-%s)/%s)))`,
			floatArg(enterEnd), floatArg(start), floatArg(phase),
			floatArg(exitStart), floatArg(exitStart), floatArg(phase),
		)
	}

	// loop+trim so a still PNG covers the whole clip; setpts aligns with content.
	// Code is already burned into the plate PNG by CompositeWithCode.
	plate := fmt.Sprintf(
		"[%d:v]format=rgba,scale=%d:%d:flags=lanczos,"+
			"loop=loop=-1:size=1:start=0,setpts=N/60/TB,trim=duration=%s,setpts=PTS-STARTPTS[kdplate]",
		p.InputIndex, targetW, targetH,
		floatArg(p.DurationSeconds),
	)

	overlay := fmt.Sprintf(
		"[%s][%s]overlay=x='%s':y=%d:eval=frame:format=auto:enable='between(t\\,%s\\,%s)':eof_action=pass:shortest=0[%s]",
		content, "kdplate", xExpr, top, floatArg(start), floatArg(end), out,
	)
	return plate + ";" + overlay, nil
}

// resolveVisibleWindow clamps the plate's on-screen window to the clip.
// start < 0 → 0; end <= 0 or past duration → duration; end <= start is rejected
// by callers / expanded to a tiny epsilon only when duration allows.
func resolveVisibleWindow(start, end, duration float64) (float64, float64) {
	if start < 0 || math.IsNaN(start) || math.IsInf(start, 0) {
		start = 0
	}
	if end <= 0 || math.IsNaN(end) || math.IsInf(end, 0) || end > duration {
		end = duration
	}
	if start >= duration {
		start = 0
	}
	if end <= start {
		end = duration
	}
	return start, end
}

func floatArg(v float64) string {
	return strconv.FormatFloat(v, 'f', 6, 64)
}

// ffmpegFilterPath escapes a filesystem path for embedding in an ffmpeg
// filtergraph (drawtext fontfile). Always uses forward slashes.
func ffmpegFilterPath(path string) string {
	path = strings.ReplaceAll(path, `\`, `/`)
	path = strings.ReplaceAll(path, `'`, `\'`)
	path = strings.ReplaceAll(path, `:`, `\:`)
	path = strings.ReplaceAll(path, `[`, `\[`)
	path = strings.ReplaceAll(path, `]`, `\]`)
	return path
}

// ffmpegDrawtextText escapes user text for drawtext's text= option.
func ffmpegDrawtextText(text string) string {
	text = strings.ReplaceAll(text, `\`, `\\`)
	text = strings.ReplaceAll(text, `'`, `\'`)
	text = strings.ReplaceAll(text, `:`, `\:`)
	text = strings.ReplaceAll(text, `%`, `%%`)
	return text
}
