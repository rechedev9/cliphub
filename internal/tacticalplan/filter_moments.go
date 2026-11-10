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

// AWPEstimated reports that the document predates the AWP flag, so an AWP
// filter reads it from AWP kills and hits instead.
func AWPEstimated(d Document) bool { return d.AnalysisVersion < AWPFlagVersion }

// ReadsAWP reports whether the filter asks about either side's AWP.
func (f Filter) ReadsAWP() bool { return f.AWP != "" || f.OpponentAWP != "" }

// presenceQuery asks whether Side had an AWP in Round; Estimate reads kills
// and hits, for analyses older than AWPFlagVersion.
type presenceQuery struct {
	Want     Presence
	Round    Round
	Side     Side
	Estimate bool
}

func (q presenceQuery) sideHadAWP(side Side) bool {
	r := q.Round
	sides := make(map[uint8]Side, len(r.Players))
	for _, pr := range r.Players {
		sides[pr.Slot] = pr.Side
		if !q.Estimate && pr.AWP && pr.Side == side {
			return true
		}
	}
	if !q.Estimate {
		return false
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

func matchPresence(q presenceQuery) bool {
	var had bool
	if q.Side == "" {
		had = q.sideHadAWP(SideCT) || q.sideHadAWP(SideT)
	} else {
		had = q.sideHadAWP(q.Side)
	}
	return had == (q.Want == PresenceWith)
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

// advantageQuery asks whether Round reached one of Want, counted from Side;
// without a side either one may be counted first.
type advantageQuery struct {
	Round Round
	Side  Side
	Want  []Advantage
}

func matchAdvantage(q advantageQuery) bool {
	for _, c := range aliveCounts(q.Round) {
		for _, a := range q.Want {
			switch q.Side {
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
