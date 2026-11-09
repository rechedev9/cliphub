package tactical

import "github.com/rechedev9/cliphub/internal/tacticalplan"

// The economy of a round: what each player took into it, read once the buy window closes.

func (s *scanner) sampleEconomyIfDue(tick int) {
	r := s.current
	if r == nil || r.economySampled || r.tickFreezeEnd == 0 || s.tickrateOrDefault() <= 0 {
		return
	}
	if float64(tick-r.tickFreezeEnd) < economyDelaySeconds*s.tickrateOrDefault() {
		return
	}
	s.sampleEconomy()
}

// economyDelaySeconds is how long after freeze-time end the equipment value is
// read. Players keep buying for a few seconds past the freeze, and by the end
// of the buy window they have already thrown utility and died, so both edges
// misreport what the team actually took into the round.
const economyDelaySeconds = 7

func (s *scanner) sampleEconomy() {
	r := s.current
	if r == nil || r.economySampled {
		return
	}
	r.economySampled = true
	r.economy.SampleTick = s.tick()
	for _, pl := range s.p.GameState().Participants().All() {
		slot, ok := s.slotFor(pl)
		if !ok {
			continue
		}
		side := sideOf(pl.Team)
		if side == "" {
			continue
		}
		if pl.IsAlive() {
			r.markPlayed(slot)
		} else if !r.played[slot] {
			// Never alive this round, like a coach: no buy and no line of the round.
			continue
		}
		value := pl.EquipmentValueCurrent()
		money := pl.Money()
		acc := r.playerStats(slot)
		acc.equipValue = value
		acc.money = money
		r.sides[slot] = side
		if side == tacticalplan.SideCT {
			r.economy.CTEquipValue += value
			r.economy.CTMoney += money
		} else {
			r.economy.TEquipValue += value
			r.economy.TMoney += money
		}
	}
}
