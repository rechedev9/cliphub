package demooverlay

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/argv"
)

const mutedColor = "0xA1A1AA"

const (
	outroScoreSize    = 34
	outroColLabelSize = 14
	outroNameSize     = 22
	outroTopNameSize  = 24
	outroStatSize     = 21
	outroPOVLabelSize = 10
)

// OverlayWindows returns the compiled-timeline windows for intro and outro
// image overlays. Intro covers the first five seconds, clamped to the video.
// Outro covers the last beats after the win banner without overlapping intro.
func OverlayWindows(durationSeconds float64) (introStart, introEnd, outroStart, outroEnd float64) {
	if durationSeconds <= 0 {
		return 0, 0, 0, 0
	}
	introStart = IntroOverlayStart()
	introEnd = IntroOverlayEnd()
	if introEnd > durationSeconds {
		introEnd = durationSeconds
	}
	if introStart >= introEnd {
		introStart, introEnd = 0, 0
	}
	outroStart = durationSeconds - OutroSeconds
	if outroStart < introEnd {
		outroStart = introEnd
	}
	if outroStart < 0 {
		outroStart = 0
	}
	return introStart, introEnd, outroStart, durationSeconds
}

// ffmpegDrawtextText escapes a value for drawtext's text option. Every
// drawtext clause here sets expansion=none, so % is literal and must not be
// doubled: escaping it as %% painted "79%%" on intro/outro stat cells.
func ffmpegDrawtextText(text string) string {
	text = strings.ReplaceAll(text, `\\`, `\\\\`)
	text = strings.ReplaceAll(text, `'`, `\\'`)
	text = strings.ReplaceAll(text, `:`, `\\:`)
	return text
}
