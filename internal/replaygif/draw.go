package replaygif

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// The Replay's colours: blue is CT only, amber T only, violet the mark pen.
var (
	plateColor = color.RGBA{0x0b, 0x0c, 0x0e, 0xff}
	ctColor    = color.RGBA{0x5d, 0x9c, 0xec, 0xff}
	tColor     = color.RGBA{0xe6, 0xa2, 0x3c, 0xff}
)

// inks are palette indexes of the colours the overlay draws with.
type inks struct {
	plate, outline, white, ct, t, dead, smoke, fire, bomb, mark uint8
}

func newInks(pal color.Palette) inks {
	at := func(c color.Color) uint8 { return uint8(pal.Index(c)) }
	return inks{
		plate:   at(plateColor),
		outline: at(color.RGBA{0x05, 0x05, 0x06, 0xff}),
		white:   at(color.RGBA{0xf3, 0xf4, 0xf7, 0xff}),
		ct:      at(ctColor),
		t:       at(tColor),
		dead:    at(color.RGBA{0x8a, 0x8f, 0x96, 0xff}),
		smoke:   at(color.RGBA{0xc8, 0xcd, 0xd2, 0xff}),
		fire:    at(color.RGBA{0xff, 0x68, 0x24, 0xff}),
		bomb:    at(color.RGBA{0xe5, 0x3b, 0x30, 0xff}),
		mark:    at(color.RGBA{0xa0, 0x7c, 0xf5, 0xff}),
	}
}

func (in inks) stroke(token string) uint8 {
	switch token {
	case "ct":
		return in.ct
	case "t":
		return in.t
	case "mark":
		return in.mark
	default:
		return in.white
	}
}

// px turns a world position into GIF pixels.
func (s Scene) px(x, y float64) (float64, float64) {
	nx, ny := s.Calibration.WorldToPixel(x, y)
	k := float64(s.Size) / float64(s.Calibration.Size)
	return nx * k, ny * k
}

// units turns world units into GIF pixels.
func (s Scene) units(world float64) float64 {
	return world / s.Calibration.Scale * float64(s.Size) / float64(s.Calibration.Size)
}

// drawAt paints one moment over the plate: utility, the bomb, the drawings, the players and the clock.
func (s Scene) drawAt(img *image.Paletted, tick int, in inks) {
	for _, e := range s.Round.Events {
		kind, radius, last := e.Kind, 0.0, e.EndTick
		switch kind {
		case tacticalplan.EventSmoke:
			radius = smokeRadius
			if last == 0 {
				last = e.Tick + int(smokeSeconds*s.Tickrate)
			}
		case tacticalplan.EventMolotov:
			radius = fireRadius
			if last == 0 {
				last = e.Tick + int(fireSeconds*s.Tickrate)
			}
		default:
			continue
		}
		if tick < e.Tick || tick > last {
			continue
		}
		x, y := s.px(e.Pos[0], e.Pos[1])
		if kind == tacticalplan.EventSmoke {
			disc(img, x, y, s.units(radius), in.smoke)
		} else {
			ring(img, ringSpec{x: x, y: y, r: s.units(radius), width: 2.5, ink: in.fire})
		}
	}
	if b := s.Round.Bomb; b != nil && b.PlantTick > 0 && tick >= b.PlantTick && (b.DefuseTick == 0 || tick < b.DefuseTick) && (b.ExplodeTick == 0 || tick < b.ExplodeTick) {
		for _, e := range s.Round.Events {
			if e.Kind == tacticalplan.EventPlant {
				x, y := s.px(e.Pos[0], e.Pos[1])
				disc(img, x, y, 6, in.outline)
				disc(img, x, y, 4.5, in.bomb)
			}
		}
	}
	for _, note := range s.Notes {
		if math.Abs(float64(note.Tick-tick))/s.Tickrate > noteSeconds {
			continue
		}
		for _, st := range note.Strokes {
			s.drawStroke(img, st, in.stroke(st.Color))
		}
	}
	r := math.Max(4, float64(s.Size)*0.0095)
	samples := samplesAt(s.Frames, tick)
	for _, smp := range samples {
		if !smp.Flags.Has(tacticalplan.FlagAlive) {
			x, y := s.px(smp.X, smp.Y)
			cross(img, x, y, r*0.7, in.dead)
		}
	}
	for _, smp := range samples {
		if !smp.Flags.Has(tacticalplan.FlagAlive) {
			continue
		}
		x, y := s.px(smp.X, smp.Y)
		side := in.ct
		if smp.Flags.Has(tacticalplan.FlagSideT) {
			side = in.t
		}
		yaw := smp.Yaw * math.Pi / 180
		line(img, lineSpec{x0: x, y0: y, x1: x + math.Cos(yaw)*r*2.2, y1: y - math.Sin(yaw)*r*2.2, width: 2, ink: side})
		disc(img, x, y, r+1.5, in.outline)
		disc(img, x, y, r, side)
	}
	caption(img, clockLabel(s.Round, tick, s.Tickrate), in)
}

func (s Scene) drawStroke(img *image.Paletted, st Stroke, ink uint8) {
	width := math.Max(1.5, float64(st.Width)*float64(s.Size)/referenceSize)
	for i := 1; i < len(st.Points); i++ {
		x0, y0 := s.px(st.Points[i-1][0], st.Points[i-1][1])
		x1, y1 := s.px(st.Points[i][0], st.Points[i][1])
		line(img, lineSpec{x0: x0, y0: y0, x1: x1, y1: y1, width: width, ink: ink})
	}
	if !st.Arrow || len(st.Points) < 2 {
		return
	}
	n := len(st.Points)
	fx, fy := s.px(st.Points[n-2][0], st.Points[n-2][1])
	tx, ty := s.px(st.Points[n-1][0], st.Points[n-1][1])
	angle := math.Atan2(ty-fy, tx-fx)
	head := width * 4
	for _, turn := range []float64{math.Pi * 0.82, -math.Pi * 0.82} {
		line(img, lineSpec{x0: tx, y0: ty, x1: tx + math.Cos(angle+turn)*head, y1: ty + math.Sin(angle+turn)*head, width: width, ink: ink})
	}
}

func set(img *image.Paletted, x, y int, ink uint8) {
	if image.Pt(x, y).In(img.Rect) {
		img.Pix[img.PixOffset(x, y)] = ink
	}
}

func disc(img *image.Paletted, cx, cy, r float64, ink uint8) {
	for y := int(math.Floor(cy - r)); y <= int(math.Ceil(cy+r)); y++ {
		for x := int(math.Floor(cx - r)); x <= int(math.Ceil(cx+r)); x++ {
			if dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy; dx*dx+dy*dy <= r*r {
				set(img, x, y, ink)
			}
		}
	}
}

type ringSpec struct {
	x, y, r, width float64
	ink            uint8
}

func ring(img *image.Paletted, s ringSpec) {
	inner := math.Max(0, s.r-s.width)
	for y := int(math.Floor(s.y - s.r)); y <= int(math.Ceil(s.y+s.r)); y++ {
		for x := int(math.Floor(s.x - s.r)); x <= int(math.Ceil(s.x+s.r)); x++ {
			dx, dy := float64(x)+0.5-s.x, float64(y)+0.5-s.y
			if d := dx*dx + dy*dy; d <= s.r*s.r && d >= inner*inner {
				set(img, x, y, s.ink)
			}
		}
	}
}

type lineSpec struct {
	x0, y0, x1, y1, width float64
	ink                   uint8
}

// line stamps discs along the segment, which gives round caps and joins.
func line(img *image.Paletted, s lineSpec) {
	length := math.Hypot(s.x1-s.x0, s.y1-s.y0)
	steps := int(math.Ceil(length))
	for i := 0; i <= steps; i++ {
		f := 0.0
		if steps > 0 {
			f = float64(i) / float64(steps)
		}
		disc(img, s.x0+(s.x1-s.x0)*f, s.y0+(s.y1-s.y0)*f, s.width/2, s.ink)
	}
}

func cross(img *image.Paletted, x, y, arm float64, ink uint8) {
	line(img, lineSpec{x0: x - arm, y0: y - arm, x1: x + arm, y1: y + arm, width: 2, ink: ink})
	line(img, lineSpec{x0: x + arm, y0: y - arm, x1: x - arm, y1: y + arm, width: 2, ink: ink})
}

// caption writes the clock on a dark box in the top-left corner.
func caption(img *image.Paletted, text string, in inks) {
	face := basicfont.Face7x13
	width := font.MeasureString(face, text).Ceil()
	box := image.Rect(6, 6, 6+width+12, 6+20)
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			set(img, x, y, in.plate)
		}
	}
	d := font.Drawer{Dst: img, Src: image.NewUniform(img.Palette[in.white]), Face: face, Dot: fixed.P(box.Min.X+6, box.Min.Y+14)}
	d.DrawString(text)
}
