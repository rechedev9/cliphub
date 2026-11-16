package parser

import (
	"math"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// killSwingInput is the round state at one kill, read by killWinSwing.
type killSwingInput struct {
	Map string
	// Alive are the living players at the kill; the victim may or may not be among them.
	Alive          []alivePlayer
	Killer, Victim alivePlayer
	// Seconds runs from freeze end; BombSeconds from the plant.
	Seconds     float64
	Planted     bool
	BombSeconds float64
}

// killWinSwing is how much the kill moved the killer side's round-win chance
// under the tacticalplan win model, rounded to four decimals.
func killWinSwing(in killSwingInput) float64 {
	if in.Killer.Team != common.TeamCounterTerrorists && in.Killer.Team != common.TeamTerrorists {
		return 0
	}
	after := tacticalplan.WinState{Map: in.Map, Seconds: in.Seconds, Planted: in.Planted, BombSeconds: in.BombSeconds}
	for _, p := range in.Alive {
		if p.SteamID64 != in.Victim.SteamID64 {
			addWinPlayer(&after, p)
		}
	}
	before := after
	addWinPlayer(&before, in.Victim)
	delta := tacticalplan.WinProbabilityCT(after) - tacticalplan.WinProbabilityCT(before)
	if in.Killer.Team == common.TeamTerrorists {
		delta = -delta
	}
	return math.Round(delta*10000) / 10000
}

func addWinPlayer(s *tacticalplan.WinState, p alivePlayer) {
	switch p.Team {
	case common.TeamCounterTerrorists:
		s.AliveCT++
		s.EquipCT += p.Equip
	case common.TeamTerrorists:
		s.AliveT++
		s.EquipT += p.Equip
	}
}
