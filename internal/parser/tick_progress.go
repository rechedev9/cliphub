package parser

import (
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

// ProgressFunc is called with the current and total demo ticks. Total may be 0
// until the parser has a header-backed Progress() value; callers tolerate that.
type ProgressFunc func(done, total int)

// AttachTickProgress registers a FrameDone hook that reports how far the
// parser has walked. Done prefers IngameTick (the match clock); total is
// derived from the parser's own 0-1 Progress(), which is header PlaybackFrames.
func AttachTickProgress(p demoinfocs.Parser, report ProgressFunc) {
	if p == nil || report == nil {
		return
	}
	p.RegisterEventHandler(func(events.FrameDone) {
		report(TickProgress(p))
	})
}

// TickProgress is the current/total pair AttachTickProgress reports.
func TickProgress(p demoinfocs.Parser) (done, total int) {
	done = p.GameState().IngameTick()
	if done <= 0 {
		done = p.CurrentFrame()
	}
	if done < 0 {
		done = 0
	}
	frac := p.Progress()
	if frac > 0 && done > 0 {
		total = int(float32(done)/frac + 0.5)
	}
	if total < done {
		total = done
	}
	return done, total
}
