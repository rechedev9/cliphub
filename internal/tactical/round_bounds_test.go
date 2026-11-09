package tactical

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestHalftimeBreakIsCutFromTheRoundStart(t *testing.T) {
	// Inferno round 13: RoundStart at 62315, the halftime break, then 15 s of
	// freeze time ending at 70525. A team switch "kill" sits inside the break.
	r := &roundAcc{
		tickStart: 62315, tickFreezeEnd: 70525, tickEnd: 74679,
		frames: []tacticalplan.Frame{{Tick: 62400}, {Tick: 69600}, {Tick: 70600}},
		events: []tacticalplan.Event{{Tick: 63048, Kind: tacticalplan.EventKill}, {Tick: 71000, Kind: tacticalplan.EventSmoke}},
	}
	got := r.settleFreeze(freezeLimits{freeze: 15 * 64, slack: 3 * 64, timeoutKeep: 60 * 64})
	if got != freezeTrimmed {
		t.Fatalf("settleFreeze = %v, want trimmed", got)
	}
	if r.tickStart != 70525-15*64 {
		t.Fatalf("tick start = %d, want %d", r.tickStart, 70525-15*64)
	}
	if len(r.frames) != 2 || r.frames[0].Tick != 69600 {
		t.Fatalf("frames = %+v, want the two at or after the new start", r.frames)
	}
	if len(r.events) != 1 || r.events[0].Kind != tacticalplan.EventSmoke {
		t.Fatalf("events = %+v, want only the smoke", r.events)
	}
}

func TestANormalFreezeTimeIsKept(t *testing.T) {
	r := &roundAcc{tickStart: 1000, tickFreezeEnd: 1000 + 20*64, tickEnd: 9000}
	if got := r.settleFreeze(freezeLimits{freeze: 20 * 64, slack: 3 * 64, timeoutKeep: 60 * 64}); got != freezeKept {
		t.Fatalf("settleFreeze = %v, want kept", got)
	}
	if r.tickStart != 1000 {
		t.Fatalf("tick start moved to %d", r.tickStart)
	}
}

func TestAMissingFreezeEndIsEstimatedFromTheFreezeTime(t *testing.T) {
	r := &roundAcc{tickStart: 1000, tickEnd: 9000}
	if got := r.settleFreeze(freezeLimits{freeze: 20 * 64, slack: 3 * 64, timeoutKeep: 60 * 64}); got != freezeEstimated {
		t.Fatalf("settleFreeze = %v, want estimated", got)
	}
	if r.tickFreezeEnd != 1000+20*64 {
		t.Fatalf("freeze end = %d, want %d", r.tickFreezeEnd, 1000+20*64)
	}

	short := &roundAcc{tickStart: 1000, tickEnd: 1500}
	short.settleFreeze(freezeLimits{freeze: 20 * 64, slack: 3 * 64, timeoutKeep: 60 * 64})
	if short.tickFreezeEnd != 1500 {
		t.Fatalf("freeze end of a round shorter than freeze time = %d, want its end", short.tickFreezeEnd)
	}
}

func TestACollateralOpeningMarksOnlyTheFirstVictim(t *testing.T) {
	// An AWP through two T on the round's first shot: two kills on one tick.
	awper, first, second := uint8(0), uint8(1), uint8(2)
	s := &scanner{tickrate: 64}
	acc := &roundAcc{
		stats: map[uint8]*playerRoundAcc{},
		sides: map[uint8]tacticalplan.Side{},
		kills: []killRecord{
			{tick: 500, killerSlot: &awper, victimSlot: first, killerSide: tacticalplan.SideCT, victimSide: tacticalplan.SideT},
			{tick: 500, killerSlot: &awper, victimSlot: second, killerSide: tacticalplan.SideCT, victimSide: tacticalplan.SideT, traded: true},
		},
		events: []tacticalplan.Event{
			{Tick: 500, Kind: tacticalplan.EventKill, ActorSlot: &awper, TargetSlot: &first},
			{Tick: 500, Kind: tacticalplan.EventKill, ActorSlot: &awper, TargetSlot: &second},
		},
	}
	round := s.buildRound(acc, map[uint8]uint8{0: 0, 1: 1, 2: 2})
	if !round.Events[0].Opening || round.Events[1].Opening {
		t.Fatalf("opening flags = %v, %v; want only the first kill", round.Events[0].Opening, round.Events[1].Opening)
	}
	if round.Events[0].Traded || !round.Events[1].Traded {
		t.Fatalf("traded flags = %v, %v; want only the second kill", round.Events[0].Traded, round.Events[1].Traded)
	}
}

// checkRealDemoBoundsAndLifetimes holds a real demo to the round bounds and
// utility ends: no freeze band past the longest freeze time any league uses,
// no death in freeze time, smokes and fires that end, bombs dropped and found.
func checkRealDemoBoundsAndLifetimes(t *testing.T, doc tacticalplan.Document) {
	t.Helper()
	rate := doc.Demo.Tickrate
	smokes, smokesEnded, fires, firesEnded, drops := 0, 0, 0, 0, 0
	for _, r := range doc.Rounds {
		if freeze := float64(r.TickFreezeEnd-r.TickStart) / rate; freeze > 25 {
			t.Errorf("round %d has %.1f s of freeze time", r.Number, freeze)
		}
		for _, ev := range r.Events {
			if ev.Tick < r.TickStart {
				t.Errorf("round %d: %s at %d is before the round start %d", r.Number, ev.Kind, ev.Tick, r.TickStart)
			}
			switch ev.Kind {
			case tacticalplan.EventKill:
				if ev.Tick < r.TickFreezeEnd {
					t.Errorf("round %d: kill at %d in freeze time (ends %d)", r.Number, ev.Tick, r.TickFreezeEnd)
				}
			case tacticalplan.EventSmoke:
				smokes++
				if ev.EndTick > ev.Tick {
					smokesEnded++
				}
			case tacticalplan.EventMolotov:
				fires++
				if ev.EndTick > ev.Tick {
					firesEnded++
				}
				// Flames burn 7 s at most; the inferno entity lingers for about 20.
				if burned := float64(ev.EndTick-ev.Tick) / rate; burned > 8 {
					t.Errorf("round %d: fire at %d burned %.1f s", r.Number, ev.Tick, burned)
				}
			case tacticalplan.EventDropBomb:
				drops++
				if ev.Pos == [3]float64{} {
					t.Errorf("round %d: dropped bomb at %d has no position", r.Number, ev.Tick)
				}
			case tacticalplan.EventExplode:
				if ev.Pos == [3]float64{} {
					t.Errorf("round %d: bomb exploded at the world origin", r.Number)
				}
			}
		}
	}
	t.Logf("%d of %d smokes and %d of %d fires end; %d bombs dropped", smokesEnded, smokes, firesEnded, fires, drops)
	if smokesEnded*10 < smokes*8 || firesEnded*10 < fires*8 {
		t.Errorf("too few utility end ticks: %d/%d smokes, %d/%d fires", smokesEnded, smokes, firesEnded, fires)
	}
	if drops == 0 {
		t.Error("no bomb was ever dropped in a whole match")
	}
}

func TestAnUnknownFreezeTimeChangesNothing(t *testing.T) {
	r := &roundAcc{tickStart: 1000, tickFreezeEnd: 9000, tickEnd: 12000}
	if got := r.settleFreeze(freezeLimits{freeze: 0, slack: 3 * 64, timeoutKeep: 60 * 64}); got != freezeKept {
		t.Fatalf("settleFreeze = %v, want kept", got)
	}
	if r.tickStart != 1000 || r.tickFreezeEnd != 9000 {
		t.Fatalf("round moved to %d..%d", r.tickStart, r.tickFreezeEnd)
	}
}
