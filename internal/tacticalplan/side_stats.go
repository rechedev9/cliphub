package tacticalplan

import (
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

// SideStatsMatch is one match of a team that the side stats read.
type SideStatsMatch struct {
	MatchID  uuid.UUID
	PlayedAt time.Time
	Document Document
	TeamKey  string
	Opponent string
}

// SideStatsOptions is the round scope: the side, both buys and the lineup.
// Roles names each SteamID's inferred role for the player cards.
type SideStatsOptions struct {
	Side         Side
	Buys         []BuyType
	OpponentBuys []BuyType
	Lineup       Lineup
	Roles        map[string]string
}

// SideStat is one team figure. Value is nil when the scope has no rounds for
// it; rates are percents, the per round figures are averages. GroupAvg is the
// mean of the same figure over the comparison teams that have it.
type SideStat struct {
	Value      *float64 `json:"value"`
	Count      float64  `json:"count"`
	Total      int      `json:"total"`
	GroupAvg   *float64 `json:"group_avg"`
	GroupTeams int      `json:"group_teams"`
}

// SideStats is a team's figures over one side, or both, in the scope.
type SideStats struct {
	MapsWon       SideStat `json:"maps_won"`
	RoundsWon     SideStat `json:"rounds_won"`
	PistolsWon    SideStat `json:"pistols_won"`
	OpeningKills  SideStat `json:"opening_kills"`
	Won5v4        SideStat `json:"won_5v4"`
	Won4v5        SideStat `json:"won_4v5"`
	Trades        SideStat `json:"trades"`
	UtilityDamage SideStat `json:"utility_damage_per_round"`
	FlashAssists  SideStat `json:"flash_assists_per_round"`
	RoundsPlayed  SideStat `json:"rounds_played"`
}

func (s *SideStats) fields() []*SideStat {
	return []*SideStat{
		&s.MapsWon, &s.RoundsWon, &s.PistolsWon, &s.OpeningKills, &s.Won5v4, &s.Won4v5,
		&s.Trades, &s.UtilityDamage, &s.FlashAssists, &s.RoundsPlayed,
	}
}

// SideStatsPlayer is one player's card: rating, K/D, ADR, KPR and DPR.
type SideStatsPlayer struct {
	SteamID64 string  `json:"steamid64"`
	Name      string  `json:"name"`
	Role      string  `json:"role"`
	Rounds    int     `json:"rounds"`
	Kills     int     `json:"kills"`
	Deaths    int     `json:"deaths"`
	Damage    int     `json:"damage"`
	Rating    float64 `json:"rating"`
	KD        float64 `json:"kd"`
	ADR       float64 `json:"adr"`
	KPR       float64 `json:"kpr"`
	DPR       float64 `json:"dpr"`
}

// SideStatsMatchRef is a match the figures stand on, for "Ver partidas".
type SideStatsMatchRef struct {
	MatchID  uuid.UUID `json:"match_id"`
	Map      string    `json:"map"`
	PlayedAt time.Time `json:"played_at"`
	Opponent string    `json:"opponent"`
	Rounds   int       `json:"rounds"`
	Won      bool      `json:"won"`
}

// SideStatsReport is a team's figures in the scope against a comparison group.
type SideStatsReport struct {
	Matches       int                 `json:"matches"`
	Rounds        int                 `json:"rounds"`
	Overall       SideStats           `json:"overall"`
	CT            SideStats           `json:"ct"`
	T             SideStats           `json:"t"`
	Players       []SideStatsPlayer   `json:"players"`
	MatchList     []SideStatsMatchRef `json:"match_list"`
	GroupTeams    int                 `json:"group_teams"`
	RatingFormula string              `json:"rating_formula"`
}

// RatingFormula states how SideStatsPlayer.Rating is computed.
const RatingFormula = "ClipHub rating: the mean of KPR / 0.68, (1 - DPR) / 0.32 and ADR / 75; 1.00 is an average player"

// playerRating is ClipHub's own rating. It is the one place to point at the
// ranking's rating once that lands.
func playerRating(kpr, dpr, adr float64) float64 {
	return round2((kpr/0.68 + (1-dpr)/0.32 + adr/75) / 3)
}

// sideAcc adds up one side, or both, of a team's kept rounds.
type sideAcc struct {
	matches, mapsWon                              int
	rounds, won, pistols, pistolsWon              int
	openings, openingKills, ahead, aheadWon       int
	behind, behindWon, deaths, traded, utilRounds int
	utilDamage, flashAssists                      int
	seenMatch                                     bool
}

func (a *sideAcc) stats() SideStats {
	return SideStats{
		MapsWon:       pctStat(a.mapsWon, a.matches),
		RoundsWon:     pctStat(a.won, a.rounds),
		PistolsWon:    pctStat(a.pistolsWon, a.pistols),
		OpeningKills:  pctStat(a.openingKills, a.openings),
		Won5v4:        pctStat(a.aheadWon, a.ahead),
		Won4v5:        pctStat(a.behindWon, a.behind),
		Trades:        pctStat(a.traded, a.deaths),
		UtilityDamage: perRoundStat(a.utilDamage, a.utilRounds),
		FlashAssists:  perRoundStat(a.flashAssists, a.utilRounds),
		RoundsPlayed:  countStat(a.rounds),
	}
}

func pctStat(count, total int) SideStat {
	s := SideStat{Count: float64(count), Total: total}
	if total > 0 {
		v := math.Round(float64(count)/float64(total)*1000) / 10
		s.Value = &v
	}
	return s
}

func perRoundStat(sum, rounds int) SideStat {
	s := SideStat{Count: float64(sum), Total: rounds}
	if rounds > 0 {
		v := round2(float64(sum) / float64(rounds))
		s.Value = &v
	}
	return s
}

func countStat(rounds int) SideStat {
	s := SideStat{Count: float64(rounds), Total: rounds}
	if rounds > 0 {
		v := float64(rounds)
		s.Value = &v
	}
	return s
}

// sideOnly drops the map result from one side's figures: a map is won by both halves.
func sideOnly(s SideStats) SideStats {
	s.MapsWon = SideStat{}
	return s
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// mapWon reads the map result from every round of the document, whatever the scope keeps.
func mapWon(doc Document, teamKey string) bool {
	own, rival := 0, 0
	for _, r := range doc.Rounds {
		side := doc.TeamSide(teamKey, r)
		switch {
		case side == "" || r.Winner == "":
		case r.Winner == side:
			own++
		default:
			rival++
		}
	}
	return own > rival
}

// countFlashAssists counts the side's kills of an enemy a teammate of the
// killer had blinded, while still blind; one assist per thrower and kill.
func countFlashAssists(r Round, side Side, sides map[uint8]Side, tickrate float64) int {
	type blind struct {
		thrower, victim uint8
		from, to        int
	}
	var blinds []blind
	assists := 0
	for _, ev := range r.Events {
		if ev.ActorSlot == nil || sides[*ev.ActorSlot] != side {
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
					blinds = append(blinds, blind{thrower: actor, victim: b.Slot, from: ev.Tick, to: ev.Tick + int(b.Seconds*tickrate)})
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
					assists++
				}
			}
		}
	}
	return assists
}

func utilityDamage(r Round, side Side, sides map[uint8]Side) int {
	total := 0
	for _, ev := range r.Events {
		if (ev.Kind == EventHE || ev.Kind == EventMolotov) && ev.Effect != nil && ev.ActorSlot != nil && sides[*ev.ActorSlot] == side {
			total += ev.Effect.EnemyDamage
		}
	}
	return total
}

func (a *sideAcc) addRound(doc Document, r Round, side Side) {
	if !a.seenMatch {
		a.seenMatch = true
		a.matches++
	}
	a.rounds++
	won := r.Winner == side
	if won {
		a.won++
	}
	if r.Class.HasTag(TagPistol) {
		a.pistols++
		if won {
			a.pistolsWon++
		}
	}
	if r.Class.OpeningSide != "" {
		a.openings++
		if r.Class.OpeningSide == side {
			a.openingKills++
			a.ahead++
			if won {
				a.aheadWon++
			}
		} else {
			a.behind++
			if won {
				a.behindWon++
			}
		}
	}
	sides := map[uint8]Side{}
	for _, pr := range r.Players {
		sides[pr.Slot] = pr.Side
		if pr.Side == side && pr.Deaths > 0 {
			a.deaths++
			if pr.Traded {
				a.traded++
			}
		}
	}
	if doc.UtilityEffects {
		a.utilRounds++
		a.utilDamage += utilityDamage(r, side, sides)
		a.flashAssists += countFlashAssists(r, side, sides, doc.Demo.Tickrate)
	}
}

type playerAccum struct {
	SideStatsPlayer
	order int
}

type teamRun struct {
	overall, ct, t sideAcc
	players        map[string]*playerAccum
	refs           []SideStatsMatchRef
}

func (run *teamRun) addMatch(m SideStatsMatch, opts SideStatsOptions) {
	doc := m.Document
	filter := Filter{TeamKey: m.TeamKey, Side: opts.Side, Buys: opts.Buys, OpponentBuys: opts.OpponentBuys, Lineup: opts.Lineup}
	run.overall.seenMatch, run.ct.seenMatch, run.t.seenMatch = false, false, false
	kept := 0
	names := map[uint8]Player{}
	for _, p := range doc.Players {
		names[p.Slot] = p
	}
	for _, r := range filter.Apply(doc) {
		side := doc.TeamSide(m.TeamKey, r)
		if r.Winner == "" {
			continue
		}
		kept++
		run.overall.addRound(doc, r, side)
		if side == SideCT {
			run.ct.addRound(doc, r, side)
		} else {
			run.t.addRound(doc, r, side)
		}
		run.addPlayers(r, side, names)
	}
	if kept == 0 {
		return
	}
	won := mapWon(doc, m.TeamKey)
	if won {
		run.overall.mapsWon++
	}
	run.refs = append(run.refs, SideStatsMatchRef{
		MatchID: m.MatchID, Map: doc.Demo.Map, PlayedAt: m.PlayedAt, Opponent: m.Opponent, Rounds: kept, Won: won,
	})
}

func (run *teamRun) addPlayers(r Round, side Side, names map[uint8]Player) {
	for _, pr := range r.Players {
		p, ok := names[pr.Slot]
		if pr.Side != side || !ok || p.SteamID64 == "" {
			continue
		}
		acc := run.players[p.SteamID64]
		if acc == nil {
			acc = &playerAccum{SideStatsPlayer: SideStatsPlayer{SteamID64: p.SteamID64}, order: len(run.players)}
			run.players[p.SteamID64] = acc
		}
		acc.Name = p.Name
		acc.Rounds++
		acc.Kills += pr.Kills
		acc.Deaths += pr.Deaths
		acc.Damage += pr.Damage
	}
}

func (run *teamRun) playerCards(roles map[string]string) []SideStatsPlayer {
	list := make([]*playerAccum, 0, len(run.players))
	for _, p := range run.players {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Rounds != list[j].Rounds {
			return list[i].Rounds > list[j].Rounds
		}
		return list[i].order < list[j].order
	})
	out := make([]SideStatsPlayer, 0, len(list))
	for _, acc := range list {
		p := acc.SideStatsPlayer
		rounds := float64(p.Rounds)
		p.KPR, p.DPR, p.ADR = round2(float64(p.Kills)/rounds), round2(float64(p.Deaths)/rounds), math.Round(float64(p.Damage)/rounds*10)/10
		p.KD = round2(float64(p.Kills) / float64(max(p.Deaths, 1)))
		p.Rating = playerRating(float64(p.Kills)/rounds, float64(p.Deaths)/rounds, float64(p.Damage)/rounds)
		p.Role = roles[p.SteamID64]
		out = append(out, p)
	}
	return out
}

func runTeam(matches []SideStatsMatch, opts SideStatsOptions) *teamRun {
	run := &teamRun{players: map[string]*playerAccum{}, refs: []SideStatsMatchRef{}}
	sorted := append([]SideStatsMatch(nil), matches...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].PlayedAt.After(sorted[j].PlayedAt) })
	for _, m := range sorted {
		run.addMatch(m, opts)
	}
	return run
}

// TeamSideStats builds a team's figures over the scope and compares every
// figure with the mean of the group teams' same figure in the same round
// scope. The lineup and the roles apply to the team only.
func TeamSideStats(team []SideStatsMatch, group map[string][]SideStatsMatch, opts SideStatsOptions) SideStatsReport {
	run := runTeam(team, opts)
	report := SideStatsReport{
		Matches: run.overall.matches, Rounds: run.overall.rounds,
		Overall: run.overall.stats(), CT: sideOnly(run.ct.stats()), T: sideOnly(run.t.stats()),
		Players: run.playerCards(opts.Roles), MatchList: run.refs, RatingFormula: RatingFormula,
	}
	groupOpts := SideStatsOptions{Side: opts.Side, Buys: opts.Buys, OpponentBuys: opts.OpponentBuys}
	var overall, ct, t []SideStats
	for _, matches := range group {
		g := runTeam(matches, groupOpts)
		if g.overall.rounds == 0 {
			continue
		}
		overall, ct, t = append(overall, g.overall.stats()), append(ct, sideOnly(g.ct.stats())), append(t, sideOnly(g.t.stats()))
	}
	report.GroupTeams = len(overall)
	applyGroupAverage(&report.Overall, overall)
	applyGroupAverage(&report.CT, ct)
	applyGroupAverage(&report.T, t)
	return report
}

// applyGroupAverage sets each figure's GroupAvg to the plain mean of that
// figure over the group teams that have it, so every team weighs the same.
func applyGroupAverage(target *SideStats, group []SideStats) {
	fields := target.fields()
	for i, field := range fields {
		sum, n := 0.0, 0
		for k := range group {
			if v := group[k].fields()[i].Value; v != nil {
				sum += *v
				n++
			}
		}
		field.GroupTeams = n
		field.GroupAvg = nil
		if n > 0 {
			avg := round2(sum / float64(n))
			field.GroupAvg = &avg
		}
	}
}
