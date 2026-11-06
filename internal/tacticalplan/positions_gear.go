package tacticalplan

import (
	"encoding/binary"
	"fmt"
	"math"
)

// A version 2 sample record is the version 1 record followed by one byte of
// gear field bits and then only the fields those bits name, in bit order.
// A field is written when it differs from the slot's previous sample in the
// same round, so the first sample of a slot in each round carries all of them
// and a round decodes on its own.
const (
	gearPitch     = 1 << iota // uint8: (pitch + 90) * 254 / 180
	gearWeapon                // uint8: active Equipment id
	gearInventory             // uint8 n, then n bytes: id | (count-1) << 6
	gearArmor                 // uint8 armor, uint8 bits: 1 helmet, 2 defuser
	gearMoney                 // uint16 money
	gearFlash                 // uint8 blinding time left in tenths of a second

	gearAll = gearPitch | gearWeapon | gearInventory | gearArmor | gearMoney | gearFlash
)

// maxInventoryEntries bounds one inventory list; a real loadout holds about ten.
const maxInventoryEntries = 16

// wireGear is the quantised gear of one sample, compared field by field
// against the previous sample of the same slot.
type wireGear struct {
	pitch, weapon, armor, kit, flash uint8
	money                            uint16
	inventoryLen                     uint8
	inventory                        [maxInventoryEntries]byte
}

// gearEncoder remembers each slot's last written gear inside one round.
type gearEncoder struct {
	prev [maxSlots]wireGear
	seen uint16
}

func (e *gearEncoder) reset() { e.seen = 0 }

func (e *gearEncoder) append(buf []byte, s *Sample) []byte {
	cur := quantizeGear(s)
	bits := byte(gearAll)
	if e.seen&(1<<s.Slot) != 0 {
		bits = changedGear(&e.prev[s.Slot], &cur)
	}
	e.prev[s.Slot] = cur
	e.seen |= 1 << s.Slot

	buf = append(buf, bits)
	if bits&gearPitch != 0 {
		buf = append(buf, cur.pitch)
	}
	if bits&gearWeapon != 0 {
		buf = append(buf, cur.weapon)
	}
	if bits&gearInventory != 0 {
		buf = append(buf, cur.inventoryLen)
		buf = append(buf, cur.inventory[:cur.inventoryLen]...)
	}
	if bits&gearArmor != 0 {
		buf = append(buf, cur.armor, cur.kit)
	}
	if bits&gearMoney != 0 {
		buf = binary.LittleEndian.AppendUint16(buf, cur.money)
	}
	if bits&gearFlash != 0 {
		buf = append(buf, cur.flash)
	}
	return buf
}

func changedGear(prev, cur *wireGear) byte {
	var bits byte
	if prev.pitch != cur.pitch {
		bits |= gearPitch
	}
	if prev.weapon != cur.weapon {
		bits |= gearWeapon
	}
	if prev.inventoryLen != cur.inventoryLen || prev.inventory != cur.inventory {
		bits |= gearInventory
	}
	if prev.armor != cur.armor || prev.kit != cur.kit {
		bits |= gearArmor
	}
	if prev.money != cur.money {
		bits |= gearMoney
	}
	if prev.flash != cur.flash {
		bits |= gearFlash
	}
	return bits
}

func quantizeGear(s *Sample) wireGear {
	w := wireGear{pitch: encodePitch(s.Pitch)}
	g := s.Gear
	if g == nil {
		return w
	}
	w.weapon = byte(validEquipment(g.ActiveWeapon))
	w.armor = clampByte(g.Armor)
	if g.Helmet {
		w.kit |= 1
	}
	if g.Defuser {
		w.kit |= 2
	}
	w.money = uint16(min(max(g.Money, 0), math.MaxUint16)) // #nosec G115 -- clamped to the uint16 range.
	w.flash = clampByte(int(math.Round(g.FlashSeconds * 10)))

	// Inventory is written sorted by id with duplicates merged, so the same
	// loadout always encodes to the same bytes and compares equal.
	var counts [maxEquipmentID + 1]int
	for _, item := range g.Inventory {
		if id := validEquipment(item.Item); id != EquipmentUnknown && item.Count > 0 {
			counts[id] += item.Count
		}
	}
	for id, count := range counts {
		if count == 0 || int(w.inventoryLen) == maxInventoryEntries {
			continue
		}
		// #nosec G115 -- id is below 64 and the count is clamped to 1..4.
		w.inventory[w.inventoryLen] = byte(id) | byte(min(count, 4)-1)<<6
		w.inventoryLen++
	}
	return w
}

func validEquipment(e Equipment) Equipment {
	if int(e) >= len(equipmentNames) {
		return EquipmentUnknown
	}
	return e
}

// gearDecoder rebuilds each slot's gear from the changed fields. A Gear value
// is replaced, never mutated, so samples can share it while it is unchanged.
type gearDecoder struct {
	pitch [maxSlots]uint8
	gear  [maxSlots]*Gear
}

// read decodes one gear block at pos into s and returns the next position.
func (d *gearDecoder) read(data []byte, pos int, s *Sample) (int, error) {
	need := func(n int) error {
		if pos+n > len(data) {
			return fmt.Errorf("decode frames: truncated gear at byte %d", pos)
		}
		return nil
	}
	if err := need(1); err != nil {
		return 0, err
	}
	bits := data[pos]
	pos++
	if bits&gearPitch != 0 {
		if err := need(1); err != nil {
			return 0, err
		}
		d.pitch[s.Slot] = data[pos]
		pos++
	}
	s.Pitch = decodePitch(d.pitch[s.Slot])

	prev := d.gear[s.Slot]
	if prev == nil {
		prev = &Gear{}
	}
	if bits&^gearPitch == 0 {
		d.gear[s.Slot] = prev
		s.Gear = prev
		return pos, nil
	}
	next := *prev
	if bits&gearWeapon != 0 {
		if err := need(1); err != nil {
			return 0, err
		}
		next.ActiveWeapon = Equipment(data[pos])
		pos++
	}
	if bits&gearInventory != 0 {
		if err := need(1); err != nil {
			return 0, err
		}
		n := int(data[pos])
		pos++
		if err := need(n); err != nil {
			return 0, err
		}
		next.Inventory = make([]InventoryItem, n)
		for i, b := range data[pos : pos+n] {
			next.Inventory[i] = InventoryItem{Item: Equipment(b & maxEquipmentID), Count: int(b>>6) + 1}
		}
		pos += n
	}
	if bits&gearArmor != 0 {
		if err := need(2); err != nil {
			return 0, err
		}
		next.Armor = int(data[pos])
		next.Helmet = data[pos+1]&1 != 0
		next.Defuser = data[pos+1]&2 != 0
		pos += 2
	}
	if bits&gearMoney != 0 {
		if err := need(2); err != nil {
			return 0, err
		}
		next.Money = int(binary.LittleEndian.Uint16(data[pos:]))
		pos += 2
	}
	if bits&gearFlash != 0 {
		if err := need(1); err != nil {
			return 0, err
		}
		next.FlashSeconds = float64(data[pos]) / 10
		pos++
	}
	d.gear[s.Slot] = &next
	s.Gear = &next
	return pos, nil
}

// encodePitch maps CS2's pitch (positive looks down, 270..360 meaning -90..0
// in some demos) onto one byte, about 0.7 degrees per step with level exact.
func encodePitch(deg float64) uint8 {
	if math.IsNaN(deg) || math.IsInf(deg, 0) {
		deg = 0
	}
	if deg > 180 {
		deg -= 360
	}
	deg = min(max(deg, -90), 90)
	return uint8(math.Round((deg + 90) * 254 / 180))
}

func decodePitch(v uint8) float64 { return float64(v)*180/254 - 90 }

func clampByte(v int) uint8 {
	return uint8(min(max(v, 0), math.MaxUint8)) // #nosec G115 -- clamped to the byte range.
}
