package tacticalplan

import (
	"errors"
	"fmt"
	"math"
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
// nearest living teammate, on average over the window, in map units.
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

// PlayerRoles is a player's role on each side of one map; empty when the sample is too small.
type PlayerRoles struct {
	CT string `json:"ct,omitempty"`
	T  string `json:"t,omitempty"`
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

// RoleFilterSide is the side a board reads when it filters on a role. A role is
// read on one map and played on one side, so it needs a map and a matching side
// (or none, which the role then sets). An empty role keeps the side as given.
func RoleFilterSide(role string, side Side, haveMap bool) (Side, error) {
	if role == "" {
		return side, nil
	}
	roleSide := RoleSide(role)
	switch {
	case roleSide == "":
		return side, fmt.Errorf("role must be a_anchor, b_anchor, rotator, a_lurk, b_lurk, half_lurk or pack, not %q", role)
	case !haveMap:
		return side, errors.New("role needs a map: a role is read on one map")
	case side != "" && side != roleSide:
		return side, fmt.Errorf("role %s is played on %s, not %s", role, roleSide, side)
	}
	return roleSide, nil
}

// RoleCounts counts what a player did in the opening window of the rounds of
// one side: the rounds read, the CT rounds held on each site and the T rounds
// lurked (toward each site) or half lurked. Counts add up across matches.
type RoleCounts struct {
	Sampled int `json:"sampled"`
	InA     int `json:"in_a,omitempty"`
	InB     int `json:"in_b,omitempty"`
	LurkA   int `json:"lurk_a,omitempty"`
	LurkB   int `json:"lurk_b,omitempty"`
	Lurk    int `json:"lurk,omitempty"`
	Half    int `json:"half,omitempty"`
}

func (t *RoleCounts) add(o RoleCounts) {
	t.Sampled += o.Sampled
	t.InA += o.InA
	t.InB += o.InB
	t.LurkA += o.LurkA
	t.LurkB += o.LurkB
	t.Lurk += o.Lurk
	t.Half += o.Half
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

// readRoleRound reads one player's opening window of one round into a tally;
// false when the window holds no usable position.
func readRoleRound(doc Document, zones map[string]siteZone, frames []Frame, round Round, slot uint8, side Side) (RoleCounts, bool) {
	var out RoleCounts
	if side == SideCT {
		zoneTally := newTally()
		for i, frame := range frames {
			for _, s := range frame.Samples {
				if s.Slot == slot && s.Flags.Has(FlagAlive) {
					zoneTally.add(zoneOf(zones, doc, s), i)
				}
			}
		}
		top := zoneTally.top()
		if top == "" {
			return out, false
		}
		out.Sampled = 1
		switch top {
		case ZoneA:
			out.InA = 1
		case ZoneB:
			out.InB = 1
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
				nearest = math.Min(nearest, math.Hypot(s.X-me.X, s.Y-me.Y))
			}
		}
		if !math.IsInf(nearest, 1) {
			withMates++
			gap += nearest
		}
	}
	if withMates < minTeammateFrames {
		return out, false
	}
	out.Sampled = 1
	mean := gap / float64(withMates)
	switch {
	case mean >= PositionLurkDistance:
		out.Lurk = 1
		switch leanedSite(zones, sumX/float64(seen), sumY/float64(seen)) {
		case ZoneA:
			out.LurkA = 1
		case ZoneB:
			out.LurkB = 1
		}
	case mean >= PositionHalfLurkDistance:
		out.Half = 1
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
