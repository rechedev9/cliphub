package keydropbanner

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// OverlayParams drives the FFmpeg filter chain that paints a KeyDrop plate
// onto an already-laid-out content label and emits a new label.
type OverlayParams struct {
	Style        Style
	Code         string
	FontPath     string
	OutputWidth  int
	OutputHeight int
	// PositionY is the vertical center of the plate as a fraction of output
	// height in [0.025, 0.975]. Zero means DefaultPositionY.
	PositionY float64
	// SlideEnabled animates the plate in from the left and out on exit.
	SlideEnabled bool
	// DurationSeconds is the clip length for slide timing and color source.
	DurationSeconds float64
	// ContentLabel is the filtergraph label of the video under the plate
	// (without brackets), e.g. "content" or "bannered".
	ContentLabel string
	// OutputLabel is the label written for the composited stream.
	OutputLabel string
	// InputIndex is the FFmpeg input index of the plate PNG (e.g. 1).
	InputIndex int
}

// BuildOverlayFilter returns the filtergraph fragment that covers the baked
// code on the plate, draws the live code, and overlays the result.
//
// The fragment assumes the plate PNG is already present as input InputIndex
// and that ContentLabel exists. It ends by defining OutputLabel.
func BuildOverlayFilter(p OverlayParams) (string, error) {
	if p.Style.ID == "" {
		return "", fmt.Errorf("keydrop overlay: style is required")
	}
	if strings.TrimSpace(p.FontPath) == "" {
		return "", fmt.Errorf("keydrop overlay: font path is required")
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

	// Cover geometry in post-scale plate pixels.
	coverX := int(math.Round(p.Style.CoverX * float64(targetW)))
	coverY := int(math.Round(p.Style.CoverY * float64(targetH)))
	coverW := int(math.Round(p.Style.CoverW * float64(targetW)))
	coverH := int(math.Round(p.Style.CoverH * float64(targetH)))
	if coverW < 1 {
		coverW = 1
	}
	if coverH < 1 {
		coverH = 1
	}
	fontSize := int(math.Round(float64(targetH) * p.Style.FontSizeFrac))
	if fontSize < 18 {
		fontSize = 18
	}
	textY := int(math.Round(p.Style.TextCenterY*float64(targetH))) - fontSize/2
	if textY < 0 {
		textY = 0
	}

	label := DisplayLabel(p.Code)
	xExpr := "(main_w-overlay_w)/2"
	if p.SlideEnabled {
		phase := math.Min(0.35, p.DurationSeconds/2)
		exitStart := p.DurationSeconds - phase
		// Slide in from the left to center, hold, slide out left.
		xExpr = fmt.Sprintf(
			`if(lt(t\,%s)\,-overlay_w+(main_w-overlay_w)/2*(t/%s)\,if(lt(t\,%s)\,(main_w-overlay_w)/2\,(main_w-overlay_w)/2-overlay_w*((t-%s)/%s)))`,
			floatArg(phase), floatArg(phase), floatArg(exitStart), floatArg(exitStart), floatArg(phase),
		)
	}

	// loop+trim so a still PNG covers the whole clip; setpts aligns with content.
	plate := fmt.Sprintf(
		"[%d:v]format=rgba,scale=%d:%d:flags=lanczos,"+
			"drawbox=x=%d:y=%d:w=%d:h=%d:color=%s@1:t=fill,"+
			"drawtext=fontfile='%s':text='%s':fontcolor=white:fontsize=%d:"+
			"borderw=2:bordercolor=black@0.75:shadowcolor=black@0.45:shadowx=2:shadowy=2:"+
			"x=(w-text_w)/2:y=%d,"+
			"loop=loop=-1:size=1:start=0,setpts=N/60/TB,trim=duration=%s,setpts=PTS-STARTPTS[kdplate]",
		p.InputIndex, targetW, targetH,
		coverX, coverY, coverW, coverH, p.Style.CoverColor,
		ffmpegFilterPath(p.FontPath), ffmpegDrawtextText(label), fontSize,
		textY,
		floatArg(p.DurationSeconds),
	)

	overlay := fmt.Sprintf(
		"[%s][%s]overlay=x='%s':y=%d:eval=frame:format=auto:eof_action=pass:shortest=0[%s]",
		content, "kdplate", xExpr, top, out,
	)
	return plate + ";" + overlay, nil
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
