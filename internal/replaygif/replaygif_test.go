package replaygif

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"math"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const rate = 64

func scene(t *testing.T) Scene {
	t.Helper()
	cal, ok := radarmap.Lookup("de_mirage")
	if !ok {
		t.Fatal("de_mirage has no calibration")
	}
	alive := tacticalplan.FlagAlive
	var frames []tacticalplan.Frame
	for tick := 1000; tick <= 1640; tick += 32 {
		ctX := -500 + float64(tick-1000)
		tFlags := alive | tacticalplan.FlagSideT
		if tick >= 1320 {
			tFlags = tacticalplan.FlagSideT
		}
		frames = append(frames, tacticalplan.Frame{Tick: tick, Samples: []tacticalplan.Sample{
			{Slot: 0, X: ctX, Y: -1000, Flags: alive},
			{Slot: 5, X: 300, Y: 200, Flags: tFlags},
		}})
	}
	return Scene{
		Calibration: cal,
		Tickrate:    rate,
		Round:       tacticalplan.Round{Number: 14, TickStart: 0, TickFreezeEnd: 1000, TickEnd: 1700},
		Frames:      frames,
		FromTick:    1000,
		ToTick:      1640,
		Size:        256,
	}
}

func TestEncodeLoopsTheTramoAtTheGifRate(t *testing.T) {
	var buf bytes.Buffer
	got, err := Encode(context.Background(), &buf, scene(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.Frames != 101 || got.FPS != 10 || got.Seconds != 10.1 {
		t.Fatalf("result = %+v, want 101 frames and 10.1 s", got)
	}
	decoded, err := gif.DecodeAll(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.LoopCount != 0 || decoded.Config.Width != 256 || decoded.Config.Height != 256 {
		t.Fatalf("loop %d, size %dx%d", decoded.LoopCount, decoded.Config.Width, decoded.Config.Height)
	}
	total := 0
	for _, d := range decoded.Delay {
		total += d
	}
	if total != 101*10 {
		t.Fatalf("delays add up to %d cs, want %d", total, 101*10)
	}
	if len(decoded.Image) < 2 || decoded.Image[1].Bounds().Dx() >= 256 {
		t.Fatalf("later frames should carry only what changed: %d frames, second %v", len(decoded.Image), decoded.Image[1].Bounds())
	}
	second := decoded.Image[1]
	transparent := 0
	for _, p := range second.Pix {
		if _, _, _, a := second.Palette[p].RGBA(); a == 0 {
			transparent++
		}
	}
	if transparent == 0 {
		t.Fatal("the pixels of a changed box that stayed the same should be transparent")
	}
}

func TestALongTramoGetsALowerFrameRate(t *testing.T) {
	s := scene(t)
	s.ToTick = s.FromTick + 115*rate
	s.Speed = 1
	for size, want := range map[int]int{256: 10, 512: 5, 1024: 1} {
		s.Size = size
		if fps, ok := frameRate(s); !ok || fps != want {
			t.Errorf("115 s at %d px = %d fps, want %d", size, fps, want)
		}
	}
}

func TestEncodeStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Encode(ctx, &bytes.Buffer{}, scene(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Encode error = %v, want context.Canceled", err)
	}
}

func TestStillFramesOnlyLengthenTheOneBefore(t *testing.T) {
	s := scene(t)
	for i := range s.Frames {
		s.Frames[i].Samples = []tacticalplan.Sample{{Slot: 0, X: 0, Y: 0, Flags: tacticalplan.FlagAlive}}
	}
	// A caption that changes only once a second; between those seconds nothing moves.
	var buf bytes.Buffer
	if _, err := Encode(context.Background(), &buf, s); err != nil {
		t.Fatal(err)
	}
	decoded, err := gif.DecodeAll(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Image) > 12 {
		t.Fatalf("%d frames for a still tramo of 10 s; the clock alone should change about once a second", len(decoded.Image))
	}
}

func TestPlayersAreDrawnWhereTheRadarPutsThem(t *testing.T) {
	s := scene(t)
	pal := color.Palette(palette.Plan9)
	in := newInks(pal)
	img := image.NewPaletted(image.Rect(0, 0, s.Size, s.Size), pal)
	s.drawAt(img, 1000, in)
	x, y := s.px(-500, -1000)
	if got := img.ColorIndexAt(int(x), int(y)); got != in.ct {
		t.Fatalf("CT dot pixel = %d, want the CT ink %d", got, in.ct)
	}
	tx, ty := s.px(300, 200)
	if got := img.ColorIndexAt(int(tx), int(ty)); got != in.t {
		t.Fatalf("T dot pixel = %d, want the T ink %d", got, in.t)
	}
	dead := image.NewPaletted(img.Rect, pal)
	s.drawAt(dead, 1400, in)
	if got := dead.ColorIndexAt(int(tx), int(ty)); got != in.dead {
		t.Fatalf("a dead player is a grey cross, got %d", got)
	}
}

func TestSamplesMoveBetweenRecordedFrames(t *testing.T) {
	s := scene(t)
	got := samplesAt(s.Frames, 1016)
	if math.Abs(got[0].X-(-484)) > 1e-9 {
		t.Fatalf("x halfway between frames = %v, want -484", got[0].X)
	}
}

func TestClockCountsDownThenFollowsTheBomb(t *testing.T) {
	r := tacticalplan.Round{Number: 14, TickFreezeEnd: 1000}
	if got := clockLabel(r, 1000+48*rate, rate); got != "R14 1:07" {
		t.Fatalf("clock = %q", got)
	}
	r.Bomb = &tacticalplan.Bomb{PlantTick: 5000}
	if got := clockLabel(r, 5000+10*rate, rate); got != "R14 Bomba 30" {
		t.Fatalf("bomb clock = %q", got)
	}
}

func TestEncodeRejectsWhatItCannotDraw(t *testing.T) {
	long := scene(t)
	long.ToTick = long.FromTick + 3000*rate
	cases := map[string]Scene{
		"no radar calibration": {Tickrate: rate, FromTick: 0, ToTick: 64},
		"the tramo is empty":   func() Scene { s := scene(t); s.ToTick = s.FromTick; return s }(),
		"too long":             long,
	}
	for want, s := range cases {
		if _, err := Encode(context.Background(), &bytes.Buffer{}, s); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Encode error = %v, want one about %q", err, want)
		}
	}
}

func TestSpeedShortensTheGif(t *testing.T) {
	if got := frameCount(1000, 1640, rate, 2.0/MaxFPS); got != 51 {
		t.Fatalf("frames at 2x = %d, want 51", got)
	}
}
