package worldoverlay

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	// labelLift raises a label above the player's eyes, over the head, in world units.
	labelLift = 14.0
	// labelMinDepth hides the label of a player almost inside the camera.
	labelMinDepth = 48.0
	// spottedHoldTicks keeps an enemy label through short gaps in CS2's spotted flag; hitHoldTicks after a hit.
	spottedHoldTicks = 16
	hitHoldTicks     = 32
	// trailHoldSeconds keeps a trail after its grenade stops; the last trailFadeSeconds fade it out.
	trailHoldSeconds = 1.2
	trailFadeSeconds = 0.6
	// trailStepTicks thins the drawn trail to one point every few ticks.
	trailStepTicks = 2
	labelFont      = "Barlow Semi Condensed Bold"
)

// CS2 side colours and grenade colours, as ASS &HBBGGRR.
var (
	sideColour = map[string]string{"CT": "&HAE795D&", "T": "&H359BDE&"}
	kindColour = map[string]string{
		KindSmoke: "&HE6E6E6&", KindFlash: "&H8CFFFF&", KindHE: "&H4646FF&", KindFire: "&H1E8CFF&", KindDecoy: "&H6ED26E&",
	}
)

// Layout places one segment's script on its capture clip.
type Layout struct {
	// Width and Height are the capture frame; the editor scales the drawing with the picture.
	Width, Height int
	FPS           int
	TickRate      int
	// StartTick is the demo tick of the clip's first frame.
	StartTick       int
	DurationSeconds float64
	Labels, Trails  bool
}

// Script is the ASS subtitle script of one segment on its clip clock: one event set per video
// frame, so labels and trails follow the camera frame by frame. It is empty when nothing shows.
func Script(seg Segment, layout Layout) (string, bool, error) {
	if layout.Width < 320 || layout.Height < 180 || layout.FPS < 1 || layout.TickRate < 1 || layout.DurationSeconds <= 0 {
		return "", false, fmt.Errorf("invalid world overlay layout")
	}
	if !layout.Labels && !layout.Trails {
		return "", false, nil
	}
	s := scriptWriter{layout: layout, sampler: newSampler(seg), scale: float64(layout.Height) / 1080}
	s.spotted, s.hits = enemyTicks(seg)
	s.trails = flights(seg.Grenades)
	fmt.Fprintf(&s.b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nScaledBorderAndShadow: yes\nWrapStyle: 2\n\n", layout.Width, layout.Height)
	s.b.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	fmt.Fprintf(&s.b, "Style: World,%s,%d,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,0,0,7,0,0,0,1\n\n", labelFont, s.px(24))
	s.b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	frames := int(math.Floor(layout.DurationSeconds * float64(layout.FPS)))
	for n := 0; n < frames; n++ {
		s.frame(n, seg.Names)
	}
	return s.b.String(), s.events > 0, nil
}

type scriptWriter struct {
	b       strings.Builder
	layout  Layout
	sampler sampler
	scale   float64
	spotted map[int][]int
	hits    map[int][]int
	trails  []flight
	events  int
	start   string
	end     string
}

func (s *scriptWriter) px(v float64) int { return max(1, int(math.Round(v*s.scale))) }

// frame writes the events of video frame n, timed from halfway to the frame before to halfway to the next.
func (s *scriptWriter) frame(n int, names []string) {
	fps := float64(s.layout.FPS)
	tick := float64(s.layout.StartTick) + float64(n)/fps*float64(s.layout.TickRate)
	camera, ok := s.sampler.camera(tick)
	if !ok {
		return
	}
	v := newView(camera, s.layout.Width, s.layout.Height)
	s.start, s.end = assTime(max(0, (float64(n)-0.5)/fps)), assTime((float64(n)+0.5)/fps)
	if s.start == s.end {
		return // Above 100 fps two frames can share a centisecond.
	}
	if s.layout.Trails {
		for _, f := range s.trails {
			s.trail(v, f, tick)
		}
	}
	if s.layout.Labels {
		for _, p := range s.sampler.players(tick) {
			if p.Side == v.side || s.seenEnemy(p.Slot, tick) {
				s.label(v, p, names[p.Slot])
			}
		}
	}
}

func (s *scriptWriter) event(layer int, text string) {
	fmt.Fprintf(&s.b, "Dialogue: %d,%s,%s,World,,0,0,0,,%s\n", layer, s.start, s.end, text)
	s.events++
}

// label draws the name over the head and a health bar under it, both in the player's side colour.
func (s *scriptWriter) label(v view, p Player, name string) {
	head := p.Eye
	head[2] += labelLift
	if v.depth(head) < labelMinDepth {
		return
	}
	x, y, ok := v.project(head)
	if !ok || !v.onFrame(x, y, 0) {
		return
	}
	colour, found := sideColour[p.Side]
	if !found {
		colour = "&HFFFFFF&"
	}
	barW, barH := float64(s.px(56)), float64(s.px(6))
	left, top := x-barW/2, y-barH
	fill := barW * float64(min(max(p.Health, 0), 100)) / 100
	s.event(1, fmt.Sprintf(`{\an7\pos(0,0)\p2\bord0\shad0\1c&H000000&\1a&H60&}%s{\p0}`, rect(left-1, top-1, barW+2, barH+2)))
	if fill > 0 {
		s.event(2, fmt.Sprintf(`{\an7\pos(0,0)\p2\bord0\shad0\1c%s}%s{\p0}`, colour, rect(left, top, fill, barH)))
	}
	s.event(2, fmt.Sprintf(`{\an2\pos(%d,%d)\bord%d\3c&H000000&\3a&H40&\1c&HFFFFFF&}%s`, int(math.Round(x)), int(math.Round(top-float64(s.px(4)))), s.px(2), assText(name)))
}

// trail draws the grenade's path up to tick as a ribbon, then fades it once the grenade stops.
func (s *scriptWriter) trail(v view, f flight, tick float64) {
	if f.side != v.side || tick < float64(f.points[0].Tick) {
		return
	}
	rest := float64(f.restTick)
	hold := trailHoldSeconds * float64(s.layout.TickRate)
	if tick > rest+hold {
		return
	}
	path := f.until(tick)
	if len(path) < 2 {
		return
	}
	width := float64(s.px(5))
	var d strings.Builder
	for i := 1; i < len(path); i++ {
		a, b, ok := v.clipNear(path[i-1], path[i])
		if !ok {
			continue
		}
		ax, ay, _ := v.project(a)
		bx, by, _ := v.project(b)
		ribbon(&d, ax, ay, bx, by, width)
	}
	if d.Len() == 0 {
		return
	}
	alpha := 0
	if fadeFrom := rest + hold - trailFadeSeconds*float64(s.layout.TickRate); tick > fadeFrom {
		alpha = int(math.Round(255 * (tick - fadeFrom) / (trailFadeSeconds * float64(s.layout.TickRate))))
	}
	s.event(0, fmt.Sprintf(`{\an7\pos(0,0)\p2\bord%d\shad0\1c%s\3c&H000000&\alpha&H%02X&\3a&H%02X&}%s{\p0}`, max(1, s.px(1)), kindColour[f.kind], min(alpha, 255), min(alpha+0xA0, 255), d.String()))
}

// seenEnemy is true when CS2 flagged the enemy as spotted from the POV, or the POV hit it, a moment ago.
// The demo has no line of sight, so an enemy the POV neither spots nor hits keeps no label.
func (s *scriptWriter) seenEnemy(slot int, tick float64) bool {
	return within(s.spotted[slot], tick, spottedHoldTicks) || within(s.hits[slot], tick, hitHoldTicks)
}

// within reports whether a tick of the sorted list lies in the hold before tick.
func within(ticks []int, tick float64, hold int) bool {
	i := sort.Search(len(ticks), func(i int) bool { return float64(ticks[i]) > tick })
	return i > 0 && tick-float64(ticks[i-1]) <= float64(hold)
}

func enemyTicks(seg Segment) (spotted, hits map[int][]int) {
	spotted, hits = map[int][]int{}, map[int][]int{}
	for _, f := range seg.Frames {
		for _, p := range f.Players {
			if p.Spotted {
				spotted[p.Slot] = append(spotted[p.Slot], f.Tick)
			}
			if p.Hit {
				hits[p.Slot] = append(hits[p.Slot], f.Tick)
			}
		}
	}
	return spotted, hits
}

// flight is a grenade's path thinned for drawing, with the tick it came to rest.
type flight struct {
	kind, side string
	points     []Point
	restTick   int
}

func flights(grenades []Grenade) []flight {
	var out []flight
	for _, g := range grenades {
		if len(g.Points) < 2 {
			continue
		}
		rest := g.Points[len(g.Points)-1].Tick
		for i := len(g.Points) - 1; i > 0; i-- {
			if dist(g.Points[i].Pos, g.Points[i-1].Pos) > 0.5 {
				rest = g.Points[i].Tick
				break
			}
		}
		var thin []Point
		for i, p := range g.Points {
			if p.Tick > rest {
				break
			}
			if i == 0 || p.Tick == rest || p.Tick-thin[len(thin)-1].Tick >= trailStepTicks {
				thin = append(thin, p)
			}
		}
		if len(thin) >= 2 {
			out = append(out, flight{kind: g.Kind, side: g.Side, points: thin, restTick: rest})
		}
	}
	return out
}

// until is the path flown by tick, ending at the grenade's interpolated position.
func (f flight) until(tick float64) []Vec {
	var out []Vec
	for i, p := range f.points {
		if float64(p.Tick) > tick {
			prev := f.points[i-1]
			w := (tick - float64(prev.Tick)) / float64(p.Tick-prev.Tick)
			return append(out, lerpVec(prev.Pos, p.Pos, w))
		}
		out = append(out, p.Pos)
	}
	return out
}

// ribbon appends a quad of the given width around segment a-b, in \p2 units (half pixels).
func ribbon(d *strings.Builder, ax, ay, bx, by, width float64) {
	dx, dy := bx-ax, by-ay
	length := math.Hypot(dx, dy)
	if length < 0.25 {
		return
	}
	nx, ny := -dy/length*width/2, dx/length*width/2
	fmt.Fprintf(d, "m %d %d l %d %d %d %d %d %d ", half(ax+nx), half(ay+ny), half(bx+nx), half(by+ny), half(bx-nx), half(by-ny), half(ax-nx), half(ay-ny))
}

func rect(x, y, w, h float64) string {
	return fmt.Sprintf("m %d %d l %d %d %d %d %d %d", half(x), half(y), half(x+w), half(y), half(x+w), half(y+h), half(x), half(y+h))
}

func half(v float64) int { return int(math.Round(v * 2)) }

func dist(a, b Vec) float64 { return math.Sqrt(dot(sub(a, b), sub(a, b))) }

// assTime is h:mm:ss.cc, rounded to the centisecond ASS keeps.
func assTime(seconds float64) string {
	cs := int(math.Round(seconds * 100))
	return fmt.Sprintf("%d:%02d:%02d.%02d", cs/360000, cs/6000%60, cs/100%60, cs%100)
}

// assText keeps a player name from opening an override block or breaking the line.
func assText(name string) string {
	name = strings.NewReplacer("{", "(", "}", ")", `\`, "/", "\n", " ", "\r", " ").Replace(strings.TrimSpace(name))
	if runes := []rune(name); len(runes) > 18 {
		name = string(runes[:17]) + "…"
	}
	return name
}
