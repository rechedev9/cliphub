package editor

import (
	"fmt"
	"math"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

type FullDemoTransitionEvidence struct {
	recapplan.TransitionBoundary
	Direction       string `json:"direction"`
	DirectionSource string `json:"direction_source"`
}

type fullDemoTransitionEdges struct {
	options              *recapplan.TransitionOptions
	in, out              *FullDemoTransitionEvidence
	frames               int64
	tailInput, tailCount int
	tailSamples          int64
	zoomFrame            *int64
}

func transitionDirections(d recapplan.Document) []FullDemoTransitionEvidence {
	var result []FullDemoTransitionEvidence
	for i, b := range d.TransitionBoundaries() {
		direction, source := d.Options.Transitions.Direction, "selected"
		if direction == "alternate" || direction == "follow-motion" {
			direction, source = "left", "alternating"
			if i%2 == 1 {
				direction = "right"
			}
			if d.Options.Transitions.Direction == "follow-motion" {
				source = "static-fallback"
			}
		}
		result = append(result, FullDemoTransitionEvidence{b, direction, source})
	}
	return result
}

func fullDemoEdges(short ShortEdit, item recapplan.TimelineItem) fullDemoTransitionEdges {
	edges := fullDemoTransitionEdges{frames: item.EndFrame - item.StartFrame}
	if short.FullDemo == nil || item.Role != "round" {
		return edges
	}
	d := short.FullDemo.Effective
	if d.Options.Transitions == nil || !d.Options.Transitions.Enabled {
		return edges
	}
	edges.options = d.Options.Transitions
	boundaries := short.FullDemo.Transitions
	if boundaries == nil {
		boundaries = transitionDirections(d)
	}
	for _, b := range boundaries {
		if b.Frame == item.StartFrame {
			edges.in = &b
		}
		if b.Frame == item.EndFrame {
			edges.out = &b
		}
	}
	if edges.out != nil && edges.options.Zoom {
		if frame, ok := zoomEventFrame(d, item); ok {
			edges.zoomFrame = &frame
		}
	}
	return edges
}

func smoothTransition(q string) string {
	return fmt.Sprintf("(pow(clip(%s,0,1),2)*(3-2*clip(%s,0,1)))", q, q)
}

// A frame-clock S curve reaches its peak on both sides of the same cut.
func transitionEnvelope(variable string, frames, head, tail int64) string {
	parts := []string{"0"}
	if head > 0 {
		peak := head - 1
		if head == 1 {
			peak = 1
		}
		parts = append(parts, fmt.Sprintf("if(lt(%s,%d),%s,0)", variable, head,
			smoothTransition(fmt.Sprintf("(%d-%s)/%d", peak, variable, max(int64(1), head-1)))))
	}
	if tail > 0 {
		start := frames - tail
		if tail == 1 {
			start--
		}
		parts = append(parts, fmt.Sprintf("if(gte(%s,%d),%s,0)", variable, frames-tail,
			smoothTransition(fmt.Sprintf("(%s-%d)/%d", variable, start, max(int64(1), tail-1)))))
	}
	return "(" + strings.Join(parts, "+") + ")"
}

func transitionAccentEnvelope(frames, head, tail int64) string {
	parts := []string{"0"}
	if head > 0 {
		parts = append(parts, fmt.Sprintf("if(lt(n,%d),%s,0)", head, smoothTransition(fmt.Sprintf("(%d-n)/%d", head, head))))
	}
	if tail > 0 {
		parts = append(parts, fmt.Sprintf("if(gte(n,%d),%s,0)", frames-tail, smoothTransition(fmt.Sprintf("(n-%d+1)/%d", frames-tail, tail))))
	}
	return "(" + strings.Join(parts, "+") + ")"
}

func zoomEventFrame(d recapplan.Document, item recapplan.TimelineItem) (int64, bool) {
	o := d.Options.Transitions
	if o == nil || o.ZoomAnchor == "cut" {
		return 0, false
	}
	for _, round := range d.Rounds {
		if round.ID != item.SourceRef {
			continue
		}
		tick := round.RoundEndTick
		if o.ZoomAnchor == "last-kill" {
			tick = -1
			for _, kill := range round.Kills {
				tick = max(tick, kill.Tick)
			}
		}
		if tick < item.SourceStartTick {
			return 0, false
		}
		frame, err := recapplan.TickFrames(tick-item.SourceStartTick, d.Clock.TickRate)
		frame -= item.SourceOffsetFrames
		return frame, err == nil && frame >= 0 && frame < item.EndFrame-item.StartFrame
	}
	return 0, false
}

func fullDemoTransitionVideo(short ShortEdit, item recapplan.TimelineItem, edges fullDemoTransitionEdges) string {
	o := edges.options
	if o == nil || (edges.in == nil && edges.out == nil) {
		return ""
	}
	var head, tail int64
	if edges.in != nil {
		head = edges.in.AfterFrames
	}
	if edges.out != nil {
		tail = edges.out.BeforeFrames
	}
	var filters []string
	whip := "0"
	if o.Whip {
		whip = transitionEnvelope("in", edges.frames, head, tail)
	}
	zoom := "0"
	if o.Zoom {
		zoomHead, zoomTail := head, tail
		if edges.in != nil {
			previous := short.FullDemo.Effective.Timeline[edges.in.OutgoingIndex]
			if _, ok := zoomEventFrame(short.FullDemo.Effective, previous); ok {
				zoomHead = 0
			}
		}
		if event, ok := zoomEventFrame(short.FullDemo.Effective, item); ok && edges.out != nil {
			zoomTail = 0
			zoom = smoothTransition(fmt.Sprintf("1-abs(in-%d)/%d", event, max(1, o.DurationFrames/2)))
		}
		zoom = "(" + zoom + "+" + transitionEnvelope("in", edges.frames, zoomHead, zoomTail) + ")"
	}
	if o.Whip || o.Zoom {
		x, y := "(iw-iw/zoom)/2", "(ih-ih/zoom)/2"
		for _, edge := range []struct {
			b          *FullDemoTransitionEvidence
			head, tail int64
			sign       int
		}{
			{edges.in, head, 0, -1}, {edges.out, 0, tail, 1},
		} {
			if !o.Whip || edge.b == nil {
				continue
			}
			direction := edge.b.Direction
			sign := edge.sign
			if direction == "right" || direction == "down" {
				sign = -sign
			}
			p := transitionEnvelope("in", edges.frames, edge.head, edge.tail)
			if direction == "up" || direction == "down" {
				y += fmt.Sprintf("+(%d)*(ih-ih/zoom)/2*%s", sign, p)
			} else {
				x += fmt.Sprintf("+(%d)*(iw-iw/zoom)/2*%s", sign, p)
			}
		}
		filters = append(filters, fmt.Sprintf("zoompan=z='1+max(%s*%s,%s*%s)':x='%s':y='%s':d=1:s=1920x1080:fps=60", decimal(2*o.WhipStrength), whip, decimal(o.ZoomPercent/100), zoom, x, y))
	}
	if o.Whip && o.BlurPixels > 0 {
		for _, edge := range []struct {
			b          *FullDemoTransitionEvidence
			head, tail int64
		}{{edges.in, head, 0}, {edges.out, 0, tail}} {
			if edge.b == nil {
				continue
			}
			x, y := o.BlurPixels, 1
			if edge.b.Direction == "up" || edge.b.Direction == "down" {
				x, y = y, x
			}
			filters = append(filters, fmt.Sprintf("avgblur=sizeX=%d:sizeY=%d:enable='gt(%s,0)'", x, y, transitionEnvelope("n", edges.frames, edge.head, edge.tail)))
		}
	}
	flashHead, flashTail := min(head, int64(o.FlashFrames-o.FlashFrames/2)), min(tail, int64(o.FlashFrames/2))
	pulse := transitionAccentEnvelope(edges.frames, flashHead, flashTail)
	if o.Flash {
		filters = append(filters, fmt.Sprintf("eq=brightness='%s*%s':eval=frame", decimal(o.FlashIntensity), pulse))
	}
	if o.RGBSplit {
		filters = append(filters, fmt.Sprintf("rgbashift=rh=%d:bh=%d:edge=smear:enable='gt(%s,0)'", o.RGBPixels, -o.RGBPixels, pulse), "format=yuv420p")
	}
	if len(filters) == 0 {
		return ""
	}
	return "," + strings.Join(filters, ",")
}

func fullDemoGameTransitionFilter(edges fullDemoTransitionEdges, samples int64) string {
	o := edges.options
	if o == nil {
		return ""
	}
	var filters []string
	fade := min(int64(o.GameFadeMS)*48, samples/2)
	if edges.in != nil && fade > 0 {
		filters = append(filters, fmt.Sprintf("afade=t=in:ss=0:ns=%d", fade))
	}
	if edges.out != nil {
		if o.GameTailLowpassHz > 0 {
			start := max(int64(0), samples-max(fade, edges.out.BeforeFrames*recapplan.SamplesPerFrame))
			filters = append(filters, fmt.Sprintf("lowpass=f=%d:enable='gte(t,%s)'", o.GameTailLowpassHz, decimal(float64(start)/recapplan.SampleRate)))
		}
		if fade > 0 {
			filters = append(filters, fmt.Sprintf("afade=t=out:ss=%d:ns=%d", samples-fade, fade))
		}
	}
	if len(filters) == 0 {
		return ""
	}
	return strings.Join(filters, ",")
}

// Continue only the source voice after the outgoing cut, never replay its last
// syllable or carry game audio into the next round. Source and output bounds
// also prevent including voices from the incoming round twice.
func fullDemoCommsTail(d recapplan.Document, incoming *FullDemoTransitionEvidence) (start, count int64) {
	if incoming == nil || !d.Options.Audio.Voice.Enabled || d.Options.Audio.Voice.Gain == 0 {
		return 0, 0
	}
	o := d.Options.Transitions
	if o == nil || !o.Enabled || o.CommsTailSeconds == 0 {
		return 0, 0
	}
	previous, next := d.Timeline[incoming.OutgoingIndex], d.Timeline[incoming.IncomingIndex]
	previousStart, err := recapplan.TickFrames(previous.SourceStartTick, d.Clock.TickRate)
	if err != nil {
		return 0, 0
	}
	nextStart, err := recapplan.TickFrames(next.SourceStartTick, d.Clock.TickRate)
	if err != nil {
		return 0, 0
	}
	start = (previousStart + previous.SourceOffsetFrames + previous.EndFrame - previous.StartFrame) * recapplan.SamplesPerFrame
	limit := (nextStart+next.SourceOffsetFrames)*recapplan.SamplesPerFrame - start
	count = min(int64(math.Round(o.CommsTailSeconds*recapplan.SampleRate)), limit, next.EndSample-next.StartSample)
	return start, max(int64(0), count)
}

// Generated, deterministic SFX need no download or licensed sound library.
// They enter the lossless mix before the existing decoded-AAC mastering gate.
func fullDemoTransitionSFX(edges fullDemoTransitionEdges, samples int64) (string, string) {
	o := edges.options
	if o == nil {
		return "", "declicked"
	}
	var clauses, labels []string
	if o.Whoosh {
		for i, b := range []*FullDemoTransitionEvidence{edges.in, edges.out} {
			if b == nil {
				continue
			}
			before, after := b.BeforeFrames*recapplan.SamplesPerFrame, b.AfterFrames*recapplan.SamplesPerFrame
			start, count, delay := before, after, int64(0)
			if i == 1 {
				start, count, delay = 0, before, samples-before
			}
			label := fmt.Sprintf("swish%d", i)
			clauses = append(clauses, transitionWhoosh(label, b.OutgoingIndex+1, before, after, start, count, delay, o.WhooshGainDB))
			labels = append(labels, "["+label+"]")
		}
		// Event-anchored punch-ins get their own swish. Nearby cut cues already
		// cover the event, so never double the sound at the same boundary.
		if edges.zoomFrame != nil && edges.out != nil && *edges.zoomFrame < edges.frames-int64(o.DurationFrames) {
			before := int64(o.DurationFrames/2) * recapplan.SamplesPerFrame
			after := int64(o.DurationFrames-o.DurationFrames/2) * recapplan.SamplesPerFrame
			offset := *edges.zoomFrame*recapplan.SamplesPerFrame - before
			start, delay := max(int64(0), -offset), max(int64(0), offset)
			count := min(before+after-start, samples-delay)
			clauses = append(clauses, transitionWhoosh("zoomwhoosh", edges.out.OutgoingIndex+997, before, after, start, count, delay, o.WhooshGainDB))
			labels = append(labels, "[zoomwhoosh]")
		}
	}
	if o.Impact && edges.in != nil {
		count := min(int64(o.ImpactDurationMS)*48, samples)
		duration := float64(count) / recapplan.SampleRate
		clauses = append(clauses, fmt.Sprintf("aevalsrc='0.7*sin(2*PI*%d*t)*exp(-7*t/%s)':s=48000:d=%s,lowpass=f=140,afade=t=in:ss=0:ns=%d,afade=t=out:ss=%d:ns=%d,aformat=channel_layouts=stereo,volume=%sdB[impact]",
			o.ImpactFrequency, decimal(duration), decimal(duration), min(int64(96), count/2), count-min(int64(960), count/2), min(int64(960), count/2), decimal(o.ImpactGainDB)))
		labels = append(labels, "[impact]")
	}
	if len(labels) == 0 {
		return "", "declicked"
	}
	clauses = append(clauses, fmt.Sprintf("[declicked]%samix=inputs=%d:duration=first:normalize=0:dropout_transition=0,apad=whole_len=%d,atrim=end_sample=%d,asetpts=N/SR/TB[transitionaudio]", strings.Join(labels, ""), len(labels)+1, samples, samples))
	return ";" + strings.Join(clauses, ";"), "transitionaudio"
}

func transitionWhoosh(label string, seed int, before, after, start, count, delay int64, gain float64) string {
	return fmt.Sprintf("anoisesrc=r=48000:c=pink:s=%d:d=%s,highpass=f=180,lowpass=f=1800,afade=t=in:ss=0:ns=%d:curve=qsin,afade=t=out:ss=%d:ns=%d:curve=qsin,atrim=start_sample=%d:end_sample=%d,asetpts=PTS-STARTPTS,aformat=channel_layouts=stereo,volume=%sdB,adelay=%dS:all=1[%s]",
		seed, decimal(float64(before+after)/recapplan.SampleRate), before, before, after, start, start+count, decimal(gain), delay, label)
}

func (e *FullDemoRenderEvidence) validateTransitions() error {
	expected := transitionDirections(e.Effective)
	if len(e.Transitions) != len(expected) {
		return fmt.Errorf("full demo transition evidence does not cover the approved boundaries")
	}
	for i, want := range expected {
		got := e.Transitions[i]
		if got.TransitionBoundary != want.TransitionBoundary {
			return fmt.Errorf("full demo transition evidence changed the frame timeline")
		}
		if got.DirectionSource == "estimated-motion" && e.Effective.Options.Transitions.Direction == "follow-motion" && e.Effective.Options.Transitions.Whip {
			switch got.Direction {
			case "left", "right", "up", "down":
				continue
			}
		}
		if got.Direction != want.Direction || got.DirectionSource != want.DirectionSource {
			return fmt.Errorf("full demo transition direction lacks approved or estimated evidence")
		}
	}
	return nil
}
