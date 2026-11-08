package vodplays

import (
	"image"
	"math"
	"sort"
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
	// A second nick of the player must hold a slot this often, so a teammate
	// whose shared notices were all misread does not pass for the player.
	minAliasNotices = 3
	// CS2 aligns notices right, so the left edge holds a notice's width to the
	// pixel even when red scenery stretches the ring's right edge.
	echoEdgeTolerance = 2
)

// Kill is one highlighted notice seen for the first time, with what could be
// read of its names.
type Kill struct {
	Seconds float64         `json:"seconds"`
	Notice  image.Rectangle `json:"-"`
	Reading `json:"-"`
	// lastTop is the top of the row where the notice was last seen.
	lastTop int
}

type track struct {
	bounds    image.Rectangle
	first     float64
	last      float64
	hits      int
	confirmed bool
	notice    int
	reading   Reading
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
			t.kills[tr.notice].Reading = tr.reading
			t.kills[tr.notice].lastTop = ring.Bounds.Min.Y
		}
	}
}

// read keeps the fullest reading of the notice that found its killer; a
// reading spoiled by the border or scenery comes back empty.
func (tr *track) read(frame []byte, ring image.Rectangle) {
	if frame == nil {
		return
	}
	r := readNotice(frame, ring)
	hasKiller, hadKiller := r.Killer.W > 0, tr.reading.Killer.W > 0
	if hasKiller != hadKiller {
		if hasKiller {
			tr.reading = r
		}
		return
	}
	if maskPixels(r.Names) > maskPixels(tr.reading.Names) {
		tr.reading = r
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

// isEcho reports a second track for a notice already counted, within a
// round's time: the same names, or the same killer and width after a flicker.
func isEcho(kept []Kill, kill Kill, nicks [][]NameMask) bool {
	for i := len(kept) - 1; i >= 0; i-- {
		k := kept[i]
		if kill.Seconds-k.Seconds > echoNamesSeconds {
			return false
		}
		if len(kill.Names) > 0 && sameNames(k.Names, kill.Names) {
			return true
		}
		sameRow := absInt(k.Notice.Min.Y-kill.Notice.Min.Y) <= matchHeightTolerance
		dw := absInt(k.Notice.Dx() - kill.Notice.Dx())
		if sameRow && kill.Seconds-k.Seconds <= echoSeconds && dw <= 2*matchWidthTolerance {
			return true
		}
		// Notices only move up as older ones leave, and new ones come in below,
		// so a notice back from a flicker is in its last row or above it.
		sameKiller := kill.Killer.W > 0 && (sameName(k.Killer, kill.Killer) ||
			(isPlayer(nicks, k.Killer) && isPlayer(nicks, kill.Killer)))
		if sameKiller && absInt(k.Notice.Min.X-kill.Notice.Min.X) <= echoEdgeTolerance &&
			kill.Notice.Min.Y <= k.lastTop+matchHeightTolerance {
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

// Kills returns the notices where the recording player is the killer, in order;
// CS2 also highlights their assists, where another name holds the killer slot.
func (t *Tracker) Kills() []Kill {
	nicks := playerNicks(t.kills)
	var notices []Kill
	for _, k := range t.kills {
		if !isEcho(notices, k, nicks) {
			notices = append(notices, k)
		}
	}
	var kills []Kill
	for _, k := range notices {
		if len(nicks) == 0 || isPlayer(nicks, k.Killer) {
			kills = append(kills, k)
		}
	}
	return kills
}

// playerNicks finds the player's nicks, each with all its readings: the slot
// name in the most notices, and any frequent one never in a notice with it.
func playerNicks(notices []Kill) [][]NameMask {
	type cluster struct {
		name     NameMask
		readings []NameMask
		notices  map[int]bool
	}
	var clusters []*cluster
	for i, n := range notices {
		for _, name := range []NameMask{n.Killer, n.Assister} {
			if name.W == 0 {
				continue
			}
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
			home.readings = append(home.readings, name)
			home.notices[i] = true
		}
	}
	sort.SliceStable(clusters, func(a, b int) bool { return len(clusters[a].notices) > len(clusters[b].notices) })
	if len(clusters) == 0 || len(clusters[0].notices) < 2 {
		return nil
	}
	var nicks [][]NameMask
	for k, c := range clusters {
		if k > 0 && len(c.notices) < minAliasNotices {
			break
		}
		// Teammates share the player's notices, and so does a rival who kills
		// the player, whose death CS2 can highlight too.
		shared := false
		for i := range c.notices {
			for _, word := range notices[i].Names {
				shared = shared || isPlayer(nicks, word)
			}
		}
		if !shared {
			nicks = append(nicks, c.readings)
		}
	}
	return nicks
}

// isPlayer matches a killer word to any reading of the player's nicks.
func isPlayer(nicks [][]NameMask, word NameMask) bool {
	if word.W == 0 {
		return false
	}
	for _, readings := range nicks {
		for _, r := range readings {
			if sameName(r, word) {
				return true
			}
		}
	}
	return false
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
