package tacticalplan

import "math"

// The post-plant win overlay shows only while the round is still open to read:
// the bomb has PostPlantMinBombLeft seconds left and each side PostPlantMinAlive alive.
const (
	PostPlantMinBombLeft = 5.0
	PostPlantMinAlive    = 2
)

// PostPlantWinPctCT is the CT side's round-win chance in whole percent, as the
// post-plant overlay shows it; ok is false whenever the overlay must stay hidden.
func PostPlantWinPctCT(s WinState, bombLeft float64) (int, bool) {
	if !s.Planted || bombLeft < PostPlantMinBombLeft || s.AliveCT < PostPlantMinAlive || s.AliveT < PostPlantMinAlive {
		return 0, false
	}
	// winFloor keeps the chance inside 1..99, so both sides always read a share.
	return int(math.Round(WinProbabilityCT(s) * 100)), true
}
