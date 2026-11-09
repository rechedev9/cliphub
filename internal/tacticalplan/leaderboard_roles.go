package tacticalplan

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/rechedev9/cliphub/internal/placename"
)

// Roles by side, read from where a player stands in the opening window. They
// extend the CT roles of PlayerPositions (anchor, rotator) with the T ones.
const (
	PositionLurkA    = "a_lurk"
	PositionLurkB    = "b_lurk"
	PositionHalfLurk = "half_lurk"
	PositionPack     = "pack"
)

// T role rules. A round is lurked when the player stays this far from the
// nearest living teammate, on average over the window, in map units (height
// counts, so a teammate one floor up on Nuke is not close).
const (
	PositionLurkDistance     = 1200.0
	PositionHalfLurkDistance = 700.0
	// PositionLurkShare is the share of sampled T rounds lurked that makes a lurker.
	PositionLurkShare = 0.4
	// PositionHalfLurkShare is the share of rounds lurked or half lurked that makes a half lurker.
	PositionHalfLurkShare = 0.5
	// minTeammateFrames is the frames with a living teammate a T round needs to be read.
	minTeammateFrames = 3
)

// Map roles name where on the map a player spends the window, by callout. The
// callout holding at least MapRoleShare of a side's rounds is the player's map
// role there; a player with no such callout is an allrounder.
const (
	MapRoleAllround = "allround"
	MapRoleShare    = 0.4
	// roleKeyPrefix and placeKeyPrefix key the rounds a player's role or position covers.
	roleKeyPrefix  = "role:"
	placeKeyPrefix = "place:"
	// breakdownPlaces caps the positions listed in a breakdown.
	breakdownPlaces = 4
	// storedPlaces caps the positions a stored pro line keeps.
	storedPlaces = 8
)

// PlayerRoles is a player's role on each side of one map; empty when the sample is too small.
type PlayerRoles struct {
	CT string `json:"ct,omitempty"`
	T  string `json:"t,omitempty"`
}

// RoleShare is a role or a position and the share of the sampled rounds it covers, in percent.
type RoleShare struct {
	Key   string  `json:"key"`
	Label string  `json:"label,omitempty"`
	Pct   float64 `json:"pct"`
}

// SideBreakdown is how a player's rounds on one side split between roles and
// between positions.
type SideBreakdown struct {
	Role    string      `json:"role,omitempty"`
	Roles   []RoleShare `json:"roles"`
	MapRole string      `json:"map_role,omitempty"`
	Places  []RoleShare `json:"places"`
}

// PlayerBreakdown is the SideBreakdown of each side.
type PlayerBreakdown struct {
	CT *SideBreakdown `json:"ct,omitempty"`
	T  *SideBreakdown `json:"t,omitempty"`
}

// MapRoleOption is a map role a board can filter on, with the players who have it.
type MapRoleOption struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Players int    `json:"players"`
}

// RoleSide is the side a role is played on, or empty for an unknown role.
func RoleSide(role string) Side {
	switch role {
	case PositionAnchorA, PositionAnchorB, PositionRotator:
		return SideCT
	case PositionLurkA, PositionLurkB, PositionHalfLurk, PositionPack:
		return SideT
	}
	return ""
}

// RoleFilterSide is the side a board reads when it filters on a role or a map
// role. Both are read on one map and played on one side, so they need a map; a
// role needs a matching side (or none, which the role then sets) and a map role
// needs the side. An empty role keeps the side as given.
func RoleFilterSide(role, mapRole string, side Side, haveMap bool) (Side, error) {
	if role != "" {
		roleSide := RoleSide(role)
		switch {
		case roleSide == "":
			return side, fmt.Errorf("role must be a_anchor, b_anchor, rotator, a_lurk, b_lurk, half_lurk or pack, not %q", role)
		case !haveMap:
			return side, errors.New("role needs a map: a role is read on one map")
		case side != "" && side != roleSide:
			return side, fmt.Errorf("role %s is played on %s, not %s", role, roleSide, side)
		}
		side = roleSide
	}
	switch {
	case mapRole == "":
	case !haveMap:
		return side, errors.New("map_role needs a map: a position is read on one map")
	case side == "":
		return side, errors.New("map_role needs a side: the same position means different things on CT and T")
	}
	return side, nil
}

// RoleCounts counts what a player did in the opening window of the rounds of
// one side: the rounds read, the CT rounds held on each site and the T rounds
// lurked (toward each site) or half lurked, and the rounds spent at each
// callout. Counts add up across matches.
type RoleCounts struct {
	Sampled int            `json:"sampled"`
	InA     int            `json:"in_a,omitempty"`
	InB     int            `json:"in_b,omitempty"`
	Other   int            `json:"other,omitempty"`
	LurkA   int            `json:"lurk_a,omitempty"`
	LurkB   int            `json:"lurk_b,omitempty"`
	Lurk    int            `json:"lurk,omitempty"`
	Half    int            `json:"half,omitempty"`
	Places  map[string]int `json:"places,omitempty"`
}

func (t *RoleCounts) add(o RoleCounts) {
	t.Sampled += o.Sampled
	t.InA += o.InA
	t.InB += o.InB
	t.Other += o.Other
	t.LurkA += o.LurkA
	t.LurkB += o.LurkB
	t.Lurk += o.Lurk
	t.Half += o.Half
	if len(o.Places) > 0 && t.Places == nil {
		t.Places = map[string]int{}
	}
	for place, n := range o.Places {
		t.Places[place] += n
	}
}

// trimmed keeps the storedPlaces most frequent callouts, so a stored line stays small.
func (t RoleCounts) trimmed() RoleCounts {
	if len(t.Places) <= storedPlaces {
		return t
	}
	keep := map[string]int{}
	for _, p := range t.topPlaces(storedPlaces) {
		keep[p.place] = p.rounds
	}
	t.Places = keep
	return t
}

type placeRounds struct {
	place  string
	rounds int
}

// topPlaces lists the callouts by rounds, most first, ties by name.
func (t RoleCounts) topPlaces(limit int) []placeRounds {
	out := make([]placeRounds, 0, len(t.Places))
	for place, n := range t.Places {
		out = append(out, placeRounds{place, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].rounds != out[j].rounds {
			return out[i].rounds > out[j].rounds
		}
		return out[i].place < out[j].place
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (t RoleCounts) role(side Side) string {
	if t.Sampled < MinReliableSample {
		return ""
	}
	n := float64(t.Sampled)
	if side == SideCT {
		switch {
		case float64(t.InA) >= PositionAnchorShare*n:
			return PositionAnchorA
		case float64(t.InB) >= PositionAnchorShare*n:
			return PositionAnchorB
		}
		return PositionRotator
	}
	lurker := float64(t.Lurk) >= PositionLurkShare*n
	switch {
	case lurker && t.LurkA >= t.LurkB && t.LurkA > 0:
		return PositionLurkA
	case lurker && t.LurkB > 0:
		return PositionLurkB
	case float64(t.Lurk+t.Half) >= PositionHalfLurkShare*n:
		return PositionHalfLurk
	}
	return PositionPack
}

// mapRole is the callout holding MapRoleShare of the rounds, else allround;
// empty when the sample is too small.
func (t RoleCounts) mapRole() string {
	if t.Sampled < MinReliableSample {
		return ""
	}
	top := t.topPlaces(1)
	if len(top) == 1 && float64(top[0].rounds) >= MapRoleShare*float64(t.Sampled) {
		return top[0].place
	}
	return MapRoleAllround
}

func pct(n, of int) float64 {
	if of <= 0 {
		return 0
	}
	return math.Round(1000*float64(n)/float64(of)) / 10
}

// roleShares splits the sampled rounds between the roles of the side.
func (t RoleCounts) roleShares(side Side) []RoleShare {
	if side == SideCT {
		return []RoleShare{{Key: PositionAnchorA, Pct: pct(t.InA, t.Sampled)}, {Key: PositionAnchorB, Pct: pct(t.InB, t.Sampled)}, {Key: PositionRotator, Pct: pct(t.Other, t.Sampled)}}
	}
	lurkNoSite := max(t.Lurk-t.LurkA-t.LurkB, 0)
	return []RoleShare{
		{Key: PositionLurkA, Pct: pct(t.LurkA, t.Sampled)}, {Key: PositionLurkB, Pct: pct(t.LurkB, t.Sampled)},
		{Key: PositionHalfLurk, Pct: pct(t.Half+lurkNoSite, t.Sampled)}, {Key: PositionPack, Pct: pct(max(t.Sampled-t.Lurk-t.Half, 0), t.Sampled)},
	}
}

func (t RoleCounts) breakdown(side Side) *SideBreakdown {
	if t.Sampled < MinReliableSample {
		return nil
	}
	out := &SideBreakdown{Role: t.role(side), Roles: t.roleShares(side), MapRole: t.mapRole(), Places: []RoleShare{}}
	for _, p := range t.topPlaces(breakdownPlaces) {
		out.Places = append(out.Places, RoleShare{Key: p.place, Label: placename.Label(p.place), Pct: pct(p.rounds, t.Sampled)})
	}
	return out
}

// roundRead is one player's opening window of one round: its counts, the role
// the round shows and the callout the player held most.
type roundRead struct {
	counts RoleCounts
	role   string
	place  string
}

// keys are the keys the round is filed under for a role or position filter.
func (r roundRead) keys() []string {
	out := []string{}
	if r.role != "" {
		out = append(out, roleKeyPrefix+r.role)
	}
	if r.place != "" {
		out = append(out, placeKeyPrefix+r.place)
	}
	return out
}

func dist3(ax, ay, az, bx, by, bz float64) float64 {
	return math.Sqrt((ax-bx)*(ax-bx) + (ay-by)*(ay-by) + (az-bz)*(az-bz))
}

// readRoleRound reads one player's opening window of one round; false when the
// window holds no usable position.
func readRoleRound(doc Document, zones map[string]siteZone, frames []Frame, round Round, slot uint8, side Side) (roundRead, bool) {
	var out roundRead
	places := newTally()
	zoneTally := newTally()
	for i, frame := range frames {
		for _, s := range frame.Samples {
			if s.Slot != slot || !s.Flags.Has(FlagAlive) {
				continue
			}
			zoneTally.add(zoneOf(zones, doc, s), i)
			if place := NearestCallout(doc.Geometry, [3]float64{s.X, s.Y, s.Z}); place != "" {
				places.add(place, i)
			}
		}
	}
	out.place = places.top()
	if out.place != "" {
		out.counts.Places = map[string]int{out.place: 1}
	}
	if side == SideCT {
		top := zoneTally.top()
		if top == "" {
			return roundRead{}, false
		}
		out.counts.Sampled = 1
		switch top {
		case ZoneA:
			out.counts.InA, out.role = 1, PositionAnchorA
		case ZoneB:
			out.counts.InB, out.role = 1, PositionAnchorB
		default:
			out.counts.Other, out.role = 1, PositionRotator
		}
		return out, true
	}
	mates := map[uint8]bool{}
	for _, pr := range round.Players {
		if pr.Side == side && pr.Slot != slot {
			mates[pr.Slot] = true
		}
	}
	var gap, sumX, sumY float64
	var seen, withMates int
	for _, frame := range frames {
		var me *Sample
		for i := range frame.Samples {
			if frame.Samples[i].Slot == slot && frame.Samples[i].Flags.Has(FlagAlive) {
				me = &frame.Samples[i]
			}
		}
		if me == nil {
			continue
		}
		seen++
		sumX += me.X
		sumY += me.Y
		nearest := math.Inf(1)
		for _, s := range frame.Samples {
			if mates[s.Slot] && s.Flags.Has(FlagAlive) {
				nearest = math.Min(nearest, dist3(s.X, s.Y, s.Z, me.X, me.Y, me.Z))
			}
		}
		if !math.IsInf(nearest, 1) {
			withMates++
			gap += nearest
		}
	}
	if withMates < minTeammateFrames {
		return roundRead{}, false
	}
	out.counts.Sampled = 1
	mean := gap / float64(withMates)
	switch {
	case mean >= PositionLurkDistance:
		out.counts.Lurk = 1
		out.role = PositionHalfLurk
		switch leanedSite(zones, sumX/float64(seen), sumY/float64(seen)) {
		case ZoneA:
			out.counts.LurkA, out.role = 1, PositionLurkA
		case ZoneB:
			out.counts.LurkB, out.role = 1, PositionLurkB
		}
	case mean >= PositionHalfLurkDistance:
		out.counts.Half, out.role = 1, PositionHalfLurk
	default:
		out.role = PositionPack
	}
	return out, true
}

// leanedSite is the bombsite nearest to a point, or empty when the map has none read.
func leanedSite(zones map[string]siteZone, x, y float64) string {
	best, bestDist := "", math.Inf(1)
	for _, zone := range []string{ZoneA, ZoneB} {
		z, ok := zones[zone]
		if !ok {
			continue
		}
		if d := math.Hypot(x-z.center[0], y-z.center[1]); d < bestDist {
			best, bestDist = zone, d
		}
	}
	return best
}

// isSiteCallout reports whether a callout token names a bombsite, whatever its case.
func isSiteCallout(name string) (string, bool) {
	switch strings.ToLower(name) {
	case "bombsitea":
		return ZoneA, true
	case "bombsiteb":
		return ZoneB, true
	}
	return "", false
}
