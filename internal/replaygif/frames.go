package replaygif

import (
	"context"
	"image"
	"image/color"
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

// Render draws every frame of the scene at fps, whole, in order and in full colour, for a video
// encoder. The image handed to each is reused by the next frame.
func Render(ctx context.Context, s Scene, fps int, each func(image.Image) error) error {
	s, err := s.checked()
	if err != nil {
		return err
	}
	sort.Slice(s.Frames, func(i, j int) bool { return s.Frames[i].Tick < s.Frames[j].Tick })
	// The overlay is drawn on its own exact palette, slot 0 left clear, then laid on the radar.
	pal := append(color.Palette{color.Transparent}, inkColors...)
	in := newInks(pal)
	base := radarRGBA(s)
	overlay := image.NewPaletted(base.Rect, pal)
	frame := image.NewRGBA(base.Rect)
	for _, tick := range frameTicks(s.FromTick, s.ToTick, s.Tickrate, s.Speed/float64(fps)) {
		if err := ctx.Err(); err != nil {
			return err
		}
		clear(overlay.Pix)
		s.drawAt(overlay, tick, in)
		copy(frame.Pix, base.Pix)
		for i, ink := range overlay.Pix {
			if ink == 0 {
				continue
			}
			r, g, b, _ := pal[ink].RGBA()
			frame.Pix[i*4], frame.Pix[i*4+1], frame.Pix[i*4+2], frame.Pix[i*4+3] = uint8(r>>8), uint8(g>>8), uint8(b>>8), 0xff
		}
		if err := each(frame); err != nil {
			return err
		}
	}
	return nil
}
