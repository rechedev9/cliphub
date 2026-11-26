package demooverlay

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/mediafont"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

const (
	SecondCameraX           = 40
	SecondCameraY           = 225
	SecondCameraWidth       = 504
	SecondCameraHeight      = 284
	SecondCameraLabelHeight = 30
	SecondCameraRadius      = 12
	SecondCameraRadarGap    = 24
	SecondCameraFadeSeconds = 0.25

	secondCameraLabelPadding = 12
	secondCameraLabelSize    = 18
	secondCameraLabelAlpha   = 0.9
)

type cameraCorners struct {
	top, bottom bool
}

func RenderSecondCameraMaskPNG(path string) error {
	mask := image.NewGray(image.Rect(0, 0, SecondCameraWidth, SecondCameraHeight))
	for y := range SecondCameraHeight {
		for x := range SecondCameraWidth {
			coverage := roundedCoverage(x, y, SecondCameraWidth, SecondCameraHeight, cameraCorners{top: true})
			mask.SetGray(x, y, color.Gray{Y: uint8(math.Round(255 * coverage))})
		}
	}
	return writeCameraPNG(path, mask)
}

func RenderSecondCameraLabelPNG(name string, palette *Palette, path string) error {
	p := nativePalette
	if palette != nil {
		p = *palette
	}
	background, err := hexColor(p.Background)
	if err != nil {
		return err
	}
	text, err := hexColor(p.Text)
	if err != nil {
		return err
	}
	bar := image.NewNRGBA(image.Rect(0, 0, SecondCameraWidth, SecondCameraLabelHeight))
	for y := range SecondCameraLabelHeight {
		for x := range SecondCameraWidth {
			coverage := roundedCoverage(x, y, SecondCameraWidth, SecondCameraLabelHeight, cameraCorners{bottom: true})
			c := background
			c.A = uint8(math.Round(255 * secondCameraLabelAlpha * coverage))
			bar.SetNRGBA(x, y, c)
		}
	}
	face, err := secondCameraFace(name)
	if err != nil {
		return err
	}
	defer face.Close()
	label := fitCameraLabel(strings.TrimSpace(name), face, SecondCameraWidth-2*secondCameraLabelPadding)
	metrics := face.Metrics()
	baseline := (fixed.I(SecondCameraLabelHeight) + metrics.Ascent - metrics.Descent) / 2
	drawer := font.Drawer{Dst: bar, Src: image.NewUniform(text), Face: face, Dot: fixed.Point26_6{X: fixed.I(secondCameraLabelPadding), Y: baseline}}
	drawer.DrawString(label)
	return writeCameraPNG(path, bar)
}

func roundedCoverage(x, y, width, height int, corners cameraCorners) float64 {
	r := float64(SecondCameraRadius)
	px, py := float64(x)+0.5, float64(y)+0.5
	cx, cy := px, py
	switch {
	case px < r:
		cx = r
	case px > float64(width)-r:
		cx = float64(width) - r
	}
	switch {
	case corners.top && py < r:
		cy = r
	case corners.bottom && py > float64(height)-r:
		cy = float64(height) - r
	}
	if cx == px || cy == py {
		return 1
	}
	return min(1, max(0, r-math.Hypot(px-cx, py-cy)+0.5))
}

func secondCameraFace(name string) (font.Face, error) {
	faces, err := mediafont.HUDFaces()
	if err != nil {
		return nil, err
	}
	var chosen *opentype.Font
	for _, candidate := range faces {
		if candidate.Weight != 700 && candidate.Weight != 800 {
			continue
		}
		parsed, err := opentype.Parse(candidate.Data)
		if err != nil {
			return nil, err
		}
		chosen = parsed
		if coversText(parsed, name) {
			break
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("second camera label has no font")
	}
	return opentype.NewFace(chosen, &opentype.FaceOptions{Size: secondCameraLabelSize, DPI: 72, Hinting: font.HintingNone})
}

func coversText(f *opentype.Font, value string) bool {
	var buf sfnt.Buffer
	for _, c := range value {
		if glyph, err := f.GlyphIndex(&buf, c); err != nil || glyph == 0 {
			return false
		}
	}
	return true
}

func fitCameraLabel(value string, face font.Face, width int) string {
	if font.MeasureString(face, value).Ceil() <= width {
		return value
	}
	runes := []rune(value)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := strings.TrimSpace(string(runes)) + "…"
		if font.MeasureString(face, candidate).Ceil() <= width {
			return candidate
		}
	}
	return ""
}

func hexColor(value string) (color.NRGBA, error) {
	parsed, err := strconv.ParseUint(value, 16, 32)
	if err != nil || len(value) != 6 {
		return color.NRGBA{}, fmt.Errorf("invalid palette colour %q", value)
	}
	return color.NRGBA{R: uint8(parsed >> 16), G: uint8(parsed >> 8), B: uint8(parsed), A: 255}, nil
}

func writeCameraPNG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}
