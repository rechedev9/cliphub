package parser

import (
	"testing"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	dp "github.com/markus-wa/godispatch"
)

type progressParser struct {
	demoinfocs.Parser
	tick     int
	frame    int
	progress float32
	handler  func(events.FrameDone)
}

func (p *progressParser) RegisterEventHandler(h any) dp.HandlerIdentifier {
	if fn, ok := h.(func(events.FrameDone)); ok {
		p.handler = fn
	}
	return nil
}

func (p *progressParser) CurrentFrame() int { return p.frame }
func (p *progressParser) Progress() float32 { return p.progress }
func (p *progressParser) GameState() demoinfocs.GameState {
	return progressGameState{tick: p.tick}
}

type progressGameState struct {
	demoinfocs.GameState
	tick int
}

func (s progressGameState) IngameTick() int { return s.tick }

func TestAttachTickProgressReportsIngameTicks(t *testing.T) {
	p := &progressParser{tick: 64000, progress: 64000.0 / 172772.0}
	var gotDone, gotTotal int
	AttachTickProgress(p, func(done, total int) {
		gotDone, gotTotal = done, total
	})
	if p.handler == nil {
		t.Fatal("FrameDone handler was not registered")
	}
	p.handler(events.FrameDone{})
	if gotDone != 64000 {
		t.Fatalf("done = %d, want 64000", gotDone)
	}
	if gotTotal < 172000 || gotTotal > 174000 {
		t.Fatalf("total = %d, want ~172772", gotTotal)
	}
}

func TestTickProgressFallsBackToCurrentFrame(t *testing.T) {
	cases := []struct {
		name     string
		tick     int
		frame    int
		progress float32
		wantDone int
		minTotal int
	}{
		{name: "neg tick uses frame", tick: -1, frame: 20, progress: 0.25, wantDone: 20, minTotal: 80},
		{name: "zero progress keeps total at done", tick: 10, frame: 10, progress: 0, wantDone: 10, minTotal: 10},
		{name: "over-progress clamps total up", tick: 50, frame: 50, progress: 2, wantDone: 50, minTotal: 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &progressParser{tick: tc.tick, frame: tc.frame, progress: tc.progress}
			done, total := TickProgress(p)
			if done != tc.wantDone || total < tc.minTotal {
				t.Fatalf("got %d/%d, want done=%d total>=%d", done, total, tc.wantDone, tc.minTotal)
			}
		})
	}
}

func TestAttachTickProgressNilIsSafe(t *testing.T) {
	AttachTickProgress(nil, func(int, int) {})
	AttachTickProgress(&progressParser{}, nil)
}
