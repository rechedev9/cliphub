package tacticalplan

import (
	"cmp"
	"fmt"
	"math"
	"slices"
)

// World units: a lineup is repeated within a step or two, landings spread
// wider, and throws further apart in height stand or land on another floor.
const (
	SpotOriginRadius  = 100
	SpotLandingRadius = 160
	SpotHeight        = 200
)

// spotBuild is a spot while throws join it: its members and running centres.
type spotBuild struct {
	kind    EventKind
	members []int
	origin  *[2]float64
	landing [2]float64
}

// assignSpots groups throws that repeat (same kind, origins and landings close
// together) greedily, oldest match first, comparing each throw with the running
// centre of every spot, and writes the spot's key on each throw. The key names
// the spot's oldest throw, so importing newer matches keeps it; a filter that
// drops that throw (dates, side, opponents) can still change it.
func assignSpots(grenades []TeamGrenade) {
	var spots []*spotBuild
	for _, i := range oldestMatchFirst(grenades) {
		g := grenades[i]
		var into *spotBuild
		for _, s := range spots {
			if spotFits(grenades, s, g) {
				into = s
				break
			}
		}
		if into == nil {
			into = &spotBuild{kind: g.Kind}
			spots = append(spots, into)
		}
		into.members = append(into.members, i)
		into.recentre(grenades)
	}
	for _, s := range spots {
		key := fmt.Sprintf("%s:%s", s.kind, grenadeID(grenades[s.members[0]]))
		for _, i := range s.members {
			grenades[i].Spot = key
		}
	}
}

// oldestMatchFirst is the order of the grenades (listed newest match first)
// with the matches reversed and the game order kept inside each.
func oldestMatchFirst(grenades []TeamGrenade) []int {
	var groups [][]int
	for i, g := range grenades {
		if n := len(groups); n > 0 && grenades[groups[n-1][0]].MatchID == g.MatchID {
			groups[n-1] = append(groups[n-1], i)
			continue
		}
		groups = append(groups, []int{i})
	}
	order := make([]int, 0, len(grenades))
	for k := len(groups) - 1; k >= 0; k-- {
		order = append(order, groups[k]...)
	}
	return order
}

// grenadeID is a stable id of one throw.
func grenadeID(g TeamGrenade) string {
	return fmt.Sprintf("%s:%d:%d:%s", g.MatchID, g.Round, g.Tick, g.ThrowerSteamID64)
}

func (s *spotBuild) recentre(grenades []TeamGrenade) {
	var lx, ly, ox, oy float64
	origins := 0
	for _, i := range s.members {
		g := grenades[i]
		lx, ly = lx+g.Landing[0], ly+g.Landing[1]
		if g.Origin != nil {
			ox, oy = ox+g.Origin[0], oy+g.Origin[1]
			origins++
		}
	}
	n := float64(len(s.members))
	s.landing = [2]float64{lx / n, ly / n}
	s.origin = nil
	if origins > 0 {
		s.origin = &[2]float64{ox / float64(origins), oy / float64(origins)}
	}
}

func spotFits(grenades []TeamGrenade, s *spotBuild, g TeamGrenade) bool {
	if s.kind != g.Kind || !near2(s.landing, g.Landing, SpotLandingRadius) {
		return false
	}
	// Heights against the spot's first throw, so A above B on Nuke never merge.
	if math.Abs(grenades[s.members[0]].Landing[2]-g.Landing[2]) > SpotHeight {
		return false
	}
	// A throw without origin joins on landing alone, so old demos still group.
	if g.Origin == nil || s.origin == nil {
		return true
	}
	for _, i := range s.members {
		if o := grenades[i].Origin; o != nil {
			if math.Abs(o[2]-g.Origin[2]) > SpotHeight {
				return false
			}
			break
		}
	}
	return near2(*s.origin, *g.Origin, SpotOriginRadius)
}

func near2(centre [2]float64, p [3]float64, radius float64) bool {
	return math.Hypot(centre[0]-p[0], centre[1]-p[1]) <= radius
}

// GrenadeSpot is one repeated throw: its grenades, the rounds it was thrown in
// and what the team did after it.
type GrenadeSpot struct {
	Key        string    `json:"key"`
	Kind       EventKind `json:"kind"`
	LandPlace  string    `json:"land_place"`
	ThrowPlace string    `json:"throw_place"`
	Throws     int       `json:"throws"`
	// Throwers is who threw it, most throws first.
	Throwers []GrenadeThrower `json:"throwers"`
	FollowUp FollowUp         `json:"follow_up"`
}

// GrenadeThrower is one player of a spot and how many times they threw it.
type GrenadeThrower struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	Throws    int    `json:"throws"`
}

// GrenadeSpots groups the grenades by their spot key, most rounds first; each
// spot's callouts are those most of its throws name.
func GrenadeSpots(grenades []TeamGrenade, rounds []GrenadeRound) []GrenadeSpot {
	order := []string{}
	members := map[string][]TeamGrenade{}
	for _, g := range grenades {
		if _, ok := members[g.Spot]; !ok {
			order = append(order, g.Spot)
		}
		members[g.Spot] = append(members[g.Spot], g)
	}
	out := make([]GrenadeSpot, 0, len(order))
	for _, key := range order {
		gs := members[key]
		s := GrenadeSpot{Key: key, Kind: gs[0].Kind, Throws: len(gs), FollowUp: FollowUpOf(gs, rounds), Throwers: []GrenadeThrower{}}
		s.LandPlace = mostNamed(gs, func(g TeamGrenade) string { return g.LandPlace })
		s.ThrowPlace = mostNamed(gs, func(g TeamGrenade) string { return g.ThrowPlace })
		byPlayer := map[string]int{}
		for _, g := range gs {
			if _, ok := byPlayer[g.ThrowerSteamID64]; !ok {
				s.Throwers = append(s.Throwers, GrenadeThrower{SteamID64: g.ThrowerSteamID64, Name: g.ThrowerName})
			}
			byPlayer[g.ThrowerSteamID64]++
		}
		for i := range s.Throwers {
			s.Throwers[i].Throws = byPlayer[s.Throwers[i].SteamID64]
		}
		slices.SortStableFunc(s.Throwers, func(a, b GrenadeThrower) int { return cmp.Compare(b.Throws, a.Throws) })
		out = append(out, s)
	}
	slices.SortStableFunc(out, func(a, b GrenadeSpot) int {
		return cmp.Or(cmp.Compare(b.FollowUp.Rounds, a.FollowUp.Rounds), cmp.Compare(b.Throws, a.Throws))
	})
	return out
}

func mostNamed(gs []TeamGrenade, name func(TeamGrenade) string) string {
	counts := map[string]int{}
	best := ""
	for _, g := range gs {
		n := name(g)
		if n == "" {
			continue
		}
		counts[n]++
		if counts[n] > counts[best] || (counts[n] == counts[best] && n < best) {
			best = n
		}
	}
	return best
}
