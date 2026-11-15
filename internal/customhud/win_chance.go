package customhud

import (
	"fmt"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// The win chance sits where the MVP card shows; one is planted, the other ended.
const (
	winChanceY      = mvpY
	winChanceWidth  = 300
	winChanceHeight = 46
)

// postPlantWinCT reads the recorded plant, bomb clock, alive players and their
// freeze-end equipment. Any missing fact hides the overlay instead of guessing it.
func postPlantWinCT(s Snapshot, mapName string, tickRate int) int {
	if (s.Phase != "planted" && s.Phase != "defusing") || s.PlantTick <= 0 || s.TimeRemaining < 0 || tickRate < 1 {
		return 0
	}
	state := tacticalplan.WinState{Map: mapName, Planted: true, BombSeconds: float64(s.Tick-s.PlantTick) / float64(tickRate)}
	for _, p := range s.Players {
		if p.Inactive {
			continue
		}
		if !p.Known {
			return 0
		}
		if !p.Alive {
			continue
		}
		if p.Equip <= 0 {
			return 0
		}
		if p.Side == "CT" {
			state.AliveCT++
			state.EquipCT += p.Equip
		} else {
			state.AliveT++
			state.EquipT += p.Equip
		}
	}
	pct, ok := tacticalplan.PostPlantWinPctCT(state, float64(s.TimeRemaining))
	if !ok {
		return 0
	}
	return pct
}

// winChance draws both sides' chance under the scoreboard, CT left like the score.
func (s *scene) winChance(ct int) {
	t := s.r.Theme
	x := Width/2 - winChanceWidth/2
	s.panel("win-chance", x, winChanceY, winChanceWidth, winChanceHeight, t.Muted)
	s.typeText("win-chance/ct", fmt.Sprintf("%d%%", ct), x+14, winChanceY+16, 18, 60, 700, t.CT, "left")
	s.typeText("win-chance/label", "WIN PROBABILITY", Width/2, winChanceY+16, 11, winChanceWidth-150, 600, t.Muted, "center")
	s.typeText("win-chance/t", fmt.Sprintf("%d%%", 100-ct), x+winChanceWidth-14, winChanceY+16, 18, 60, 700, t.T, "right")
	barX, barY, barW := x+14, winChanceY+32, winChanceWidth-28
	ctW := barW * ct / 100
	s.rect("win-chance/ct-bar", barX, barY, ctW, 6, t.CT, 4)
	s.rect("win-chance/t-bar", barX+ctW, barY, barW-ctW, 6, t.T, 4)
}
