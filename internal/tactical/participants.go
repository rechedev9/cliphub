package tactical

import (
	"slices"

	"github.com/rechedev9/demo-parser-rsn/pkg/demoinfocs/common"
)

// demoinfocs keeps a reconnected player's stale controller in Participants().All(), and can list
// one *Player under two user IDs, so a slot can come twice. Each read keeps one player per slot.

type slotPlayer struct {
	slot   uint8
	player *common.Player
}

type slotPicker struct {
	// lastEntity is the EntityID picked for each slot last time; 0 when none was.
	lastEntity [16]int
	warned     bool
}

// playersBySlot answers one player per slot, in slot order.
func (s *scanner) playersBySlot(tick int, all []*common.Player) []slotPlayer {
	var picked [16]*common.Player
	for _, pl := range all {
		slot, ok := s.slotFor(pl)
		if !ok {
			continue
		}
		prev := picked[slot]
		if prev == nil {
			picked[slot] = pl
			continue
		}
		if !s.picker.warned {
			s.picker.warned = true
			s.warn("player %q is listed twice at tick %d; one entry per player was kept", pl.Name, tick)
		}
		next, kept := s.candidateRank(slot, pl), s.candidateRank(slot, prev)
		if slices.Compare(next[:], kept[:]) > 0 {
			picked[slot] = pl
		}
	}
	players := make([]slotPlayer, 0, len(all))
	for slot := uint8(0); slot < 16; slot++ {
		pl := picked[slot]
		if pl == nil {
			continue
		}
		s.picker.lastEntity[slot] = pl.EntityID
		players = append(players, slotPlayer{slot: slot, player: pl})
	}
	return players
}

// candidateRank orders two entries of one slot: the larger rank is the live player.
// A T or CT team comes first so an entry the side filter drops never beats one it keeps.
// UserID breaks the last tie so the pick does not follow the map order of All().
func (s *scanner) candidateRank(slot uint8, p *common.Player) [8]int {
	pos := p.Position()
	return [8]int{
		rankBit(sideOf(p.Team) != ""),
		rankBit(p.IsConnected && p.Entity != nil),
		rankBit(p.PlayerPawnEntity() != nil),
		rankBit(p.IsAlive()),
		rankBit(pos.X != 0 || pos.Y != 0 || pos.Z != 0),
		rankBit(p.EntityID != 0 && p.EntityID == s.picker.lastEntity[slot]),
		p.EntityID,
		p.UserID,
	}
}

func rankBit(b bool) int {
	if b {
		return 1
	}
	return 0
}
