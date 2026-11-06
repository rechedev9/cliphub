package tactical

import (
	"strings"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"

	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// sampleGear reads what a player holds at a sampled tick, the loadout a
// Skybox-style player card shows.
func sampleGear(p *common.Player) *tacticalplan.Gear {
	g := &tacticalplan.Gear{
		ActiveWeapon: equipmentOf(p.ActiveWeapon()),
		Armor:        p.Armor(),
		Helmet:       p.HasHelmet(),
		Defuser:      p.HasDefuseKit(),
		Money:        p.Money(),
		FlashSeconds: p.FlashDurationTimeRemaining().Seconds(),
	}
	for _, w := range p.Weapons() {
		item := equipmentOf(w)
		if item == tacticalplan.EquipmentUnknown {
			continue
		}
		count := 1
		// Flashbangs are the one grenade a player can carry two of, and the
		// count lives on the pawn rather than on the weapon entity.
		if w.Type == common.EqFlash {
			count = max(count, int(min(p.FlashbangCount(), 4))) // #nosec G115 -- clamped to the 2-bit wire count.
		}
		g.Inventory = append(g.Inventory, tacticalplan.InventoryItem{Item: item, Count: count})
	}
	return g
}

func equipmentOf(w *common.Equipment) tacticalplan.Equipment {
	if w == nil {
		return tacticalplan.EquipmentUnknown
	}
	name := parser.WeaponName(w)
	if item := tacticalplan.EquipmentByName(name); item != tacticalplan.EquipmentUnknown {
		return item
	}
	// Knife skins keep their own entity names (knife_karambit, bayonet).
	if w.Type == common.EqKnife || strings.HasPrefix(name, "knife") || name == "bayonet" {
		return tacticalplan.EquipmentByName("knife")
	}
	return tacticalplan.EquipmentUnknown
}
