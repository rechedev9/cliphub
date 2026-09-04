package keydropbanner

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CompositeWithCode writes a PNG of the plate with the baked code covered and
// the live sponsor code drawn on top. ffmpegPath must resolve to a working
// FFmpeg binary; outPath's parent directory must already exist.
func CompositeWithCode(ffmpegPath, family, styleID, code, fontPath, outPath string) error {
	style, ok := Lookup(family, styleID)
	if !ok {
		return fmt.Errorf("unknown %s banner style %q", strings.ToLower(FamilyLabel(EffectiveFamily(family, styleID))), styleID)
	}
	platePath, err := Materialize(family, styleID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(ffmpegPath) == "" {
		return fmt.Errorf("composite keydrop banner: ffmpeg path is required")
	}
	if strings.TrimSpace(fontPath) == "" {
		return fmt.Errorf("composite keydrop banner: font path is required")
	}
	if strings.TrimSpace(outPath) == "" {
		return fmt.Errorf("composite keydrop banner: output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return fmt.Errorf("composite keydrop banner: create output directory: %w", err)
	}

	w, h := style.Width, style.Height
	coverX := int(math.Round(style.CoverX * float64(w)))
	coverY := int(math.Round(style.CoverY * float64(h)))
	coverW := int(math.Round(style.CoverW * float64(w)))
	coverH := int(math.Round(style.CoverH * float64(h)))
	label := DisplayLabelFor(family, styleID, code)
	fontSize := fitFontSize(label, int(math.Round(float64(h)*style.FontSizeFrac)), coverW)
	textY := int(math.Round(style.TextCenterY*float64(h))) - fontSize/2
	if textY < 0 {
		textY = 0
	}
	vf := fmt.Sprintf(
		"%s,"+
			"drawtext=fontfile='%s':text='%s':fontcolor=white:fontsize=%d:"+
			"borderw=2:bordercolor=black@0.75:shadowcolor=black@0.45:shadowx=2:shadowy=2:"+
			"x=(w-text_w)/2:y=%d",
		coverFilter(style, coverX, coverY, coverW, coverH),
		ffmpegFilterPath(fontPath), ffmpegDrawtextText(label), fontSize, textY,
	)
	// #nosec G204 -- ffmpegPath is the host FFmpeg resolved by ClipHub config;
	// plate/font/out paths are local materializations under ClipHub control.
	cmd := exec.Command(ffmpegPath, //nolint:gosec
		"-y", "-hide_banner", "-loglevel", "error",
		"-i", platePath,
		"-vf", vf,
		"-frames:v", "1",
		outPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("composite keydrop banner: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// coverGradientSteps is how many horizontal strips a gradient cover uses; on a
// ~120px band that is a step every ~10px, below what the eye separates.
const coverGradientSteps = 12

// coverFilter paints the code bay. A flat CoverColor is one drawbox; with
// CoverColorBottom set it becomes a stack of strips interpolated top to bottom.
func coverFilter(style Style, x, y, w, h int) string {
	if style.CoverColorBottom == "" {
		return fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=%s@1:t=fill", x, y, w, h, style.CoverColor)
	}
	top, okTop := parseHexColor(style.CoverColor)
	bottom, okBottom := parseHexColor(style.CoverColorBottom)
	if !okTop || !okBottom {
		return fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=%s@1:t=fill", x, y, w, h, style.CoverColor)
	}
	steps := coverGradientSteps
	if h < steps {
		steps = h
	}
	parts := make([]string, 0, steps)
	for i := 0; i < steps; i++ {
		y0 := y + i*h/steps
		y1 := y + (i+1)*h/steps
		t := float64(i) / float64(steps-1)
		if steps == 1 {
			t = 0
		}
		parts = append(parts, fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=%s@1:t=fill",
			x, y0, w, y1-y0, mixHexColor(top, bottom, t)))
	}
	return strings.Join(parts, ",")
}

// parseHexColor reads the 0xRRGGBB form the catalog uses.
func parseHexColor(value string) ([3]int, bool) {
	value = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "0x")
	if len(value) != 6 {
		return [3]int{}, false
	}
	var rgb [3]int
	for i := 0; i < 3; i++ {
		var component int
		if _, err := fmt.Sscanf(value[2*i:2*i+2], "%02x", &component); err != nil {
			return [3]int{}, false
		}
		rgb[i] = component
	}
	return rgb, true
}

func mixHexColor(a, b [3]int, t float64) string {
	var out [3]int
	for i := range out {
		out[i] = int(math.Round(float64(a[i]) + (float64(b[i])-float64(a[i]))*t))
	}
	return fmt.Sprintf("0x%02x%02x%02x", out[0], out[1], out[2])
}

// Heavy display glyphs average about this fraction of the font size in width;
// enough to keep a long code inside the bay without measuring the font.
const labelGlyphWidthFrac = 0.62

// minFontSize keeps a very long label legible rather than shrinking forever.
const minFontSize = 18

// fitFontSize shrinks fontSize until label fits the cover width, so a
// sixteen-character code never runs past the plate's bar.
func fitFontSize(label string, fontSize, coverW int) int {
	if fontSize < minFontSize {
		fontSize = minFontSize
	}
	runes := len([]rune(label))
	if runes == 0 || coverW <= 0 {
		return fontSize
	}
	maxWidth := float64(coverW) * 0.96
	if float64(runes)*float64(fontSize)*labelGlyphWidthFrac <= maxWidth {
		return fontSize
	}
	fitted := int(math.Floor(maxWidth / (float64(runes) * labelGlyphWidthFrac)))
	if fitted < minFontSize {
		return minFontSize
	}
	return fitted
}
