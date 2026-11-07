package prolineups

import (
	"cmp"
	"math"
	"slices"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Tolerances that make two throws the same lineup. A player who repeats a
// lineup stands within a few units and aims within about a degree.
const (
	spotRadius   = 40.0
	spotHeight   = 24.0
	aimTolerance = 2.0
)

// landingRadius is how far apart two landings of one lineup may be. Flashes and
// HEs pop in the air along their arc, so they get more room than a smoke.
func landingRadius(kind tacticalplan.EventKind) float64 {
	switch kind {
	case tacticalplan.EventSmoke:
		return 160
	case tacticalplan.EventMolotov:
		return 180
	default:
		return 260
	}
}

// techniqueKey groups movements whose throws fly alike: standing and walking
// release the grenade near enough the same way, running and crouching do not.
func techniqueKey(t Throw) string {
	move := "still"
	switch t.Movement {
	case "running":
		move = "run"
	case "crouching":
		move = "crouch"
	}
	if t.Jump {
		return move + "+jump"
	}
	return move
}

// cluster is a lineup being built: the mean throw spot, aim and landing of its
// members. Yaw is averaged as a unit vector so 179 and -179 meet at 180.
type cluster struct {
	kind      tacticalplan.EventKind
	technique string
	members   []int
	origin    [3]float64
	landing   [3]float64
	yawX      float64
	yawY      float64
	pitch     float64
}

func (c *cluster) yaw() float64 {
	return math.Atan2(c.yawY, c.yawX) * 180 / math.Pi
}

func (c *cluster) add(i int, t Throw) {
	n := float64(len(c.members))
	c.members = append(c.members, i)
	mix := func(old, v float64) float64 { return (old*n + v) / (n + 1) }
	for k := range 3 {
		c.origin[k] = mix(c.origin[k], t.Origin[k])
		c.landing[k] = mix(c.landing[k], t.Landing[k])
	}
	rad := t.Yaw * math.Pi / 180
	c.yawX = mix(c.yawX, math.Cos(rad))
	c.yawY = mix(c.yawY, math.Sin(rad))
	c.pitch = mix(c.pitch, t.Pitch)
}

// sameThrow reports whether t is thrown from the cluster's spot with its aim
// and technique, whatever its landing.
func (c *cluster) sameThrow(t Throw) bool {
	if t.Kind != c.kind || techniqueKey(t) != c.technique {
		return false
	}
	if math.Hypot(t.Origin[0]-c.origin[0], t.Origin[1]-c.origin[1]) > spotRadius || math.Abs(t.Origin[2]-c.origin[2]) > spotHeight {
		return false
	}
	return angleGap(t.Yaw, c.yaw()) <= aimTolerance && math.Abs(t.Pitch-c.pitch) <= aimTolerance
}

func (c *cluster) sameLanding(t Throw) bool {
	return distance(t.Landing, c.landing) <= landingRadius(c.kind)
}

func angleGap(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	return min(d, 360-d)
}

func distance(a, b [3]float64) float64 {
	return math.Sqrt((a[0]-b[0])*(a[0]-b[0]) + (a[1]-b[1])*(a[1]-b[1]) + (a[2]-b[2])*(a[2]-b[2]))
}

// gridKey buckets throws and clusters by kind, technique and a spot cell twice
// the spot radius wide, so a lookup only reads the 3x3 cells around a spot.
type gridKey struct {
	kind      tacticalplan.EventKind
	technique string
	x, y      int
}

func cellOf(kind tacticalplan.EventKind, technique string, origin [3]float64) gridKey {
	size := 2 * spotRadius
	return gridKey{kind: kind, technique: technique, x: int(math.Floor(origin[0] / size)), y: int(math.Floor(origin[1] / size))}
}

func neighbours(k gridKey) []gridKey {
	out := make([]gridKey, 0, 9)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			out = append(out, gridKey{kind: k.kind, technique: k.technique, x: k.x + dx, y: k.y + dy})
		}
	}
	return out
}

// clusterThrows groups one map's throws greedily, oldest first so a lineup's
// centre settles on how it was first thrown; each throw joins the nearest
// matching cluster or starts one.
func clusterThrows(throws []Throw) []*cluster {
	order := make([]int, len(throws))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		if c := throws[a].PlayedAt.Compare(throws[b].PlayedAt); c != 0 {
			return c
		}
		return cmp.Compare(throws[a].ThrowTick, throws[b].ThrowTick)
	})
	out := []*cluster{}
	grid := map[gridKey][]*cluster{}
	for _, i := range order {
		t := throws[i]
		key := cellOf(t.Kind, techniqueKey(t), t.Origin)
		var best *cluster
		bestDist := math.Inf(1)
		for _, k := range neighbours(key) {
			for _, c := range grid[k] {
				if !c.sameThrow(t) || !c.sameLanding(t) {
					continue
				}
				if d := distance(t.Origin, c.origin) + distance(t.Landing, c.landing)/10; d < bestDist {
					best, bestDist = c, d
				}
			}
		}
		if best == nil {
			best = &cluster{kind: t.Kind, technique: techniqueKey(t)}
			out = append(out, best)
			grid[key] = append(grid[key], best)
		}
		best.add(i, t)
	}
	return out
}

// throwGrid buckets throw indexes like clusters, for attempt counts.
func throwGrid(throws []Throw) map[gridKey][]int {
	g := map[gridKey][]int{}
	for i, t := range throws {
		k := cellOf(t.Kind, techniqueKey(t), t.Origin)
		g[k] = append(g[k], i)
	}
	return g
}

// attempts counts the throws from a lineup's spot, aim and technique, wherever
// they landed, and how many of them landed where the lineup lands.
func attempts(c *cluster, throws []Throw, grid map[gridKey][]int) (tries, landed int) {
	for _, k := range neighbours(cellOf(c.kind, c.technique, c.origin)) {
		for _, i := range grid[k] {
			t := throws[i]
			if !c.sameThrow(t) {
				continue
			}
			tries++
			if c.sameLanding(t) {
				landed++
			}
		}
	}
	return tries, landed
}
