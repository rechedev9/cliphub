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
func CompositeWithCode(ffmpegPath, styleID, code, fontPath, outPath string) error {
	style, ok := Lookup(styleID)
	if !ok {
		return fmt.Errorf("unknown keydrop banner style %q", styleID)
	}
	platePath, err := Materialize(styleID)
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
	fontSize := int(math.Round(float64(h) * style.FontSizeFrac))
	if fontSize < 18 {
		fontSize = 18
	}
	textY := int(math.Round(style.TextCenterY*float64(h))) - fontSize/2
	if textY < 0 {
		textY = 0
	}
	label := DisplayLabel(code)
	vf := fmt.Sprintf(
		"drawbox=x=%d:y=%d:w=%d:h=%d:color=%s@1:t=fill,"+
			"drawtext=fontfile='%s':text='%s':fontcolor=white:fontsize=%d:"+
			"borderw=2:bordercolor=black@0.75:shadowcolor=black@0.45:shadowx=2:shadowy=2:"+
			"x=(w-text_w)/2:y=%d",
		coverX, coverY, coverW, coverH, style.CoverColor,
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
