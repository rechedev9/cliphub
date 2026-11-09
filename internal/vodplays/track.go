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
	edgeTolerance = 2
	// noticeSeconds is how long CS2 shows a highlighted notice. A notice that
	// detection lost for longer than trackGapSeconds can come back until then.
	noticeSeconds = 8.0
)

// Kill is one highlighted notice seen for the first time, with what could be
// read of its names.
type Kill struct {
	Seconds float64         `json:"seconds"`
	Notice  image.Rectangle `json:"-"`
	Reading `json:"-"`
}

type track struct {
	bounds    image.Rectangle
	first     float64
	last      float64
	hits      int
	confirmed bool
	notice    int
	readings  []Reading
	reading   Reading
	// reads counts the sightings read since the notice was last found.
	reads int
}

// Tracker follows notices across samples and reports each one once.
type Tracker struct {
	tracks []*track
	// lost holds notices no longer seen that can still be on screen.
	lost  []*track
	kills []Kill
}

// readHits is how many sightings of a notice are read for names, again when it
// comes back; the first ones can catch it while it fades or slides in.
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
			tr := t.revive(ring.Bounds)
			if tr == nil {
				tr = &track{bounds: ring.Bounds, first: s.Seconds, last: s.Seconds, hits: 1}
				tr.read(s.Frame, ring.Bounds)
				t.tracks = append(t.tracks, tr)
				used = append(used, true)
				continue
			}
			t.tracks = append(t.tracks, tr)
			used = append(used, true)
			best = len(t.tracks) - 1
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
		if tr.reads < readHits {
			tr.read(s.Frame, ring.Bounds)
		}
		if !tr.confirmed && tr.hits >= confirmHits {
			tr.confirmed = true
			tr.notice = len(t.kills)
			t.kills = append(t.kills, Kill{Seconds: tr.first, Notice: tr.bounds})
		}
		if tr.confirmed {
			t.kills[tr.notice].Reading = tr.reading
		}
	}
}

// read reads the notice once more and keeps the reading most sightings agree
// on: the border or scenery spoils a reading now and then, seldom the same way.
func (tr *track) read(frame []byte, ring image.Rectangle) {
	tr.reads++
	if frame == nil {
		return
	}
	tr.readings = append(tr.readings, readNotice(frame, ring))
	tr.reading = mostAgreed(tr.readings)
}

// mostAgreed picks the reading whose killer the most readings share, then the
// fullest one; a reading that found a killer always beats one that did not.
func mostAgreed(readings []Reading) Reading {
	var best Reading
	bestVotes, bestPixels := -1, -1
	for _, r := range readings {
		votes := 0
		for _, other := range readings {
			if r.Killer.W > 0 && sameName(r.Killer, other.Killer) {
				votes++
			}
		}
		pixels := maskPixels(r.Names)
		if votes > bestVotes || (votes == bestVotes && pixels > bestPixels) {
			best, bestVotes, bestPixels = r, votes, pixels
		}
	}
	return best
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
	// Red scenery can stretch the ring's right edge, never its left one.
	dx := absInt(tracked.Min.X - ring.Min.X)
	if (dw <= matchWidthTolerance || dx <= edgeTolerance) && dh <= matchHeightTolerance {
		return float64(matchHeightTolerance + min(dw, dx) + dh + dy/20), true
	}
	return 0, false
}

// revive takes back a lost notice that a ring shows again. Notices only move up
// as older ones leave and new ones come in below, so it is in its row or above.
func (t *Tracker) revive(ring image.Rectangle) *track {
	for i := len(t.lost) - 1; i >= 0; i-- {
		tr := t.lost[i]
		if absInt(tr.bounds.Min.X-ring.Min.X) <= edgeTolerance &&
			absInt(tr.bounds.Dy()-ring.Dy()) <= matchHeightTolerance &&
			ring.Min.Y <= tr.bounds.Min.Y+matchHeightTolerance {
			t.lost = append(t.lost[:i], t.lost[i+1:]...)
			tr.reads = 0
			return tr
		}
	}
	return nil
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

// Kills returns the notices where the recording player is the killer, in order;
// CS2 also highlights their assists, where another name holds the killer slot.
func (t *Tracker) Kills() []Kill {
	var notices []Kill
	for _, k := range t.kills {
		if !isEcho(notices, k) {
			notices = append(notices, k)
		}
	}
	nicks := playerNicks(notices)
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
	shares := func(c *cluster) bool {
		for i := range c.notices {
			for _, word := range notices[i].Names {
				if isPlayer(nicks, word) {
					return true
				}
			}
		}
		return false
	}
	for k, c := range clusters {
		if k > 0 && len(c.notices) < minAliasNotices {
			break
		}
		// Teammates share the player's notices, and so does a rival who kills
		// the player, whose death CS2 can highlight too.
		if !shares(c) {
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
		switch {
		case now-tr.last <= trackGapSeconds:
			kept = append(kept, tr)
		case tr.confirmed:
			t.lost = append(t.lost, tr)
		}
	}
	t.tracks = kept
	lost := t.lost[:0]
	for _, tr := range t.lost {
		if now-tr.first <= noticeSeconds {
			lost = append(lost, tr)
		}
	}
	t.lost = lost
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
