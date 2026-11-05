package streamclips

import (
	"fmt"
	"math"
	"strings"
)

const (
	// The players HUD of a 1080p CS2 frame ends about this far from the top;
	// the redrawn feed starts just below it.
	killFeedHUDBottom1080 = 155.0
	// killFeedPitch1080 is the distance between two notice rows in 1080p pixels.
	killFeedPitch1080  = 35.0
	killFeedFadeSecs   = 0.15
	killFeedSlideSecs  = 0.2
	killFeedMinTopEdge = 8
)

// KillFeedNotice is one notice sprite already scaled to output pixels, with
// the seconds it is on screen relative to the clip start in source time.
type KillFeedNotice struct {
	Path             string
	Width, Height    int
	AppearSeconds    float64
	DisappearSeconds float64
}

// panelScale is how many output pixels one source pixel of the game panel
// covers, and how far the centered crop shifts the panel up.
func panelScale(layout LayoutVariant, crop CropRect, srcW, srcH int) (float64, float64) {
	w, h := crop.Width*float64(srcW), crop.Height*float64(srcH)
	s := math.Max(float64(layout.OutputWidth)/w, float64(layout.GameOutputHeight)/h)
	return s, (math.Round(h*s) - float64(layout.GameOutputHeight)) / 2
}

// KillFeedScale maps a sprite in 1080p feed pixels to output pixels, so the
// notice keeps the size it has on the game panel.
func KillFeedScale(plan EditPlan, srcW, srcH int) (float64, error) {
	layout, ok := VariantByName(plan.Variant)
	if !ok {
		return 0, unknownVariantError(plan.Variant)
	}
	if srcW <= 0 || srcH <= 0 {
		return 0, fmt.Errorf("kill feed needs the source size")
	}
	s, _ := panelScale(layout, plan.GameplayCrop, srcW, srcH)
	return s * float64(srcH) / 1080, nil
}

// killFeedTop is the output y of the first notice row: the plan position, or
// just below where the players HUD lands on the game panel.
func killFeedTop(layout LayoutVariant, plan EditPlan, srcW, srcH int) int {
	if plan.KillFeed.PositionY != nil {
		return int(math.Round(*plan.KillFeed.PositionY * 1920))
	}
	s, shift := panelScale(layout, plan.GameplayCrop, srcW, srcH)
	hud := (killFeedHUDBottom1080*float64(srcH)/1080 - plan.GameplayCrop.Y*float64(srcH)) * s
	panelTop := 0
	if !layout.FullFrame {
		panelTop = layout.FaceOutputHeight
	}
	return panelTop + max(killFeedMinTopEdge, int(math.Round(hud-shift)))
}

// killFeedFilter overlays the notices on the named content in RGB, where the
// game composed them. Each one fades in and out, centered, and slides up when
// an older notice above it leaves.
func killFeedFilter(in killFeedGraph) string {
	var b strings.Builder
	current := in.contentLabel
	pitch := killFeedPitch1080 * in.scale
	for i, n := range in.notices {
		sprite := fmt.Sprintf("kf%d", i)
		fmt.Fprintf(&b, "[%d:v]format=rgba", in.firstInput+i)
		if n.AppearSeconds > 0 {
			fmt.Fprintf(&b, ",fade=t=in:st=%s:d=%s:alpha=1", floatArg(n.AppearSeconds), floatArg(killFeedFadeSecs))
		}
		if n.DisappearSeconds < in.duration {
			out := math.Max(n.DisappearSeconds-killFeedFadeSecs, 0)
			fmt.Fprintf(&b, ",fade=t=out:st=%s:d=%s:alpha=1", floatArg(out), floatArg(killFeedFadeSecs))
		}
		fmt.Fprintf(&b, "[%s];", sprite)
		var y strings.Builder
		fmt.Fprintf(&y, "%d", in.top)
		for _, older := range in.notices[:i] {
			if older.DisappearSeconds <= n.AppearSeconds {
				continue
			}
			fmt.Fprintf(&y, `+%s*clip((%s-t)/%s\,0\,1)`,
				floatArg(pitch), floatArg(older.DisappearSeconds+killFeedSlideSecs), floatArg(killFeedSlideSecs))
		}
		next := fmt.Sprintf("kfo%d", i)
		fmt.Fprintf(&b, `[%s][%s]overlay=x=%d:y='%s':format=rgb:enable='between(t\,%s\,%s)'`,
			current, sprite, (in.outputWidth-n.Width)/2, y.String(),
			floatArg(math.Max(n.AppearSeconds, 0)), floatArg(n.DisappearSeconds))
		if i < len(in.notices)-1 {
			fmt.Fprintf(&b, "[%s];", next)
		}
		current = next
	}
	return b.String()
}

type killFeedGraph struct {
	contentLabel string
	notices      []KillFeedNotice
	firstInput   int
	scale        float64
	top          int
	outputWidth  int
	duration     float64
}
