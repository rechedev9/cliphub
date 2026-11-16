package worldoverlay

import (
	"math"
	"sort"
)

const (
	// defaultFOV is CS2's horizontal field of view on a 4:3 frame; wider frames widen it (Hor+).
	defaultFOV = 90
	// nearDepth drops points closer than this to the eye, where a projection explodes.
	nearDepth = 8.0
	// maxTickGap is the longest gap between two samples that is still interpolated.
	maxTickGap = 4
	// zoomSettleTicks hides the overlay after a zoom change: CS2 animates the FOV meanwhile.
	zoomSettleTicks = 8
)

// view projects world points onto a capture frame of width by height pixels.
type view struct {
	eye            Vec
	fwd, right, up Vec
	focal, cx, cy  float64
	side           string
	width, height  float64
}

// newView builds CS2's first-person projection at the default FOV: the vertical half-angle
// tangent is 0.75 on every aspect ratio, so the focal length is height / 1.5 pixels.
func newView(c Camera, width, height int) view {
	yaw, pitch := c.Yaw*math.Pi/180, normalizePitch(c.Pitch)*math.Pi/180
	sy, cy, sp, cp := math.Sin(yaw), math.Cos(yaw), math.Sin(pitch), math.Cos(pitch)
	h := float64(height)
	return view{
		eye:   c.Eye,
		fwd:   Vec{cp * cy, cp * sy, -sp},
		right: Vec{sy, -cy, 0},
		up:    Vec{sp * cy, sp * sy, cp},
		focal: h / (2 * 0.75 * math.Tan(defaultFOV*math.Pi/360)),
		cx:    float64(width) / 2,
		cy:    h / 2,
		side:  c.Side,
		width: float64(width), height: h,
	}
}

// depth is the distance of p in front of the eye along the view axis.
func (v view) depth(p Vec) float64 { return dot(sub(p, v.eye), v.fwd) }

// project returns p in frame pixels; ok is false behind or too close to the eye.
func (v view) project(p Vec) (x, y float64, ok bool) {
	d := sub(p, v.eye)
	z := dot(d, v.fwd)
	if z < nearDepth {
		return 0, 0, false
	}
	return v.cx + v.focal*dot(d, v.right)/z, v.cy - v.focal*dot(d, v.up)/z, true
}

// onFrame reports whether a pixel lies within margin of the frame.
func (v view) onFrame(x, y, margin float64) bool {
	return x >= -margin && x <= v.width+margin && y >= -margin && y <= v.height+margin
}

// clipNear cuts the part of segment a-b that lies behind the near plane; ok is false when all of it does.
func (v view) clipNear(a, b Vec) (Vec, Vec, bool) {
	da, db := v.depth(a), v.depth(b)
	if da < nearDepth && db < nearDepth {
		return a, b, false
	}
	if da < nearDepth {
		a = lerpVec(a, b, (nearDepth-da)/(db-da))
	} else if db < nearDepth {
		b = lerpVec(a, b, (da-nearDepth)/(da-db))
	}
	return a, b, true
}

// sampler reads a segment at a fractional demo tick, interpolating between the two samples around it.
type sampler struct {
	seg    Segment
	steady []bool
}

func newSampler(seg Segment) sampler {
	steady := make([]bool, len(seg.Frames))
	lastZoom := math.MinInt32
	var lastKey [2]int
	for i, f := range seg.Frames {
		key := [2]int{-1, 0}
		if f.Camera != nil {
			key = [2]int{f.Camera.FOV, boolInt(f.Camera.Scoped)}
			if key[0] == 0 {
				key[0] = defaultFOV // The pawn reports 0 or 90 for the same default view.
			}
		}
		if i > 0 && key != lastKey {
			lastZoom = f.Tick
		}
		lastKey = key
		c := f.Camera
		steady[i] = c != nil && !c.Scoped && (c.FOV == 0 || c.FOV == defaultFOV) && f.Tick-lastZoom >= zoomSettleTicks
	}
	return sampler{seg: seg, steady: steady}
}

// around returns the samples before and after tick and the weight of the second.
func (s sampler) around(tick float64) (int, int, float64, bool) {
	frames := s.seg.Frames
	j := sort.Search(len(frames), func(i int) bool { return float64(frames[i].Tick) > tick })
	if j == 0 {
		return 0, 0, 0, false
	}
	i := j - 1
	if j == len(frames) || float64(frames[i].Tick) == tick {
		return i, i, 0, float64(frames[i].Tick) == tick
	}
	gap := frames[j].Tick - frames[i].Tick
	if gap > maxTickGap {
		return 0, 0, 0, false
	}
	return i, j, (tick - float64(frames[i].Tick)) / float64(gap), true
}

// camera is the POV's eye at tick; ok is false while it is dead, unknown or zoomed.
func (s sampler) camera(tick float64) (Camera, bool) {
	i, j, w, ok := s.around(tick)
	if !ok || !s.steady[i] || !s.steady[j] {
		return Camera{}, false
	}
	a, b := *s.seg.Frames[i].Camera, *s.seg.Frames[j].Camera
	a.Eye = lerpVec(a.Eye, b.Eye, w)
	a.Yaw += angleDelta(a.Yaw, b.Yaw) * w
	a.Pitch = normalizePitch(a.Pitch) + (normalizePitch(b.Pitch)-normalizePitch(a.Pitch))*w
	return a, true
}

// players are the living players at tick, each interpolated when both samples hold it.
func (s sampler) players(tick float64) []Player {
	i, j, w, ok := s.around(tick)
	if !ok {
		return nil
	}
	next := map[int]Player{}
	for _, p := range s.seg.Frames[j].Players {
		next[p.Slot] = p
	}
	var out []Player
	for _, p := range s.seg.Frames[i].Players {
		q, found := next[p.Slot]
		if !found {
			if w > 0 {
				continue // Dead or gone at the next sample.
			}
			q = p
		}
		p.Eye = lerpVec(p.Eye, q.Eye, w)
		out = append(out, p)
	}
	return out
}

func normalizePitch(p float64) float64 {
	p = math.Mod(p, 360)
	if p > 180 {
		p -= 360
	} else if p < -180 {
		p += 360
	}
	return p
}

// angleDelta is the signed shortest turn from a to b in degrees.
func angleDelta(a, b float64) float64 {
	d := math.Mod(b-a, 360)
	if d > 180 {
		d -= 360
	} else if d < -180 {
		d += 360
	}
	return d
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func sub(a, b Vec) Vec             { return Vec{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func dot(a, b Vec) float64         { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func lerp(a, b, w float64) float64 { return a + (b-a)*w }
func lerpVec(a, b Vec, w float64) Vec {
	return Vec{lerp(a[0], b[0], w), lerp(a[1], b[1], w), lerp(a[2], b[2], w)}
}
