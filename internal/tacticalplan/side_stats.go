package tacticalplan

import (
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

// GroupMinRounds is the denominator a group team needs for its figure to enter the mean.
const GroupMinRounds = 20

// SideStatsMatch is one match of a team that the side stats read.
type SideStatsMatch struct {
	MatchID  uuid.UUID
	PlayedAt time.Time
	Document Document
	TeamKey  string
	Opponent string
}

// SideStatsOptions is the round scope: the side, the lineup and the team scope's round
// criteria (buys, AWP, advantage, range). Roles names each SteamID's inferred role.
type SideStatsOptions struct {
	Side   Side
	Rounds Filter
	Lineup Lineup
	Roles  map[string]string
	// MapRoles is each player's role per side when the scope reads one map.
	MapRoles map[string]PlayerMapRoles
}

// SideStat is one figure (nil Value without data; rates in percent) and the mean
// of the group teams with at least GroupMinRounds behind it, GroupTeams of them.
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
	Matches        int                 `json:"matches"`
	Rounds         int                 `json:"rounds"`
	Overall        SideStats           `json:"overall"`
	CT             SideStats           `json:"ct"`
	T              SideStats           `json:"t"`
	Players        []SideStatsPlayer   `json:"players"`
	MatchList      []SideStatsMatchRef `json:"match_list"`
	GroupTeams     int                 `json:"group_teams"`
	GroupMatches   int                 `json:"group_matches"`
	GroupMinRounds int                 `json:"group_min_rounds"`
	RatingFormula  string              `json:"rating_formula"`
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

// roundView is one kept round seen from the team: its side, every slot's side
// and whether the demo was scanned with utility effects.
type roundView struct {
	Round
	side     Side
	sides    map[uint8]Side
	tickrate float64
	utility  bool
}

// countFlashAssists counts the side's kills of an enemy a teammate of the
// killer had blinded, while still blind; one assist per thrower and kill.
func countFlashAssists(v roundView) int {
	type blind struct {
		thrower, victim uint8
		from, to        int
	}
	var blinds []blind
	assists := 0
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
					assists++
				}
			}
		}
	}
	return assists
}

func utilityDamage(v roundView) int {
	total := 0
	for _, ev := range v.Events {
		if (ev.Kind == EventHE || ev.Kind == EventMolotov) && ev.Effect != nil && ev.ActorSlot != nil && v.sides[*ev.ActorSlot] == v.side {
			total += ev.Effect.EnemyDamage
		}
	}
	return total
}

func (a *sideAcc) addRound(v roundView) {
	if !a.seenMatch {
		a.seenMatch = true
		a.matches++
	}
	a.rounds++
	won := v.Winner == v.side
	if won {
		a.won++
	}
	if v.Class.HasTag(TagPistol) {
		a.pistols++
		if won {
			a.pistolsWon++
		}
	}
	if v.Class.OpeningSide != "" {
		a.openings++
		if v.Class.OpeningSide == v.side {
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
	for _, pr := range v.Players {
		if pr.Side == v.side && pr.Deaths > 0 {
			a.deaths++
			if pr.Traded {
				a.traded++
			}
		}
	}
	if v.utility {
		a.utilRounds++
		a.utilDamage += utilityDamage(v)
		a.flashAssists += countFlashAssists(v)
	}
}

type playerAccum struct {
	SideStatsPlayer
	Assists    int
	KASTRounds int
	order      int
}

// teamRun adds up one team's matches; a group team skips the cards and the match list.
type teamRun struct {
	overall, ct, t sideAcc
	players        map[string]*playerAccum
	refs           []SideStatsMatchRef
	group          bool
	added          int
}

func newTeamRun(group bool) *teamRun {
	return &teamRun{players: map[string]*playerAccum{}, refs: []SideStatsMatchRef{}, group: group}
}

func (run *teamRun) addMatch(m SideStatsMatch, opts SideStatsOptions) {
	run.added++
	doc := m.Document
	filter := opts.Rounds
	filter.TeamKey, filter.Side, filter.Lineup, filter.Slots, filter.Subjects = m.TeamKey, opts.Side, opts.Lineup, nil, nil
	run.overall.seenMatch, run.ct.seenMatch, run.t.seenMatch = false, false, false
	kept := 0
	names := map[uint8]Player{}
	for _, p := range doc.Players {
		names[p.Slot] = p
	}
	for _, r := range filter.Apply(doc) {
		if r.Winner == "" {
			continue
		}
		v := roundView{Round: r, side: doc.TeamSide(m.TeamKey, r), sides: map[uint8]Side{}, tickrate: doc.Demo.Tickrate, utility: doc.UtilityEffects}
		for _, pr := range r.Players {
			v.sides[pr.Slot] = pr.Side
		}
		kept++
		run.overall.addRound(v)
		if v.side == SideCT {
			run.ct.addRound(v)
		} else {
			run.t.addRound(v)
		}
		if !run.group {
			run.addPlayers(v, names)
		}
	}
	if kept == 0 {
		return
	}
	won := mapWon(doc, m.TeamKey)
	if won {
		run.overall.mapsWon++
	}
	if !run.group {
		run.refs = append(run.refs, SideStatsMatchRef{
			MatchID: m.MatchID, Map: doc.Demo.Map, PlayedAt: m.PlayedAt, Opponent: m.Opponent, Rounds: kept, Won: won,
		})
	}
}

func (run *teamRun) addPlayers(v roundView, names map[uint8]Player) {
	for _, pr := range v.Players {
		p, ok := names[pr.Slot]
		if pr.Side != v.side || !ok || p.SteamID64 == "" {
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
		acc.Assists += pr.Assists
		if pr.Kills > 0 || pr.Assists > 0 || pr.Survived || pr.Traded {
			acc.KASTRounds++
		}
	}
}

func (run *teamRun) playerCards(opts SideStatsOptions) []SideStatsPlayer {
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
		p.Rating = round2(Rating(RatingLine{Rounds: p.Rounds, Kills: p.Kills, Deaths: p.Deaths, Assists: acc.Assists, Damage: p.Damage, KASTRounds: acc.KASTRounds}))
		p.Role = opts.Roles[p.SteamID64]
		if r, ok := opts.MapRoles[p.SteamID64]; ok {
			p.MapRoles = &r
		}
		out = append(out, p)
	}
	return out
}

// SideStatsGroup folds the comparison teams' matches one at a time, so the
// caller can drop each document as soon as it is added.
type SideStatsGroup struct {
	opts    SideStatsOptions
	teams   map[string]*teamRun
	matches map[uuid.UUID]bool
}

// NewSideStatsGroup starts a group over the round scope; the lineup never narrows a group.
func NewSideStatsGroup(scope SideStatsOptions) *SideStatsGroup {
	opts := SideStatsOptions{Side: scope.Side, Rounds: scope.Rounds}
	return &SideStatsGroup{opts: opts, teams: map[string]*teamRun{}, matches: map[uuid.UUID]bool{}}
}

// Add folds one match of a group team into its figures.
func (g *SideStatsGroup) Add(teamID string, m SideStatsMatch) {
	run := g.teams[teamID]
	if run == nil {
		run = newTeamRun(true)
		g.teams[teamID] = run
	}
	g.matches[m.MatchID] = true
	run.addMatch(m, g.opts)
}

// Added is how many matches a group team has had folded in.
func (g *SideStatsGroup) Added(teamID string) int {
	if run := g.teams[teamID]; run != nil {
		return run.added
	}
	return 0
}

// SideStatsInput is a side stats build: the team's matches, its round scope and the folded group (nil for none).
type SideStatsInput struct {
	Team    []SideStatsMatch
	Options SideStatsOptions
	Group   *SideStatsGroup
}

// groupTeam is one group team's figures and the rounds behind them.
type groupTeam struct {
	stats  SideStats
	rounds int
}

// TeamSideStats builds the team's figures over the scope, each with the group mean.
// The lineup and the roles apply to the team only.
func TeamSideStats(in SideStatsInput) SideStatsReport {
	run := newTeamRun(false)
	sorted := append([]SideStatsMatch(nil), in.Team...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].PlayedAt.After(sorted[j].PlayedAt) })
	for _, m := range sorted {
		run.addMatch(m, in.Options)
	}
	report := SideStatsReport{
		Matches: run.overall.matches, Rounds: run.overall.rounds,
		Overall: run.overall.stats(), CT: sideOnly(run.ct.stats()), T: sideOnly(run.t.stats()),
		Players: run.playerCards(in.Options), MatchList: run.refs, GroupMinRounds: GroupMinRounds, RatingFormula: RatingFormula,
	}
	var overall, ct, t []groupTeam
	if in.Group != nil {
		report.GroupMatches = len(in.Group.matches)
		for _, g := range in.Group.teams {
			if g.overall.rounds == 0 {
				continue
			}
			overall = append(overall, groupTeam{g.overall.stats(), g.overall.rounds})
			ct = append(ct, groupTeam{sideOnly(g.ct.stats()), g.ct.rounds})
			t = append(t, groupTeam{sideOnly(g.t.stats()), g.t.rounds})
		}
	}
	report.GroupTeams = len(overall)
	applyGroupAverage(&report.Overall, overall)
	applyGroupAverage(&report.CT, ct)
	applyGroupAverage(&report.T, t)
	return report
}

// applyGroupAverage sets each figure's GroupAvg to the plain mean over the group
// teams with GroupMinRounds in its denominator; maps won and rounds read the team's rounds.
func applyGroupAverage(target *SideStats, group []groupTeam) {
	fields := target.fields()
	for i, field := range fields {
		roundBased := field == &target.MapsWon || field == &target.RoundsPlayed
		sum, n := 0.0, 0
		for k := range group {
			stat := group[k].stats.fields()[i]
			denominator := stat.Total
			if roundBased {
				denominator = group[k].rounds
			}
			if stat.Value != nil && denominator >= GroupMinRounds {
				sum += *stat.Value
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
