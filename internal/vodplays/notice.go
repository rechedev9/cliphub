package vodplays

import (
	"image"
	"sort"
)

// A kill notice reads "killer [+ assister] weapon icons victim". Player
// names are drawn in their team colour; the plus, weapon and icons are white.
const (
	textMinBright = 95
	// The notice plate is translucent, so text is bright relative to the
	// plate behind it rather than in absolute terms.
	textAbovePlate  = 45
	minSegmentWidth = 3
	nameMinSat      = 0.3
	segmentMinGap   = 5
	ringInset       = 3
	minNamePixels   = 12
	maskMatchIoU    = 0.8
	maskWidthSlack  = 3
	maskShiftSearch = 1
)

// NameMask is the shape of one player name: which pixels of its box are text.
type NameMask struct {
	W, H int
	Bits []bool
}

// readNames returns the player names inside a highlighted notice, left to
// right, as shapes. The same name drawn again matches itself whatever its
// team colour, so no text has to be read.
func readNames(rgb []byte, ring image.Rectangle) []NameMask {
	inner := ring.Inset(ringInset).Intersect(image.Rect(0, 0, FeedWidth, FeedHeight))
	if inner.Empty() {
		return nil
	}
	w, h := inner.Dx(), inner.Dy()
	threshold := max(textMinBright, plateLevel(rgb, inner)+textAbovePlate)
	bright := make([]bool, w*h)
	saturated := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := ((inner.Min.Y+y)*FeedWidth + inner.Min.X + x) * 3
			r, g, b := int(rgb[i]), int(rgb[i+1]), int(rgb[i+2])
			hi, lo := max(r, g, b), min(r, g, b)
			// The highlight border and its blur are red; names never are.
			if hi < threshold || (r*5 > g*8 && r*5 > b*8) {
				continue
			}
			bright[y*w+x] = true
			saturated[y*w+x] = float64(hi-lo)/float64(hi) >= nameMinSat
		}
	}
	var names []NameMask
	for _, seg := range textSegments(bright, w, h) {
		total, sat := 0, 0
		for y := 0; y < h; y++ {
			for x := seg[0]; x < seg[1]; x++ {
				if bright[y*w+x] {
					total++
					if saturated[y*w+x] {
						sat++
					}
				}
			}
		}
		if seg[1]-seg[0] >= minSegmentWidth && total >= minNamePixels && sat*2 >= total {
			mask := cutMask(bright, w, h, seg)
			// A name is far shorter than its notice; a taller shape has border
			// or scenery in it, and the whole reading is unreliable.
			if mask.H*2 > ring.Dy() {
				return nil
			}
			names = append(names, mask)
		}
	}
	return names
}

// plateLevel is the brightness of the notice plate: most of a notice is
// plate, so a low percentile of its pixels lands on it.
func plateLevel(rgb []byte, inner image.Rectangle) int {
	levels := make([]int, 0, inner.Dx()*inner.Dy())
	for y := inner.Min.Y; y < inner.Max.Y; y++ {
		for x := inner.Min.X; x < inner.Max.X; x++ {
			i := (y*FeedWidth + x) * 3
			levels = append(levels, max(int(rgb[i]), int(rgb[i+1]), int(rgb[i+2])))
		}
	}
	sort.Ints(levels)
	return levels[len(levels)*3/10]
}

// textSegments splits the columns holding text into runs separated by at
// least segmentMinGap empty columns; letters of one word sit closer.
func textSegments(bright []bool, w, h int) [][2]int {
	var segs [][2]int
	start, gap := -1, 0
	for x := 0; x <= w; x++ {
		filled := false
		for y := 0; x < w && y < h; y++ {
			if bright[y*w+x] {
				filled = true
				break
			}
		}
		switch {
		case filled && start < 0:
			start, gap = x, 0
		case filled:
			gap = 0
		case start >= 0:
			gap++
			if gap >= segmentMinGap || x == w {
				segs = append(segs, [2]int{start, x - gap + 1})
				start = -1
			}
		}
	}
	return segs
}

func cutMask(bright []bool, w, h int, seg [2]int) NameMask {
	top, bottom := h, -1
	for y := 0; y < h; y++ {
		for x := seg[0]; x < seg[1]; x++ {
			if bright[y*w+x] {
				top, bottom = min(top, y), max(bottom, y)
			}
		}
	}
	mask := NameMask{W: seg[1] - seg[0], H: bottom - top + 1}
	mask.Bits = make([]bool, mask.W*mask.H)
	for y := 0; y < mask.H; y++ {
		for x := 0; x < mask.W; x++ {
			mask.Bits[y*mask.W+x] = bright[(top+y)*w+seg[0]+x]
		}
	}
	return mask
}

func maskPixels(names []NameMask) int {
	total := 0
	for _, n := range names {
		for _, on := range n.Bits {
			if on {
				total++
			}
		}
	}
	return total
}

// sameName compares two name shapes, allowing a pixel of shift either way.
func sameName(a, b NameMask) bool {
	return nameSimilarity(a, b) >= maskMatchIoU
}

func nameSimilarity(a, b NameMask) float64 {
	if absInt(a.W-b.W) > maskWidthSlack || absInt(a.H-b.H) > maskWidthSlack {
		return 0
	}
	best := 0.0
	for dy := -maskShiftSearch; dy <= maskShiftSearch; dy++ {
		for dx := -maskShiftSearch; dx <= maskShiftSearch; dx++ {
			inter, union := 0, 0
			for y := 0; y < max(a.H, b.H); y++ {
				for x := 0; x < max(a.W, b.W); x++ {
					pa := a.at(x, y)
					pb := b.at(x+dx, y+dy)
					if pa && pb {
						inter++
					}
					if pa || pb {
						union++
					}
				}
			}
			if union > 0 {
				best = max(best, float64(inter)/float64(union))
			}
		}
	}
	return best
}

func (m NameMask) at(x, y int) bool {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return false
	}
	return m.Bits[y*m.W+x]
}
