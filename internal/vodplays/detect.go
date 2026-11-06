// Package vodplays finds the recording player's kills in a CS2 gameplay video
// (a stream VOD or a local recording) from the red highlight CS2 draws around
// that player's kill-feed notices, and groups them into scored plays. It reads
// pixels only: no demo, OCR, model or network.
package vodplays

import (
	"image"
	"sort"
)

// The scan crops the kill-feed corner and scales it to the size it has in a
// 1920x1080 frame, so every threshold below is in 1080p pixels.
const (
	FeedWidth  = 768
	FeedHeight = 324
	// The corner is the right feedSpan of the width from feedLeft, and the top
	// feedDepth of the height, all as fractions of the source frame.
	feedLeft  = 0.6
	feedSpan  = 0.4
	feedDepth = 0.3

	// Compression blends the thin border with what is behind it, so a border
	// pixel is red relative to its other channels rather than pure red; green
	// well above blue is the orange of walls and sand.
	minBorderRed     = 120
	minRedMargin     = 40
	maxGreenOverBlue = 30
	// A border edge is a long horizontal red line; gaps come from blur and
	// from scenery of the same colour as the plate.
	minEdgeLength  = 40
	maxEdgeGap     = 12
	maxEdgeRows    = 6
	edgeSpanSlack  = 6
	minSideCover   = 0.5
	sideSearch     = 3
	maxInteriorRed = 0.3
	// A notice is about 32px tall and CS2 aligns every notice to the right
	// edge of the screen, so anything shorter or further left is scenery.
	minRingHeight = 22
	maxRingHeight = 1080 / 12
	maxRightGap   = 60
	minRingAspect = 2
)

// Ring is one highlighted notice in a feed frame, in feed pixels.
type Ring struct {
	Bounds image.Rectangle
}

type edge struct {
	minX, maxX int
	top, rows  int
}

func (e edge) bottom() int { return e.top + e.rows - 1 }

// DetectRings returns the highlighted notice borders in one RGB24 feed frame
// of FeedWidth x FeedHeight. A border is a top and a bottom red line of the
// same span joined by a red left side, with a plate inside that is not red.
// Matching edges by span keeps two stacked notices apart even when the blur
// joins their borders, and a red death plate fails as solid.
func DetectRings(rgb []byte) []Ring {
	if len(rgb) < FeedWidth*FeedHeight*3 {
		return nil
	}
	red := make([]bool, FeedWidth*FeedHeight)
	for i := range red {
		r, g, b := int(rgb[i*3]), int(rgb[i*3+1]), int(rgb[i*3+2])
		red[i] = r >= minBorderRed && r-max(g, b) >= minRedMargin && g-b <= maxGreenOverBlue
	}
	edges := findEdges(red)
	var rings []Ring
	used := make([]bool, len(edges))
	for i, top := range edges {
		if used[i] {
			continue
		}
		for j := i + 1; j < len(edges); j++ {
			bottom := edges[j]
			height := bottom.bottom() - top.top + 1
			if height > maxRingHeight {
				break
			}
			if used[j] || height < minRingHeight || !sameSpan(top, bottom) {
				continue
			}
			bounds, ok := ringBounds(red, image.Rect(min(top.minX, bottom.minX), top.top, max(top.maxX, bottom.maxX)+1, bottom.bottom()+1))
			if ok {
				rings = append(rings, Ring{Bounds: bounds})
				used[i], used[j] = true, true
				break
			}
		}
	}
	sort.Slice(rings, func(a, b int) bool { return rings[a].Bounds.Min.Y < rings[b].Bounds.Min.Y })
	return rings
}

// findEdges returns the horizontal red lines that reach the right edge of the
// feed, joining consecutive rows that start together; a line thicker than
// maxEdgeRows is a filled plate, not a border.
func findEdges(red []bool) []edge {
	var open, edges []edge
	for y := 0; y < FeedHeight; y++ {
		var next []edge
		for _, run := range rowRuns(red, y) {
			grown := false
			for k, e := range open {
				if absInt(e.minX-run[0]) <= edgeSpanSlack {
					e.minX, e.maxX = min(e.minX, run[0]), max(e.maxX, run[1])
					e.rows++
					next = append(next, e)
					open = append(open[:k], open[k+1:]...)
					grown = true
					break
				}
			}
			if !grown {
				next = append(next, edge{minX: run[0], maxX: run[1], top: y, rows: 1})
			}
		}
		for _, e := range open {
			if e.rows <= maxEdgeRows {
				edges = append(edges, e)
			}
		}
		open = next
	}
	for _, e := range open {
		if e.rows <= maxEdgeRows {
			edges = append(edges, e)
		}
	}
	sort.Slice(edges, func(a, b int) bool { return edges[a].top < edges[b].top })
	return edges
}

// rowRuns returns the long red runs of one row that end near the right edge.
func rowRuns(red []bool, y int) [][2]int {
	var runs [][2]int
	start, last := -1, -1
	flush := func() {
		if start >= 0 && last-start+1 >= minEdgeLength && FeedWidth-1-last <= maxRightGap {
			runs = append(runs, [2]int{start, last})
		}
	}
	for x := 0; x < FeedWidth; x++ {
		if !red[y*FeedWidth+x] {
			continue
		}
		if start >= 0 && x-last > maxEdgeGap {
			flush()
			start = -1
		}
		if start < 0 {
			start = x
		}
		last = x
	}
	flush()
	return runs
}

// sameSpan compares where two edges start: every notice ends at the right of
// the screen, and red scenery there can stretch one edge but not the other.
func sameSpan(a, b edge) bool {
	return absInt(a.minX-b.minX) <= edgeSpanSlack
}

// ringBounds trims the box between two matching edges to its red sides, since
// the edges can run into red scenery next to the notice, and checks that the
// interior is mostly not red.
func ringBounds(red []bool, b image.Rectangle) (image.Rectangle, bool) {
	left, okLeft := sideColumn(red, b, b.Min.X, min(b.Max.X-1, b.Min.X+maxEdgeGap+sideSearch))
	right, okRight := sideColumn(red, b, b.Max.X-1, max(b.Min.X, b.Max.X-1-maxEdgeGap-sideSearch))
	if !okLeft || !okRight {
		return image.Rectangle{}, false
	}
	b.Min.X, b.Max.X = left, right+1
	if b.Dx() < minRingAspect*b.Dy() || FeedWidth-b.Max.X > maxRightGap {
		return image.Rectangle{}, false
	}
	inner := b.Inset(sideSearch + 2)
	if inner.Empty() {
		return image.Rectangle{}, false
	}
	interior := 0
	for y := inner.Min.Y; y < inner.Max.Y; y++ {
		for x := inner.Min.X; x < inner.Max.X; x++ {
			if red[y*FeedWidth+x] {
				interior++
			}
		}
	}
	return b, float64(interior) <= maxInteriorRed*float64(inner.Dx()*inner.Dy())
}

// sideColumn returns the column between from and to that is red over most
// of the box height and over more of it than any other: a vertical side of
// the notice rather than red scenery beside it.
func sideColumn(red []bool, b image.Rectangle, from, to int) (int, bool) {
	step := 1
	if to < from {
		step = -1
	}
	best, bestCovered := 0, -1
	for x := from; ; x += step {
		covered := 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if red[y*FeedWidth+x] {
				covered++
			}
		}
		if covered > bestCovered {
			best, bestCovered = x, covered
		}
		if x == to {
			break
		}
	}
	return best, float64(bestCovered) >= minSideCover*float64(b.Dy())
}
