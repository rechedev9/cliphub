package tactical

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// checkRealDemoGear asserts a real demo's samples carry a believable loadout,
// and logs what the gear costs over the version 1 layout.
func checkRealDemoGear(t *testing.T, frames []tacticalplan.Frame, blobBytes int) {
	t.Helper()
	knife := tacticalplan.EquipmentByName("knife")
	alive, armed, withKnife, pitched, samples := 0, 0, 0, 0, 0
	armored, helmet, defuser, money, grenades, flashed := false, false, false, false, false, false
	for _, f := range frames {
		for _, s := range f.Samples {
			samples++
			if s.Gear == nil {
				t.Fatalf("tick %d slot %d has no gear", f.Tick, s.Slot)
			}
			g := s.Gear
			money = money || g.Money > 0
			if !s.Flags.Has(tacticalplan.FlagAlive) {
				continue
			}
			alive++
			if g.ActiveWeapon != tacticalplan.EquipmentUnknown {
				armed++
			}
			if s.Pitch != 0 {
				pitched++
			}
			armored = armored || g.Armor > 0
			helmet = helmet || g.Helmet
			defuser = defuser || g.Defuser
			flashed = flashed || g.FlashSeconds > 0
			for _, it := range g.Inventory {
				withKnife += boolInt(it.Item == knife)
				switch it.Item.String() {
				case "flashbang", "smokegrenade", "hegrenade", "molotov", "incgrenade", "decoy":
					grenades = true
				}
			}
		}
	}
	t.Logf("gear: %d alive samples, %d armed, %d with a knife, %d pitched; armor %v helmet %v defuser %v money %v grenades %v flashed %v",
		alive, armed, withKnife, pitched, armored, helmet, defuser, money, grenades, flashed)
	if alive == 0 || armed*10 < alive*9 || withKnife*10 < alive*9 || pitched*2 < alive {
		t.Fatalf("living samples lack weapons, knives or pitch: %d alive, %d armed, %d knives, %d pitched", alive, armed, withKnife, pitched)
	}
	if !armored || !helmet || !defuser || !money || !grenades || !flashed {
		t.Fatal("a full match must show armor, a helmet, a defuser, money, grenades and a flashed player")
	}
	v1 := tacticalplan.PositionsHeaderSize + len(frames)*6 + samples*10
	t.Logf("blob %d bytes; version 1 layout %d bytes (+%.1f%%) for %d frames and %d samples",
		blobBytes, v1, float64(blobBytes-v1)*100/float64(v1), len(frames), samples)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
