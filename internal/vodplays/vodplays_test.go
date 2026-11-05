package vodplays

import (
	"image"
	"testing"
)

func feedFrame() []byte {
	return make([]byte, FeedWidth*FeedHeight*3)
}

func paint(frame []byte, x0, y0, x1, y1 int) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := (y*FeedWidth + x) * 3
			frame[i], frame[i+1], frame[i+2] = 220, 20, 20
		}
	}
}

// drawNotice paints a 2px border like CS2's highlight around a notice.
func drawNotice(frame []byte, r image.Rectangle) {
	paint(frame, r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+2)
	paint(frame, r.Min.X, r.Max.Y-2, r.Max.X, r.Max.Y)
	paint(frame, r.Min.X, r.Min.Y, r.Min.X+2, r.Max.Y)
	paint(frame, r.Max.X-2, r.Min.Y, r.Max.X, r.Max.Y)
}

func TestDetectRingsFindsEachNotice(t *testing.T) {
	frame := feedFrame()
	first := image.Rect(354, 110, 756, 142)
	second := image.Rect(500, 150, 756, 182)
	drawNotice(frame, first)
	drawNotice(frame, second)
	rings := DetectRings(frame)
	if len(rings) != 2 || rings[0].Bounds != first || rings[1].Bounds != second {
		t.Fatalf("rings = %+v, want %v and %v", rings, first, second)
	}
}

func TestDetectRingsRejectsTheDeathPlate(t *testing.T) {
	frame := feedFrame()
	paint(frame, 400, 100, 756, 132)
	if rings := DetectRings(frame); len(rings) != 0 {
		t.Fatalf("rings = %+v, want none for a solid red plate", rings)
	}
}

func TestDetectRingsJoinsASplitBorder(t *testing.T) {
	frame := feedFrame()
	notice := image.Rect(354, 110, 756, 142)
	drawNotice(frame, notice)
	// A gap in the top and bottom edges splits the border in two pieces.
	for y := notice.Min.Y; y < notice.Max.Y; y++ {
		for x := 600; x < 610; x++ {
			i := (y*FeedWidth + x) * 3
			frame[i] = 0
		}
	}
	rings := DetectRings(frame)
	if len(rings) != 1 || rings[0].Bounds != notice {
		t.Fatalf("rings = %+v, want one ring %v", rings, notice)
	}
}

func TestDetectRingsSeparatesStackedNotices(t *testing.T) {
	frame := feedFrame()
	first := image.Rect(354, 74, 756, 106)
	second := image.Rect(480, 106, 756, 138)
	drawNotice(frame, first)
	drawNotice(frame, second)
	rings := DetectRings(frame)
	if len(rings) != 2 || rings[0].Bounds != first || rings[1].Bounds != second {
		t.Fatalf("rings = %+v, want %v and %v", rings, first, second)
	}
}

func ring(x0, y0, x1, y1 int) Ring {
	return Ring{Bounds: image.Rect(x0, y0, x1, y1)}
}

func TestTrackerCountsAMovingNoticeOnce(t *testing.T) {
	var tr Tracker
	tr.Add(Sample{Seconds: 10, Rings: []Ring{ring(354, 75, 756, 107)}})
	tr.Add(Sample{Seconds: 10.25, Rings: []Ring{ring(354, 75, 756, 107)}})
	// A newer notice of another size pushes the first one down a row.
	tr.Add(Sample{Seconds: 10.5, Rings: []Ring{ring(500, 75, 756, 107), ring(354, 110, 756, 142)}})
	tr.Add(Sample{Seconds: 10.75, Rings: []Ring{ring(500, 75, 756, 107), ring(354, 110, 756, 142)}})
	kills := tr.Kills()
	if len(kills) != 2 || kills[0].Seconds != 10 || kills[1].Seconds != 10.5 {
		t.Fatalf("kills = %+v, want kills at 10 and 10.5", kills)
	}
}

func TestTrackerKeepsANoticeHiddenByAFlash(t *testing.T) {
	var tr Tracker
	notice := ring(354, 75, 756, 107)
	tr.Add(Sample{Seconds: 1, Rings: []Ring{notice}})
	tr.Add(Sample{Seconds: 1.25, Rings: []Ring{notice}})
	tr.Add(Sample{Seconds: 2, Rings: nil})
	tr.Add(Sample{Seconds: 3, Rings: []Ring{ring(600, 75, 756, 107)}})
	tr.Add(Sample{Seconds: 3.25, Rings: []Ring{notice}})
	if kills := tr.Kills(); len(kills) != 1 {
		t.Fatalf("kills = %+v, want one kill", kills)
	}
}

func TestTrackerIgnoresASingleSampleBlip(t *testing.T) {
	var tr Tracker
	tr.Add(Sample{Seconds: 1, Rings: []Ring{ring(354, 75, 756, 107)}})
	tr.Add(Sample{Seconds: 1.25})
	tr.Add(Sample{Seconds: 5})
	if kills := tr.Kills(); len(kills) != 0 {
		t.Fatalf("kills = %+v, want none", kills)
	}
}

func TestGroupPlaysJoinsCloseKillsAndScoresThem(t *testing.T) {
	kills := []Kill{{Seconds: 3}, {Seconds: 4.5}, {Seconds: 12}, {Seconds: 100}}
	plays := GroupPlays(kills, 101)
	if len(plays) != 2 {
		t.Fatalf("plays = %+v, want 2", plays)
	}
	triple := plays[0]
	if triple.Label != "Triple kill" || triple.StartSeconds != 0 || triple.EndSeconds != 15 || triple.Score != 70 {
		t.Fatalf("first play = %+v, want a triple kill from 0 to 15 scoring 70", triple)
	}
	single := plays[1]
	if single.ID != "play-002" || single.StartSeconds != 94 || single.EndSeconds != 101 || single.Score != 20 {
		t.Fatalf("second play = %+v, want play-002 from 94 to 101 scoring 20", single)
	}
	best := BestPlays(plays, 2, 5)
	if len(best) != 1 || best[0].ID != "play-001" {
		t.Fatalf("best plays = %+v, want only the triple kill", best)
	}
}

type word int

const (
	wordFull word = iota
	wordTop
	wordBottom
)

func paintRect(frame []byte, r image.Rectangle, c [3]byte) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := (y*FeedWidth + x) * 3
			frame[i], frame[i+1], frame[i+2] = c[0], c[1], c[2]
		}
	}
}

// drawWord paints four 3px letters 2px apart; the kind changes their shape.
func drawWord(frame []byte, x, y int, kind word, c [3]byte) {
	for l := 0; l < 4; l++ {
		lx := x + l*5
		switch kind {
		case wordFull:
			paintRect(frame, image.Rect(lx, y, lx+3, y+10), c)
		case wordTop:
			paintRect(frame, image.Rect(lx, y, lx+3, y+5), c)
		case wordBottom:
			paintRect(frame, image.Rect(lx, y+5, lx+3, y+10), c)
		}
	}
}

var (
	blueName   = [3]byte{110, 150, 215}
	yellowName = [3]byte{228, 193, 94}
	whiteIcon  = [3]byte{240, 240, 240}
)

// drawFeedNotice draws a highlighted notice whose names have the given
// shapes, with a white weapon icon before the last name.
func drawFeedNotice(frame []byte, y int, names ...word) image.Rectangle {
	ring := image.Rect(300, y, 756, y+32)
	paintRect(frame, ring, [3]byte{40, 40, 40})
	drawNotice(frame, ring)
	x := 320
	for i, n := range names {
		if i == len(names)-1 {
			paintRect(frame, image.Rect(x, y+12, x+40, y+20), whiteIcon)
			x += 55
		}
		color := blueName
		if i == len(names)-1 {
			color = yellowName
		}
		drawWord(frame, x, y+11, n, color)
		x += 40
	}
	return ring
}

func TestReadNamesSkipsIconsAndTheBorder(t *testing.T) {
	frame := feedFrame()
	ring := drawFeedNotice(frame, 75, wordFull, wordTop)
	names := readNames(frame, ring)
	if len(names) != 2 || names[0].W != 18 || names[0].H != 10 || names[1].H != 5 {
		t.Fatalf("names = %+v, want a full and a top-half name", names)
	}
	if sameName(names[0], names[1]) {
		t.Fatal("different names match")
	}
}

func TestKillsKeepOnlyNoticesWhereThePlayerKills(t *testing.T) {
	notices := [][]word{
		{wordFull, wordTop},             // the player kills
		{wordTop, wordFull, wordBottom}, // the player assists
		{wordBottom, wordFull},          // the player dies
		{wordFull, wordBottom, wordTop}, // the player kills with an assist
	}
	var tr Tracker
	for i, names := range notices {
		frame := feedFrame()
		ring := drawFeedNotice(frame, 75, names...)
		at := float64(i * 30)
		for hit := 0; hit < 3; hit++ {
			tr.Add(Sample{Seconds: at + float64(hit)*0.25, Rings: []Ring{{Bounds: ring}}, Frame: frame})
		}
	}
	kills := tr.Kills()
	if len(kills) != 2 || kills[0].Seconds != 0 || kills[1].Seconds != 90 {
		t.Fatalf("kills = %+v, want the kills at 0 and 90", kills)
	}
}
