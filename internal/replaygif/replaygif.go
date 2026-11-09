// Package replaygif draws one round's tramo of the 2D Replay as an animated
// GIF: CS2's radar when Studio has it, the players, smokes, fires, the planted
// bomb and the round's saved drawings, with the round clock in a corner.
package replaygif

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"io"
	"math"
	"sort"

	xdraw "golang.org/x/image/draw"

	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const (
	// FPS is the GIF's frame rate; radar dots need no more to read as motion.
	FPS = 10
	// MaxFrames caps one GIF at 90 s of output, so its frames fit in memory.
	MaxFrames = 900
	// DefaultSize is the GIF's side in pixels.
	DefaultSize  = 512
	roundSeconds = 115
	bombSeconds  = 40
	// noteSeconds mirrors NOTE_VISIBLE_SECONDS in the web: a note shows this long either side of its tick.
	noteSeconds = 3
	// referenceSize is the radar size stroke widths are measured at.
	referenceSize = 1024
	smokeRadius   = 144
	fireRadius    = 120
	smokeSeconds  = 18
	fireSeconds   = 7
)

// Stroke is one line of a saved drawing: a colour token, a width in radar
// pixels at the 1024 reference, and points as world x and y.
type Stroke struct {
	Color  string
	Width  int
	Arrow  bool
	Points [][2]float64
}

// Note is a saved drawing anchored to a tick of the round.
type Note struct {
	Tick    int
	Strokes []Stroke
}

// Scene is what one GIF draws.
type Scene struct {
	Calibration radarmap.Calibration
	// Radar is CS2's overview image; nil draws the players on a plain plate.
	Radar    image.Image
	Tickrate float64
	Round    tacticalplan.Round
	Frames   []tacticalplan.Frame
	Notes    []Note
	FromTick int
	ToTick   int
	// Speed plays the tramo faster (2) or slower (0.5); 0 means 1.
	Speed float64
	Size  int
}

// Result says how long the GIF plays.
type Result struct {
	Frames  int
	Seconds float64
}

// Encode writes the scene as a looping GIF. Frames after the first carry only
// the rectangle that changed, and a frame like the one before only lengthens it.
func Encode(w io.Writer, s Scene) (Result, error) {
	if !s.Calibration.Valid() {
		return Result{}, errors.New("the match has no radar calibration")
	}
	if s.Tickrate <= 0 {
		return Result{}, errors.New("the match has no tickrate")
	}
	if s.Size == 0 {
		s.Size = DefaultSize
	}
	if s.Size < 128 || s.Size > 1024 {
		return Result{}, errors.New("size must be from 128 to 1024")
	}
	if s.Speed <= 0 {
		s.Speed = 1
	}
	n := frameCount(s.FromTick, s.ToTick, s.Tickrate, s.Speed)
	if n == 0 {
		return Result{}, errors.New("the tramo is empty")
	}
	if n > MaxFrames {
		return Result{}, fmt.Errorf("the GIF would last %d s; keep it under %d s, with a shorter tramo or a higher speed", n/FPS, MaxFrames/FPS)
	}
	ticks := frameTicks(s.FromTick, s.ToTick, s.Tickrate, s.Speed)
	sort.Slice(s.Frames, func(i, j int) bool { return s.Frames[i].Tick < s.Frames[j].Tick })
	pal := color.Palette(palette.Plan9)
	in := newInks(pal)
	base := plate(s, pal)
	bounds := base.Bounds()
	anim := &gif.GIF{Config: image.Config{ColorModel: pal, Width: s.Size, Height: s.Size}}
	prev := image.NewPaletted(bounds, pal)
	cur := image.NewPaletted(bounds, pal)
	const delay = 100 / FPS
	for i, tick := range ticks {
		copy(cur.Pix, base.Pix)
		s.drawAt(cur, tick, in)
		box := bounds
		if i > 0 {
			box = changed(prev, cur)
		}
		if box.Empty() {
			anim.Delay[len(anim.Delay)-1] += delay
		} else {
			anim.Image = append(anim.Image, crop(cur, box))
			anim.Delay = append(anim.Delay, delay)
			anim.Disposal = append(anim.Disposal, gif.DisposalNone)
		}
		prev, cur = cur, prev
	}
	if err := gif.EncodeAll(w, anim); err != nil {
		return Result{}, fmt.Errorf("encode gif: %w", err)
	}
	return Result{Frames: len(ticks), Seconds: float64(len(ticks)) / FPS}, nil
}

// frameCount is how many GIF frames the tramo takes at this speed.
func frameCount(from, to int, tickrate, speed float64) int {
	if to <= from {
		return 0
	}
	return int(float64(to-from)/(tickrate*speed/FPS)) + 1
}

// frameTicks are the round ticks of each GIF frame, from `from` towards `to`.
func frameTicks(from, to int, tickrate, speed float64) []int {
	step := tickrate * speed / FPS
	out := make([]int, frameCount(from, to, tickrate, speed))
	for i := range out {
		out[i] = from + int(math.Round(float64(i)*step))
	}
	return out
}

// plate is the radar scaled to the GIF and reduced to its palette once.
func plate(s Scene, pal color.Palette) *image.Paletted {
	r := image.Rect(0, 0, s.Size, s.Size)
	rgba := image.NewRGBA(r)
	xdraw.Draw(rgba, r, image.NewUniform(plateColor), image.Point{}, xdraw.Src)
	if s.Radar != nil {
		// The overview spans the calibration's native size whatever its pixel count.
		xdraw.ApproxBiLinear.Scale(rgba, r, s.Radar, s.Radar.Bounds(), xdraw.Over, nil)
	}
	out := image.NewPaletted(r, pal)
	xdraw.FloydSteinberg.Draw(out, r, rgba, image.Point{})
	return out
}

// changed is the smallest rectangle where two frames of the same size differ.
func changed(a, b *image.Paletted) image.Rectangle {
	box := image.Rectangle{}
	w := a.Rect.Dx()
	for i := range a.Pix {
		if a.Pix[i] == b.Pix[i] {
			continue
		}
		p := image.Rect(i%w, i/w, i%w+1, i/w+1)
		if box.Empty() {
			box = p
		} else {
			box = box.Union(p)
		}
	}
	return box
}

func crop(src *image.Paletted, box image.Rectangle) *image.Paletted {
	out := image.NewPaletted(box, src.Palette)
	for y := box.Min.Y; y < box.Max.Y; y++ {
		copy(out.Pix[out.PixOffset(box.Min.X, y):out.PixOffset(box.Max.X, y)], src.Pix[src.PixOffset(box.Min.X, y):src.PixOffset(box.Max.X, y)])
	}
	return out
}

// samplesAt are the players at a tick, moved between the two recorded frames around it.
func samplesAt(frames []tacticalplan.Frame, tick int) []tacticalplan.Sample {
	i := sort.Search(len(frames), func(i int) bool { return frames[i].Tick > tick }) - 1
	if i < 0 {
		if len(frames) == 0 {
			return nil
		}
		return frames[0].Samples
	}
	if i == len(frames)-1 || frames[i+1].Tick == frames[i].Tick {
		return frames[i].Samples
	}
	a, b := frames[i], frames[i+1]
	f := float64(tick-a.Tick) / float64(b.Tick-a.Tick)
	out := make([]tacticalplan.Sample, len(a.Samples))
	for j, sa := range a.Samples {
		out[j] = sa
		for _, sb := range b.Samples {
			if sb.Slot == sa.Slot && sa.Flags.Has(tacticalplan.FlagAlive) && sb.Flags.Has(tacticalplan.FlagAlive) {
				out[j].X = sa.X + (sb.X-sa.X)*f
				out[j].Y = sa.Y + (sb.Y-sa.Y)*f
			}
		}
	}
	return out
}

// clockLabel is the round clock at a tick, or the bomb timer once it is planted.
func clockLabel(r tacticalplan.Round, tick int, tickrate float64) string {
	if b := r.Bomb; b != nil && b.PlantTick > 0 && tick >= b.PlantTick {
		left := math.Max(0, bombSeconds-float64(tick-b.PlantTick)/tickrate)
		return fmt.Sprintf("R%d Bomba %d", r.Number, int(math.Ceil(left)))
	}
	left := math.Min(roundSeconds, math.Max(0, roundSeconds-float64(tick-r.TickFreezeEnd)/tickrate))
	whole := int(math.Ceil(left))
	return fmt.Sprintf("R%d %d:%02d", r.Number, whole/60, whole%60)
}
