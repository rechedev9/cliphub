package tacticalplan

import "math"

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

// roleTally counts what a player did in the opening window of the rounds of one side.
type roleTally struct {
	sampled                  int
	zoneA, zoneB             int
	lurkA, lurkB, lurk, half int
}

func (t *roleTally) add(o roleTally) {
	t.sampled += o.sampled
	t.zoneA += o.zoneA
	t.zoneB += o.zoneB
	t.lurkA += o.lurkA
	t.lurkB += o.lurkB
	t.lurk += o.lurk
	t.half += o.half
}

func (t roleTally) role(side Side) string {
	if t.sampled < MinReliableSample {
		return ""
	}
	n := float64(t.sampled)
	if side == SideCT {
		switch {
		case float64(t.zoneA) >= PositionAnchorShare*n:
			return PositionAnchorA
		case float64(t.zoneB) >= PositionAnchorShare*n:
			return PositionAnchorB
		}
		return PositionRotator
	}
	lurker := float64(t.lurk) >= PositionLurkShare*n
	switch {
	case lurker && t.lurkA >= t.lurkB && t.lurkA > 0:
		return PositionLurkA
	case lurker && t.lurkB > 0:
		return PositionLurkB
	case float64(t.lurk+t.half) >= PositionHalfLurkShare*n:
		return PositionHalfLurk
	}
	return PositionPack
}

// readRoleRound reads one player's opening window of one round into a tally;
// false when the window holds no usable position.
func readRoleRound(doc Document, zones map[string]siteZone, frames []Frame, round Round, slot uint8, side Side) (roleTally, bool) {
	var out roleTally
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
		out.sampled = 1
		switch top {
		case ZoneA:
			out.zoneA = 1
		case ZoneB:
			out.zoneB = 1
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
	out.sampled = 1
	mean := gap / float64(withMates)
	switch {
	case mean >= PositionLurkDistance:
		out.lurk = 1
		switch leanedSite(zones, sumX/float64(seen), sumY/float64(seen)) {
		case ZoneA:
			out.lurkA = 1
		case ZoneB:
			out.lurkB = 1
		}
	case mean >= PositionHalfLurkDistance:
		out.half = 1
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
