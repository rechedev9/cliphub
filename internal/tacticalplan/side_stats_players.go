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

// perfTotals adds up player rounds: one player's, or every player of a team.
type perfTotals struct {
	rounds, kills, deaths, damage, assists, kastRounds, headshots int
	utilRounds, utilDamage, flashAssists                          int
	// ratingRounds is the sum of each player's rating times its rounds, for the pooled rating.
	ratingRounds float64
}

func (t *perfTotals) add(o perfTotals) {
	t.rounds += o.rounds
	t.kills += o.kills
	t.deaths += o.deaths
	t.damage += o.damage
	t.assists += o.assists
	t.kastRounds += o.kastRounds
	t.headshots += o.headshots
	t.utilRounds += o.utilRounds
	t.utilDamage += o.utilDamage
	t.flashAssists += o.flashAssists
	t.ratingRounds += o.ratingRounds
}

func (t perfTotals) rating() float64 {
	return Rating(RatingLine{Rounds: t.rounds, Kills: t.kills, Deaths: t.deaths, Assists: t.assists, Damage: t.damage, KASTRounds: t.kastRounds})
}

func ptr(v float64) *float64 { return &v }

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// line reads the totals as a Player Performance line; the rating is the rounds-weighted mean of
// the players' ratings, so a team line sits among its players.
func (t perfTotals) line() PlayerLine {
	if t.rounds == 0 {
		return PlayerLine{}
	}
	rounds := float64(t.rounds)
	l := PlayerLine{
		Rating: ptr(round2(t.ratingRounds / rounds)),
		KPR:    ptr(round2(float64(t.kills) / rounds)),
		KD:     ptr(round2(float64(t.kills) / float64(max(t.deaths, 1)))),
		DPR:    ptr(round2(float64(t.deaths) / rounds)),
		ADR:    ptr(round1(float64(t.damage) / rounds)),
		KAST:   ptr(round1(100 * float64(t.kastRounds) / rounds)),
	}
	if t.utilRounds > 0 {
		l.UDR = ptr(round2(float64(t.utilDamage) / float64(t.utilRounds)))
		l.FAR = ptr(round2(float64(t.flashAssists) / float64(t.utilRounds)))
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

// flashAssistsBySlot credits each thrower of the side with the kills of an enemy a teammate made
// while that enemy was still blind from the thrower's flash; one assist per thrower and kill.
func flashAssistsBySlot(v roundView) map[uint8]int {
	type blind struct {
		thrower, victim uint8
		from, to        int
	}
	var blinds []blind
	assists := map[uint8]int{}
	for _, ev := range v.Events {
		if ev.ActorSlot == nil || v.sides[*ev.ActorSlot] != v.side {
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

// utilityDamageBySlot is the enemy damage of each side player's HE grenades and fires.
func utilityDamageBySlot(v roundView) map[uint8]int {
	damage := map[uint8]int{}
	for _, ev := range v.Events {
		if (ev.Kind == EventHE || ev.Kind == EventMolotov) && ev.Effect != nil && ev.ActorSlot != nil && v.sides[*ev.ActorSlot] == v.side {
			damage[*ev.ActorSlot] += ev.Effect.EnemyDamage
		}
	}
	return damage
}

// headshotsBySlot counts each player's headshot kills of an enemy.
func headshotsBySlot(v roundView) map[uint8]int {
	hs := map[uint8]int{}
	for _, ev := range v.Events {
		if ev.Kind != EventKill || !ev.Headshot || ev.ActorSlot == nil || ev.TargetSlot == nil {
			continue
		}
		if v.sides[*ev.ActorSlot] == v.side && v.sides[*ev.TargetSlot] != v.side {
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
		t.rounds++
		t.kills += pr.Kills
		t.deaths += pr.Deaths
		t.damage += pr.Damage
		t.assists += pr.Assists
		t.headshots += hs[pr.Slot]
		if pr.Kills > 0 || pr.Assists > 0 || pr.Survived || pr.Traded {
			t.kastRounds++
		}
		if v.utility {
			t.utilRounds++
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

// pooled adds every player of the run into one line.
func (run *teamRun) pooled() perfTotals {
	var all perfTotals
	for _, acc := range run.players {
		t := acc.totals
		t.ratingRounds = round2(t.rating()) * float64(t.rounds)
		all.add(t)
	}
	return all
}

func (run *teamRun) playerCards(opts SideStatsOptions) []SideStatsPlayer {
	list := run.sortedPlayers()
	out := make([]SideStatsPlayer, 0, len(list))
	for _, acc := range list {
		t := acc.totals
		l := t.line()
		p := SideStatsPlayer{
			SteamID64: acc.steamID, Name: acc.name, Role: opts.Roles[acc.steamID],
			Rounds: t.rounds, Kills: t.kills, Deaths: t.deaths, Damage: t.damage,
			Rating: round2(t.rating()), KD: *l.KD, ADR: *l.ADR, KPR: *l.KPR, DPR: *l.DPR, KAST: *l.KAST,
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
func groupPlayerLine(teams []*teamRun) PlayerLine {
	var lines []PlayerLine
	for _, run := range teams {
		if run.overall.rounds < GroupMinRounds {
			continue
		}
		l := run.pooled().line()
		if run.overall.utilRounds < GroupMinRounds {
			l.UDR, l.FAR = nil, nil
		}
		lines = append(lines, l)
	}
	var out PlayerLine
	for i, field := range out.fields() {
		sum, n := 0.0, 0
		for k := range lines {
			if v := *lines[k].fields()[i]; v != nil {
				sum += *v
				n++
			}
		}
		if n > 0 {
			*field = ptr(round2(sum / float64(n)))
		}
	}
	return out
}
