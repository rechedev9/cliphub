package tacticalplan

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/placename"
)

// Position rules. They ship inside every report, next to the numbers.
const (
	// PositionWindowSeconds is how long after freeze end a player's position is read.
	PositionWindowSeconds = 25
	// PositionPlaceLimit caps the places listed per side, most frequent first.
	PositionPlaceLimit = 5
	// PositionAnchorShare is the share of sampled CT rounds spent on one site that
	// makes the player that site's anchor.
	PositionAnchorShare = 0.5
	// positionSiteRadius matches the scan's site radius (internal/tactical).
	positionSiteRadius = 500.0
)

// CT position roles, and the zones a round's position falls in.
const (
	PositionAnchorA = "a_anchor"
	PositionAnchorB = "b_anchor"
	PositionRotator = "rotator"
	ZoneA           = "A"
	ZoneB           = "B"
	ZoneOther       = "other"
)

// PositionMatch is one match of the player: its document and, per round
// number, the frames of the opening window (OpeningFrames). Nil frames leave
// the match's rounds unsampled but still count their AWP kills.
type PositionMatch struct {
	MatchID  uuid.UUID
	PlayedAt time.Time
	Document Document
	Opening  map[int][]Frame
}

// PlayerPositionsReport is where a player stands early in their rounds, per
// map and side, with their AWP share.
type PlayerPositionsReport struct {
	SteamID64         string               `json:"steamid64"`
	MatchCount        int                  `json:"match_count"`
	WindowSeconds     int                  `json:"window_seconds"`
	PlaceLimit        int                  `json:"place_limit"`
	AnchorShare       float64              `json:"anchor_share"`
	MinReliableSample int                  `json:"min_reliable_sample"`
	Maps              []PlayerMapPositions `json:"maps"`
}

// PlayerMapPositions is one map, with both sides in CT, T order.
type PlayerMapPositions struct {
	Map     string                `json:"map"`
	Matches int                   `json:"matches"`
	Sides   []PlayerSidePositions `json:"sides"`
}

// PlayerSidePositions is one side of one map. Sampled counts the rounds with a
// position read; zone and place rates are over it. Role is set on CT only.
type PlayerSidePositions struct {
	Side      Side            `json:"side"`
	Rounds    int             `json:"rounds"`
	Sampled   int             `json:"sampled"`
	Role      string          `json:"role,omitempty"`
	Zones     []PositionZone  `json:"zones"`
	Places    []PositionPlace `json:"places"`
	AWPKills  Rate            `json:"awp_kills"`
	AWPRounds Rate            `json:"awp_rounds"`
}

// PositionZone is how many sampled rounds the player spent mostly on a site.
type PositionZone struct {
	Zone   string `json:"zone"`
	Rounds Rate   `json:"rounds"`
}

// PositionPlace is a callout where the player spent most of a round's window.
// Place is the CS2 token; Label is how players say it.
type PositionPlace struct {
	Place    string     `json:"place"`
	Label    string     `json:"label"`
	Rounds   Rate       `json:"rounds"`
	Examples []Evidence `json:"examples"`
}

// OpeningFrames reads, for every round of a document, the frames from freeze
// end to PositionWindowSeconds later. Rounds without frames are left out.
func OpeningFrames(blob []byte, doc Document) (map[int][]Frame, error) {
	header, err := DecodeHeader(blob)
	if err != nil {
		return nil, err
	}
	out := map[int][]Frame{}
	if doc.Demo.Tickrate <= 0 {
		return out, nil
	}
	for _, r := range doc.Rounds {
		offset, ok := doc.Positions.Offset(r.Number)
		if !ok || offset.FrameCount == 0 {
			continue
		}
		frames, err := DecodeFrames(blob, offset.ByteOffset, offset.FrameCount, header)
		if err != nil {
			return nil, fmt.Errorf("round %d: %w", r.Number, err)
		}
		end := r.TickFreezeEnd + int(PositionWindowSeconds*doc.Demo.Tickrate)
		var window []Frame
		for _, frame := range frames {
			if frame.Tick < r.TickFreezeEnd {
				continue
			}
			if frame.Tick > end || (r.TickEnd > 0 && frame.Tick > r.TickEnd) {
				break
			}
			window = append(window, frame)
		}
		if len(window) > 0 {
			out[r.Number] = window
		}
	}
	return out, nil
}

type siteZone struct {
	center [2]float64
	level  string
}

// siteZones places each bombsite at the mean of its plants in the document,
// or at its "Bombsite" callout when nobody planted there.
func siteZones(d Document) map[string]siteZone {
	sums := map[Site][3]float64{}
	counts := map[Site]int{}
	for _, r := range d.Rounds {
		for _, ev := range r.Events {
			if ev.Kind != EventPlant || !isBombsite(ev.Site) {
				continue
			}
			s := sums[ev.Site]
			sums[ev.Site] = [3]float64{s[0] + ev.Pos[0], s[1] + ev.Pos[1], s[2] + ev.Pos[2]}
			counts[ev.Site]++
		}
	}
	out := map[string]siteZone{}
	for site, n := range counts {
		s := sums[site]
		out[string(site)] = siteZone{
			center: [2]float64{s[0] / float64(n), s[1] / float64(n)},
			level:  d.Geometry.Calibration.Level(s[2] / float64(n)),
		}
	}
	for _, c := range d.Geometry.Callouts {
		zone, _ := isSiteCallout(c.Name)
		if _, known := out[zone]; zone != "" && !known {
			out[zone] = siteZone{center: c.Center, level: c.Level}
		}
	}
	return out
}

func zoneOf(zones map[string]siteZone, d Document, s Sample) string {
	for _, zone := range []string{ZoneA, ZoneB} {
		z, ok := zones[zone]
		if !ok || math.Hypot(s.X-z.center[0], s.Y-z.center[1]) > positionSiteRadius {
			continue
		}
		if z.level == "" || d.Geometry.Calibration.Level(s.Z) == z.level {
			return zone
		}
	}
	return ZoneOther
}

// tally counts how often each key was seen and when it was last seen, so the
// most seen key wins and a tie goes to the later one.
type tally struct {
	counts map[string]int
	last   map[string]int
}

func newTally() tally { return tally{counts: map[string]int{}, last: map[string]int{}} }

func (t tally) add(key string, at int) {
	t.counts[key]++
	t.last[key] = at
}

func (t tally) top() string {
	best := ""
	for key, n := range t.counts {
		tie := n == t.counts[best] && (t.last[key] > t.last[best] || (t.last[key] == t.last[best] && key < best))
		if best == "" || n > t.counts[best] || tie {
			best = key
		}
	}
	return best
}

type placeCount struct {
	rounds   int
	examples []Evidence
}

type sidePositionsAcc struct {
	rounds, sampled, kills, awpKills, awpRounds int
	zones                                       map[string]int
	places                                      map[string]*placeCount
}

type mapPositionsAcc struct {
	matches map[uuid.UUID]bool
	sides   map[Side]*sidePositionsAcc
}

func newMapPositionsAcc() *mapPositionsAcc {
	acc := &mapPositionsAcc{matches: map[uuid.UUID]bool{}, sides: map[Side]*sidePositionsAcc{}}
	for _, side := range []Side{SideCT, SideT} {
		acc.sides[side] = &sidePositionsAcc{zones: map[string]int{}, places: map[string]*placeCount{}}
	}
	return acc
}

func (s *sidePositionsAcc) addRound(m PositionMatch, r Round, slot uint8, zones map[string]siteZone) {
	s.rounds++
	awp := false
	for _, ev := range r.Events {
		if ev.Kind != EventKill || ev.ActorSlot == nil || *ev.ActorSlot != slot {
			continue
		}
		s.kills++
		if ev.Weapon == awpWeapon {
			s.awpKills++
			awp = true
		}
	}
	if awp {
		s.awpRounds++
	}
	places, zoneTally := newTally(), newTally()
	seen := false
	for i, frame := range m.Opening[r.Number] {
		for _, sample := range frame.Samples {
			if sample.Slot != slot || !sample.Flags.Has(FlagAlive) {
				continue
			}
			seen = true
			zoneTally.add(zoneOf(zones, m.Document, sample), i)
			if place := NearestCallout(m.Document.Geometry, [3]float64{sample.X, sample.Y, sample.Z}); place != "" {
				places.add(place, i)
			}
		}
	}
	if !seen {
		return
	}
	s.sampled++
	s.zones[zoneTally.top()]++
	if place := places.top(); place != "" {
		acc, ok := s.places[place]
		if !ok {
			acc = &placeCount{}
			s.places[place] = acc
		}
		acc.rounds++
		acc.examples = appendExample(acc.examples, Evidence{MatchID: m.MatchID, Round: r.Number, Tick: r.TickFreezeEnd})
	}
}

func (s *sidePositionsAcc) finish(side Side) PlayerSidePositions {
	out := PlayerSidePositions{
		Side: side, Rounds: s.rounds, Sampled: s.sampled, Zones: []PositionZone{}, Places: []PositionPlace{},
		AWPKills: newRate(s.awpKills, s.kills), AWPRounds: newRate(s.awpRounds, s.rounds),
	}
	for _, zone := range []string{ZoneA, ZoneB, ZoneOther} {
		out.Zones = append(out.Zones, PositionZone{Zone: zone, Rounds: newRate(s.zones[zone], s.sampled)})
	}
	if side == SideCT && s.sampled > 0 {
		out.Role = PositionRotator
		switch {
		case float64(s.zones[ZoneA]) >= PositionAnchorShare*float64(s.sampled):
			out.Role = PositionAnchorA
		case float64(s.zones[ZoneB]) >= PositionAnchorShare*float64(s.sampled):
			out.Role = PositionAnchorB
		}
	}
	for place, acc := range s.places {
		out.Places = append(out.Places, PositionPlace{
			Place: place, Label: placename.Label(place), Rounds: newRate(acc.rounds, s.sampled), Examples: acc.examples,
		})
	}
	sort.Slice(out.Places, func(i, j int) bool {
		if out.Places[i].Rounds.Count != out.Places[j].Rounds.Count {
			return out.Places[i].Rounds.Count > out.Places[j].Rounds.Count
		}
		return out.Places[i].Place < out.Places[j].Place
	})
	if len(out.Places) > PositionPlaceLimit {
		out.Places = out.Places[:PositionPlaceLimit]
	}
	return out
}

func playerSlotOf(d Document, steamID string) (uint8, bool) {
	for _, p := range d.Players {
		if p.SteamID64 == steamID {
			return p.Slot, true
		}
	}
	return 0, false
}

// PlayerPositions reads where a player stands in the first
// PositionWindowSeconds of each round: per round, the callout and the zone
// they were seen in most often while alive. Examples follow the newest match first.
func PlayerPositions(steamID string, matches []PositionMatch) PlayerPositionsReport {
	ordered := append([]PositionMatch(nil), matches...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].PlayedAt.After(ordered[j].PlayedAt) })
	report := PlayerPositionsReport{
		SteamID64: steamID, WindowSeconds: PositionWindowSeconds, PlaceLimit: PositionPlaceLimit,
		AnchorShare: PositionAnchorShare, MinReliableSample: MinReliableSample, Maps: []PlayerMapPositions{},
	}
	maps := map[string]*mapPositionsAcc{}
	for _, m := range ordered {
		slot, ok := playerSlotOf(m.Document, steamID)
		if !ok {
			continue
		}
		report.MatchCount++
		acc, found := maps[m.Document.Demo.Map]
		if !found {
			acc = newMapPositionsAcc()
			maps[m.Document.Demo.Map] = acc
		}
		acc.matches[m.MatchID] = true
		zones := siteZones(m.Document)
		for _, r := range m.Document.Rounds {
			for _, pr := range r.Players {
				if pr.Slot == slot && acc.sides[pr.Side] != nil {
					acc.sides[pr.Side].addRound(m, r, slot, zones)
				}
			}
		}
	}
	for name, acc := range maps {
		report.Maps = append(report.Maps, PlayerMapPositions{
			Map: name, Matches: len(acc.matches),
			Sides: []PlayerSidePositions{acc.sides[SideCT].finish(SideCT), acc.sides[SideT].finish(SideT)},
		})
	}
	sort.Slice(report.Maps, func(i, j int) bool {
		a, b := report.Maps[i], report.Maps[j]
		if a.Matches != b.Matches {
			return a.Matches > b.Matches
		}
		return a.Map < b.Map
	})
	return report
}
