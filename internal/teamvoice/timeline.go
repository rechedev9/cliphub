package teamvoice

import (
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Round is one round of the demo as voice reads it, in demo milliseconds from tick 0.
type Round struct {
	Number      int
	StartMS     int64
	FreezeEndMS int64
	EndMS       int64
	// PlantMS is 0 for a round without a plant.
	PlantMS int64
	// NextStartMS ends the time after the round: the next round's start, or EndMS for the last one.
	NextStartMS int64
	KillMS      []int64
	// DeathMS is when each player (by SteamID64) died in the round.
	DeathMS map[string]int64
}

// Timeline is the demo's rounds in play order and its length.
type Timeline struct {
	Tickrate   float64
	DurationMS int64
	Rounds     []Round
}

// afterLastRoundMS is how long the last round's after-time runs, as the demo usually ends soon after it.
const afterLastRoundMS = 10_000

// TimelineOf reads the rounds of a tactical document.
func TimelineOf(doc tacticalplan.Document) Timeline {
	rate := doc.Demo.Tickrate
	if rate <= 0 {
		rate = 64
	}
	ms := func(tick int) int64 { return int64(float64(tick) * 1000 / rate) }
	steamOf := map[uint8]string{}
	for _, p := range doc.Players {
		steamOf[p.Slot] = p.SteamID64
	}
	t := Timeline{Tickrate: rate, DurationMS: ms(doc.Demo.DurationTicks)}
	for i, r := range doc.Rounds {
		round := Round{Number: r.Number, StartMS: ms(r.TickStart), FreezeEndMS: ms(max(r.TickFreezeEnd, r.TickStart)), EndMS: ms(r.TickEnd), DeathMS: map[string]int64{}}
		round.NextStartMS = min(round.EndMS+afterLastRoundMS, t.DurationMS)
		if i+1 < len(doc.Rounds) {
			round.NextStartMS = ms(doc.Rounds[i+1].TickStart)
		}
		round.NextStartMS = max(round.NextStartMS, round.EndMS)
		if r.Bomb != nil && r.Bomb.PlantTick > 0 {
			round.PlantMS = ms(r.Bomb.PlantTick)
		}
		for _, e := range r.Events {
			if e.Kind == tacticalplan.EventKill || e.Kind == tacticalplan.EventPlant {
				round.KillMS = append(round.KillMS, ms(e.Tick))
			}
		}
		for _, p := range r.Players {
			if p.DeathTick > 0 && steamOf[p.Slot] != "" {
				round.DeathMS[steamOf[p.Slot]] = ms(p.DeathTick)
			}
		}
		t.Rounds = append(t.Rounds, round)
	}
	t.DurationMS = max(t.DurationMS, lastEnd(t.Rounds))
	return t
}

func lastEnd(rounds []Round) int64 {
	if len(rounds) == 0 {
		return 0
	}
	return rounds[len(rounds)-1].NextStartMS
}

// Tick is the demo tick of a demo time.
func (t Timeline) Tick(demoMS int64) int {
	return int(float64(demoMS) * t.Tickrate / 1000)
}

// RoundAt is the round a demo time falls in, from its start to the next round's start.
func (t Timeline) RoundAt(demoMS int64) (Round, bool) {
	for _, r := range t.Rounds {
		if demoMS >= r.StartMS && demoMS < r.NextStartMS {
			return r, true
		}
	}
	return Round{}, false
}
