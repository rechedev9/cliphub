package tacticalplan

import "testing"

func TestPostPlantWinPctCTGates(t *testing.T) {
	open := WinState{Map: "de_mirage", AliveCT: 3, AliveT: 2, EquipCT: 12000, EquipT: 9000, Planted: true, BombSeconds: 10}
	pct, ok := PostPlantWinPctCT(open, 30)
	if !ok || pct < 1 || pct > 99 {
		t.Fatalf("open post-plant = %d, %v; want a shown 1..99 share", pct, ok)
	}
	later := open
	later.BombSeconds = 30
	if late, _ := PostPlantWinPctCT(later, 10); late >= pct {
		t.Fatalf("CT chance should fall as the bomb timer runs: %d then %d", pct, late)
	}
	for name, tc := range map[string]struct {
		state    WinState
		bombLeft float64
	}{
		"not planted":     {WinState{AliveCT: 3, AliveT: 3, EquipCT: 1, EquipT: 1}, 30},
		"bomb nearly out": {open, 4.9},
		"one CT left":     {WinState{AliveCT: 1, AliveT: 3, EquipCT: 1, EquipT: 1, Planted: true}, 30},
		"one T left":      {WinState{AliveCT: 3, AliveT: 1, EquipCT: 1, EquipT: 1, Planted: true}, 30},
	} {
		if _, ok := PostPlantWinPctCT(tc.state, tc.bombLeft); ok {
			t.Errorf("%s: overlay shown, want hidden", name)
		}
	}
	if _, ok := PostPlantWinPctCT(open, PostPlantMinBombLeft); !ok {
		t.Error("exactly the minimum bomb time left must still show")
	}
}
