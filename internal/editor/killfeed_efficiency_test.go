package editor

import (
	"image"
	"image/color"
	"image/draw"
	"math/rand"
	"reflect"
	"testing"
)

// Hides concrete pixel access while retaining the exact same image contents.
type genericImage struct{ image.Image }

func TestRedPixelFastPathsMatchColorInterface(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	bounds := image.Rect(7, 11, 47, 41)
	rgba := image.NewRGBA(bounds)
	nrgba := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.SetRGBA(x, y, color.RGBA{uint8(rng.Intn(256)), uint8(rng.Intn(256)), uint8(rng.Intn(256)), uint8(rng.Intn(256))})
			nrgba.SetNRGBA(x, y, color.NRGBA{uint8(rng.Intn(256)), uint8(rng.Intn(256)), uint8(rng.Intn(256)), uint8(rng.Intn(256))})
		}
	}
	region := image.Rect(10, 14, 43, 38)
	for _, frame := range []image.Image{rgba, nrgba, rgba.SubImage(region), nrgba.SubImage(region), image.NewGray(bounds)} {
		for _, threshold := range [][2]uint32{{150, 55}, {120, 70}, {0, 255}, {255, 0}, {128, 128}} {
			actual := redPixelMatcher(frame, threshold[0], threshold[1])
			expected := redPixelMatcher(genericImage{frame}, threshold[0], threshold[1])
			for y := bounds.Min.Y - 1; y <= bounds.Max.Y; y++ {
				for x := bounds.Min.X - 1; x <= bounds.Max.X; x++ {
					if actual(x, y) != expected(x, y) {
						t.Fatalf("%T pixel %d,%d", frame, x, y)
					}
				}
			}
			got := redComponents(frame, region, threshold[0], threshold[1])
			want := legacyRedComponents(frame, region, threshold[0], threshold[1])
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%T components differ", frame)
			}
			gotBounds, gotCount := redPixelBounds(frame, region, threshold[0], threshold[1])
			wantBounds, wantCount := redPixelBounds(genericImage{frame}, region, threshold[0], threshold[1])
			if gotBounds != wantBounds || gotCount != wantCount {
				t.Fatalf("%T bounds differ", frame)
			}
		}
	}
}

// Previous two-grid, interface-per-pixel implementation retained only as an oracle.
func legacyRedComponents(frame image.Image, region image.Rectangle, minRed, maxGreenBlue uint32) []redComponent {
	w, h := region.Dx(), region.Dy()
	if w <= 0 || h <= 0 {
		return nil
	}
	red := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := frame.At(region.Min.X+x, region.Min.Y+y).RGBA()
			red[y*w+x] = r>>8 > minRed && g>>8 < maxGreenBlue && b>>8 < maxGreenBlue
		}
	}
	visited := make([]bool, w*h)
	var comps []redComponent
	queue := make([]int, 0, 64)
	for start := 0; start < w*h; start++ {
		if !red[start] || visited[start] {
			continue
		}
		visited[start] = true
		queue = queue[:0]
		queue = append(queue, start)
		sx, sy := start%w, start/w
		minX, minY, maxX, maxY := sx, sy, sx, sy
		count := 0
		for len(queue) > 0 {
			idx := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			count++
			cx, cy := idx%w, idx/w
			minX = min(minX, cx)
			minY = min(minY, cy)
			maxX = max(maxX, cx)
			maxY = max(maxY, cy)
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := cx+dx, cy+dy
					if nx < 0 || nx >= w || ny < 0 || ny >= h {
						continue
					}
					nidx := ny*w + nx
					if red[nidx] && !visited[nidx] {
						visited[nidx] = true
						queue = append(queue, nidx)
					}
				}
			}
		}
		comps = append(comps, redComponent{image.Rect(region.Min.X+minX, region.Min.Y+minY, region.Min.X+maxX+1, region.Min.Y+maxY+1), count})
	}
	return comps
}

func BenchmarkKillfeedComponents(b *testing.B) {
	frame := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	draw.Draw(frame, image.Rect(1450, 30, 1880, 32), image.NewUniform(color.RGBA{255, 0, 0, 255}), image.Point{}, draw.Src)
	region := image.Rect(1152, 0, 1920, 324)
	for _, tc := range []struct {
		name string
		run  func(image.Image, image.Rectangle, uint32, uint32) []redComponent
	}{
		{"legacy", legacyRedComponents}, {"indexed", redComponents},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = tc.run(frame, region, 150, 55)
			}
		})
	}
}
