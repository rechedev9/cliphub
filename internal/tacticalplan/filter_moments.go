package tacticalplan

import (
	"fmt"
	"strconv"
	"strings"
)

// Presence asks whether a side had something in a round: an AWP, for now.
type Presence string

// Presence values; the empty one does not filter.
const (
	PresenceWith    Presence = "with"
	PresenceWithout Presence = "without"
)

// MaxAdvantagePlayers is the largest side count an advantage names.
const MaxAdvantagePlayers = 5

// Advantage is a player count at one moment of a round: Own alive against
// Rival alive, read from the filter's perspective ("4v5").
type Advantage struct {
	Own   int `json:"own"`
	Rival int `json:"rival"`
}

// String writes the advantage the way the query names it.
func (a Advantage) String() string { return fmt.Sprintf("%dv%d", a.Own, a.Rival) }

// sideHadAWP reports whether a player of the side held an AWP in the round.
// Analyses before the AWP flag existed fall back to an AWP kill or hit.
func sideHadAWP(r Round, side Side) bool {
	sides := make(map[uint8]Side, len(r.Players))
	for _, pr := range r.Players {
		sides[pr.Slot] = pr.Side
		if pr.AWP && pr.Side == side {
			return true
		}
	}
	for _, e := range r.Events {
		if e.Kind == EventKill && e.Weapon == awpWeapon && e.ActorSlot != nil && sides[*e.ActorSlot] == side {
			return true
		}
	}
	for _, h := range r.Hits {
		if h.Weapon == awpWeapon && sides[h.ActorSlot] == side {
			return true
		}
	}
	return false
}

func matchPresence(want Presence, r Round, side Side) bool {
	var had bool
	if side == "" {
		had = sideHadAWP(r, SideCT) || sideHadAWP(r, SideT)
	} else {
		had = sideHadAWP(r, side)
	}
	return had == (want == PresenceWith)
}

// aliveCounts lists every CT and T count the round went through, from the
// start of play to its end, one entry per kill.
func aliveCounts(r Round) [][2]int {
	sides := make(map[uint8]Side, len(r.Players))
	var ct, t int
	for _, pr := range r.Players {
		sides[pr.Slot] = pr.Side
		switch pr.Side {
		case SideCT:
			ct++
		case SideT:
			t++
		}
	}
	out := [][2]int{{ct, t}}
	for _, e := range r.Events {
		if e.Kind != EventKill || e.TargetSlot == nil || (r.TickEnd > 0 && e.Tick > r.TickEnd) {
			continue
		}
		switch sides[*e.TargetSlot] {
		case SideCT:
			ct--
		case SideT:
			t--
		default:
			continue
		}
		out = append(out, [2]int{ct, t})
	}
	return out
}

// matchAdvantage reports whether the round reached one of the counts. Without
// a perspective either side may be the one counted first.
func matchAdvantage(r Round, side Side, want []Advantage) bool {
	for _, c := range aliveCounts(r) {
		for _, a := range want {
			switch side {
			case SideCT:
				if c[0] == a.Own && c[1] == a.Rival {
					return true
				}
			case SideT:
				if c[1] == a.Own && c[0] == a.Rival {
					return true
				}
			default:
				if (c[0] == a.Own && c[1] == a.Rival) || (c[1] == a.Own && c[0] == a.Rival) {
					return true
				}
			}
		}
	}
	return false
}

func parsePresence(raw, name string) (Presence, error) {
	switch v := Presence(strings.ToLower(strings.TrimSpace(raw))); v {
	case "", PresenceWith, PresenceWithout:
		return v, nil
	default:
		return "", fmt.Errorf("filter: %s must be with or without, not %q", name, raw)
	}
}

// ParseAdvantage reads "4v5": the perspective's players alive against the rival's.
func ParseAdvantage(raw string) (Advantage, error) {
	own, rival, ok := strings.Cut(strings.ToLower(strings.TrimSpace(raw)), "v")
	a := Advantage{}
	var err error
	if ok {
		if a.Own, err = strconv.Atoi(own); err == nil {
			a.Rival, err = strconv.Atoi(rival)
		}
	}
	if !ok || err != nil || a.Own < 0 || a.Rival < 0 || a.Own > MaxAdvantagePlayers || a.Rival > MaxAdvantagePlayers || a.Own+a.Rival == 0 {
		return Advantage{}, fmt.Errorf("filter: advantage %q must be like 4v5, from 0 to %d a side", raw, MaxAdvantagePlayers)
	}
	return a, nil
}

func parseAdvantages(raw []string) ([]Advantage, error) {
	var out []Advantage
	for _, item := range raw {
		a, err := ParseAdvantage(item)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}
