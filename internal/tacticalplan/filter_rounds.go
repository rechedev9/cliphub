package tacticalplan

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// EndReason groups the scan's end_reason slugs into the ways a round is won.
type EndReason string

// Round end reasons a filter can ask for.
const (
	EndElimination EndReason = "elimination"
	EndExploded    EndReason = "bomb_exploded"
	EndDefused     EndReason = "bomb_defused"
	EndTime        EndReason = "time_expired"
)

// EndReasons lists the end reasons in the order the filter offers them.
var EndReasons = []EndReason{EndElimination, EndExploded, EndDefused, EndTime}

// WeaponRule keeps the rounds in which one side used or bought a weapon a certain way.
type WeaponRule string

// Weapon rules, each read from the round's stored kills, hits, AWP flags and equipment values.
const (
	WeaponScout   WeaponRule = "scout"
	WeaponNoScout WeaponRule = "no_scout"
	WeaponHeroAK  WeaponRule = "hero_ak"
	WeaponHeroAWP WeaponRule = "hero_awp"
)

// WeaponRules lists the weapon rules in the order the filter offers them.
var WeaponRules = []WeaponRule{WeaponScout, WeaponNoScout, WeaponHeroAK, WeaponHeroAWP}

const (
	scoutWeapon = "ssg08"
	akWeapon    = "ak47"
	// HeroTeammateMaxEquip is the equipment each teammate of a hero stays under, the eco line per player.
	HeroTeammateMaxEquip = 1000
	// MaxEquipPerPlayer bounds an equipment range; no loadout reaches it.
	MaxEquipPerPlayer = 20000
)

// EquipRange keeps the rounds whose side spent From to To dollars of equipment
// per player (the side's value at the economy sample over its players). Zero ends are open.
type EquipRange struct {
	From int `json:"from,omitempty"`
	To   int `json:"to,omitempty"`
}

// Empty reports whether the range keeps every round.
func (e EquipRange) Empty() bool { return e.From == 0 && e.To == 0 }

func (e EquipRange) keeps(value int) bool {
	return value >= e.From && (e.To == 0 || value <= e.To)
}

// roundSide is one side of one round.
type roundSide struct {
	Round Round
	Side  Side
}

// perPlayerEquip is the side's sampled equipment value over the players it fielded.
func (s roundSide) perPlayerEquip() int {
	n := 0
	for _, pr := range s.Round.Players {
		if pr.Side == s.Side {
			n++
		}
	}
	if n == 0 {
		n = MaxAdvantagePlayers
	}
	return s.Round.Economy.EquipValue(s.Side) / n
}

// equipQuery asks whether Side's equipment per player fell in Want; without a side either one may.
type equipQuery struct {
	Round Round
	Side  Side
	Want  EquipRange
}

func matchEquip(q equipQuery) bool {
	if q.Side == "" {
		return q.Want.keeps(roundSide{q.Round, SideCT}.perPlayerEquip()) || q.Want.keeps(roundSide{q.Round, SideT}.perPlayerEquip())
	}
	return q.Want.keeps(roundSide{q.Round, q.Side}.perPlayerEquip())
}

// IsSecondRound reports that r is the round after a regulation half's pistol round.
func IsSecondRound(d Document, r Round) bool {
	if r.Overtime > 0 {
		return false
	}
	earlier := 0
	for _, o := range d.Rounds {
		if o.Half == r.Half && o.Overtime == 0 && o.Number < r.Number {
			earlier++
		}
	}
	return earlier == 1
}

func endReasonOf(slug string) EndReason {
	switch slug {
	case "ct_eliminated_t", "t_eliminated_ct":
		return EndElimination
	case string(EndExploded), string(EndDefused), string(EndTime):
		return EndReason(slug)
	default:
		return ""
	}
}

func matchEndReason(r Round, want []EndReason) bool {
	got := endReasonOf(r.EndReason)
	return got != "" && slices.Contains(want, got)
}

// weaponQuery asks whether Side used Rule in Round; Estimate reads the AWP from kills and hits.
type weaponQuery struct {
	Rule     WeaponRule
	Round    Round
	Side     Side
	Estimate bool
}

// usedBy lists the slots of the side that killed or damaged an enemy with weapon.
func (s roundSide) usedBy(weapon string) map[uint8]bool {
	r, side := s.Round, s.Side
	sides := make(map[uint8]Side, len(r.Players))
	for _, pr := range r.Players {
		sides[pr.Slot] = pr.Side
	}
	out := map[uint8]bool{}
	for _, e := range r.Events {
		if e.Kind == EventKill && e.Weapon == weapon && e.ActorSlot != nil && sides[*e.ActorSlot] == side {
			out[*e.ActorSlot] = true
		}
	}
	for _, h := range r.Hits {
		if h.Weapon == weapon && sides[h.ActorSlot] == side {
			out[h.ActorSlot] = true
		}
	}
	return out
}

// hero reports that one of carriers is the side's only player with
// equipment at or over HeroTeammateMaxEquip.
func (s roundSide) hero(carriers map[uint8]bool) bool {
	heroes := 0
	for _, pr := range s.Round.Players {
		if pr.Side != s.Side {
			continue
		}
		if carriers[pr.Slot] {
			heroes++
			continue
		}
		if pr.EquipValue >= HeroTeammateMaxEquip {
			return false
		}
	}
	return heroes == 1
}

func (q weaponQuery) sideMatches(side Side) bool {
	s := roundSide{q.Round, side}
	switch q.Rule {
	case WeaponScout:
		return len(s.usedBy(scoutWeapon)) > 0
	case WeaponNoScout:
		return len(s.usedBy(scoutWeapon)) == 0
	case WeaponHeroAK:
		return s.hero(s.usedBy(akWeapon))
	case WeaponHeroAWP:
		carriers := s.usedBy(awpWeapon)
		if !q.Estimate {
			carriers = map[uint8]bool{}
			for _, pr := range q.Round.Players {
				if pr.Side == side && pr.AWP {
					carriers[pr.Slot] = true
				}
			}
		}
		return s.hero(carriers)
	}
	return false
}

func matchWeapon(q weaponQuery) bool {
	if q.Side == "" {
		return q.sideMatches(SideCT) || q.sideMatches(SideT)
	}
	return q.sideMatches(q.Side)
}

func parseEndReasons(raw []string) ([]EndReason, error) {
	var out []EndReason
	for _, item := range raw {
		v := EndReason(strings.ToLower(item))
		if !slices.Contains(EndReasons, v) {
			return nil, fmt.Errorf("filter: unknown end_reason %q", item)
		}
		out = append(out, v)
	}
	return out, nil
}

func parseWeaponRule(raw, name string) (WeaponRule, error) {
	v := WeaponRule(strings.ToLower(strings.TrimSpace(raw)))
	if v == "" || slices.Contains(WeaponRules, v) {
		return v, nil
	}
	return "", fmt.Errorf("filter: %s must be scout, no_scout, hero_ak or hero_awp, not %q", name, raw)
}

func parseSecondRound(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "false":
		return false, nil
	case "true", "1":
		return true, nil
	default:
		return false, fmt.Errorf("filter: second_round must be true or false, not %q", raw)
	}
}

// equipEnd is one end of an equipment range as the query names it.
type equipEnd struct {
	raw, name string
	least     int
}

func (e equipEnd) parse() (int, error) {
	raw := strings.TrimSpace(e.raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < e.least || n > MaxEquipPerPlayer {
		return 0, fmt.Errorf("filter: %s must be an integer from %d to %d", e.name, e.least, MaxEquipPerPlayer)
	}
	return n, nil
}

// parseEquipRange reads prefix_from and prefix_to; to is at least 1, so 0 stays the open end.
func parseEquipRange(values map[string][]string, prefix string) (EquipRange, error) {
	get := func(key string) string {
		if list := values[key]; len(list) > 0 {
			return list[0]
		}
		return ""
	}
	var out EquipRange
	var err error
	if out.From, err = (equipEnd{raw: get(prefix + "_from"), name: prefix + "_from"}).parse(); err != nil {
		return EquipRange{}, err
	}
	if out.To, err = (equipEnd{raw: get(prefix + "_to"), name: prefix + "_to", least: 1}).parse(); err != nil {
		return EquipRange{}, err
	}
	if out.To > 0 && out.From > out.To {
		return EquipRange{}, fmt.Errorf("filter: %s_from %d is over %s_to %d", prefix, out.From, prefix, out.To)
	}
	return out, nil
}
