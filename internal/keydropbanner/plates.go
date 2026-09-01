package keydropbanner

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

// generateFamilyPlate paints a family-owned PNG that never shares bytes with
// the bundled KeyDrop art. The code bay uses the style's Cover* fractions so
// CompositeWithCode covers the same rectangle it later draws CODE into.
func generateFamilyPlate(style Style, bg, stripe, bay color.RGBA) []byte {
	w, h := style.Width, style.Height
	if w <= 0 || h <= 0 {
		return nil
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
	stripeH := h / 7
	if stripeH < 24 {
		stripeH = 24
	}
	draw.Draw(img, image.Rect(0, 0, w, stripeH), &image.Uniform{C: stripe}, image.Point{}, draw.Src)
	bayX := int(math.Round(style.CoverX * float64(w)))
	bayY := int(math.Round(style.CoverY * float64(h)))
	bayW := int(math.Round(style.CoverW * float64(w)))
	bayH := int(math.Round(style.CoverH * float64(h)))
	draw.Draw(img, image.Rect(bayX, bayY, bayX+bayW, bayY+bayH), &image.Uniform{C: bay}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func registerGeneratedFamilyPlates() {
	classic := Style{
		Family:       FamilyCSGOSkins,
		ID:           StyleClassic,
		FileName:     "csgoskins-classic.png",
		Width:        1080,
		Height:       475,
		CoverX:       0.18,
		CoverY:       0.54,
		CoverW:       0.64,
		CoverH:       0.22,
		CoverColor:   "0x061614",
		TextCenterY:  0.65,
		FontSizeFrac: 0.12,
	}
	classic.Data = generateFamilyPlate(classic, color.RGBA{R: 8, G: 36, B: 32, A: 255}, color.RGBA{R: 20, G: 168, B: 132, A: 255}, color.RGBA{R: 6, G: 22, B: 20, A: 255})
	styles[catalogKey(FamilyCSGOSkins, StyleClassic)] = classic

	operator := Style{
		Family:       FamilyCSGOSkins,
		ID:           StyleOperator,
		FileName:     "csgoskins-operator.png",
		Width:        1080,
		Height:       722,
		CoverX:       0.28,
		CoverY:       0.442,
		CoverW:       0.62,
		CoverH:       0.148,
		CoverColor:   "0x081014",
		TextCenterY:  0.516,
		FontSizeFrac: 0.095,
	}
	operator.Data = generateFamilyPlate(operator, color.RGBA{R: 6, G: 22, B: 28, A: 255}, color.RGBA{R: 14, G: 120, B: 140, A: 255}, color.RGBA{R: 8, G: 16, B: 20, A: 255})
	styles[catalogKey(FamilyCSGOSkins, StyleOperator)] = operator
}
