package parser

import (
	"math"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"

	"github.com/rechedev9/cliphub/internal/killplan"
)

// Flick: the target's view turned at least flickDegrees within flickWindowSeconds before a gun kill.
const (
	flickDegrees       = 45
	flickWindowSeconds = 0.2
	flickTrailSize     = 32
)

// Eco: the killer's team bought at most ecoRatio of the enemy's average
// freeze-end equipment, and the enemy averaged at least ecoEnemyMinEquip.
const (
	ecoRatio         = 0.5
	ecoEnemyMinEquip = 2000
)

// isOpeningCandidate reports whether a kill can be a round's opening kill: two players of opposite teams.
func isOpeningCandidate(e events.Kill) bool {
	if e.Killer == nil || e.Victim == nil || e.Killer.SteamID64 == e.Victim.SteamID64 {
		return false
	}
	return sideLabel(e.Killer.Team) != "" && sideLabel(e.Victim.Team) != "" && e.Killer.Team != e.Victim.Team
}

// isGunKill keeps the weapons a flick can aim: pistols, SMGs, heavy guns and rifles.
func isGunKill(w *common.Equipment) bool {
	if w == nil {
		return false
	}
	switch w.Class() {
	case common.EqClassPistols, common.EqClassSMG, common.EqClassHeavy, common.EqClassRifle:
		return true
	default:
		return false
	}
}

// roundPlayers lists every playing player, dead ones included, with their freeze-end equipment.
func roundPlayers(players []*common.Player) []alivePlayer {
	out := make([]alivePlayer, 0, len(players))
	for _, p := range players {
		if p != nil {
			out = append(out, alivePlayer{SteamID64: p.SteamID64, Team: p.Team, Equip: p.EquipmentValueFreezeTimeEnd()})
		}
	}
	return out
}

// ecoRound reports whether side played the round on a clearly smaller buy than its enemy.
func ecoRound(players []alivePlayer, side common.Team) bool {
	if side != common.TeamCounterTerrorists && side != common.TeamTerrorists {
		return false
	}
	own, enemy, ownN, enemyN := 0, 0, 0, 0
	for _, p := range players {
		switch {
		case p.Team == side:
			own += p.Equip
			ownN++
		case sideLabel(p.Team) != "":
			enemy += p.Equip
			enemyN++
		}
	}
	if ownN == 0 || enemyN == 0 {
		return false
	}
	ownAvg, enemyAvg := float64(own)/float64(ownN), float64(enemy)/float64(enemyN)
	return enemyAvg >= ecoEnemyMinEquip && ownAvg <= ecoRatio*enemyAvg
}

type viewSample struct {
	tick       int
	yaw, pitch float64
}

// viewTrail keeps the target's last view angles, one sample per frame, for the flick check.
type viewTrail struct {
	samples [flickTrailSize]viewSample
	n, next int
}

func (t *viewTrail) reset() { t.n, t.next = 0, 0 }

func (t *viewTrail) record(s viewSample) {
	t.samples[t.next] = s
	t.next = (t.next + 1) % flickTrailSize
	t.n = min(t.n+1, flickTrailSize)
}

// flick reports whether the view at now sits flickDegrees or more from any sample in the window before it.
func (t *viewTrail) flick(now viewSample, windowTicks int) bool {
	for i := range t.n {
		s := t.samples[i]
		if s.tick >= now.tick || now.tick-s.tick > windowTicks {
			continue
		}
		if math.Hypot(angleDelta(s.yaw, now.yaw), angleDelta(s.pitch, now.pitch)) >= flickDegrees {
			return true
		}
	}
	return false
}

// angleDelta is the shortest turn from a to b in degrees, wrapping at 360.
func angleDelta(a, b float64) float64 {
	d := math.Mod(b-a, 360)
	if d > 180 {
		d -= 360
	} else if d < -180 {
		d += 360
	}
	return d
}

// markRoundResults sets what each kill's round result decides: the clutch
// result, a retake (a CT kill after the plant in a round CT won, so by defuse)
// and an eco win. It reads each round's first known winner.
func markRoundResults(kills []RawKill, roundEnds []RoundEnd) {
	winners := make(map[int]string, len(roundEnds))
	for _, re := range roundEnds {
		if _, ok := winners[re.Round]; !ok && re.Winner != "" {
			winners[re.Round] = re.Winner
		}
	}
	for i := range kills {
		k := &kills[i]
		winner, ok := winners[k.Round]
		won := ok && winner == k.Killer.TeamAtKill
		switch {
		case k.ClutchVs <= 0 || !ok:
			k.ClutchResult = ""
		case won:
			k.ClutchResult = killplan.ClutchWon
		default:
			k.ClutchResult = killplan.ClutchLost
		}
		k.Retake = won && k.AfterPlant && winner == "CT"
		k.EcoWin = won && k.Eco
	}
}
