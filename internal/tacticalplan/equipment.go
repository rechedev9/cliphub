package tacticalplan

import "fmt"

// Equipment is a stable item id in the position blob. The numbers are wire
// format: append new items at the end, never renumber or reuse one. The web
// mirror is web/lib/tactical-equipment.ts and must list the same names in the
// same order. Ids stay below 64 because an inventory entry packs its count in
// the top two bits of the id byte.
type Equipment uint8

// EquipmentUnknown is an item the table does not name, or no item at all.
const EquipmentUnknown Equipment = 0

// equipmentNames is indexed by Equipment. The names are CS2 entity names
// without the "weapon_" prefix, the vocabulary parser.WeaponName returns.
var equipmentNames = [...]string{
	"",
	// Pistols
	"glock", "usp_silencer", "hkp2000", "p250", "elite", "fiveseven", "tec9", "cz75a", "deagle", "revolver",
	// SMGs
	"mac10", "mp9", "mp7", "mp5sd", "ump45", "p90", "bizon",
	// Heavy
	"nova", "xm1014", "sawedoff", "mag7", "m249", "negev",
	// Rifles
	"galilar", "famas", "ak47", "m4a1", "m4a1_silencer", "sg556", "aug", "ssg08", "awp", "g3sg1", "scar20",
	// Grenades
	"flashbang", "smokegrenade", "hegrenade", "molotov", "incgrenade", "decoy",
	// Other
	"c4", "knife", "taser", "healthshot",
}

// maxEquipmentID is the highest id an inventory byte can carry.
const maxEquipmentID = 63

var equipmentByName = func() map[string]Equipment {
	if len(equipmentNames) > maxEquipmentID+1 {
		panic(fmt.Sprintf("equipment table has %d entries; inventory bytes address %d", len(equipmentNames), maxEquipmentID+1))
	}
	m := make(map[string]Equipment, len(equipmentNames))
	for i, name := range equipmentNames {
		if name != "" {
			m[name] = Equipment(i) // #nosec G115 -- the table length is checked against the id width above.
		}
	}
	return m
}()

// EquipmentByName maps a CS2 item name to its id; unknown names map to
// EquipmentUnknown.
func EquipmentByName(name string) Equipment { return equipmentByName[name] }

// String is the CS2 item name, or "" for an unknown id.
func (e Equipment) String() string {
	if int(e) < len(equipmentNames) {
		return equipmentNames[e]
	}
	return ""
}

// MarshalText writes the item name so the JSON frames read like the rest of
// the document.
func (e Equipment) MarshalText() ([]byte, error) { return []byte(e.String()), nil }

// UnmarshalText reads an item name; an unknown name reads as EquipmentUnknown.
func (e *Equipment) UnmarshalText(text []byte) error {
	*e = EquipmentByName(string(text))
	return nil
}

// InventoryItem is one carried item and how many of it (two flashbangs).
type InventoryItem struct {
	Item  Equipment `json:"item"`
	Count int       `json:"count"`
}

// Gear is what a player holds at a sample: the Skybox-style loadout. Decoded
// Gear values are shared between consecutive samples of a slot while nothing
// changes, so callers must treat them, and the Inventory slice, as read-only.
type Gear struct {
	ActiveWeapon Equipment       `json:"active_weapon"`
	Inventory    []InventoryItem `json:"inventory"`
	Armor        int             `json:"armor"`
	Helmet       bool            `json:"helmet"`
	Defuser      bool            `json:"defuser"`
	Money        int             `json:"money"`
	// FlashSeconds is the blinding time left, at 0.1 s resolution.
	FlashSeconds float64 `json:"flash_seconds"`
}
