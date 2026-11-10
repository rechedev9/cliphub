package replaygif

import (
	"context"
	"image"
	"image/color"
	"image/color/palette"
	"sort"
)

// VideoFrames is how many frames the scene takes as a video at fps.
func VideoFrames(s Scene, fps int) (int, error) {
	s, err := s.checked()
	if err != nil || fps <= 0 {
		return 0, err
	}
	return frameCount(s.FromTick, s.ToTick, s.Tickrate, s.Speed/float64(fps)), nil
}

// Render draws every frame of the scene at fps, whole and in order, for a video encoder.
// The image handed to each is reused by the next frame.
func Render(ctx context.Context, s Scene, fps int, each func(image.Image) error) error {
	s, err := s.checked()
	if err != nil {
		return err
	}
	sort.Slice(s.Frames, func(i, j int) bool { return s.Frames[i].Tick < s.Frames[j].Tick })
	pal := append(color.Palette{}, palette.Plan9...)
	in := newInks(pal)
	base := plate(s, pal)
	cur := image.NewPaletted(base.Bounds(), pal)
	for _, tick := range frameTicks(s.FromTick, s.ToTick, s.Tickrate, s.Speed/float64(fps)) {
		if err := ctx.Err(); err != nil {
			return err
		}
		copy(cur.Pix, base.Pix)
		s.drawAt(cur, tick, in)
		if err := each(cur); err != nil {
			return err
		}
	}
	return nil
}
