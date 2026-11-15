package radaroverlay

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

// World radii and timings the 2D Replay draws utility with (internal/replaygif).
const (
	smokeRadius = 144.0
	fireRadius  = 120.0
	// burstSteps rings of burstStepSeconds each draw a flash or HE going off.
	burstSteps       = 3
	burstStepSeconds = 0.15
	// teleportUnits is a jump between two samples no player can run: a respawn, never a move.
	teleportUnits = 400.0
	fadeMillis    = 150
	utilityFadeMS = 400
)

// CS2 side colours of the 2D Replay and the overlay inks, as ASS &HBBGGRR&.
var sideInk = map[string]string{"CT": "&HEC9C5D&", "T": "&H3CA2E6&"}

const (
	inkSmoke   = "&HD2CDC8&"
	inkFire    = "&H2468FF&"
	inkFlash   = "&HFFFFFF&"
	inkHE      = "&H2D50FF&"
	inkBomb    = "&H303BE5&"
	inkOutline = "&H060505&"
	inkPOV     = "&HFFFFFF&"
	inkNeutral = "&HB4B4B4&"
)

// Layout places the radar on the output frame.
type Layout struct {
	Width, Height int
	TickRate      int
	Calibration   radarmap.Calibration
	Box           Rect
	Opacity       float64
}

// Span is one captured play on the edited timeline: timeline second Offset shows demo tick
// StartTick, and the radar is visible from From to Until. Level is the overview the plate shows.
type Span struct {
	Segment     Segment
	StartTick   int
	Offset      float64
	From, Until float64
	Level       string
}

// Script is the ASS script of the radar's moving layer over every span, on the edited timeline;
// it is empty when nothing shows. The plate with the map art is drawn under it by the editor.
func Script(layout Layout, spans []Span) (string, error) {
	if layout.Width < 320 || layout.Height < 180 || layout.TickRate < 1 || !layout.Calibration.Valid() || layout.Box.W < 32 {
		return "", fmt.Errorf("invalid radar layout")
	}
	w := &writer{l: layout, dot: math.Max(3, float64(layout.Box.W)*0.019)}
	b := &w.b
	fmt.Fprintf(b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nScaledBorderAndShadow: yes\nWrapStyle: 2\n\n", layout.Width, layout.Height)
	b.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	b.WriteString("Style: Radar,Barlow Semi Condensed Bold,20,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,0,0,7,0,0,0,1\n\n")
	b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for _, s := range spans {
		if s.Until-s.From < 0.05 {
			continue
		}
		v := view{Span: s, tickRate: float64(layout.TickRate)}
		w.utility(v)
		w.deaths(v)
		w.bomb(v)
		w.players(v)
	}
	if w.events == 0 {
		return "", nil
	}
	return w.b.String(), nil
}

// view converts between a span's demo ticks and the edited timeline.
type view struct {
	Span
	tickRate float64
}

func (v view) time(tick int) float64 { return v.Offset + float64(tick-v.StartTick)/v.tickRate }

// visible is the part of [start, end] the span shows, and whether there is any.
func (v view) visible(start, end float64) (float64, float64, bool) {
	a, z := max(start, v.From), min(end, v.Until)
	return a, z, z-a >= 0.005
}

type writer struct {
	b      strings.Builder
	l      Layout
	dot    float64
	events int
}

// point turns a world position into frame pixels inside the radar box.
func (w *writer) point(x, y float64) (float64, float64) {
	px, py := w.l.Calibration.WorldToPixel(x, y)
	k := float64(w.l.Box.W) / float64(w.l.Calibration.Size)
	return float64(w.l.Box.X) + px*k, float64(w.l.Box.Y) + py*k
}

// units turns world units into frame pixels.
func (w *writer) units(world float64) float64 {
	return world / w.l.Calibration.Scale * float64(w.l.Box.W) / float64(w.l.Calibration.Size)
}

// alpha is the ASS transparency of an ink drawn at base transparency, faded by the radar's opacity
// and dimmed on a level the plate does not show.
func (w *writer) alpha(base int, v view, z float64) int {
	opacity := w.l.Opacity
	if c := w.l.Calibration; c.MultiLevel() && c.Level(z) != v.Level {
		opacity *= 0.4
	}
	return min(255, int(math.Round(255-float64(255-base)*opacity)))
}

type paint struct {
	fill, rim         string
	fillAlpha, rimAlp int
	border            float64
}

// shape writes one drawing event: pos is \pos or \move, fade a \fad tag or empty.
func (w *writer) shape(layer int, start, end float64, pos, fade string, p paint, drawing string) {
	a, z := assTime(start), assTime(end)
	if a == z {
		return
	}
	box := w.l.Box
	fmt.Fprintf(&w.b, `Dialogue: %d,%s,%s,Radar,,0,0,0,,{\an7%s%s\clip(%d,%d,%d,%d)\p2\bord%.1f\shad0\1c%s\3c%s\1a&H%02X&\3a&H%02X&}%s{\p0}`+"\n",
		layer, a, z, pos, fade, box.X, box.Y, box.X+box.W, box.Y+box.H, p.border, p.fill, p.rim, p.fillAlpha, p.rimAlp, drawing)
	w.events++
}

// fadeTag fades an event in when it starts at the span's first frame, or as its utility appears or goes.
func fadeTag(inMS, outMS int) string {
	if inMS == 0 && outMS == 0 {
		return ""
	}
	return fmt.Sprintf(`\fad(%d,%d)`, inMS, outMS)
}

func (v view) entryFade(start float64) int {
	if math.Abs(start-v.From) < 0.001 {
		return fadeMillis
	}
	return 0
}

// utility draws smokes and fires while they burn and a short burst where a flash or HE goes off.
func (w *writer) utility(v view) {
	for _, u := range v.Segment.Utility {
		cx, cy := w.point(u.X, u.Y)
		pos := fmt.Sprintf(`\pos(%.1f,%.1f)`, cx, cy)
		switch u.Kind {
		case KindSmoke, KindFire:
			endTick := u.EndTick
			if endTick == 0 {
				endTick = v.Segment.TickEnd
			}
			start, end := v.time(u.Tick), v.time(endTick)
			a, z, ok := v.visible(start, end)
			if !ok {
				continue
			}
			in, out := v.entryFade(a), 0
			if a == start {
				in = utilityFadeMS
			}
			if z == end && u.EndTick != 0 {
				out = utilityFadeMS
			}
			p := paint{fill: inkSmoke, rim: "&H5A5A5A&", fillAlpha: w.alpha(0x40, v, u.Z), rimAlp: w.alpha(0x60, v, u.Z), border: 1}
			radius := smokeRadius
			if u.Kind == KindFire {
				p = paint{fill: inkFire, rim: "&H1E46D2&", fillAlpha: w.alpha(0x68, v, u.Z), rimAlp: w.alpha(0x20, v, u.Z), border: 1.5}
				radius = fireRadius
			}
			w.shape(0, a, z, pos, fadeTag(min(in, int((z-a)*500)), min(out, int((z-a)*500))), p, circle(w.units(radius)))
		case KindFlash, KindHE:
			ink, reach := inkFlash, 110.0
			if u.Kind == KindHE {
				ink, reach = inkHE, 100.0
			}
			for step := range burstSteps {
				start := v.time(u.Tick) + float64(step)*burstStepSeconds
				a, z, ok := v.visible(start, start+burstStepSeconds)
				if !ok {
					continue
				}
				share := float64(step+1) / burstSteps
				base := 0x30 + step*0x40
				p := paint{fill: ink, rim: ink, fillAlpha: w.alpha(base, v, u.Z), rimAlp: 255, border: 0}
				w.shape(1, a, z, pos, "", p, circle(w.units(reach*share)))
			}
		}
	}
}

// deaths marks where each player of the round died with a cross in its side colour.
func (w *writer) deaths(v view) {
	for _, d := range v.Segment.Deaths {
		end := v.Until
		if d.Until != 0 {
			end = v.time(d.Until)
		}
		a, z, ok := v.visible(v.time(d.Tick), end)
		if !ok {
			continue
		}
		x, y := w.point(d.X, d.Y)
		ink := cmpInk(sideInk[d.Side])
		p := paint{fill: ink, rim: inkOutline, fillAlpha: w.alpha(0x50, v, d.Z), rimAlp: w.alpha(0x90, v, d.Z), border: 1}
		w.shape(2, a, z, fmt.Sprintf(`\pos(%.1f,%.1f)`, x, y), fadeTag(v.entryFade(a), 0), p, cross(w.dot*0.85, math.Max(1.6, w.dot*0.35)))
	}
}

// bomb draws the C4 on the ground or planted, and a marker on the player who carries it.
func (w *writer) bomb(v view) {
	frames := v.Segment.Frames
	for i := 0; i < len(frames); {
		b := frames[i].Bomb
		j := i + 1
		if b == nil || b.State == BombCarried {
			if b != nil {
				w.carried(v, i, b.Carrier)
			}
			i = j
			continue
		}
		for j < len(frames) && sameGround(frames[j].Bomb, b) {
			j++
		}
		endTick := v.Segment.TickEnd
		if j < len(frames) {
			endTick = frames[j].Tick
		}
		startTick := frames[i].Tick
		i = j
		a, z, ok := v.visible(v.time(startTick), v.time(endTick))
		if !ok {
			continue
		}
		x, y := w.point(b.X, b.Y)
		p := paint{fill: inkBomb, rim: inkOutline, fillAlpha: w.alpha(0, v, b.Z), rimAlp: w.alpha(0x20, v, b.Z), border: 1.2}
		drawing := square(w.dot * 1.15)
		if b.State == BombPlanted {
			p.rim, p.border, drawing = inkPOV, 1.6, circle(w.dot*0.95)
		}
		w.shape(3, a, z, fmt.Sprintf(`\pos(%.1f,%.1f)`, x, y), fadeTag(v.entryFade(a), 0), p, drawing)
	}
}

func sameGround(b, ref *Bomb) bool {
	return b != nil && b.State == ref.State && math.Abs(b.X-ref.X) < 1 && math.Abs(b.Y-ref.Y) < 1
}

// carried pins a small bomb mark beside the carrier's dot for one sample interval.
func (w *writer) carried(v view, i, carrier int) {
	frames := v.Segment.Frames
	from, ok := find(frames[i].Players, carrier)
	if !ok {
		return
	}
	move, a, z, ok := w.track(v, i, from)
	if !ok {
		return
	}
	p := paint{fill: inkBomb, rim: inkOutline, fillAlpha: w.alpha(0, v, from.Z), rimAlp: w.alpha(0x20, v, from.Z), border: 1}
	off := w.dot * 0.95
	w.shape(5, a, z, move, "", p, circleAt(off, -off, w.dot*0.5))
}

// players draws each living player's dot and view wedge, moving between samples.
func (w *writer) players(v view) {
	pov := v.Segment.POV
	for i, f := range v.Segment.Frames {
		for _, p := range f.Players {
			move, a, z, ok := w.track(v, i, p)
			if !ok {
				continue
			}
			ink := cmpInk(sideInk[p.Side])
			paintP := paint{fill: ink, rim: inkOutline, fillAlpha: w.alpha(0, v, p.Z), rimAlp: w.alpha(0x10, v, p.Z), border: 1.2}
			layer := 4
			if p.Slot == pov {
				paintP.rim, paintP.border, layer = inkPOV, 2.2, 6
			}
			w.shape(layer, a, z, move, fadeTag(v.entryFade(a), 0), paintP, dotWithWedge(w.dot, p.Yaw))
		}
	}
}

// track is the \pos or \move of player p from frame i to the next sample, on the visible timeline.
func (w *writer) track(v view, i int, p Player) (string, float64, float64, bool) {
	frames := v.Segment.Frames
	endTick := v.Segment.TickEnd
	next, moving := p, false
	if i+1 < len(frames) {
		endTick = frames[i+1].Tick
		if n, ok := find(frames[i+1].Players, p.Slot); ok && math.Hypot(n.X-p.X, n.Y-p.Y) < teleportUnits {
			next, moving = n, true
		}
	}
	t0, t1 := v.time(frames[i].Tick), v.time(endTick)
	if t1 <= t0 {
		return "", 0, 0, false
	}
	a, z, ok := v.visible(t0, t1)
	if !ok {
		return "", 0, 0, false
	}
	x0, y0 := w.point(lerp(p.X, next.X, (a-t0)/(t1-t0)), lerp(p.Y, next.Y, (a-t0)/(t1-t0)))
	if !moving {
		return fmt.Sprintf(`\pos(%.1f,%.1f)`, x0, y0), a, z, true
	}
	x1, y1 := w.point(lerp(p.X, next.X, (z-t0)/(t1-t0)), lerp(p.Y, next.Y, (z-t0)/(t1-t0)))
	return fmt.Sprintf(`\move(%.1f,%.1f,%.1f,%.1f)`, x0, y0, x1, y1), a, z, true
}

func find(players []Player, slot int) (Player, bool) {
	i := sort.Search(len(players), func(i int) bool { return players[i].Slot >= slot })
	if i < len(players) && players[i].Slot == slot {
		return players[i], true
	}
	for _, p := range players {
		if p.Slot == slot {
			return p, true
		}
	}
	return Player{}, false
}

func cmpInk(ink string) string {
	if ink == "" {
		return inkNeutral
	}
	return ink
}

func lerp(a, b, f float64) float64 { return a + (b-a)*f }

// assTime is h:mm:ss.cc, rounded to the centisecond ASS keeps.
func assTime(seconds float64) string {
	cs := int(math.Round(max(0, seconds) * 100))
	return fmt.Sprintf("%d:%02d:%02d.%02d", cs/360000, cs/6000%60, cs/100%60, cs%100)
}
