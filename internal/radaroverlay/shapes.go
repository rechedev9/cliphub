package radaroverlay

import (
	"fmt"
	"math"
	"strings"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

// Drawings are written at \p2, so a drawing unit is half a pixel. libass fills overlapping
// contours of the same orientation as one shape and cuts a hole where they turn opposite ways.

// bezierArc is the control distance that bends four cubic Béziers into a circle.
const bezierArc = 0.5523

type pt struct{ x, y float64 }

func unit(v float64) int { return int(math.Round(v * 2)) }

// circleAt is a circle of radius r around (cx, cy), wound right, bottom, left, top.
func circleAt(cx, cy, r float64) string {
	k := r * bezierArc
	p := func(x, y float64) string { return fmt.Sprintf("%d %d", unit(cx+x), unit(cy+y)) }
	return fmt.Sprintf("m %s b %s %s %s b %s %s %s b %s %s %s b %s %s %s ",
		p(r, 0),
		p(r, k), p(k, r), p(0, r),
		p(-k, r), p(-r, k), p(-r, 0),
		p(-r, -k), p(-k, -r), p(0, -r),
		p(k, -r), p(r, -k), p(r, 0))
}

func circle(r float64) string { return circleAt(0, 0, r) }

// polygon closes points into one contour wound like circleAt, so the two fill as one shape.
func polygon(points []pt) string {
	area := 0.0
	for i, p := range points {
		q := points[(i+1)%len(points)]
		area += p.x*q.y - q.x*p.y
	}
	if area < 0 {
		for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
			points[i], points[j] = points[j], points[i]
		}
	}
	var b strings.Builder
	for i, p := range points {
		cmd := "l"
		if i == 0 {
			cmd = "m"
		}
		fmt.Fprintf(&b, "%s %d %d ", cmd, unit(p.x), unit(p.y))
	}
	return b.String()
}

func square(side float64) string {
	h := side / 2
	return polygon([]pt{{-h, -h}, {h, -h}, {h, h}, {-h, h}})
}

// cross is an X of two bars, each size from the centre to a tip and thick wide.
func cross(size, thick float64) string {
	bar := func(dx, dy float64) string {
		n := pt{-dy * thick / 2, dx * thick / 2}
		return polygon([]pt{{-dx*size + n.x, -dy*size + n.y}, {dx*size + n.x, dy*size + n.y}, {dx*size - n.x, dy*size - n.y}, {-dx*size - n.x, -dy*size - n.y}})
	}
	d := math.Sqrt2 / 2
	return bar(d, d) + bar(d, -d)
}

// dotWithWedge is a player's dot with a point toward where they look. CS2's yaw turns counter-clockwise
// from +X, and the radar's Y grows downward, so the screen direction is (cos, -sin).
func dotWithWedge(r, yaw float64) string {
	rad := yaw * math.Pi / 180
	d := pt{math.Cos(rad), -math.Sin(rad)}
	n := pt{-d.y, d.x}
	tip := pt{d.x * r * 1.9, d.y * r * 1.9}
	base := pt{d.x * r * 0.35, d.y * r * 0.35}
	wedge := polygon([]pt{tip, {base.x + n.x*r*0.8, base.y + n.y*r*0.8}, {base.x - n.x*r*0.8, base.y - n.y*r*0.8}})
	return circle(r) + wedge
}

// POVLevel is the overview the filmed player stood on for most of the segment; a single-level map,
// or a segment that never showed the POV, is always the default level.
func (s Segment) POVLevel(cal radarmap.Calibration) string {
	if !cal.MultiLevel() || s.POV < 0 {
		return radarmap.LevelDefault
	}
	lower, seen := 0, 0
	for _, f := range s.Frames {
		if p, ok := find(f.Players, s.POV); ok {
			seen++
			if cal.Level(p.Z) == radarmap.LevelLower {
				lower++
			}
		}
	}
	if seen > 0 && lower*2 > seen {
		return radarmap.LevelLower
	}
	return radarmap.LevelDefault
}
