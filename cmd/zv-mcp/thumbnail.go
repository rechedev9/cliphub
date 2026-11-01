package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	thumbWidth   = 640
	thumbHeight  = 360
	thumbQuality = 80
)

// thumbBackground stands in for gameplay behind the transparent HUD preview.
var thumbBackground = color.RGBA{R: 0x1a, G: 0x1f, B: 0x26, A: 0xff}

// hudThumbnail turns a full-size transparent WebP preview into a small JPEG,
// so a dozen of them stay well inside a client's tool-result budget.
func hudThumbnail(webpData []byte) ([]byte, error) {
	src, err := webp.Decode(bytes.NewReader(webpData))
	if err != nil {
		return nil, err
	}
	dst := image.NewRGBA(image.Rect(0, 0, thumbWidth, thumbHeight))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(thumbBackground), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: thumbQuality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
