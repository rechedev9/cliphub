package demooverlay

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

// renderIntroChromePNG draws left/right intro panel chrome on a transparent
// 1920x1080 canvas. Flag image assets remain pending license approval; country
// codes are rendered as text badges in the ffmpeg filter graph instead.
func renderIntroChromePNG(doc Document) ([]byte, error) {
	l := DefaultLayout()
	theme := ResolveTheme(doc)
	img := image.NewRGBA(image.Rect(0, 0, FrameWidth, FrameHeight))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{}}, image.Point{}, draw.Src)
	drawIntroPanel(img, l.Intro.LeftPanelX, l.Intro.PanelTop, l.Intro.PanelWidth, l.Intro.PanelHeight, theme)
	drawIntroPanel(img, l.Intro.RightPanelX, l.Intro.PanelTop, l.Intro.PanelWidth, l.Intro.PanelHeight, theme)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawIntroPanel(img *image.RGBA, x, y, w, h int, theme CardTheme) {
	accent := parseHexColor(theme.Accent)
	soft := parseHexColor(theme.AccentSoft)
	fill := color.RGBA{R: 18, G: 18, B: 22, A: uint8(math.Round(0.88 * 255))}
	drawRoundedRect(img, x, y, w, h, introCardRadius, fill)
	for step := introCardGlowSteps; step >= 1; step-- {
		alpha := uint8(18 + step*10)
		glow := color.RGBA{R: soft.R, G: soft.G, B: soft.B, A: alpha}
		inset := introCardGlowSteps - step
		drawRoundedRectBorder(img, x+inset, y+inset, w-2*inset, h-2*inset, introCardRadius, 1, glow)
	}
	drawRoundedRectBorder(img, x, y, w, h, introCardRadius, 2, accent)
}

func parseHexColor(hex string) color.RGBA {
	hex = stringsTrimPrefix(hex, "0x")
	if len(hex) != 6 {
		return color.RGBA{R: 255, G: 85, B: 0, A: 255}
	}
	var r, g, b uint8
	_, _ = fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func stringsTrimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

func drawRoundedRect(img *image.RGBA, x, y, w, h, radius int, c color.RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	bounds := image.Rect(x, y, x+w, y+h)
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			if insideRoundedRect(px, py, x, y, w, h, radius) {
				img.Set(px, py, c)
			}
		}
	}
}

func drawRoundedRectBorder(img *image.RGBA, x, y, w, h, radius, thickness int, c color.RGBA) {
	if w <= 0 || h <= 0 || thickness <= 0 {
		return
	}
	outer := image.Rect(x, y, x+w, y+h)
	for py := outer.Min.Y; py < outer.Max.Y; py++ {
		for px := outer.Min.X; px < outer.Max.X; px++ {
			if !insideRoundedRect(px, py, x, y, w, h, radius) {
				continue
			}
			onBorder := false
			for t := 1; t <= thickness; t++ {
				if !insideRoundedRect(px, py, x+t, y+t, w-2*t, h-2*t, max(0, radius-t)) {
					onBorder = true
					break
				}
			}
			if onBorder {
				blendPixel(img, px, py, c)
			}
		}
	}
}

func insideRoundedRect(px, py, x, y, w, h, radius int) bool {
	if px < x || py < y || px >= x+w || py >= y+h {
		return false
	}
	if radius <= 0 {
		return true
	}
	if px-x < radius && py-y < radius {
		return cornerInside(px, py, x+radius, y+radius, radius)
	}
	if px >= x+w-radius && py-y < radius {
		return cornerInside(px, py, x+w-radius-1, y+radius, radius)
	}
	if px-x < radius && py >= y+h-radius {
		return cornerInside(px, py, x+radius, y+h-radius-1, radius)
	}
	if px >= x+w-radius && py >= y+h-radius {
		return cornerInside(px, py, x+w-radius-1, y+h-radius-1, radius)
	}
	return true
}

func cornerInside(px, py, cx, cy, radius int) bool {
	dx := float64(px - cx)
	dy := float64(py - cy)
	return dx*dx+dy*dy <= float64(radius*radius)
}

func blendPixel(img *image.RGBA, x, y int, c color.RGBA) {
	if !image.Pt(x, y).In(img.Bounds()) {
		return
	}
	dst := img.RGBAAt(x, y)
	alpha := float64(c.A) / 255
	inv := 1 - alpha
	out := color.RGBA{
		R: uint8(float64(dst.R)*inv + float64(c.R)*alpha),
		G: uint8(float64(dst.G)*inv + float64(c.G)*alpha),
		B: uint8(float64(dst.B)*inv + float64(c.B)*alpha),
		A: uint8(math.Min(255, float64(dst.A)+float64(c.A)*(1-float64(dst.A)/255))),
	}
	img.Set(x, y, out)
}
