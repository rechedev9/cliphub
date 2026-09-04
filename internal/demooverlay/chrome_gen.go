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
// 1920x1080 canvas: panel, header divider, one rounded card per roster slot
// with its avatar ring, FACEIT level ring, badge pills and stats band. Text
// stays in the ffmpeg filter graph. Flag image assets remain pending license
// approval; country codes are rendered as text badges instead.
func renderIntroChromePNG(doc Document) ([]byte, error) {
	l := DefaultLayout().Intro
	if NormalizeSource(doc.Source) == SourceFACEIT {
		l = faceitIntroLayout()
	}
	theme := ResolveTheme(doc)
	img := image.NewRGBA(image.Rect(0, 0, FrameWidth, FrameHeight))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{}}, image.Point{}, draw.Src)
	// Card chrome follows the FACEIT layout; non-FACEIT docs keep the generic
	// drawtext column (introColumn) on DefaultLayout geometry, so they only get
	// the bare panel here or the two layers would draw badges twice.
	left, right := doc.Intro.Left, doc.Intro.Right
	if NormalizeSource(doc.Source) != SourceFACEIT {
		left, right = nil, nil
	}
	drawIntroPanel(img, l.LeftPanelX, l.PanelTop, l, theme, defaultFaceitLayout.Intro, left, doc)
	drawIntroPanel(img, l.RightPanelX, l.PanelTop, l, theme, defaultFaceitLayout.Intro, right, doc)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawIntroPanel(img *image.RGBA, x, y int, layout IntroLayout, theme CardTheme, spec introLayoutSpec, cards []PlayerCard, doc Document) {
	w, h := layout.PanelWidth, layout.PanelHeight
	accent := parseOverlayColor(theme.Accent)
	soft := parseOverlayColor(theme.AccentSoft)
	chrome := spec.Chrome
	palette := defaultFaceitLayout.Palette
	panel := image.Rect(x, y, x+w, y+h)
	drawRoundedRect(img, x, y, w, h, chrome.PanelRadius, parseOverlayColor(palette.IntroPanelFill))
	drawPanelTexture(img, panel, chrome.TextureSpacing, parseOverlayColor(palette.IntroTexture))
	drawRoundedRectBorder(img, x, y, w, h, chrome.PanelRadius, chrome.PanelBorder, parseOverlayColor(palette.IntroPanelBorder))
	drawRoundedRectClipped(img, x, y, w, h, chrome.PanelRadius, withAlpha(accent, 0.92), image.Rect(x, y, x+chrome.AccentWidth, y+h))
	dividerY := y + layout.HeaderH - chrome.HeaderDivider
	drawRectOver(img, image.Rect(x+chrome.AccentWidth, dividerY, x+w, dividerY+chrome.HeaderDivider), withAlpha(soft, 0.55))

	if len(cards) > layout.MaxPlayers {
		cards = cards[:layout.MaxPlayers]
	}
	for i, card := range cards {
		cy := y + layout.HeaderH + i*layout.RowHeight
		drawIntroCard(img, x, cy, card, doc.IsPOV(card), theme, spec)
	}
}

func drawIntroCard(img *image.RGBA, panelX, cy int, card PlayerCard, isPOV bool, theme CardTheme, spec introLayoutSpec) {
	chrome := spec.Chrome
	palette := defaultFaceitLayout.Palette
	accent := parseOverlayColor(theme.Accent)
	cx, cyTop := panelX+spec.Card.X, cy+spec.Card.Y
	cw, ch := spec.Card.Width, spec.Card.Height

	drawRoundedRect(img, cx, cyTop, cw, ch, chrome.CardRadius, parseOverlayColor(palette.IntroCardFill))
	bandTop := cy + spec.Stats.BandY
	drawRoundedRectClipped(img, cx, cyTop, cw, ch, chrome.CardRadius, parseOverlayColor(palette.IntroStatsBand), image.Rect(cx, bandTop, cx+cw, cyTop+ch))
	drawRectOver(img, image.Rect(cx+chrome.CardAccentWidth, bandTop, cx+cw, bandTop+1), parseOverlayColor(palette.IntroCardDivider))
	border := parseOverlayColor(palette.IntroCardBorder)
	barAlpha := 0.5
	if isPOV {
		drawRoundedRect(img, cx, cyTop, cw, ch, chrome.CardRadius, withAlpha(accent, 0.14))
		border = withAlpha(accent, 0.7)
		barAlpha = 1
	}
	drawRoundedRectBorder(img, cx, cyTop, cw, ch, chrome.CardRadius, 1, border)
	drawRoundedRectClipped(img, cx, cyTop, cw, ch, chrome.CardRadius, withAlpha(accent, barAlpha), image.Rect(cx, cyTop, cx+chrome.CardAccentWidth, cyTop+ch))

	geo := faceitIntroCardGeometry(panelX, cy, card, isPOV)
	avatarR := float64(spec.Avatar.Width) / 2
	avatarCX := float64(geo.Avatar.X) + avatarR
	avatarCY := float64(geo.Avatar.Y) + avatarR
	drawDisc(img, avatarCX, avatarCY, avatarR, parseOverlayColor(palette.AvatarFill))
	ringAlpha := 0.7
	if isPOV {
		ringAlpha = 1
	}
	drawRing(img, avatarCX, avatarCY, avatarR, avatarR+float64(chrome.AvatarRing), withAlpha(accent, ringAlpha))

	if card.SkillLevel != nil {
		r := float64(spec.Level.Rect.Width) / 2
		lx := float64(geo.Level.X) + r
		ly := float64(geo.Level.Y) + r
		drawDisc(img, lx, ly, r, parseOverlayColor(palette.LevelRingFill))
		drawRing(img, lx, ly, r-float64(chrome.LevelRing), r, parseOverlayColor(skillFill(*card.SkillLevel)))
	}
	if card.Ranking != nil {
		rk := geo.Rank
		drawRoundedRect(img, rk.X, rk.Y, rk.Width, rk.Height, rk.Height/2, parseOverlayColor(palette.RankFill))
	}
	if card.Country != "" {
		c := geo.Country
		drawRoundedRect(img, c.X, c.Y, c.Width, c.Height, 4, color.NRGBA{R: 255, G: 255, B: 255, A: 20})
		drawRoundedRectBorder(img, c.X, c.Y, c.Width, c.Height, 4, 1, color.NRGBA{R: 255, G: 255, B: 255, A: 38})
	}
	if isPOV {
		p := geo.POV
		drawRoundedRect(img, p.X, p.Y, p.Width, p.Height, 4, parseOverlayColor(palette.POVFill))
	}
}

// faceitIntroCardGeometry resolves the frame-space rects of one FACEIT intro
// card so the chrome bitmap and the ffmpeg text filter agree on placement.
type introCardGeometry struct {
	Avatar  rectSpec
	Country rectSpec
	POV     rectSpec
	Level   rectSpec
	Rank    rectSpec
	// ELORight is the frame-space right edge shared by the ELO value and label.
	ELORight int
}

func faceitIntroCardGeometry(panelX, cy int, card PlayerCard, isPOV bool) introCardGeometry {
	spec := defaultFaceitLayout.Intro
	at := func(r rectSpec) rectSpec {
		return rectSpec{X: panelX + r.X, Y: cy + r.Y, Width: r.Width, Height: r.Height}
	}
	geo := introCardGeometry{
		Avatar:   at(spec.Avatar),
		Country:  at(spec.Country.Rect),
		POV:      at(spec.POV.Rect),
		Level:    at(spec.Level.Rect),
		Rank:     at(spec.Rank.Rect),
		ELORight: panelX + spec.Panel.Width - spec.ELO.Right,
	}
	if isPOV && strings.TrimSpace(card.Country) == "" {
		geo.POV.X = geo.Country.X
	}
	return geo
}

// renderOutroChromePNG draws the stacked FACEIT scoreboard surface: one
// rounded board per team with an accent score chip, a label band, side-tinted
// name cells and an accent-tinted POV row. All text stays in the ffmpeg filter.
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
		rows := min(spec.MaxPlayers, len(team.Players))
		boardTop := layout.HeaderY + shift + chrome.BoardTop
		boardBottom := layout.Row0 + shift + rows*layout.RowHeight + chrome.BoardBottom
		bx, by := layout.Margin+chrome.BoardLeft, boardTop
		bw, bh := FrameWidth-layout.Margin+chrome.BoardRight-bx, boardBottom-boardTop
		drawRoundedRect(img, bx, by, bw, bh, chrome.BoardRadius, parseOverlayColor(palette.OutroBoardFill))
		drawRoundedRectBorder(img, bx, by, bw, bh, chrome.BoardRadius, 1, parseOverlayColor(palette.OutroBoardBorder))
		drawRoundedRectClipped(img, bx, by, bw, bh, chrome.BoardRadius, withAlpha(accent, 0.95), image.Rect(bx, by, bx+chrome.AccentWidth, by+bh))

		score := spec.Score.Rect
		drawRoundedRect(img, layout.Margin+score.X, layout.HeaderY+shift+score.Y, score.Width, score.Height, 6, accent)

		labelTop := layout.HeaderY + shift + chrome.LabelTop
		drawRectOver(img, image.Rect(layout.Margin, labelTop, FrameWidth-layout.Margin, layout.Row0+shift+chrome.LabelBottom), parseOverlayColor(palette.OutroLabelFill))

		sideFill, sideStripe := parseOverlayColor(palette.OutroTName), parseOverlayColor(palette.OutroTStripe)
		if team.Side == "CT" {
			sideFill, sideStripe = parseOverlayColor(palette.OutroCTName), parseOverlayColor(palette.OutroCTStripe)
		}
		for row := 0; row < rows; row++ {
			card := team.Players[row]
			y := layout.Row0 + shift + row*layout.RowHeight
			rowBottom := y + layout.RowHeight - chrome.RowBottomGap
			rowFill := parseOverlayColor(palette.OutroRowEven)
			if row%2 == 1 {
				rowFill = parseOverlayColor(palette.OutroRowOdd)
			}
			drawRectOver(img, image.Rect(layout.Margin, y, FrameWidth-layout.Margin, rowBottom), rowFill)
			isPOV := doc.IsPOV(card)
			if isPOV {
				drawRectOver(img, image.Rect(layout.Margin, y, FrameWidth-layout.Margin, rowBottom), withAlpha(accent, 0.16))
			}
			drawRectOver(img, image.Rect(layout.Margin, y, layout.Margin+layout.NameWidth-chrome.NameRightGap, rowBottom), sideFill)
			stripe := sideStripe
			if isPOV {
				stripe = accent
			}
			drawRectOver(img, image.Rect(layout.Margin, y, layout.Margin+chrome.SideStripeWidth, rowBottom), stripe)
			if isPOV {
				p := spec.POV.Rect
				drawRoundedRect(img, layout.Margin+p.X, y+p.Y, p.Width, p.Height, 4, parseOverlayColor(palette.POVFill))
			}
			drawRectOver(img, image.Rect(layout.Margin, rowBottom-chrome.DividerHeight, FrameWidth-layout.Margin, rowBottom), parseOverlayColor(palette.OutroDivider))
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

// roundedRectDistance is the signed distance from a pixel center to the edge
// of a rounded rectangle: negative inside, positive outside.
func roundedRectDistance(px, py float64, x, y, w, h, radius int) float64 {
	r := math.Min(float64(radius), math.Min(float64(w), float64(h))/2)
	cx := float64(x) + float64(w)/2
	cy := float64(y) + float64(h)/2
	hx := float64(w)/2 - r
	hy := float64(h)/2 - r
	dx := math.Abs(px-cx) - hx
	dy := math.Abs(py-cy) - hy
	outside := math.Hypot(math.Max(dx, 0), math.Max(dy, 0))
	inside := math.Min(math.Max(dx, dy), 0)
	return outside + inside - r
}

func coverage(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func drawRoundedRect(img *image.RGBA, x, y, w, h, radius int, c color.NRGBA) {
	drawRoundedRectClipped(img, x, y, w, h, radius, c, image.Rect(x, y, x+w, y+h))
}

// drawRoundedRectClipped fills an anti-aliased rounded rectangle but only
// paints pixels inside clip, so accent bars and bands inherit the corners.
func drawRoundedRectClipped(img *image.RGBA, x, y, w, h, radius int, c color.NRGBA, clip image.Rectangle) {
	if w <= 0 || h <= 0 || c.A == 0 {
		return
	}
	bounds := image.Rect(x, y, x+w, y+h).Intersect(clip).Intersect(img.Bounds())
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			cov := coverage(0.5 - roundedRectDistance(float64(px)+0.5, float64(py)+0.5, x, y, w, h, radius))
			if cov <= 0 {
				continue
			}
			blendPixel(img, px, py, scaleAlpha(c, cov))
		}
	}
}

func drawRoundedRectBorder(img *image.RGBA, x, y, w, h, radius, thickness int, c color.NRGBA) {
	if w <= 0 || h <= 0 || thickness <= 0 || c.A == 0 {
		return
	}
	bounds := image.Rect(x, y, x+w, y+h).Intersect(img.Bounds())
	t := float64(thickness)
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			d := roundedRectDistance(float64(px)+0.5, float64(py)+0.5, x, y, w, h, radius)
			cov := coverage(0.5-d) - coverage(0.5-(d+t))
			if cov <= 0 {
				continue
			}
			blendPixel(img, px, py, scaleAlpha(c, cov))
		}
	}
}

func drawDisc(img *image.RGBA, cx, cy, r float64, c color.NRGBA) {
	drawRing(img, cx, cy, 0, r, c)
}

// drawRing fills the anti-aliased annulus between inner and outer radius.
func drawRing(img *image.RGBA, cx, cy, inner, outer float64, c color.NRGBA) {
	if outer <= 0 || c.A == 0 {
		return
	}
	bounds := image.Rect(int(math.Floor(cx-outer))-1, int(math.Floor(cy-outer))-1, int(math.Ceil(cx+outer))+1, int(math.Ceil(cy+outer))+1).Intersect(img.Bounds())
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			d := math.Hypot(float64(px)+0.5-cx, float64(py)+0.5-cy)
			cov := coverage(outer + 0.5 - d)
			if inner > 0 {
				cov = math.Min(cov, coverage(d-inner+0.5))
			}
			if cov <= 0 {
				continue
			}
			blendPixel(img, px, py, scaleAlpha(c, cov))
		}
	}
}

func scaleAlpha(c color.NRGBA, factor float64) color.NRGBA {
	c.A = uint8(math.Round(float64(c.A) * factor))
	return c
}

func blendPixel(img *image.RGBA, x, y int, c color.NRGBA) {
	if !image.Pt(x, y).In(img.Bounds()) || c.A == 0 {
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
