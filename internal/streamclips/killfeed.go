package streamclips

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	// The players HUD of a 1080p CS2 frame ends about this far from the top;
	// the moved feed starts just below it.
	killFeedHUDBottom1080 = 155.0
	killFeedFadeSecs      = 0.2
	killFeedMinTopEdge    = 8
	// Notices closer than this in time share one window, so the feed does not
	// blink out between two kills.
	killFeedMergeGapSecs = 1.0
	// The cut reaches this far past the notices, in 1080p pixels, so the
	// feathered edge falls on the game around them and not on a notice.
	killFeedCutMargin1080 = 14.0
	killFeedSideMargin    = 24
	// The feather is this share of the cut's height, and the cut keeps this
	// opacity, as an editor would blend a second copy of the feed.
	killFeedFeatherShare = 0.14
	killFeedMinFeather   = 4
	killFeedOpacity      = 0.92
	// delogo needs a few pixels to interpolate from; the margin covers the
	// frames where the notice fades in or out before the scan sees it.
	killFeedMinMask        = 4
	killFeedMaskMarginSecs = 0.2
)

// KillFeedOverlay is the streamer's kill feed cut live from the source and
// laid centered on the Short while their notices are on screen.
type KillFeedOverlay struct {
	// Area is the source rectangle the cut covers, in source fractions.
	Area    CropRect
	Windows []KillFeedWindow
	// Masks are where the streamer's own notices sat, so the game panel can hide them.
	Masks []KillFeedMask
}

// KillFeedWindow is a span, in seconds from the clip start, the cut is shown.
type KillFeedWindow struct {
	From, To float64
}

// KillFeedMask is one source rectangle with the seconds, relative to the clip
// start, that the original notice held it.
type KillFeedMask struct {
	Area     CropRect
	From, To float64
}

// NewKillFeedOverlay cuts the area every notice row covered and shows it while
// any notice is on screen; nil when no notice falls inside the clip.
func NewKillFeedOverlay(masks []KillFeedMask, srcW, srcH int, duration float64) *KillFeedOverlay {
	var spans []KillFeedWindow
	minX, minY, maxX, maxY := 1.0, 1.0, 0.0, 0.0
	for _, m := range masks {
		from, to := math.Max(m.From, 0), math.Min(m.To, duration)
		if to <= from {
			continue
		}
		spans = append(spans, KillFeedWindow{From: from, To: to})
		minX, minY = math.Min(minX, m.Area.X), math.Min(minY, m.Area.Y)
		maxX, maxY = math.Max(maxX, m.Area.X+m.Area.Width), math.Max(maxY, m.Area.Y+m.Area.Height)
	}
	if len(spans) == 0 || srcW <= 0 || srcH <= 0 {
		return nil
	}
	mx := killFeedCutMargin1080 * float64(srcH) / 1080 / float64(srcW)
	my := killFeedCutMargin1080 / 1080
	minX, minY = math.Max(minX-mx, 0), math.Max(minY-my, 0)
	maxX, maxY = math.Min(maxX+mx, 1), math.Min(maxY+my, 1)
	return &KillFeedOverlay{
		Area:    CropRect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY},
		Windows: mergeWindows(spans),
		Masks:   masks,
	}
}

func mergeWindows(spans []KillFeedWindow) []KillFeedWindow {
	sort.Slice(spans, func(a, b int) bool { return spans[a].From < spans[b].From })
	out := []KillFeedWindow{spans[0]}
	for _, s := range spans[1:] {
		last := &out[len(out)-1]
		if s.From-last.To <= killFeedMergeGapSecs {
			last.To = math.Max(last.To, s.To)
			continue
		}
		out = append(out, s)
	}
	return out
}

// panelScale is how many output pixels one source pixel of the game panel
// covers, and how far the centered crop shifts the panel up.
func panelScale(layout LayoutVariant, crop CropRect, srcW, srcH int) (float64, float64) {
	w, h := crop.Width*float64(srcW), crop.Height*float64(srcH)
	s := math.Max(float64(layout.OutputWidth)/w, float64(layout.GameOutputHeight)/h)
	return s, (math.Round(h*s) - float64(layout.GameOutputHeight)) / 2
}

// killFeedTop is the output y of the cut: the plan position, or just below
// where the players HUD lands on the game panel.
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

type killFeedGraph struct {
	overlay KillFeedOverlay
	// crop is the source pixels of the cut; width and height its output size.
	cropX, cropY, cropW, cropH int
	width, height, feather     int
	top                        int
	outputWidth                int
	duration                   float64
	// sourceMask is the filter chain that hides the original notices.
	sourceMask string
}

// newKillFeedGraph sizes the cut as the game panel draws it, narrowed to fit
// the output with a side margin.
func newKillFeedGraph(layout LayoutVariant, plan EditPlan, in FFmpegInputs, duration float64) killFeedGraph {
	o := *in.KillFeed
	srcW, srcH := float64(in.SourceWidth), float64(in.SourceHeight)
	x, y := int(math.Floor(o.Area.X*srcW)), int(math.Floor(o.Area.Y*srcH))
	w := evenAtLeast2(int(math.Ceil((o.Area.X+o.Area.Width)*srcW)) - x)
	h := evenAtLeast2(int(math.Ceil((o.Area.Y+o.Area.Height)*srcH)) - y)
	x, y = min(x, in.SourceWidth-w), min(y, in.SourceHeight-h)
	s, _ := panelScale(layout, plan.GameplayCrop, in.SourceWidth, in.SourceHeight)
	s = math.Min(s, float64(layout.OutputWidth-2*killFeedSideMargin)/float64(w))
	outW, outH := evenAtLeast2(int(math.Round(float64(w)*s))), evenAtLeast2(int(math.Round(float64(h)*s)))
	feather := max(killFeedMinFeather, int(math.Round(float64(outH)*killFeedFeatherShare)))
	return killFeedGraph{
		overlay: o, cropX: max(x, 0), cropY: max(y, 0), cropW: w, cropH: h,
		width: outW, height: outH, feather: min(feather, outH/3, outW/3),
		top: killFeedTop(layout, plan, in.SourceWidth, in.SourceHeight), outputWidth: layout.OutputWidth,
		duration: duration, sourceMask: killFeedMask(o.Masks, in.SourceWidth, in.SourceHeight),
	}
}

func evenAtLeast2(v int) int { return max(2, v-v%2) }

// killFeedFilter cuts the feed from [kfsrc], feathers its edges with a blurred
// mask, and lays it centered on contentLabel during each window, fading at both ends.
func killFeedFilter(kf killFeedGraph, contentLabel string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[kfsrc]crop=%d:%d:%d:%d,scale=%d:%d:flags=lanczos,format=yuva420p[kfcut];",
		kf.cropW, kf.cropH, kf.cropX, kf.cropY, kf.width, kf.height)
	gray := int(math.Round(killFeedOpacity * 255))
	fmt.Fprintf(&b, "color=c=0x%02X%02X%02X:s=%dx%d:r=%d:d=%s,format=gray,pad=%d:%d:%d:%d:black,gblur=sigma=%s[kfmask];",
		gray, gray, gray, kf.width-2*kf.feather, kf.height-2*kf.feather, outputFPS, floatArg(kf.duration),
		kf.width, kf.height, kf.feather, kf.feather, floatArg(float64(kf.feather)/2))
	b.WriteString("[kfcut][kfmask]alphamerge")
	n := len(kf.overlay.Windows)
	if n > 1 {
		fmt.Fprintf(&b, ",split=%d", n)
		for i := range n {
			fmt.Fprintf(&b, "[kfa%d]", i)
		}
	} else {
		b.WriteString("[kfa0]")
	}
	b.WriteString(";")
	for i, win := range kf.overlay.Windows {
		fmt.Fprintf(&b, "[kfa%d]null", i)
		if win.From > 0 {
			fmt.Fprintf(&b, ",fade=t=in:st=%s:d=%s:alpha=1", floatArg(win.From), floatArg(killFeedFadeSecs))
		}
		if win.To < kf.duration {
			fmt.Fprintf(&b, ",fade=t=out:st=%s:d=%s:alpha=1", floatArg(math.Max(win.To-killFeedFadeSecs, 0)), floatArg(killFeedFadeSecs))
		}
		fmt.Fprintf(&b, "[kfw%d];", i)
	}
	current := contentLabel
	for i, win := range kf.overlay.Windows {
		next := fmt.Sprintf("kfo%d", i)
		fmt.Fprintf(&b, `[%s][kfw%d]overlay=x=%d:y=%d:enable='between(t\,%s\,%s)'`,
			current, i, (kf.outputWidth-kf.width)/2, kf.top, floatArg(win.From), floatArg(win.To))
		if i < n-1 {
			fmt.Fprintf(&b, "[%s];", next)
		}
		current = next
	}
	return b.String()
}

// killFeedMask hides the streamer's own copy of each notice on the source, so
// the edge of the vertical crop does not show it next to the moved one.
func killFeedMask(masks []KillFeedMask, srcW, srcH int) string {
	var parts []string
	for _, m := range masks {
		x := max(1, int(math.Floor(m.Area.X*float64(srcW))))
		y := max(1, int(math.Floor(m.Area.Y*float64(srcH))))
		w := min(srcW-1, int(math.Ceil((m.Area.X+m.Area.Width)*float64(srcW)))) - x
		h := min(srcH-1, int(math.Ceil((m.Area.Y+m.Area.Height)*float64(srcH)))) - y
		if w < killFeedMinMask || h < killFeedMinMask {
			continue
		}
		parts = append(parts, fmt.Sprintf(`delogo=x=%d:y=%d:w=%d:h=%d:enable='between(t\,%s\,%s)'`, x, y, w, h,
			floatArg(m.From-killFeedMaskMarginSecs), floatArg(m.To+killFeedMaskMarginSecs)))
	}
	return strings.Join(parts, ",")
}
