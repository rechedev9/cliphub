package demooverlay

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strconv"
	"strings"
)

// renderIntroChromePNG draws left/right intro panel chrome on a transparent
// 1920x1080 canvas. Flag image assets remain pending license approval; country
// codes are rendered as text badges in the ffmpeg filter graph instead.
func renderIntroChromePNG(doc Document) ([]byte, error) {
	l := DefaultLayout().Intro
	if NormalizeSource(doc.Source) == SourceFACEIT {
		l = faceitIntroLayout()
	}
	theme := ResolveTheme(doc)
	img := image.NewRGBA(image.Rect(0, 0, FrameWidth, FrameHeight))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{}}, image.Point{}, draw.Src)
	drawIntroPanel(img, l.LeftPanelX, l.PanelTop, l, theme, defaultFaceitLayout.Intro)
	drawIntroPanel(img, l.RightPanelX, l.PanelTop, l, theme, defaultFaceitLayout.Intro)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawIntroPanel(img *image.RGBA, x, y int, layout IntroLayout, theme CardTheme, spec introLayoutSpec) {
	w, h := layout.PanelWidth, layout.PanelHeight
	accent := parseOverlayColor(theme.Accent)
	soft := parseOverlayColor(theme.AccentSoft)
	chrome := spec.Chrome
	palette := defaultFaceitLayout.Palette
	drawRoundedRect(img, x, y, w, h, chrome.PanelRadius, parseOverlayColor(palette.IntroPanelFill))
	drawPanelTexture(img, image.Rect(x, y, x+w, y+h), chrome.TextureSpacing, parseOverlayColor(palette.IntroTexture))
	drawRoundedRectBorder(img, x, y, w, h, chrome.PanelRadius, chrome.PanelBorder, parseOverlayColor(palette.IntroPanelBorder))
	drawRectOver(img, image.Rect(x, y, x+chrome.AccentWidth, y+h), withAlpha(accent, 0.92))
	drawRectOver(img, image.Rect(x+chrome.AccentWidth, y+layout.HeaderH-1, x+w, y+layout.HeaderH-1+chrome.HeaderDivider), withAlpha(soft, 0.40))
}

// renderOutroChromePNG draws the dense, stacked FACEIT scoreboard surface.
// All text remains in the ffmpeg filter graph; this bitmap is chrome only.
func renderOutroChromePNG(doc Document) ([]byte, error) {
	layout := faceitOutroLayout()
	spec := defaultFaceitLayout.Outro
	chrome := spec.Chrome
	palette := defaultFaceitLayout.Palette
	theme := ResolveTheme(doc)
	accent := parseOverlayColor(theme.Accent)
	img := image.NewRGBA(image.Rect(0, 0, FrameWidth, FrameHeight))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{}}, image.Point{}, draw.Src)
	drawRectOver(img, img.Bounds(), parseOverlayColor(palette.OutroBackdrop))

	for teamIndex := 0; teamIndex < min(2, len(doc.Outro.Teams)); teamIndex++ {
		team := doc.Outro.Teams[teamIndex]
		shift := teamIndex * spec.TeamYGap
		boardTop := layout.HeaderY + shift + chrome.BoardTop
		boardBottom := layout.Row0 + shift + min(spec.MaxPlayers, len(team.Players))*layout.RowHeight + chrome.BoardBottom
		board := image.Rect(layout.Margin+chrome.BoardLeft, boardTop, FrameWidth-layout.Margin+chrome.BoardRight, boardBottom)
		drawRectOver(img, board, parseOverlayColor(palette.OutroBoardFill))
		drawRectBorder(img, board, 1, parseOverlayColor(palette.OutroBoardBorder))
		drawRectOver(img, image.Rect(board.Min.X, board.Min.Y, board.Min.X+chrome.AccentWidth, board.Max.Y), withAlpha(accent, 0.90))

		labelTop := layout.HeaderY + shift + chrome.LabelTop
		drawRectOver(img, image.Rect(layout.Margin, labelTop, FrameWidth-layout.Margin, layout.Row0+shift+chrome.LabelBottom), parseOverlayColor(palette.OutroLabelFill))
		for _, column := range spec.Columns {
			x := layout.Margin + column.X
			drawRectOver(img, image.Rect(x, labelTop, x+chrome.ColumnGuideWidth, boardBottom), parseOverlayColor(palette.OutroColumnGuide))
		}
		for row := 0; row < min(spec.MaxPlayers, len(team.Players)); row++ {
			y := layout.Row0 + shift + row*layout.RowHeight
			rowFill := parseOverlayColor(palette.OutroRowEven)
			if row%2 == 1 {
				rowFill = parseOverlayColor(palette.OutroRowOdd)
			}
			drawRectOver(img, image.Rect(layout.Margin, y, FrameWidth-layout.Margin, y+layout.RowHeight-chrome.RowBottomGap), rowFill)
			nameFill := parseOverlayColor(palette.OutroTName)
			if team.Side == "CT" {
				nameFill = parseOverlayColor(palette.OutroCTName)
			}
			drawRectOver(img, image.Rect(layout.Margin, y, layout.Margin+layout.NameWidth-chrome.NameRightGap, y+layout.RowHeight-chrome.RowBottomGap), nameFill)
			drawRectOver(img, image.Rect(layout.Margin, y+layout.RowHeight-chrome.RowBottomGap-chrome.DividerHeight, FrameWidth-layout.Margin, y+layout.RowHeight-chrome.RowBottomGap), parseOverlayColor(palette.OutroDivider))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawRectOver(img *image.RGBA, rect image.Rectangle, c color.Color) {
	draw.Draw(img, rect.Intersect(img.Bounds()), &image.Uniform{C: c}, image.Point{}, draw.Over)
}

func drawRectBorder(img *image.RGBA, rect image.Rectangle, thickness int, c color.Color) {
	if thickness < 1 || rect.Empty() {
		return
	}
	drawRectOver(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+thickness), c)
	drawRectOver(img, image.Rect(rect.Min.X, rect.Max.Y-thickness, rect.Max.X, rect.Max.Y), c)
	drawRectOver(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+thickness, rect.Max.Y), c)
	drawRectOver(img, image.Rect(rect.Max.X-thickness, rect.Min.Y, rect.Max.X, rect.Max.Y), c)
}

func parseOverlayColor(value string) color.NRGBA {
	parts := strings.SplitN(value, "@", 2)
	hex := strings.TrimPrefix(parts[0], "0x")
	rgb, err := strconv.ParseUint(hex, 16, 24)
	if err != nil || len(hex) != 6 {
		return color.NRGBA{R: 255, G: 85, B: 0, A: 255}
	}
	alpha := 1.0
	if len(parts) == 2 {
		if parsed, parseErr := strconv.ParseFloat(parts[1], 64); parseErr == nil {
			alpha = parsed
		}
	}
	return color.NRGBA{
		R: uint8(rgb >> 16),
		G: uint8(rgb >> 8),
		B: uint8(rgb),
		A: uint8(math.Round(alpha * 255)),
	}
}

func withAlpha(c color.NRGBA, alpha float64) color.NRGBA {
	c.A = uint8(math.Round(alpha * 255))
	return c
}

func drawPanelTexture(img *image.RGBA, bounds image.Rectangle, spacing int, c color.NRGBA) {
	if spacing < 1 || bounds.Empty() || c.A == 0 {
		return
	}
	for start := bounds.Min.X - bounds.Dy(); start < bounds.Max.X; start += spacing {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			x := start + (y - bounds.Min.Y)
			if x >= bounds.Min.X && x < bounds.Max.X {
				blendPixel(img, x, y, c)
			}
		}
	}
}

func drawRoundedRect(img *image.RGBA, x, y, w, h, radius int, c color.Color) {
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

func drawRoundedRectBorder(img *image.RGBA, x, y, w, h, radius, thickness int, c color.NRGBA) {
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

func blendPixel(img *image.RGBA, x, y int, c color.NRGBA) {
	if !image.Pt(x, y).In(img.Bounds()) {
		return
	}
	dst := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	srcA := float64(c.A) / 255
	dstA := float64(dst.A) / 255
	outA := srcA + dstA*(1-srcA)
	if outA == 0 {
		img.Set(x, y, color.NRGBA{})
		return
	}
	blend := func(src, under uint8) uint8 {
		v := (float64(src)*srcA + float64(under)*dstA*(1-srcA)) / outA
		return uint8(math.Round(v))
	}
	img.Set(x, y, color.NRGBA{
		R: blend(c.R, dst.R),
		G: blend(c.G, dst.G),
		B: blend(c.B, dst.B),
		A: uint8(math.Round(outA * 255)),
	})
}
