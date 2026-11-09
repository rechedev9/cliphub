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
	// wordTall stands for scenery caught in a word, taller than any name.
	wordTall
)

func paintRect(frame []byte, r image.Rectangle, c [3]byte) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := (y*FeedWidth + x) * 3
			frame[i], frame[i+1], frame[i+2] = c[0], c[1], c[2]
		}
	}
}

// pen paints test shapes in one colour.
type pen struct {
	frame []byte
	color [3]byte
}

// word paints 3px letters 2px apart from at and returns the width it took.
func (p pen) word(at image.Point, kind word) int {
	letters := 4
	if kind == wordLong {
		letters = 6
	}
	for l := 0; l < letters; l++ {
		lx := at.X + l*5
		top := at.Y
		switch {
		case kind == wordShort || (kind == wordStep && l%2 == 1):
			top = at.Y + 5
		case kind == wordTall:
			top = at.Y - 8
		}
		paintRect(p.frame, image.Rect(lx, top, lx+3, at.Y+10), p.color)
	}
	return letters*5 - 2
}

var (
	blueName   = [3]byte{110, 150, 215}
	yellowName = [3]byte{228, 193, 94}
	dimBlueTag = [3]byte{74, 93, 119}
	greyStroke = [3]byte{112, 110, 118}
	whiteIcon  = [3]byte{240, 240, 240}
	plate      = [3]byte{40, 40, 40}
)

// player is one name in a notice, with the clan tag drawn before it if any.
type player struct {
	name word
	tag  bool
}

func named(w word) player  { return player{name: w} }
func tagged(w word) player { return player{name: w, tag: true} }

// feedNotice is a highlighted notice "killer [+ assister] weapon victim": a
// white plus after the killer when there is an assister, and a white weapon
// before the victim. plusGap is the space after the plus.
type feedNotice struct {
	left, top int
	plusGap   int
	players   []player
}

func notice(players ...player) feedNotice {
	return feedNotice{left: 250, top: 75, plusGap: 6, players: players}
}

func (n feedNotice) at(left, top int) feedNotice {
	n.left, n.top = left, top
	return n
}

func (n feedNotice) draw(frame []byte) image.Rectangle {
	y := n.top
	ring := image.Rect(n.left, y, 756, y+32)
	paintRect(frame, ring, plate)
	drawNotice(frame, ring)
	x := n.left + 12
	for i, p := range n.players {
		color := blueName
		if i == len(n.players)-1 {
			paintRect(frame, image.Rect(x, y+12, x+40, y+20), whiteIcon)
			x += 50
			color = yellowName
		}
		if p.tag {
			// The tag is dimmer than the name, between brackets that blur greys.
			paintRect(frame, image.Rect(x, y+9, x+2, y+23), greyStroke)
			x += 4 + pen{frame, dimBlueTag}.word(image.Pt(x+4, y+11), wordShort) + 2
			paintRect(frame, image.Rect(x, y+9, x+2, y+23), greyStroke)
			x += 8
		}
		x += pen{frame, color}.word(image.Pt(x, y+11), p.name)
		if i == 0 && len(n.players) == 3 {
			x += 7
			paintRect(frame, image.Rect(x, y+15, x+7, y+16), whiteIcon)
			paintRect(frame, image.Rect(x+3, y+12, x+4, y+19), whiteIcon)
			x += 7 + n.plusGap
			continue
		}
		x += 10
	}
	return ring
}

// add feeds the notice to the tracker for three samples from at.
func (n feedNotice) add(tr *Tracker, at float64) {
	sightingOf(n).seen(tr, at, at+0.25, at+0.5)
}

// sighting is a notice drawn in a frame of its own.
type sighting struct {
	frame []byte
	ring  image.Rectangle
}

func sightingOf(n feedNotice) sighting {
	frame := feedFrame()
	return sighting{frame: frame, ring: n.draw(frame)}
}

// seen feeds the ring to the tracker at each of the given seconds.
func (s sighting) seen(tr *Tracker, times ...float64) {
	for _, at := range times {
		tr.Add(Sample{Seconds: at, Rings: []Ring{{Bounds: s.ring}}, Frame: s.frame})
	}
}

func TestReadNoticeFindsTheKillerAndAssisterSlots(t *testing.T) {
	frame := feedFrame()
	ring := notice(tagged(wordFull), tagged(wordShort), named(wordLong)).draw(frame)
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
	n := notice(tagged(wordShort), tagged(wordFull), named(wordLong))
	n.plusGap = 2
	r := readNotice(frame, n.draw(frame))
	if r.Killer.W == 0 || r.Killer.H != 5 {
		t.Fatalf("killer = %dx%d, want the first name before the plus", r.Killer.W, r.Killer.H)
	}
	if r.Assister.W == 0 {
		t.Fatal("no assister slot: the plus was taken for part of a word")
	}
}

func TestReadNoticeIgnoresTheBorderBlurAboveTheText(t *testing.T) {
	frame := feedFrame()
	ring := notice(named(wordShort), named(wordFull), named(wordLong)).draw(frame)
	// Over a bright background the border blurs into a pale line inside the
	// ring that runs over the killer, the plus and the assister.
	paintRect(frame, image.Rect(ring.Min.X+5, ring.Min.Y+4, ring.Min.X+150, ring.Min.Y+5), [3]byte{220, 160, 120})
	r := readNotice(frame, ring)
	if r.Killer.H != 5 || r.Assister.H != 10 {
		t.Fatalf("killer %dx%d, assister %dx%d, want the short name and the full one", r.Killer.W, r.Killer.H, r.Assister.W, r.Assister.H)
	}
}

func TestReadNoticeLeavesATallShapeInItsSlot(t *testing.T) {
	frame := feedFrame()
	// Scenery over the assister's name: its tag must not take the slot.
	r := readNotice(frame, notice(tagged(wordFull), tagged(wordTall), named(wordLong)).draw(frame))
	if len(r.Names) != 5 || r.Names[3].W != 0 {
		t.Fatalf("names = %+v, want the tall shape kept as an unread word", r.Names)
	}
	if r.Killer.H != 10 || r.Assister.W != 0 {
		t.Fatalf("killer %dx%d, assister %dx%d, want the killer and no assister", r.Killer.W, r.Killer.H, r.Assister.W, r.Assister.H)
	}
}

func TestTrackReadKeepsAReadingWithItsKiller(t *testing.T) {
	marked := sightingOf(notice(named(wordShort), named(wordShort)))
	// Scenery hides the weapon and adds words: more text, but no slot to read.
	unmarked := feedFrame()
	paintRect(unmarked, marked.ring, plate)
	drawNotice(unmarked, marked.ring)
	for x := marked.ring.Min.X + 12; x < marked.ring.Min.X+200; x += 40 {
		pen{unmarked, blueName}.word(image.Pt(x, marked.ring.Min.Y+11), wordLong)
	}
	for _, order := range [][][]byte{{marked.frame, unmarked}, {unmarked, marked.frame}} {
		var tr track
		for _, frame := range order {
			tr.read(frame, marked.ring)
		}
		if tr.reading.Killer.W == 0 {
			t.Fatalf("reading = %+v, want the one with a killer", tr.reading)
		}
	}
}

func TestTrackReadKeepsTheReadingMostSightingsAgreeOn(t *testing.T) {
	clean := sightingOf(notice(named(wordFull), named(wordShort)))
	// Scenery joins the killer's name to more text in one sighting.
	spoiled := sightingOf(notice(named(wordLong), named(wordLong)))
	var tr track
	for _, frame := range [][]byte{clean.frame, clean.frame, spoiled.frame, clean.frame} {
		tr.read(frame, clean.ring)
	}
	if tr.reading.Killer.W != 18 {
		t.Fatalf("killer = %dx%d, want the name three sightings agree on", tr.reading.Killer.W, tr.reading.Killer.H)
	}
}

func TestTrackerReadsANoticeAgainWhenItComesBack(t *testing.T) {
	var tr Tracker
	spoiled := sightingOf(notice(named(wordLong), named(wordLong)))
	spoiled.seen(&tr, 20, 20.25, 20.5, 20.75)
	// Back from a flicker, the notice shows its names clean for longer.
	clean := sightingOf(notice(named(wordFull), named(wordShort)))
	clean.seen(&tr, 24, 24.25, 24.5, 24.75, 25, 25.25, 25.5)
	if len(tr.kills) != 1 || tr.kills[0].Killer.W != 18 {
		t.Fatalf("kills = %d, killer = %+v, want one notice read clean", len(tr.kills), tr.kills[0].Killer)
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
	notice(named(wordFull), named(wordShort)).add(&tr, 0)                   // the player kills
	notice(named(wordShort), named(wordFull), named(wordLong)).add(&tr, 30) // the player assists
	notice(named(wordFull), named(wordLong), named(wordShort)).add(&tr, 60) // the player kills with an assist
	if got := killTimes(tr.Kills()); len(got) != 2 || got[0] != 0 || got[1] != 60 {
		t.Fatalf("kills at %v, want 0 and 60", got)
	}
}

func TestKillsIgnoreAClanTagTheTeamShares(t *testing.T) {
	var tr Tracker
	// The player and a teammate wear the same tag, which is then the most
	// common word; it must not stand for the player.
	notice(tagged(wordFull), named(wordShort)).add(&tr, 0)
	notice(tagged(wordLong), tagged(wordFull), named(wordShort)).add(&tr, 30)
	notice(tagged(wordFull), tagged(wordLong), named(wordStep)).add(&tr, 60)
	notice(tagged(wordLong), tagged(wordFull), named(wordStep)).add(&tr, 90)
	if got := killTimes(tr.Kills()); len(got) != 2 || got[0] != 0 || got[1] != 60 {
		t.Fatalf("kills at %v, want 0 and 60", got)
	}
}

func TestKillsNeverTakeARivalForThePlayer(t *testing.T) {
	var tr Tracker
	for i := 0; i < 4; i++ {
		notice(named(wordFull), named(wordStep)).add(&tr, float64(i*30))
	}
	// CS2 can highlight the player's death too; the rival who keeps killing
	// the player holds the killer slot there and must not pass for a nick.
	for i := 4; i < 7; i++ {
		notice(named(wordLong), named(wordFull)).add(&tr, float64(i*30))
	}
	if got := killTimes(tr.Kills()); len(got) != 4 || got[3] != 90 {
		t.Fatalf("kills at %v, want the four up to 90", got)
	}
}

func TestKillsNeverTakeAFlickeringTeammateForThePlayer(t *testing.T) {
	var tr Tracker
	for i := 0; i < 4; i++ {
		notice(named(wordFull), named(wordShort)).add(&tr, float64(i*30))
	}
	// An assist where the player's name was misread flickers out twice: still
	// one notice, too few for the teammate to pass for a second nick.
	sightingOf(notice(named(wordLong), named(wordStep), named(wordShort))).seen(&tr, 200, 200.25, 203, 203.25, 206, 206.25)
	if got := killTimes(tr.Kills()); len(got) != 4 || got[3] != 90 {
		t.Fatalf("kills at %v, want the four up to 90", got)
	}
}

func TestKillsCountAFlickeringNoticeOnce(t *testing.T) {
	var tr Tracker
	notice(named(wordFull), named(wordLong)).add(&tr, 0) // another kill, so the player is known
	// The notice flickers out for longer than a track waits, moves up a row
	// meanwhile, and comes back with its victim read differently.
	sightingOf(notice(named(wordFull), named(wordShort)).at(250, 109)).seen(&tr, 20, 20.25)
	sightingOf(notice(named(wordFull), named(wordStep))).seen(&tr, 23.5, 23.75)
	// A later kill in the row the first notice left has another width.
	sightingOf(notice(named(wordFull), named(wordLong)).at(270, 109)).seen(&tr, 27, 27.25)
	// A double kill: two notices of one width, the newer one below.
	double := feedFrame()
	upper := notice(named(wordFull), named(wordShort)).at(230, 75).draw(double)
	lower := notice(named(wordFull), named(wordStep)).at(230, 109).draw(double)
	for _, at := range []float64{45, 45.25} {
		tr.Add(Sample{Seconds: at, Rings: []Ring{{Bounds: upper}, {Bounds: lower}}, Frame: double})
	}
	if got := killTimes(tr.Kills()); len(got) != 5 || got[1] != 20 || got[2] != 27 || got[3] != 45 || got[4] != 45 {
		t.Fatalf("kills at %v, want 0, 20, 27 and two at 45", got)
	}
}

func TestKillsCountTwoKillsOfOneWidthInOneRow(t *testing.T) {
	var tr Tracker
	notice(named(wordFull), named(wordLong)).add(&tr, 0) // another kill, so the player is known
	// Detection loses the first notice early, and it leaves the feed later; the
	// next kill, as wide, then takes its row.
	sightingOf(notice(named(wordFull), named(wordShort))).seen(&tr, 20, 20.25, 20.5)
	next := 20 + noticeSeconds + 1
	sightingOf(notice(named(wordFull), named(wordStep))).seen(&tr, next, next+0.25)
	if got := killTimes(tr.Kills()); len(got) != 3 || got[1] != 20 || got[2] != next {
		t.Fatalf("kills at %v, want 0, 20 and %v", got, next)
	}
}

func TestTrackerFollowsANoticeStretchedByRedScenery(t *testing.T) {
	var tr Tracker
	tr.Add(Sample{Seconds: 10, Rings: []Ring{ring(354, 109, 756, 141)}})
	tr.Add(Sample{Seconds: 10.25, Rings: []Ring{ring(354, 109, 756, 141)}})
	// The notice above it leaves; red scenery behind the new row stretches
	// the ring's right edge, but never its left one.
	tr.Add(Sample{Seconds: 10.5, Rings: []Ring{ring(354, 75, 768, 107)}})
	tr.Add(Sample{Seconds: 10.75, Rings: []Ring{ring(354, 75, 768, 107)}})
	if kills := tr.Kills(); len(kills) != 1 {
		t.Fatalf("kills = %+v, want one kill", kills)
	}
}

func TestKillsFollowThePlayerAcrossARename(t *testing.T) {
	var tr Tracker
	// The player kills as wordShort, then renames to wordFull; wordLong is a
	// teammate the player assists under the new nick.
	for i := 0; i < 3; i++ {
		notice(named(wordShort), named(wordStep)).add(&tr, float64(i*30))
	}
	for i := 3; i < 7; i++ {
		notice(named(wordFull), named(wordStep)).add(&tr, float64(i*30))
	}
	for i := 7; i < 10; i++ {
		notice(named(wordLong), named(wordFull), named(wordStep)).add(&tr, float64(i*30))
	}
	if got := tr.Kills(); len(got) != 7 || got[6].Seconds != 180 {
		t.Fatalf("kills at %v, want the seven kills up to 180", killTimes(got))
	}
}
