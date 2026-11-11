package tacticalplan

import (
	"math"
	"sort"
)

// SideStatsPlayer is one player's card and Player Performance row.
type SideStatsPlayer struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	// MapRoles is the player's role per side on the map, set when the scope reads one map.
	MapRoles *PlayerMapRoles `json:"map_roles,omitempty"`
	Rounds   int             `json:"rounds"`
	Kills    int             `json:"kills"`
	Deaths   int             `json:"deaths"`
	Damage   int             `json:"damage"`
	Rating   float64         `json:"rating"`
	KD       float64         `json:"kd"`
	ADR      float64         `json:"adr"`
	KPR      float64         `json:"kpr"`
	DPR      float64         `json:"dpr"`
	// KAST and HS are percents; UDR and FAR are per round over the rounds scanned with utility effects.
	KAST float64  `json:"kast"`
	UDR  *float64 `json:"udr"`
	FAR  *float64 `json:"far"`
	HS   *float64 `json:"hs"`
}

// PlayerLine is the Player Performance footer: the team's pooled line, or the group's mean of
// those lines. A nil figure has no data.
type PlayerLine struct {
	Rating *float64 `json:"rating"`
	KPR    *float64 `json:"kpr"`
	KD     *float64 `json:"kd"`
	DPR    *float64 `json:"dpr"`
	ADR    *float64 `json:"adr"`
	KAST   *float64 `json:"kast"`
	UDR    *float64 `json:"udr"`
	FAR    *float64 `json:"far"`
	HS     *float64 `json:"hs"`
}

func (l *PlayerLine) fields() []**float64 {
	return []**float64{&l.Rating, &l.KPR, &l.KD, &l.DPR, &l.ADR, &l.KAST, &l.UDR, &l.FAR, &l.HS}
}

// perfTotals adds up player rounds: one player's, or every player of a team. box holds damage,
// assists, KAST rounds and utility rounds as the baseline counts them.
type perfTotals struct {
	box                                                        BoxScore
	rounds, kills, deaths, headshots, utilDamage, flashAssists int
}

func (t *perfTotals) add(o perfTotals) {
	t.box.add(o.box)
	t.rounds += o.rounds
	t.kills += o.kills
	t.deaths += o.deaths
	t.headshots += o.headshots
	t.utilDamage += o.utilDamage
	t.flashAssists += o.flashAssists
}

func (t perfTotals) rating() float64 {
	return Rating(RatingLine{Rounds: t.rounds, Kills: t.kills, Deaths: t.deaths, Assists: t.box.Assists, Damage: t.box.Damage, KASTRounds: t.box.KASTRounds})
}

func ptr(v float64) *float64 { return &v }

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// line reads the totals as a Player Performance line.
func (t perfTotals) line() PlayerLine {
	if t.rounds == 0 {
		return PlayerLine{}
	}
	rounds := float64(t.rounds)
	l := PlayerLine{
		Rating: ptr(round2(t.rating())),
		KPR:    ptr(round2(float64(t.kills) / rounds)),
		KD:     ptr(round2(float64(t.kills) / float64(max(t.deaths, 1)))),
		DPR:    ptr(round2(float64(t.deaths) / rounds)),
		ADR:    ptr(round1(float64(t.box.Damage) / rounds)),
		KAST:   ptr(round1(100 * float64(t.box.KASTRounds) / rounds)),
	}
	if u := float64(t.box.UtilityRounds); u > 0 {
		l.UDR = ptr(round2(float64(t.utilDamage) / u))
		l.FAR = ptr(round2(float64(t.flashAssists) / u))
	}
	if t.kills > 0 {
		l.HS = ptr(round1(100 * float64(t.headshots) / float64(t.kills)))
	}
	return l
}

type playerAccum struct {
	steamID, name string
	totals        perfTotals
	order         int
}

// flashAssistsBySlot gives a side thrower one assist per kill a teammate makes on an enemy inside the
// window its flash blinded that enemy (detonation tick plus blind seconds); an estimate, not CS2's own flag.
func flashAssistsBySlot(v roundView) map[uint8]int {
	type blind struct {
		thrower, victim uint8
		from, to        int
	}
	var blinds []blind
	assists := map[uint8]int{}
	for _, ev := range v.Events {
		if !v.ours(ev.ActorSlot) {
			continue
		}
		actor := *ev.ActorSlot
		switch ev.Kind {
		case EventFlash:
			if ev.Effect == nil {
				continue
			}
			for _, b := range ev.Effect.Blinded {
				if b.Enemy {
					blinds = append(blinds, blind{thrower: actor, victim: b.Slot, from: ev.Tick, to: ev.Tick + int(b.Seconds*v.tickrate)})
				}
			}
		case EventKill:
			if ev.TargetSlot == nil {
				continue
			}
			credited := map[uint8]bool{}
			for _, b := range blinds {
				if b.victim == *ev.TargetSlot && b.thrower != actor && !credited[b.thrower] && ev.Tick >= b.from && ev.Tick <= b.to {
					credited[b.thrower] = true
					assists[b.thrower]++
				}
			}
		}
	}
	return assists
}

// ours says whether the slot plays on the team's side this round.
func (v roundView) ours(slot *uint8) bool { return slot != nil && v.sides[*slot] == v.side }

// utilityDamageBySlot is the enemy damage of each side player's grenades; only HE and fire carry it.
func utilityDamageBySlot(v roundView) map[uint8]int {
	damage := map[uint8]int{}
	for _, ev := range v.Events {
		if ev.Effect != nil && v.ours(ev.ActorSlot) {
			damage[*ev.ActorSlot] += ev.Effect.EnemyDamage
		}
	}
	return damage
}

// headshotsBySlot counts each player's headshot kills of an enemy.
func headshotsBySlot(v roundView) map[uint8]int {
	hs := map[uint8]int{}
	for _, ev := range v.Events {
		if ev.Kind == EventKill && ev.Headshot && ev.TargetSlot != nil && v.ours(ev.ActorSlot) && !v.ours(ev.TargetSlot) {
			hs[*ev.ActorSlot]++
		}
	}
	return hs
}

func sumSlots(bySlot map[uint8]int) int {
	total := 0
	for _, n := range bySlot {
		total += n
	}
	return total
}

func (run *teamRun) addPlayers(v roundView, names map[uint8]Player) {
	hs := headshotsBySlot(v)
	for _, pr := range v.Players {
		p, ok := names[pr.Slot]
		if pr.Side != v.side || !ok || p.SteamID64 == "" {
			continue
		}
		acc := run.players[p.SteamID64]
		if acc == nil {
			acc = &playerAccum{steamID: p.SteamID64, order: len(run.players)}
			run.players[p.SteamID64] = acc
		}
		acc.name = p.Name
		t := &acc.totals
		addBoxRound(&t.box, pr, v.utility)
		t.rounds++
		t.kills += pr.Kills
		t.deaths += pr.Deaths
		t.headshots += hs[pr.Slot]
		if v.utility {
			t.utilDamage += v.utilDamage[pr.Slot]
			t.flashAssists += v.flashAssists[pr.Slot]
		}
	}
}

// sortedPlayers lists the run's players by rounds played, then by first appearance.
func (run *teamRun) sortedPlayers() []*playerAccum {
	list := make([]*playerAccum, 0, len(run.players))
	for _, p := range run.players {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].totals.rounds != list[j].totals.rounds {
			return list[i].totals.rounds > list[j].totals.rounds
		}
		return list[i].order < list[j].order
	})
	return list
}

// teamLine pools every player of the run; its rating is the rounds-weighted mean of the players'
// ratings, so the team's line sits among its players.
func (run *teamRun) teamLine() PlayerLine {
	var all perfTotals
	weighted := 0.0
	for _, acc := range run.players {
		all.add(acc.totals)
		weighted += round2(acc.totals.rating()) * float64(acc.totals.rounds)
	}
	l := all.line()
	if all.rounds > 0 {
		l.Rating = ptr(round2(weighted / float64(all.rounds)))
	}
	return l
}

func (run *teamRun) playerCards(opts SideStatsOptions) []SideStatsPlayer {
	list := run.sortedPlayers()
	out := make([]SideStatsPlayer, 0, len(list))
	for _, acc := range list {
		t := acc.totals
		l := t.line()
		p := SideStatsPlayer{
			SteamID64: acc.steamID, Name: acc.name, Role: opts.Roles[acc.steamID],
			Rounds: t.rounds, Kills: t.kills, Deaths: t.deaths, Damage: t.box.Damage,
			Rating: *l.Rating, KD: *l.KD, ADR: *l.ADR, KPR: *l.KPR, DPR: *l.DPR, KAST: *l.KAST,
			UDR: l.UDR, FAR: l.FAR, HS: l.HS,
		}
		if r, ok := opts.MapRoles[acc.steamID]; ok {
			p.MapRoles = &r
		}
		out = append(out, p)
	}
	return out
}

// groupPlayerLine is the plain mean of the group teams' pooled lines, each figure counting a team
// with at least GroupMinRounds rounds behind it (rounds with utility effects for UDR and FAR).
func groupPlayerLine(teams map[string]*teamRun) PlayerLine {
	var out PlayerLine
	fields := out.fields()
	sums, counts := make([]float64, len(fields)), make([]int, len(fields))
	for _, run := range teams {
		if run.overall.rounds < GroupMinRounds {
			continue
		}
		l := run.teamLine()
		if run.overall.utilRounds < GroupMinRounds {
			l.UDR, l.FAR = nil, nil
		}
		for i, v := range l.fields() {
			if *v != nil {
				sums[i] += **v
				counts[i]++
			}
		}
	}
	for i, field := range fields {
		if counts[i] > 0 {
			*field = ptr(round2(sums[i] / float64(counts[i])))
		}
	}
	return out
}
