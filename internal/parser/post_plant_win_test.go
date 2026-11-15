package parser

import (
	"reflect"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func TestPostPlantWatchSpansAbutAndCloseWhenHidden(t *testing.T) {
	w := &postPlantWatch{planted: true}
	w.sample(100, 40, true)
	w.sample(101, 40, true)
	w.sample(102, 38, true)
	w.sample(103, 0, false)
	w.sample(150, 30, true)
	w.stop(160)
	want := []killplan.WinProbSpan{{TickStart: 100, TickEnd: 102, CTPct: 40}, {TickStart: 102, TickEnd: 103, CTPct: 38}, {TickStart: 150, TickEnd: 160, CTPct: 30}}
	if !reflect.DeepEqual(w.spans, want) {
		t.Fatalf("spans = %+v, want %+v", w.spans, want)
	}
	if w.planted || w.open {
		t.Fatal("round end left the watch open")
	}
}

func TestWithWinProbClipsSpansToEachSegment(t *testing.T) {
	segs := []killplan.Segment{{ID: "a", TickStart: 0, TickEnd: 120}, {ID: "b", TickStart: 200, TickEnd: 300}, {ID: "c", TickStart: 400, TickEnd: 500}}
	withWinProb(segs, []killplan.WinProbSpan{{TickStart: 100, TickEnd: 210, CTPct: 40}, {TickStart: 210, TickEnd: 260, CTPct: 35}})
	if got := segs[0].WinProb; !reflect.DeepEqual(got, []killplan.WinProbSpan{{TickStart: 100, TickEnd: 120, CTPct: 40}}) {
		t.Fatalf("segment a = %+v", got)
	}
	if got := segs[1].WinProb; len(got) != 2 || got[0].TickStart != 200 || got[1].TickEnd != 260 {
		t.Fatalf("segment b = %+v", got)
	}
	if segs[2].WinProb != nil {
		t.Fatalf("segment outside every span got %+v", segs[2].WinProb)
	}
}
