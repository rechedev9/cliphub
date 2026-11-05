package vodplays

import (
	"image"
	"math"
)

const (
	// A notice keeps its size while newer notices push it down the feed.
	matchWidthTolerance  = 6
	matchHeightTolerance = 4
	// A flash or smoke can hide a notice for a moment; it is the same notice
	// when it comes back within this gap.
	trackGapSeconds = 2.5
	confirmHits     = 2
	echoSeconds     = 1.0
	// A round lasts longer than this, so a repeat of the same names within
	// it is the same notice seen again.
	echoNamesSeconds = 10.0
)

// Kill is one highlighted notice seen for the first time. Names holds the
// player names in it, left to right, when the frame could be read.
type Kill struct {
	Seconds float64         `json:"seconds"`
	Notice  image.Rectangle `json:"-"`
	Names   []NameMask      `json:"-"`
}

type track struct {
	bounds    image.Rectangle
	first     float64
	last      float64
	hits      int
	confirmed bool
	notice    int
	names     []NameMask
}

// Tracker follows notices across samples and reports each one once.
type Tracker struct {
	tracks []*track
	kills  []Kill
}

// readHits is how many sightings of a notice are read for names; the first
// ones can catch the notice while it fades or slides in.
const readHits = 6

func (t *Tracker) Add(s Sample) {
	t.expire(s.Seconds)
	used := make([]bool, len(t.tracks))
	for _, ring := range s.Rings {
		best := -1
		bestCost := math.MaxFloat64
		for i, tr := range t.tracks {
			if used[i] {
				continue
			}
			cost, ok := matchCost(tr.bounds, ring.Bounds)
			if ok && cost < bestCost {
				best, bestCost = i, cost
			}
		}
		if best < 0 {
			tr := &track{bounds: ring.Bounds, first: s.Seconds, last: s.Seconds, hits: 1}
			tr.read(s.Frame, ring.Bounds)
			t.tracks = append(t.tracks, tr)
			used = append(used, true)
			continue
		}
		used[best] = true
		tr := t.tracks[best]
		tr.last = s.Seconds
		// A narrower ring in the same row is a partial view: keep the full size
		// so the notice still matches after it moves.
		partial := ring.Bounds.Dx()+matchWidthTolerance < tr.bounds.Dx() &&
			absInt(ring.Bounds.Min.Y-tr.bounds.Min.Y) <= matchHeightTolerance
		if !partial {
			tr.bounds = ring.Bounds
		}
		tr.hits++
		if tr.hits <= readHits {
			tr.read(s.Frame, ring.Bounds)
		}
		if !tr.confirmed && tr.hits >= confirmHits {
			tr.confirmed = true
			tr.notice = len(t.kills)
			t.kills = append(t.kills, Kill{Seconds: tr.first, Notice: tr.bounds})
		}
		if tr.confirmed {
			t.kills[tr.notice].Names = tr.names
		}
	}
}

// read keeps the fullest reading of the notice's names; a reading spoiled by
// the border or scenery comes back empty.
func (tr *track) read(frame []byte, ring image.Rectangle) {
	if frame == nil {
		return
	}
	if names := readNames(frame, ring); maskPixels(names) > maskPixels(tr.names) {
		tr.names = names
	}
}

// matchCost says whether a ring can be a tracked notice: either it stayed in
// place (a partial detection keeps its row), or it kept its size and moved
// because newer notices pushed it down.
func matchCost(tracked, ring image.Rectangle) (float64, bool) {
	dy := absInt(tracked.Min.Y - ring.Min.Y)
	dw := absInt(tracked.Dx() - ring.Dx())
	dh := absInt(tracked.Dy() - ring.Dy())
	if dy <= matchHeightTolerance {
		return float64(dy), true
	}
	if dw <= matchWidthTolerance && dh <= matchHeightTolerance {
		return float64(matchHeightTolerance + dw + dh + dy/20), true
	}
	return 0, false
}

// isEcho reports a second track for a notice already counted: the same row
// and size seen a moment later, or the same names within a round's time,
// since one player cannot kill the same victim twice in a round.
func isEcho(kept []Kill, kill Kill) bool {
	for i := len(kept) - 1; i >= 0; i-- {
		k := kept[i]
		if kill.Seconds-k.Seconds > echoNamesSeconds {
			return false
		}
		if len(kill.Names) > 0 && sameNames(k.Names, kill.Names) {
			return true
		}
		if kill.Seconds-k.Seconds <= echoSeconds &&
			absInt(k.Notice.Min.Y-kill.Notice.Min.Y) <= matchHeightTolerance &&
			absInt(k.Notice.Dx()-kill.Notice.Dx()) <= 2*matchWidthTolerance {
			return true
		}
	}
	return false
}

func sameNames(a, b []NameMask) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameName(a[i], b[i]) {
			return false
		}
	}
	return true
}

// Kills returns the notices where the recording player is the killer, in
// the order they appeared. CS2 also highlights the player's assists and
// death; there the player's name is not the first one.
func (t *Tracker) Kills() []Kill {
	var notices []Kill
	for _, k := range t.kills {
		if !isEcho(notices, k) {
			notices = append(notices, k)
		}
	}
	local, ok := localName(notices)
	var kills []Kill
	for _, k := range notices {
		if !ok || (len(k.Names) > 0 && sameName(k.Names[0], local)) {
			kills = append(kills, k)
		}
	}
	return kills
}

// localName finds the recording player's name: the one name present in more
// highlighted notices than any other. With fewer than two notices sharing a
// name it cannot tell, and ok is false.
func localName(notices []Kill) (NameMask, bool) {
	type cluster struct {
		name    NameMask
		notices map[int]bool
	}
	var clusters []*cluster
	for i, n := range notices {
		for _, name := range n.Names {
			var home *cluster
			for _, c := range clusters {
				if sameName(c.name, name) {
					home = c
					break
				}
			}
			if home == nil {
				home = &cluster{name: name, notices: map[int]bool{}}
				clusters = append(clusters, home)
			}
			home.notices[i] = true
		}
	}
	var best *cluster
	for _, c := range clusters {
		if best == nil || len(c.notices) > len(best.notices) {
			best = c
		}
	}
	if best == nil || len(best.notices) < 2 {
		return NameMask{}, false
	}
	return best.name, true
}

func (t *Tracker) expire(now float64) {
	kept := t.tracks[:0]
	for _, tr := range t.tracks {
		if now-tr.last <= trackGapSeconds {
			kept = append(kept, tr)
		}
	}
	t.tracks = kept
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
