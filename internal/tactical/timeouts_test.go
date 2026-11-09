package tactical

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

var limits = freezeLimits{freeze: 15 * 64, slack: 3 * 64, timeoutKeep: 60 * 64}

func TestATimeoutKeepsItsPartOfTheFreeze(t *testing.T) {
	// A 30 s tactical timeout from 50000, then 15 s of freeze.
	r := &roundAcc{tickStart: 49000, tickFreezeEnd: 50000 + 45*64, tickEnd: 60000, frames: []tacticalplan.Frame{{Tick: 49500}, {Tick: 50100}}}
	r.addTimeout(tacticalplan.Timeout{Side: tacticalplan.SideCT, Tick: 50000, EndTick: 50000 + 30*64})
	if r.settleFreeze(limits) != freezeTrimmed || r.tickStart != 50000 || len(r.frames) != 1 {
		t.Fatalf("tick start = %d, frames %+v; want the whole timeout kept from 50000", r.tickStart, r.frames)
	}
}

func TestALongTechnicalPauseKeepsItsLastMinute(t *testing.T) {
	// Inferno round 13: T's technical timeout runs 63048 to 69565 (102 s), then 15 s of freeze.
	r := &roundAcc{
		tickStart: 62315, tickFreezeEnd: 70525, tickEnd: 74679,
		frames: []tacticalplan.Frame{{Tick: 62400}, {Tick: 63100}, {Tick: 70600}},
	}
	var log timeoutLog
	log.start(tacticalplan.SideT, 63048)
	log.setTechnical(true)
	got, ok := log.stop(tacticalplan.SideT, 69565)
	if !ok {
		t.Fatal("the T timeout was not running")
	}
	r.addTimeout(got)
	if r.settleFreeze(limits) != freezeTrimmed || r.tickStart != 69565-60*64 {
		t.Fatalf("tick start = %d, want the pause's last minute from %d", r.tickStart, 69565-60*64)
	}
	// The timeout keeps its real start, so its one band can say it ran 102 s.
	want := tacticalplan.Timeout{Side: tacticalplan.SideT, Tick: 63048, EndTick: 69565, Technical: true}
	if len(r.timeouts) != 1 || r.timeouts[0] != want {
		t.Fatalf("timeouts = %+v, want %+v", r.timeouts, want)
	}
	if len(r.frames) != 1 {
		t.Fatalf("frames = %+v, want the one inside the kept minute", r.frames)
	}
}

func TestTimeoutLogFollowsOneSideAtATime(t *testing.T) {
	var log timeoutLog
	log.setTechnical(true)
	log.start(tacticalplan.SideCT, 100)
	if _, ok := log.stop(tacticalplan.SideT, 200); ok {
		t.Fatal("T's flag cannot end CT's timeout")
	}
	log.setTechnical(false)
	got, ok := log.stop(tacticalplan.SideCT, 300)
	if !ok || !got.Technical || got.EndTick != 300 {
		t.Fatalf("timeout = %+v, %v; want technical, ending at 300", got, ok)
	}
	if _, ok := log.stop(tacticalplan.SideCT, 400); ok {
		t.Fatal("a stopped timeout stops once")
	}
}

func TestATimeoutStartedBeforeTheRoundIsClippedToIt(t *testing.T) {
	r := &roundAcc{tickStart: 1000}
	r.addTimeout(tacticalplan.Timeout{Side: tacticalplan.SideCT, Tick: 900, EndTick: 2000})
	r.addTimeout(tacticalplan.Timeout{Side: tacticalplan.SideT, Tick: 500, EndTick: 800})
	if len(r.timeouts) != 1 || r.timeouts[0].Tick != 1000 {
		t.Fatalf("timeouts = %+v, want the CT one from the round start", r.timeouts)
	}
}
