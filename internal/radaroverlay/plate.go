package radaroverlay

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
)

// The 2D Replay's plate colour, opaque: the radar's opacity alone decides how much of the scene shows.
var plateInk = color.NRGBA{R: 0x0b, G: 0x0c, B: 0x0e, A: 0xff}

const (
	cornerShare = 0.06
	rimAlpha    = 0.22
)

// Plate is the radar's still layer: the map art on a dark rounded square, side pixels wide, with
// every pixel's alpha scaled by opacity. Art may be nil for a map the bundle has no picture of.
func Plate(art image.Image, side int, opacity float64) *image.NRGBA {
	r := image.Rect(0, 0, side, side)
	canvas := image.NewRGBA(r)
	xdraw.Draw(canvas, r, image.NewUniform(plateInk), image.Point{}, xdraw.Src)
	if art != nil {
		xdraw.CatmullRom.Scale(canvas, r, art, art.Bounds(), xdraw.Over, nil)
	}
	out := image.NewNRGBA(r)
	xdraw.Draw(out, r, canvas, image.Point{}, xdraw.Src)
	radius := math.Max(4, float64(side)*cornerShare)
	for y := range side {
		for x := range side {
			inside := roundedCoverage(float64(x)+0.5, float64(y)+0.5, float64(side), radius)
			c := out.NRGBAAt(x, y)
			if inside <= 0 {
				out.SetNRGBA(x, y, color.NRGBA{})
				continue
			}
			// A faint light rim along the edge separates the plate from a dark scene.
			if rim := rimCoverage(float64(x)+0.5, float64(y)+0.5, float64(side), radius); rim > 0 {
				c = mixWhite(c, rim*rimAlpha)
			}
			c.A = uint8(math.Round(float64(c.A) * inside * opacity))
			out.SetNRGBA(x, y, c)
		}
	}
	return out
}

// edgeDistance is how far (x, y) lies inside a side x side square with corners rounded by radius.
func edgeDistance(x, y, side, radius float64) float64 {
	cx := math.Max(radius-x, x-(side-radius))
	cy := math.Max(radius-y, y-(side-radius))
	if cx > 0 && cy > 0 {
		return radius - math.Hypot(cx, cy)
	}
	return math.Min(math.Min(x, side-x), math.Min(y, side-y))
}

func roundedCoverage(x, y, side, radius float64) float64 {
	return math.Max(0, math.Min(1, edgeDistance(x, y, side, radius)+0.5))
}

func rimCoverage(x, y, side, radius float64) float64 {
	d := edgeDistance(x, y, side, radius)
	return math.Max(0, math.Min(1, 2-d))
}

func mixWhite(c color.NRGBA, f float64) color.NRGBA {
	mix := func(v uint8) uint8 { return uint8(math.Round(float64(v) + (255-float64(v))*f)) }
	return color.NRGBA{R: mix(c.R), G: mix(c.G), B: mix(c.B), A: max(c.A, uint8(math.Round(255*f)))}
}

// WritePlate decodes the PNG map art (empty for none), bakes the plate and writes it as a PNG at path.
func WritePlate(path string, art []byte, side int, opacity float64) error {
	var decoded image.Image
	if len(art) > 0 {
		img, err := png.Decode(bytes.NewReader(art))
		if err != nil {
			return fmt.Errorf("decode radar art: %w", err)
		}
		decoded = img
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(out, Plate(decoded, side, opacity)); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
