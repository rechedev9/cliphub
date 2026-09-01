package demooverlay

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"math"
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
		"professional-intro.jpg", "premier-intro.jpg", "faceit-intro.jpg",
		"professional-outro.jpg", "premier-outro.jpg", "faceit-outro.jpg",
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

// TestMeasureFaceitIntroCircles dumps avatar circle centers from the real FACEIT intro plate.
func TestMeasureFaceitIntroCircles(t *testing.T) {
	if os.Getenv("MEASURE_PLATES") == "" {
		t.Skip("set MEASURE_PLATES=1 to dump plate row geometry")
	}
	path := filepath.Join("..", "..", "data", "overlay-assets", "plates", "faceit-intro.jpg")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(f)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	l := DefaultLayout()
	centers := detectIntroAvatarCircleCenters(img, l.Intro.LeftPanelCropX)
	fmt.Printf("\n=== faceit-intro avatar circles (frame Y) ===\n")
	for i, cy := range centers {
		fmt.Printf("  row %d center=%d\n", i, cy)
	}
	// 2D ring search in left panel for calibration.
	b := img.Bounds()
	for _, wantFrameY := range []int{276, 437, 598, 764, 929} {
		srcY := unmapPlateY(wantFrameY, b)
		bestX, bestY, best := 0, 0, 0
		for x := b.Min.X + 20; x < b.Min.X+180; x++ {
			for y := srcY - 25; y <= srcY+25; y++ {
				s := ringScore(img, x, y, 17)
				if s > best {
					bestX, bestY, best = x, y, s
				}
			}
		}
		fmt.Printf("  near frame %d: best src (%d,%d) score=%d -> frame y=%d\n",
			wantFrameY, bestX, bestY, best, mapPlateY(bestY, b))
	}
}

func ringScore(img image.Image, cx, cy, r int) int {
	score := 0
	for deg := 0; deg < 360; deg += 15 {
		rad := float64(deg) * 3.14159265 / 180
		x := cx + int(float64(r)*math.Cos(rad)+0.5)
		y := cy + int(float64(r)*math.Sin(rad)+0.5)
		if sampleLum(img, x, y) < 90 {
			score++
		}
	}
	if sampleLum(img, cx, cy) > 120 {
		score /= 2
	}
	return score
}

func unmapPlateY(frameY int, bounds image.Rectangle) int {
	sw, sh := float64(bounds.Dx()), float64(bounds.Dy())
	scale := float64(FrameWidth) / sw
	if float64(FrameHeight)/sh > scale {
		scale = float64(FrameHeight) / sh
	}
	scaledH := sh * scale
	offY := (scaledH - float64(FrameHeight)) / 2
	return int((float64(frameY)+offY)/scale + 0.5)
}

// detectIntroAvatarCircleCenters finds avatar ring centers in the left intro panel column.
func detectIntroAvatarCircleCenters(img image.Image, panelLeftFrameX int) []int {
	b := img.Bounds()
	avatarCX := panelLeftFrameX + DefaultLayout().Intro.AvatarXOff + DefaultLayout().Intro.AvatarSize/2
	srcCX := unmapPlateX(avatarCX, b)
	rSrc := 17
	y0 := b.Min.Y + 20
	y1 := b.Max.Y - 20
	type peak struct {
		y     int
		score int
	}
	var peaks []peak
	minSep := 55 // src pixels between row circles
	for y := y0; y < y1; y++ {
		score := darkDiskScore(img, srcCX, y, rSrc)
		if score < 120 {
			continue
		}
		if len(peaks) == 0 || y-peaks[len(peaks)-1].y >= minSep {
			peaks = append(peaks, peak{y, score})
			continue
		}
		if score > peaks[len(peaks)-1].score {
			peaks[len(peaks)-1] = peak{y, score}
		}
	}
	var out []int
	for _, p := range peaks {
		out = append(out, mapPlateY(p.y, b))
	}
	return out
}

func darkDiskScore(img image.Image, cx, cy, r int) int {
	score := 0
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy > r*r {
				continue
			}
			if sampleLum(img, cx+dx, cy+dy) < 55 {
				score++
			}
		}
	}
	return score
}

func sampleLum(img image.Image, x, y int) int {
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return 255
	}
	r, g, bl, _ := img.At(x, y).RGBA()
	return (299*int(r>>8) + 587*int(g>>8) + 114*int(bl>>8)) / 1000
}

func unmapPlateX(frameX int, bounds image.Rectangle) int {
	sw, sh := float64(bounds.Dx()), float64(bounds.Dy())
	scale := float64(FrameWidth) / sw
	if float64(FrameHeight)/sh > scale {
	 scale = float64(FrameHeight) / sh
	}
	scaledW := sw * scale
	offX := (scaledW - float64(FrameWidth)) / 2
	return int((float64(frameX)+offX)/scale + 0.5)
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

func mapPlateX(srcX int, bounds image.Rectangle) int {
	sw, sh := float64(bounds.Dx()), float64(bounds.Dy())
	scale := float64(FrameWidth) / sw
	if float64(FrameHeight)/sh > scale {
		scale = float64(FrameHeight) / sh
	}
	scaledW := sw * scale
	offX := (scaledW - float64(FrameWidth)) / 2
	return int(float64(srcX)*scale - offX + 0.5)
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
