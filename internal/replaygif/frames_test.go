package replaygif

import (
	"context"
	"image"
	"testing"
)

func TestRenderDrawsEveryVideoFrame(t *testing.T) {
	s := scene(t)
	s.Speed = 2
	want, err := VideoFrames(s, 30)
	if err != nil {
		t.Fatal(err)
	}
	// 10 s of round at speed 2 is 5 s of video: 150 frames and the last tick.
	if want != 151 {
		t.Fatalf("VideoFrames = %d, want 151", want)
	}
	got := 0
	err = Render(context.Background(), s, 30, func(img image.Image) error {
		if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
			t.Fatalf("frame is %v, want 256 px", img.Bounds())
		}
		got++
		return nil
	})
	if err != nil || got != want {
		t.Fatalf("Render drew %d frames err=%v, want %d", got, err, want)
	}
}

func TestHiddenPlayersLeaveThePlate(t *testing.T) {
	s := scene(t)
	s.Hide = Layers{Players: true, Utility: true, Bomb: true, Clock: true}
	var first []uint8
	err := Render(context.Background(), s, 30, func(img image.Image) error {
		p, ok := img.(*image.Paletted)
		if !ok {
			t.Fatal("frame is not paletted")
		}
		if first == nil {
			first = append([]uint8{}, p.Pix...)
			return nil
		}
		for i := range p.Pix {
			if p.Pix[i] != first[i] {
				t.Fatal("a frame with every layer hidden changed")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
