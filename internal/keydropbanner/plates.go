package keydropbanner

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

// generateFamilyPlate paints a family-owned PNG that never shares bytes with
// the bundled KeyDrop art. CompositeWithCode covers the bay and draws CODE.
func generateFamilyPlate(w, h int, bg, stripe, bay color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
	stripeH := h / 7
	if stripeH < 24 {
		stripeH = 24
	}
	draw.Draw(img, image.Rect(0, 0, w, stripeH), &image.Uniform{C: stripe}, image.Point{}, draw.Src)
	bayX := int(float64(w) * 0.18)
	bayY := int(float64(h) * 0.54)
	bayW := int(float64(w) * 0.64)
	bayH := int(float64(h) * 0.22)
	draw.Draw(img, image.Rect(bayX, bayY, bayX+bayW, bayY+bayH), &image.Uniform{C: bay}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func registerGeneratedFamilyPlates() {
	styles[catalogKey(FamilyCSGOSkins, StyleClassic)] = Style{
		Family:       FamilyCSGOSkins,
		ID:           StyleClassic,
		FileName:     "csgoskins-classic.png",
		Data:         generateFamilyPlate(1080, 475, color.RGBA{R: 8, G: 36, B: 32, A: 255}, color.RGBA{R: 20, G: 168, B: 132, A: 255}, color.RGBA{R: 6, G: 22, B: 20, A: 255}),
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
	styles[catalogKey(FamilyCSGOSkins, StyleOperator)] = Style{
		Family:       FamilyCSGOSkins,
		ID:           StyleOperator,
		FileName:     "csgoskins-operator.png",
		Data:         generateFamilyPlate(1080, 722, color.RGBA{R: 6, G: 22, B: 28, A: 255}, color.RGBA{R: 14, G: 120, B: 140, A: 255}, color.RGBA{R: 8, G: 16, B: 20, A: 255}),
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
}
