package demooverlay

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// TestMeasurePlateGeometry prints row geometry from real plates; run with:
// go test ./internal/demooverlay -run TestMeasurePlateGeometry -count=1 -v
func TestMeasurePlateGeometry(t *testing.T) {
	if os.Getenv("MEASURE_PLATES") == "" {
		t.Skip("set MEASURE_PLATES=1 to dump plate row geometry")
	}
	dir := filepath.Join("..", "..", "data", "overlay-assets", "plates")
	for _, name := range []string{
		"professional-intro.jpg", "premier-intro.jpg",
		"professional-outro.jpg", "premier-outro.jpg",
	} {
		path := filepath.Join(dir, name)
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		img, _, err := image.Decode(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("\n=== %s %dx%d ===\n", name, img.Bounds().Dx(), img.Bounds().Dy())
		for _, sliceX := range []int{img.Bounds().Dx() / 4, img.Bounds().Dx() * 3 / 4} {
			fmt.Printf("  slice x=%d\n", sliceX)
			bands := detectHorizontalBands(img, sliceX)
			for i, b := range bands {
				fmt.Printf("    band %d src y=%d..%d -> frame y=%d..%d center=%d\n",
					i, b.y0, b.y1, mapPlateY(b.y0, img.Bounds()), mapPlateY(b.y1, img.Bounds()), mapPlateY((b.y0+b.y1)/2, img.Bounds()))
			}
		}
	}
}

type yBand struct{ y0, y1 int }

func mapPlateY(srcY int, bounds image.Rectangle) int {
	sw, sh := float64(bounds.Dx()), float64(bounds.Dy())
	scale := float64(FrameWidth) / sw
	if float64(FrameHeight)/sh > scale {
		scale = float64(FrameHeight) / sh
	}
	scaledH := sh * scale
	offY := (scaledH - float64(FrameHeight)) / 2
	return int(float64(srcY)*scale - offY + 0.5)
}

func detectHorizontalBands(img image.Image, x int) []yBand {
	b := img.Bounds()
	x0 := max(b.Min.X, x-8)
	x1 := min(b.Max.X, x+8)
	rowLum := make([]int, b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		sum := 0
		n := 0
		for x := x0; x < x1; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			sum += (299*int(r>>8) + 587*int(g>>8) + 114*int(bl>>8)) / 1000
			n++
		}
		rowLum[y-b.Min.Y] = sum / n
	}
	// Divider lines show up as local minima in row luminance.
	var dividers []int
	for i := 2; i < len(rowLum)-2; i++ {
		if rowLum[i] < rowLum[i-1]-8 && rowLum[i] < rowLum[i+1]-8 && rowLum[i] < 80 {
			dividers = append(dividers, i+b.Min.Y)
		}
	}
	if len(dividers) == 0 {
		return detectDarkBands(img, x)
	}
	// cluster nearby dividers
	var edges []int
	for i, d := range dividers {
		if i == 0 || d-edges[len(edges)-1] > 4 {
			edges = append(edges, d)
		}
	}
	start := b.Min.Y
	var bands []yBand
	for _, edge := range edges {
		if edge-start > 12 {
			bands = append(bands, yBand{start, edge})
		}
		start = edge
	}
	if b.Max.Y-start > 12 {
		bands = append(bands, yBand{start, b.Max.Y})
	}
	return bands
}

func detectDarkBands(img image.Image, x int) []yBand {
	b := img.Bounds()
	var edges []int
	prevDark := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		r, g, bl, _ := img.At(x, y).RGBA()
		lum := (299*int(r>>8) + 587*int(g>>8) + 114*int(bl>>8)) / 1000
		dark := lum < 55
		if dark && !prevDark {
			edges = append(edges, y)
		}
		if !dark && prevDark {
			edges = append(edges, y)
		}
		prevDark = dark
	}
	if len(edges)%2 == 1 {
		edges = append(edges, b.Max.Y)
	}
	var bands []yBand
	for i := 0; i+1 < len(edges); i += 2 {
		y0, y1 := edges[i], edges[i+1]
		if y1-y0 < 8 {
			continue
		}
		bands = append(bands, yBand{y0, y1})
	}
	return bands
}
