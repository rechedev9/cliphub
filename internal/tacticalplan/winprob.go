package tacticalplan

import (
	"math"
	"sort"
)

// The win probability model is a logistic regression with fixed coefficients,
// read from the CT side. It is set by hand from rough pro round-win rates (an
// opening kill wins about 70 % of rounds, an anti-eco about 85 %, a plant about
// 70 % for T), not fitted, so every number below can be read and argued with.
const (
	// WinModelVersion names the coefficients, so a change shows in every report.
	WinModelVersion = "wp1"
	// winAliveCoef weighs each player of difference in the alive count.
	winAliveCoef = 0.9
	// winEquipCoef weighs ln(average CT equipment / average T equipment) of the alive players.
	winEquipCoef = 1.0
	// winClockCoef grows the CT edge as the round clock runs with no plant.
	winClockCoef = 1.2
	// winPlantCoef is the T edge the plant itself gives.
	winPlantCoef = -1.1
	// winBombCoef grows the T edge as the bomb timer runs.
	winBombCoef = -1.5
	// RoundSeconds and BombSeconds are the CS2 round and bomb timers.
	RoundSeconds = 115.0
	BombSeconds  = 40.0
	// winMinEquip is the floor of one player's equipment: a default pistol.
	winMinEquip = 200
	// winFloor keeps an open round away from a certain outcome.
	winFloor = 0.01
	// With no T alive after a plant, CT defuses with certainty up to this many
	// bomb seconds, and with none after winDefuseLateSeconds (no kit is assumed
	// lost or kept; the 10 s defuse is the bound).
	winDefuseSafeSeconds = 25.0
	winDefuseLateSeconds = 35.0
)

// winMapPrior is the CT edge of each map at an even state, in logit units.
// Maps missing here start even.
var winMapPrior = map[string]float64{
	"de_nuke":     0.20,
	"de_train":    0.15,
	"de_overpass": 0.10,
	"de_ancient":  0.10,
	"de_inferno":  0.05,
	"de_mirage":   0.00,
	"de_vertigo":  0.00,
	"de_dust2":    -0.10,
	"de_anubis":   -0.15,
}

// WinState is a round's state at one tick, as the model reads it.
type WinState struct {
	Map     string
	AliveCT int
	AliveT  int
	// EquipCT and EquipT add up the round-start equipment of the alive players.
	EquipCT int
	EquipT  int
	// Seconds runs from freeze end; BombSeconds from the plant.
	Seconds     float64
	Planted     bool
	BombSeconds float64
}

// WinProbabilityCT is the chance the CT side wins the round from this state.
func WinProbabilityCT(s WinState) float64 {
	if s.AliveCT <= 0 {
		return 0
	}
	if s.AliveT <= 0 {
		if !s.Planted {
			return 1
		}
		switch {
		case s.BombSeconds <= winDefuseSafeSeconds:
			return 1 - winFloor
		case s.BombSeconds >= winDefuseLateSeconds:
			return winFloor
		default:
			left := (winDefuseLateSeconds - s.BombSeconds) / (winDefuseLateSeconds - winDefuseSafeSeconds)
			return winFloor + (1-2*winFloor)*left
		}
	}
	logit := winMapPrior[s.Map]
	logit += winAliveCoef * float64(s.AliveCT-s.AliveT)
	avgCT := math.Max(float64(s.EquipCT)/float64(s.AliveCT), winMinEquip)
	avgT := math.Max(float64(s.EquipT)/float64(s.AliveT), winMinEquip)
	logit += winEquipCoef * math.Log(avgCT/avgT)
	if s.Planted {
		logit += winPlantCoef + winBombCoef*math.Min(math.Max(s.BombSeconds, 0)/BombSeconds, 1)
	} else {
		logit += winClockCoef * math.Min(math.Max(s.Seconds, 0)/RoundSeconds, 1)
	}
	p := 1 / (1 + math.Exp(-logit))
	return math.Min(math.Max(p, winFloor), 1-winFloor)
}

// WinModel is the model a report used, shown next to the numbers.
type WinModel struct {
	Version      string             `json:"version"`
	AliveCoef    float64            `json:"alive_coef"`
	EquipCoef    float64            `json:"equip_coef"`
	ClockCoef    float64            `json:"clock_coef"`
	PlantCoef    float64            `json:"plant_coef"`
	BombCoef     float64            `json:"bomb_coef"`
	RoundSeconds float64            `json:"round_seconds"`
	BombSeconds  float64            `json:"bomb_seconds"`
	MinEquip     int                `json:"min_equip"`
	MapPriors    map[string]float64 `json:"map_priors"`
}

func winModel(maps []string) WinModel {
	priors := map[string]float64{}
	sorted := append([]string(nil), maps...)
	sort.Strings(sorted)
	for _, m := range sorted {
		priors[m] = winMapPrior[m]
	}
	return WinModel{
		Version: WinModelVersion, AliveCoef: winAliveCoef, EquipCoef: winEquipCoef, ClockCoef: winClockCoef,
		PlantCoef: winPlantCoef, BombCoef: winBombCoef, RoundSeconds: RoundSeconds, BombSeconds: BombSeconds,
		MinEquip: winMinEquip, MapPriors: priors,
	}
}
