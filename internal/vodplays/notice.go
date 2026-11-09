package vodplays

import (
	"image"
	"sort"
)

// A notice reads "[tag] killer + [tag] assister weapon icons [tag] victim";
// names and clan tags are in team colours, the plus, weapon and icons white.
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
	// Blur (under minRunPixels) and thin strokes such as a tag's greyed
	// bracket cannot tell a name from a mark, so they take their word's.
	minRunPixels  = 5
	maxThinRun    = 4
	minMarkPixels = 10
	// Marks are white with grey edges; thin coloured strokes over a bright
	// background lose some colour, but far less than this.
	maxMarkSatShare = 0.25
	// Rows of text further apart than this are not one word: the border's
	// blur or scenery above or below the text.
	maxRowGap       = 2
	maskMatchIoU    = 0.8
	maskWidthSlack  = 3
	maskShiftSearch = 1
)

// NameMask is the shape of one player name: which pixels of its box are text.
type NameMask struct {
	W, H int
	Bits []bool
}

// Reading is what a highlighted notice says: every coloured word, left to
// right, and the names in the killer and assister slots (zero when absent).
type Reading struct {
	Names    []NameMask
	Killer   NameMask
	Assister NameMask
}

// A token's colour is unknown while it holds only thin or faint runs.
type tokenClass int

const (
	classUnknown tokenClass = iota
	className
	classMark
)

type token struct {
	from, to   int
	class      tokenClass
	total, sat int
}

// textMap marks the text pixels inside a notice, and which of them are coloured.
type textMap struct {
	w, h      int
	bright    []bool
	saturated []bool
}

// readNotice reads a notice's words as shapes. A name is the word right before
// a white mark (the plus, the weapon), never a clan tag, which precedes it.
func readNotice(rgb []byte, ring image.Rectangle) Reading {
	inner := ring.Inset(ringInset).Intersect(image.Rect(0, 0, FeedWidth, FeedHeight))
	if inner.Empty() {
		return Reading{}
	}
	w, h := inner.Dx(), inner.Dy()
	threshold := max(textMinBright, plateLevel(rgb, inner)+textAbovePlate)
	m := textMap{w: w, h: h, bright: make([]bool, w*h), saturated: make([]bool, w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := ((inner.Min.Y+y)*FeedWidth + inner.Min.X + x) * 3
			r, g, b := int(rgb[i]), int(rgb[i+1]), int(rgb[i+2])
			hi, lo := max(r, g, b), min(r, g, b)
			// The highlight border and its blur are red; names never are.
			if hi < threshold || (r*5 > g*8 && r*5 > b*8) {
				continue
			}
			m.bright[y*w+x] = true
			m.saturated[y*w+x] = float64(hi-lo)/float64(hi) >= nameMinSat
		}
	}
	// A line of the border's blur would join every word it runs over.
	top, bottom := m.band(0, w)
	for y := 0; y < h; y++ {
		if y < top || y > bottom {
			clear(m.bright[y*w : (y+1)*w])
		}
	}
	var reading Reading
	slots, last := 0, -1
	afterName := false
	for _, t := range m.tokens() {
		if t.class == classMark {
			// The first mark after a word closes a slot; the marks of one
			// group (weapon and icons, plus and flash) close only one.
			if afterName && slots < 2 {
				if slots == 0 {
					reading.Killer = reading.Names[last]
				} else {
					reading.Assister = reading.Names[last]
				}
				slots++
			}
			afterName = false
			continue
		}
		mask := m.cut(t.from, t.to)
		// A name is far shorter than its notice; a taller shape has scenery
		// in it and stays an unread word, so no other word takes its slot.
		if mask.H*2 > ring.Dy() {
			mask = NameMask{}
		}
		reading.Names = append(reading.Names, mask)
		last, afterName = len(reading.Names)-1, true
	}
	return reading
}

// tokens splits a notice into coloured words and white marks. A white run next
// to a coloured one starts a new token, so the plus never joins a near tag.
func (m textMap) tokens() []token {
	var tokens []token
	add := func(from, to int) {
		total, sat := m.pixels(from, to)
		class := classUnknown
		if total >= minRunPixels && to-from > maxThinRun {
			class = classOf(total, sat)
		}
		n := len(tokens)
		if n == 0 || from-tokens[n-1].to >= segmentMinGap {
			tokens = append(tokens, token{from: from, to: to, class: class, total: total, sat: sat})
			return
		}
		last := &tokens[n-1]
		switch {
		case class == classUnknown || class == last.class:
		case last.class == classUnknown:
			last.class = class
		default:
			tokens = append(tokens, token{from: from, to: to, class: class, total: total, sat: sat})
			return
		}
		last.to, last.total, last.sat = to, last.total+total, last.sat+sat
	}
	start := -1
	for x := 0; x <= m.w; x++ {
		filled := false
		for y := 0; x < m.w && y < m.h; y++ {
			if m.bright[y*m.w+x] {
				filled = true
				break
			}
		}
		if filled && start < 0 {
			start = x
		}
		if !filled && start >= 0 {
			add(start, x)
			start = -1
		}
	}
	kept := tokens[:0]
	for _, t := range tokens {
		if t.class == classUnknown {
			t.class = classOf(t.total, t.sat)
		}
		if (t.class == classMark && t.total >= minMarkPixels) || (t.class == className && t.to-t.from >= minSegmentWidth && t.total >= minNamePixels) {
			kept = append(kept, t)
		}
	}
	return kept
}

func classOf(total, sat int) tokenClass {
	if float64(sat) < maxMarkSatShare*float64(total) {
		return classMark
	}
	return className
}

// pixels counts the text pixels between the columns from and to, and how many
// of them are coloured.
func (m textMap) pixels(from, to int) (int, int) {
	total, sat := 0, 0
	for y := 0; y < m.h; y++ {
		for x := from; x < to; x++ {
			if m.bright[y*m.w+x] {
				total++
				if m.saturated[y*m.w+x] {
					sat++
				}
			}
		}
	}
	return total, sat
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

// band returns the rows holding most of the text between the columns from and
// to, leaving out stray pixels above or below; top is -1 without text.
func (m textMap) band(from, to int) (int, int) {
	top, bottom := -1, -1
	bandTop, bandPixels, bestPixels, gap := -1, 0, 0, 0
	for y := 0; y < m.h; y++ {
		row := 0
		for x := from; x < to; x++ {
			if m.bright[y*m.w+x] {
				row++
			}
		}
		if row == 0 {
			if gap++; gap > maxRowGap {
				bandTop = -1
			}
			continue
		}
		if bandTop < 0 {
			bandTop, bandPixels = y, 0
		}
		bandPixels += row
		gap = 0
		if bandPixels > bestPixels {
			top, bottom, bestPixels = bandTop, y, bandPixels
		}
	}
	return top, bottom
}

// cut cuts the shape of the word between the columns from and to, from the
// band of rows holding most of it.
func (m textMap) cut(from, to int) NameMask {
	top, bottom := m.band(from, to)
	if top < 0 {
		return NameMask{}
	}
	mask := NameMask{W: to - from, H: bottom - top + 1}
	mask.Bits = make([]bool, mask.W*mask.H)
	for y := 0; y < mask.H; y++ {
		for x := 0; x < mask.W; x++ {
			mask.Bits[y*mask.W+x] = m.bright[(top+y)*m.w+from+x]
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
