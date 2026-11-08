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

// Each kind of word has its own shape once cut to its box.
const (
	wordFull word = iota
	wordShort
	wordLong
	wordStep
)

func paintRect(frame []byte, r image.Rectangle, c [3]byte) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := (y*FeedWidth + x) * 3
			frame[i], frame[i+1], frame[i+2] = c[0], c[1], c[2]
		}
	}
}

// drawWord paints 3px letters 2px apart and returns the width it took.
func drawWord(frame []byte, x, y int, kind word, c [3]byte) int {
	letters := 4
	if kind == wordLong {
		letters = 6
	}
	for l := 0; l < letters; l++ {
		lx := x + l*5
		top := y
		if kind == wordShort || (kind == wordStep && l%2 == 1) {
			top = y + 5
		}
		paintRect(frame, image.Rect(lx, top, lx+3, y+10), c)
	}
	return letters*5 - 2
}

var (
	blueName   = [3]byte{110, 150, 215}
	yellowName = [3]byte{228, 193, 94}
	dimBlueTag = [3]byte{74, 93, 119}
	greyStroke = [3]byte{112, 110, 118}
	whiteIcon  = [3]byte{240, 240, 240}
)

// player is one name in a notice, with the clan tag drawn before it if any.
type player struct {
	name word
	tag  bool
}

func named(w word) player  { return player{name: w} }
func tagged(w word) player { return player{name: w, tag: true} }

// drawFeedNotice draws a highlighted notice "killer [+ assister] weapon
// victim": a white plus after the killer when there is an assister, and a
// white weapon before the victim. plusGap is the space after the plus.
func drawFeedNotice(frame []byte, y, plusGap int, players ...player) image.Rectangle {
	return drawFeedNoticeFrom(frame, 250, y, plusGap, players...)
}

func drawFeedNoticeFrom(frame []byte, left, y, plusGap int, players ...player) image.Rectangle {
	ring := image.Rect(left, y, 756, y+32)
	paintRect(frame, ring, [3]byte{40, 40, 40})
	drawNotice(frame, ring)
	x := left + 12
	for i, p := range players {
		color := blueName
		if i == len(players)-1 {
			paintRect(frame, image.Rect(x, y+12, x+40, y+20), whiteIcon)
			x += 50
			color = yellowName
		}
		if p.tag {
			// The tag is dimmer than the name, between brackets that blur greys.
			paintRect(frame, image.Rect(x, y+9, x+2, y+23), greyStroke)
			x += 4 + drawWord(frame, x+4, y+11, wordShort, dimBlueTag) + 2
			paintRect(frame, image.Rect(x, y+9, x+2, y+23), greyStroke)
			x += 8
		}
		x += drawWord(frame, x, y+11, p.name, color)
		if i == 0 && len(players) == 3 {
			x += 7
			paintRect(frame, image.Rect(x, y+15, x+7, y+16), whiteIcon)
			paintRect(frame, image.Rect(x+3, y+12, x+4, y+19), whiteIcon)
			x += 7 + plusGap
			continue
		}
		x += 10
	}
	return ring
}

func TestReadNoticeFindsTheKillerAndAssisterSlots(t *testing.T) {
	frame := feedFrame()
	ring := drawFeedNotice(frame, 75, 6, tagged(wordFull), tagged(wordShort), named(wordLong))
	r := readNotice(frame, ring)
	if len(r.Names) != 5 {
		t.Fatalf("names = %d, want two tags and three names", len(r.Names))
	}
	if !sameName(r.Killer, r.Names[1]) || !sameName(r.Assister, r.Names[3]) || sameName(r.Killer, r.Assister) {
		t.Fatalf("killer and assister are not the words before the plus and the weapon: %+v", r)
	}
	if r.Killer.H != 10 || r.Assister.H != 5 {
		t.Fatalf("killer %dx%d, assister %dx%d, want the names and not their tags", r.Killer.W, r.Killer.H, r.Assister.W, r.Assister.H)
	}
}

func TestReadNoticeKeepsThePlusApartFromACloseTag(t *testing.T) {
	frame := feedFrame()
	// Two pixels of space would join the plus to the assister's tag if words
	// were split by gaps alone.
	ring := drawFeedNotice(frame, 75, 2, tagged(wordShort), tagged(wordFull), named(wordLong))
	r := readNotice(frame, ring)
	if r.Killer.W == 0 || r.Killer.H != 5 {
		t.Fatalf("killer = %dx%d, want the first name before the plus", r.Killer.W, r.Killer.H)
	}
	if r.Assister.W == 0 {
		t.Fatal("no assister slot: the plus was taken for part of a word")
	}
}

func TestReadNoticeIgnoresTheBorderBlurAboveTheText(t *testing.T) {
	frame := feedFrame()
	ring := drawFeedNotice(frame, 75, 6, named(wordShort), named(wordFull), named(wordLong))
	// Over a bright background the border blurs into a pale line inside the
	// ring that runs over the killer, the plus and the assister.
	paintRect(frame, image.Rect(ring.Min.X+5, ring.Min.Y+4, ring.Min.X+150, ring.Min.Y+5), [3]byte{220, 160, 120})
	r := readNotice(frame, ring)
	if r.Killer.H != 5 || r.Assister.H != 10 {
		t.Fatalf("killer %dx%d, assister %dx%d, want the short name and the full one", r.Killer.W, r.Killer.H, r.Assister.W, r.Assister.H)
	}
}

// addNotice feeds a notice to the tracker for three samples from at.
func addNotice(tr *Tracker, at float64, players ...player) {
	frame := feedFrame()
	ring := drawFeedNotice(frame, 75, 6, players...)
	for hit := 0; hit < 3; hit++ {
		tr.Add(Sample{Seconds: at + float64(hit)*0.25, Rings: []Ring{{Bounds: ring}}, Frame: frame})
	}
}

func killTimes(kills []Kill) []float64 {
	var times []float64
	for _, k := range kills {
		times = append(times, k.Seconds)
	}
	return times
}

func TestKillsKeepOnlyNoticesWhereThePlayerKills(t *testing.T) {
	var tr Tracker
	addNotice(&tr, 0, named(wordFull), named(wordShort))                   // the player kills
	addNotice(&tr, 30, named(wordShort), named(wordFull), named(wordLong)) // the player assists
	addNotice(&tr, 60, named(wordFull), named(wordLong), named(wordShort)) // the player kills with an assist
	if got := killTimes(tr.Kills()); len(got) != 2 || got[0] != 0 || got[1] != 60 {
		t.Fatalf("kills at %v, want 0 and 60", got)
	}
}

func TestKillsIgnoreAClanTagTheTeamShares(t *testing.T) {
	var tr Tracker
	// The player and a teammate wear the same tag, which is then the most
	// common word; it must not stand for the player.
	addNotice(&tr, 0, tagged(wordFull), named(wordShort))
	addNotice(&tr, 30, tagged(wordLong), tagged(wordFull), named(wordShort))
	addNotice(&tr, 60, tagged(wordFull), tagged(wordLong), named(wordStep))
	addNotice(&tr, 90, tagged(wordLong), tagged(wordFull), named(wordStep))
	if got := killTimes(tr.Kills()); len(got) != 2 || got[0] != 0 || got[1] != 60 {
		t.Fatalf("kills at %v, want 0 and 60", got)
	}
}

func TestKillsNeverTakeARivalForThePlayer(t *testing.T) {
	var tr Tracker
	for i := 0; i < 4; i++ {
		addNotice(&tr, float64(i*30), named(wordFull), named(wordStep))
	}
	// CS2 can highlight the player's death too; the rival who keeps killing
	// the player holds the killer slot there and must not pass for a nick.
	for i := 4; i < 7; i++ {
		addNotice(&tr, float64(i*30), named(wordLong), named(wordFull))
	}
	if got := killTimes(tr.Kills()); len(got) != 4 || got[3] != 90 {
		t.Fatalf("kills at %v, want the four up to 90", got)
	}
}

// sight feeds one ring to the tracker at each of the given seconds.
func sight(tr *Tracker, frame []byte, ring image.Rectangle, times ...float64) {
	for _, at := range times {
		tr.Add(Sample{Seconds: at, Rings: []Ring{{Bounds: ring}}, Frame: frame})
	}
}

func TestKillsCountAFlickeringNoticeOnce(t *testing.T) {
	var tr Tracker
	addNotice(&tr, 0, named(wordFull), named(wordLong)) // another kill, so the player is known
	first, again := feedFrame(), feedFrame()
	// The notice flickers out for longer than a track waits, moves up a row
	// meanwhile, and comes back with its victim read differently.
	sight(&tr, first, drawFeedNotice(first, 109, 6, named(wordFull), named(wordShort)), 20, 20.25)
	sight(&tr, again, drawFeedNotice(again, 75, 6, named(wordFull), named(wordStep)), 23.5, 23.75)
	// A later kill in the row the first notice left has another width.
	next := feedFrame()
	sight(&tr, next, drawFeedNoticeFrom(next, 270, 109, 6, named(wordFull), named(wordStep)), 27, 27.25)
	// A double kill: two notices of one width, the newer one below.
	double := feedFrame()
	upper := drawFeedNoticeFrom(double, 230, 75, 6, named(wordFull), named(wordShort))
	lower := drawFeedNoticeFrom(double, 230, 109, 6, named(wordFull), named(wordStep))
	for _, at := range []float64{45, 45.25} {
		tr.Add(Sample{Seconds: at, Rings: []Ring{{Bounds: upper}, {Bounds: lower}}, Frame: double})
	}
	if got := killTimes(tr.Kills()); len(got) != 5 || got[1] != 20 || got[2] != 27 || got[3] != 45 || got[4] != 45 {
		t.Fatalf("kills at %v, want 0, 20, 27 and two at 45", got)
	}
}

func TestKillsFollowThePlayerAcrossARename(t *testing.T) {
	var tr Tracker
	// The player kills as wordShort, then renames to wordFull; wordLong is a
	// teammate the player assists under the new nick.
	for i := 0; i < 3; i++ {
		addNotice(&tr, float64(i*30), named(wordShort), named(wordStep))
	}
	for i := 3; i < 7; i++ {
		addNotice(&tr, float64(i*30), named(wordFull), named(wordStep))
	}
	for i := 7; i < 10; i++ {
		addNotice(&tr, float64(i*30), named(wordLong), named(wordFull), named(wordStep))
	}
	if got := tr.Kills(); len(got) != 7 || got[6].Seconds != 180 {
		t.Fatalf("kills at %v, want the seven kills up to 180", killTimes(got))
	}
}
