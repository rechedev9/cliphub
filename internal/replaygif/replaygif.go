// Package replaygif draws one round's tramo of the 2D Replay as an animated
// GIF: CS2's radar when Studio has it, the players, smokes, fires, the planted
// bomb and the round's saved drawings, with the round clock in a corner.
package replaygif

import (
	"context"
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
	// MaxFPS is the GIF's frame rate when the tramo is short; radar dots need no more to read as motion.
	MaxFPS = 10
	// maxPixels bounds the frames held until the GIF is written (about 157 MB):
	// 600 frames at 512 px, 150 at 1024. A longer tramo gets a lower frame rate.
	maxPixels = 1024 * 1024 * 150
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
	Hide  Layers
}

// Layers are what a frame leaves out; the zero value draws every layer.
type Layers struct {
	Players, Utility, Bomb, Clock bool
}

// Result says how long the GIF plays and at what frame rate.
type Result struct {
	Frames  int
	FPS     int
	Seconds float64
}

// gifRates are the frame rates whose delay is a whole number of centiseconds, fastest first.
var gifRates = []int{MaxFPS, 5, 4, 2, 1}

// Encode writes the scene as a looping GIF. Frames after the first carry only
// the rectangle that changed, with the pixels inside it that did not change
// left transparent, and a frame like the one before only lengthens it.
func Encode(ctx context.Context, w io.Writer, s Scene) (Result, error) {
	s, err := s.checked()
	if err != nil {
		return Result{}, err
	}
	fps, ok := frameRate(s)
	if !ok {
		return Result{}, errors.New("the tramo is too long for one GIF even at 1 frame a second: pick a shorter one, a higher speed or a smaller size")
	}
	ticks := frameTicks(s.FromTick, s.ToTick, s.Tickrate, s.Speed/float64(fps))
	sort.Slice(s.Frames, func(i, j int) bool { return s.Frames[i].Tick < s.Frames[j].Tick })
	// The Plan9 colours but the last, which becomes the transparent slot.
	pal := append(color.Palette{}, palette.Plan9[:255]...)
	pal = append(pal, color.Transparent)
	transparent := uint8(len(pal) - 1)
	in := newInks(pal)
	base := plate(s, pal)
	bounds := base.Bounds()
	anim := &gif.GIF{Config: image.Config{ColorModel: pal, Width: s.Size, Height: s.Size}}
	prev := image.NewPaletted(bounds, pal)
	cur := image.NewPaletted(bounds, pal)
	delay := 100 / fps
	for i, tick := range ticks {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		copy(cur.Pix, base.Pix)
		s.drawAt(cur, tick, in)
		box := bounds
		if i > 0 {
			box = changed(prev, cur)
		}
		if box.Empty() {
			anim.Delay[len(anim.Delay)-1] += delay
		} else {
			var keep *image.Paletted
			if i > 0 {
				keep = prev
			}
			anim.Image = append(anim.Image, crop(cur, cropSpec{box: box, prev: keep, transparent: transparent}))
			anim.Delay = append(anim.Delay, delay)
			anim.Disposal = append(anim.Disposal, gif.DisposalNone)
		}
		prev, cur = cur, prev
	}
	if err := gif.EncodeAll(w, anim); err != nil {
		return Result{}, fmt.Errorf("encode gif: %w", err)
	}
	return Result{Frames: len(ticks), FPS: fps, Seconds: float64(len(ticks)) / float64(fps)}, nil
}

// checked is the scene with its defaults, or why it cannot be drawn.
func (s Scene) checked() (Scene, error) {
	if !s.Calibration.Valid() {
		return s, errors.New("the match has no radar calibration")
	}
	if s.Tickrate <= 0 {
		return s, errors.New("the match has no tickrate")
	}
	if s.Size == 0 {
		s.Size = DefaultSize
	}
	if s.Size < 128 || s.Size > 1024 {
		return s, errors.New("size must be from 128 to 1024")
	}
	if s.Speed <= 0 {
		s.Speed = 1
	}
	if s.ToTick <= s.FromTick {
		return s, errors.New("the tramo is empty")
	}
	return s, nil
}

// frameRate is the fastest GIF rate whose frames, at this size, stay within maxPixels.
func frameRate(s Scene) (int, bool) {
	budget := maxPixels / (s.Size * s.Size)
	for _, fps := range gifRates {
		if frameCount(s.FromTick, s.ToTick, s.Tickrate, s.Speed/float64(fps)) <= budget {
			return fps, true
		}
	}
	return 0, false
}

// frameCount is how many GIF frames the tramo takes when each frame advances `step` seconds of play times the tickrate.
func frameCount(from, to int, tickrate, step float64) int {
	if to <= from {
		return 0
	}
	return int(float64(to-from)/(tickrate*step)) + 1
}

// frameTicks are the round ticks of each GIF frame, from `from` towards `to`; `step` is the
// seconds of the round one frame advances: speed over the frame rate.
func frameTicks(from, to int, tickrate, step float64) []int {
	stepTicks := tickrate * step
	out := make([]int, frameCount(from, to, tickrate, step))
	for i := range out {
		out[i] = from + int(math.Round(float64(i)*stepTicks))
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

type cropSpec struct {
	box image.Rectangle
	// prev, when set, turns every pixel it shares with the frame into transparent.
	prev        *image.Paletted
	transparent uint8
}

func crop(src *image.Paletted, c cropSpec) *image.Paletted {
	out := image.NewPaletted(c.box, src.Palette)
	for y := c.box.Min.Y; y < c.box.Max.Y; y++ {
		row := out.Pix[out.PixOffset(c.box.Min.X, y):out.PixOffset(c.box.Max.X, y)]
		at := src.PixOffset(c.box.Min.X, y)
		copy(row, src.Pix[at:at+len(row)])
		if c.prev == nil {
			continue
		}
		for x, p := range c.prev.Pix[at : at+len(row)] {
			if p == row[x] {
				row[x] = c.transparent
			}
		}
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
