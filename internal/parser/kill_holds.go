package parser

import (
	"math"
	"strconv"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/demo-parser-rsn/pkg/demoinfocs/common"
)

type holdPoint struct {
	tick int
	x, y float64
}

type holdPosition struct {
	steamID uint64
	x, y    float64
}

type holdTrail struct {
	last    int
	sampled bool
	points  map[uint64][]holdPoint
}

func holdStep(rate int) int { return max(1, rate/recapplan.HoldSampleHz) }

func (t *holdTrail) reset() { *t = holdTrail{} }

func (t *holdTrail) record(tick, rate int, players []holdPosition) {
	if rate <= 0 {
		return
	}
	if t.sampled && tick < t.last {
		t.reset()
	}
	if t.sampled && tick-t.last < holdStep(rate) {
		return
	}
	t.last, t.sampled = tick, true
	if t.points == nil {
		t.points = map[uint64][]holdPoint{}
	}
	keep := tick - (2*recapplan.SecondCameraLeadSeconds+1)*rate/2
	for _, p := range players {
		if p.steamID == 0 {
			continue
		}
		points := t.points[p.steamID]
		first := 0
		for first < len(points) && points[first].tick < keep {
			first++
		}
		t.points[p.steamID] = append(points[first:], holdPoint{tick: tick, x: p.x, y: p.y})
	}
}

func (t *holdTrail) hold(victim uint64, tick, rate int, at [3]float64) recapplan.KillHold {
	h := recapplan.KillHold{Tick: tick, VictimSteamID64: strconv.FormatUint(victim, 10)}
	if rate <= 0 {
		return h
	}
	step := holdStep(rate)
	start := tick - recapplan.SecondCameraLeadSeconds*rate
	seen, begins, gap, previous, move := false, false, false, 0, 0.0
	for _, p := range t.points[victim] {
		if p.tick > tick || p.tick < start-step {
			continue
		}
		if !seen {
			begins = p.tick <= start
		} else if p.tick-previous > 2*step {
			gap = true
		}
		seen, previous = true, p.tick
		move = max(move, math.Hypot(p.x-at[0], p.y-at[1]))
	}
	if math.IsNaN(move) || math.IsInf(move, 0) {
		return h
	}
	h.MoveUnits = math.Round(move*100) / 100
	h.Covered = seen && begins && !gap && tick-previous <= 2*step
	return h
}

func (c *Collector) RecordKillHold(h recapplan.KillHold) {
	if len(c.holds) < recapplan.MaxKillHolds {
		c.holds = append(c.holds, h)
	}
}

func holdPositions(players []*common.Player, targetID uint64) []holdPosition {
	positions := make([]holdPosition, 0, len(players))
	for _, player := range players {
		if player == nil || player.SteamID64 == targetID || !player.IsAlive() {
			continue
		}
		at := playerPosition(player)
		positions = append(positions, holdPosition{steamID: player.SteamID64, x: at[0], y: at[1]})
	}
	return positions
}
