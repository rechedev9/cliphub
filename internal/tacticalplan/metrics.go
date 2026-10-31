package tacticalplan

import (
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Advanced metric rules. They travel in every report so the UI can state them.
const (
	// MetricDuelEvenRatio bounds an even duel: neither player carried more than
	// this many times the other's round-start equipment.
	MetricDuelEvenRatio = 1.5
	// MetricFlashMinSeconds is the blind that counts as a flashed player.
	MetricFlashMinSeconds = 1.0
	// MetricTradeSeconds is the trade window, the one the scan uses.
	MetricTradeSeconds = 5.0
	// MetricExampleLimit caps each example list.
	MetricExampleLimit = 8
	// metricSwingLimit caps the best and worst swing lists.
	metricSwingLimit = 5
)

// Swing kinds: what moved the win probability.
const (
	SwingKill     = "kill"
	SwingDeath    = "death"
	SwingTeamKill = "team_kill"
	SwingPlant    = "plant"
	SwingDefuse   = "defuse"
)

// MetricsMatch is one match the metrics read.
type MetricsMatch struct {
	MatchID  uuid.UUID
	PlayedAt time.Time
	Document Document
	// Setup holds the frame ScoutSetupSeconds after freeze end, per round. It
	// is nil when the match has no position blob.
	Setup map[int]Frame
	// TeamKey limits the player lines to one team; empty keeps both teams.
	TeamKey string
}

// MetricsOptions narrows whose numbers are kept. The win probability always
// follows every player, because every kill moves it.
type MetricsOptions struct {
	SteamIDs []string
	Side     Side
	// Roles is the inferred role per SteamID, for the role breakdown.
	Roles     map[string]string
	Timelines bool
}

// MetricsThresholds are the rules the metrics applied.
type MetricsThresholds struct {
	DuelEvenRatio   float64 `json:"duel_even_ratio"`
	FlashMinSeconds float64 `json:"flash_min_seconds"`
	TradeSeconds    float64 `json:"trade_seconds"`
	SetupSeconds    float64 `json:"setup_seconds"`
	ExampleLimit    int     `json:"example_limit"`
}

// MetricCount is a count with the first rounds it counted.
type MetricCount struct {
	Count    int        `json:"count"`
	Examples []Evidence `json:"examples"`
}

func (m *MetricCount) add(e Evidence) {
	m.Count++
	if len(m.Examples) < MetricExampleLimit {
		m.Examples = append(m.Examples, e)
	}
}

// DuelBucket is the duels won and lost in one equipment matchup.
type DuelBucket struct {
	Won  MetricCount `json:"won"`
	Lost MetricCount `json:"lost"`
}

// DuelMetrics splits a player's duels by who carried more equipment.
type DuelMetrics struct {
	Even         DuelBucket `json:"even"`
	Advantage    DuelBucket `json:"advantage"`
	Disadvantage DuelBucket `json:"disadvantage"`
}

// EcoKill is one kill and what it was worth once equipment is accounted for.
type EcoKill struct {
	Evidence
	Value       float64 `json:"value"`
	KillerEquip int     `json:"killer_equip"`
	VictimEquip int     `json:"victim_equip"`
}

// EcoMetrics are kills and deaths weighed by equipment. A duel is worth
// 2·Ev/(Ek+Ev): 1 between equals, less against a poorer victim, more against a
// richer one. The same value is the killer's kill and the victim's death.
type EcoMetrics struct {
	Kills  float64 `json:"kills"`
	Deaths float64 `json:"deaths"`
	// Examples are the player's kills worth the most.
	Examples []EcoKill `json:"examples"`
}

// SwingEvent is one moment a player moved their side's win probability.
type SwingEvent struct {
	Evidence
	Kind   string  `json:"kind"`
	Delta  float64 `json:"delta"`
	Before float64 `json:"before"`
	After  float64 `json:"after"`
}

// SwingMetrics is win probability added: a kill credits the killer with the
// gain of their side and debits the victim with the loss of theirs, a plant
// credits the planter and a defuse the defuser.
type SwingMetrics struct {
	WPA      float64      `json:"wpa"`
	PerRound float64      `json:"per_round"`
	Best     []SwingEvent `json:"best"`
	Worst    []SwingEvent `json:"worst"`
}

// TradeMetrics counts trades inside MetricTradeSeconds.
type TradeMetrics struct {
	TradeKills     MetricCount `json:"trade_kills"`
	TradedDeaths   MetricCount `json:"traded_deaths"`
	UntradedDeaths MetricCount `json:"untraded_deaths"`
}

// FlashMetrics is what a player's flashes did.
type FlashMetrics struct {
	Thrown            int         `json:"thrown"`
	EnemiesBlinded    MetricCount `json:"enemies_blinded"`
	TeamBlinded       MetricCount `json:"team_blinded"`
	EnemyBlindSeconds float64     `json:"enemy_blind_seconds"`
	// Assists are teammate kills of an enemy the player had blinded, while blind.
	Assists MetricCount `json:"assists"`
}

// GrenadeDamage is what a player's HEs or molotovs did. Examples are the
// throws with the most enemy damage.
type GrenadeDamage struct {
	Thrown      int        `json:"thrown"`
	EnemyDamage int        `json:"enemy_damage"`
	TeamDamage  int        `json:"team_damage"`
	Examples    []Evidence `json:"examples"`
}

// UtilityMetrics is a player's utility quality. Measured is false when none of
// the player's matches was scanned with utility effects.
type UtilityMetrics struct {
	Measured bool          `json:"measured"`
	Flash    FlashMetrics  `json:"flash"`
	HE       GrenadeDamage `json:"he"`
	Molotov  GrenadeDamage `json:"molotov"`
}

// PositionLine is a player's rounds at one spot, read ScoutSetupSeconds after
// freeze end, and how they went.
type PositionLine struct {
	Side     Side       `json:"side"`
	Callout  string     `json:"callout"`
	Rounds   int        `json:"rounds"`
	Won      int        `json:"won"`
	Kills    int        `json:"kills"`
	Deaths   int        `json:"deaths"`
	WPA      float64    `json:"wpa"`
	Examples []Evidence `json:"examples"`
}

// PlayerMatchLine is a player's numbers in one match.
type PlayerMatchLine struct {
	MatchID  uuid.UUID `json:"match_id"`
	Map      string    `json:"map"`
	PlayedAt time.Time `json:"played_at"`
	TeamKey  string    `json:"team_key"`
	Rounds   int       `json:"rounds"`
	Kills    int       `json:"kills"`
	Deaths   int       `json:"deaths"`
	EcoKills float64   `json:"eco_kills"`
	WPA      float64   `json:"wpa"`
}

// PlayerMetrics is one player's advanced metrics over the matches read.
type PlayerMetrics struct {
	SteamID64  string            `json:"steamid64"`
	Name       string            `json:"name"`
	Role       string            `json:"role"`
	Matches    int               `json:"matches"`
	Rounds     int               `json:"rounds"`
	Kills      int               `json:"kills"`
	Deaths     int               `json:"deaths"`
	Eco        EcoMetrics        `json:"eco"`
	Duels      DuelMetrics       `json:"duels"`
	Swing      SwingMetrics      `json:"swing"`
	Trades     TradeMetrics      `json:"trades"`
	Utility    UtilityMetrics    `json:"utility"`
	Positions  []PositionLine    `json:"positions"`
	MatchLines []PlayerMatchLine `json:"match_lines"`
}

// RoleLine adds up the players of one inferred role.
type RoleLine struct {
	Role          string   `json:"role"`
	Players       []string `json:"players"`
	Rounds        int      `json:"rounds"`
	Kills         int      `json:"kills"`
	Deaths        int      `json:"deaths"`
	EcoKills      float64  `json:"eco_kills"`
	EcoDeaths     float64  `json:"eco_deaths"`
	WPA           float64  `json:"wpa"`
	FlashAssists  int      `json:"flash_assists"`
	UtilityDamage int      `json:"utility_damage"`
}

// WinPoint is the CT win probability right after one event of a round.
type WinPoint struct {
	Tick          int     `json:"tick"`
	Seconds       float64 `json:"seconds"`
	Kind          string  `json:"kind"`
	ActorSlot     *uint8  `json:"actor_slot,omitempty"`
	TargetSlot    *uint8  `json:"target_slot,omitempty"`
	AliveCT       int     `json:"alive_ct"`
	AliveT        int     `json:"alive_t"`
	ProbabilityCT float64 `json:"probability_ct"`
}

// Timeline point kinds besides the event kinds.
const (
	WinPointStart = "start"
	WinPointEnd   = "end"
)

// RoundTimeline is a round's win probability from freeze end to its end.
type RoundTimeline struct {
	MatchID uuid.UUID  `json:"match_id"`
	Round   int        `json:"round"`
	Winner  Side       `json:"winner"`
	Points  []WinPoint `json:"points"`
}

// MetricsReport is the advanced metrics of a set of matches.
type MetricsReport struct {
	Model      WinModel          `json:"model"`
	Thresholds MetricsThresholds `json:"thresholds"`
	Matches    int               `json:"matches"`
	Rounds     int               `json:"rounds"`
	// UtilityMatches counts the matches scanned with utility effects.
	UtilityMatches int             `json:"utility_matches"`
	Players        []PlayerMetrics `json:"players"`
	Roles          []RoleLine      `json:"roles"`
	Timelines      []RoundTimeline `json:"timelines,omitempty"`
}

type playerAcc struct {
	m          PlayerMetrics
	matches    map[uuid.UUID]bool
	swings     []swingAt
	ecoKills   []ecoAt
	he         []damageAt
	molotov    []damageAt
	positions  map[positionKey]*PositionLine
	utilityHit bool
}

type swingAt struct {
	order int
	ev    SwingEvent
}

type ecoAt struct {
	order int
	kill  EcoKill
}

type damageAt struct {
	order  int
	damage int
	ev     Evidence
}

type positionKey struct {
	side    Side
	callout string
}

type blindWindow struct {
	thrower uint8
	victim  uint8
	from    int
	to      int
	ev      Evidence
}

type metricsRun struct {
	opts    MetricsOptions
	players map[string]*playerAcc
	order   int
}

// Metrics computes the advanced metrics of the matches. It is pure: the same
// matches and options give the same report, whatever order they arrive in.
func Metrics(matches []MetricsMatch, opts MetricsOptions) MetricsReport {
	sorted := append([]MetricsMatch(nil), matches...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].PlayedAt.Equal(sorted[j].PlayedAt) {
			return sorted[i].PlayedAt.After(sorted[j].PlayedAt)
		}
		return sorted[i].MatchID.String() < sorted[j].MatchID.String()
	})
	run := &metricsRun{opts: opts, players: map[string]*playerAcc{}}
	report := MetricsReport{
		Thresholds: MetricsThresholds{
			DuelEvenRatio: MetricDuelEvenRatio, FlashMinSeconds: MetricFlashMinSeconds,
			TradeSeconds: MetricTradeSeconds, SetupSeconds: ScoutSetupSeconds, ExampleLimit: MetricExampleLimit,
		},
		Players: []PlayerMetrics{}, Roles: []RoleLine{},
	}
	maps := []string{}
	seenMap := map[string]bool{}
	for _, m := range sorted {
		report.Matches++
		if m.Document.UtilityEffects {
			report.UtilityMatches++
		}
		if name := m.Document.Demo.Map; !seenMap[name] {
			seenMap[name] = true
			maps = append(maps, name)
		}
		for _, r := range m.Document.Rounds {
			if run.round(m, r, &report) {
				report.Rounds++
			}
		}
	}
	report.Model = winModel(maps)
	report.Players = run.finish()
	report.Roles = roleLines(report.Players)
	return report
}

// tracked reports whether a player's numbers are kept in this round.
func (run *metricsRun) tracked(m MetricsMatch, p Player, side Side) bool {
	if p.SteamID64 == "" || side == "" {
		return false
	}
	if m.TeamKey != "" && p.TeamKey != m.TeamKey {
		return false
	}
	if run.opts.Side != "" && side != run.opts.Side {
		return false
	}
	return len(run.opts.SteamIDs) == 0 || containsString(run.opts.SteamIDs, p.SteamID64)
}

func (run *metricsRun) acc(p Player) *playerAcc {
	a, ok := run.players[p.SteamID64]
	if !ok {
		role := run.opts.Roles[p.SteamID64]
		if role == "" {
			role = "unknown"
		}
		a = &playerAcc{
			m:         PlayerMetrics{SteamID64: p.SteamID64, Name: p.Name, Role: role, Positions: []PositionLine{}, MatchLines: []PlayerMatchLine{}},
			matches:   map[uuid.UUID]bool{},
			positions: map[positionKey]*PositionLine{},
		}
		run.players[p.SteamID64] = a
	}
	return a
}

func (a *playerAcc) line(m MetricsMatch, p Player) *PlayerMatchLine {
	if !a.matches[m.MatchID] {
		a.matches[m.MatchID] = true
		a.m.Matches++
		a.m.MatchLines = append(a.m.MatchLines, PlayerMatchLine{
			MatchID: m.MatchID, Map: m.Document.Demo.Map, PlayedAt: m.PlayedAt, TeamKey: p.TeamKey,
		})
	}
	return &a.m.MatchLines[len(a.m.MatchLines)-1]
}

// roundState is the simulation of one round's win probability.
type roundState struct {
	m       MetricsMatch
	r       Round
	side    map[uint8]Side
	equip   map[uint8]int
	alive   map[uint8]bool
	planted bool
	plantAt int
	rate    float64
}

func (s *roundState) win(tick int) WinState {
	w := WinState{Map: s.m.Document.Demo.Map, Planted: s.planted}
	for slot, alive := range s.alive {
		if !alive {
			continue
		}
		if s.side[slot] == SideCT {
			w.AliveCT++
			w.EquipCT += s.equip[slot]
		} else {
			w.AliveT++
			w.EquipT += s.equip[slot]
		}
	}
	if s.rate > 0 {
		if s.planted {
			w.BombSeconds = float64(tick-s.plantAt) / s.rate
		} else if s.r.TickFreezeEnd > 0 {
			w.Seconds = math.Max(float64(tick-s.r.TickFreezeEnd)/s.rate, 0)
		}
	}
	return w
}

func (s *roundState) point(tick int, kind string, ev *Event, p float64) WinPoint {
	w := s.win(tick)
	point := WinPoint{Tick: tick, Kind: kind, AliveCT: w.AliveCT, AliveT: w.AliveT, ProbabilityCT: round4(p)}
	if s.rate > 0 && s.r.TickFreezeEnd > 0 && tick > s.r.TickFreezeEnd {
		point.Seconds = round4(float64(tick-s.r.TickFreezeEnd) / s.rate)
	}
	if ev != nil {
		point.ActorSlot, point.TargetSlot = ev.ActorSlot, ev.TargetSlot
	}
	return point
}

type killAt struct {
	tick   int
	killer uint8
	victim uint8
	side   Side
	ev     Evidence
	traded bool
}

// round runs one round and reports whether any tracked player played it.
func (run *metricsRun) round(m MetricsMatch, r Round, report *MetricsReport) bool {
	doc := m.Document
	s := &roundState{
		m: m, r: r, side: map[uint8]Side{}, equip: map[uint8]int{}, alive: map[uint8]bool{},
		rate: doc.Demo.Tickrate,
	}
	tracked := map[uint8]*playerAcc{}
	identity := map[uint8]Player{}
	for _, pr := range r.Players {
		s.side[pr.Slot] = pr.Side
		s.equip[pr.Slot] = max(pr.EquipValue, winMinEquip)
		s.alive[pr.Slot] = true
		p, ok := doc.PlayerBySlot(pr.Slot)
		if !ok {
			continue
		}
		identity[pr.Slot] = p
		if run.tracked(m, p, pr.Side) {
			a := run.acc(p)
			tracked[pr.Slot] = a
			a.m.Rounds++
			a.line(m, p).Rounds++
			if doc.UtilityEffects {
				a.utilityHit = true
			}
		}
	}
	roundWPA := map[uint8]float64{}
	// After the round end the outcome is settled: late kills still count as
	// kills, but move no probability and draw no point.
	live := true
	credit := func(slot uint8, kind string, tick int, before, after float64) {
		a := tracked[slot]
		if a == nil || !live {
			return
		}
		delta := after - before
		a.m.Swing.WPA += delta
		a.line(m, identity[slot]).WPA += delta
		roundWPA[slot] += delta
		run.order++
		a.swings = append(a.swings, swingAt{order: run.order, ev: SwingEvent{
			Evidence: Evidence{MatchID: m.MatchID, Round: r.Number, Tick: tick},
			Kind:     kind, Delta: delta, Before: before, After: after,
		}})
	}

	var timeline *RoundTimeline
	if run.opts.Timelines {
		timeline = &RoundTimeline{MatchID: m.MatchID, Round: r.Number, Winner: r.Winner}
		start := max(r.TickFreezeEnd, r.TickStart)
		timeline.Points = append(timeline.Points, s.point(start, WinPointStart, nil, WinProbabilityCT(s.win(start))))
	}

	var kills []killAt
	var blinds []blindWindow
	seenUtility := map[grenadeKey]bool{}
	for i := range r.Events {
		ev := &r.Events[i]
		evid := Evidence{MatchID: m.MatchID, Round: r.Number, Tick: ev.Tick}
		live = r.TickEnd <= 0 || ev.Tick <= r.TickEnd
		draw := timeline != nil && live
		switch ev.Kind {
		case EventKill:
			if ev.TargetSlot == nil {
				continue
			}
			victim := *ev.TargetSlot
			victimSide := s.side[victim]
			beforeCT := WinProbabilityCT(s.win(ev.Tick))
			s.alive[victim] = false
			afterCT := WinProbabilityCT(s.win(ev.Tick))
			vBefore, vAfter := sideProbability(beforeCT, victimSide), sideProbability(afterCT, victimSide)
			if draw {
				timeline.Points = append(timeline.Points, s.point(ev.Tick, string(EventKill), ev, afterCT))
			}
			if ev.ActorSlot == nil {
				credit(victim, SwingDeath, ev.Tick, vBefore, vAfter)
				death(tracked[victim])
				continue
			}
			killer := *ev.ActorSlot
			if s.side[killer] == victimSide {
				credit(killer, SwingTeamKill, ev.Tick, vBefore, vAfter)
				death(tracked[victim])
				continue
			}
			credit(killer, SwingKill, ev.Tick, 1-vBefore, 1-vAfter)
			credit(victim, SwingDeath, ev.Tick, vBefore, vAfter)
			run.duel(tracked[killer], tracked[victim], s.equip[killer], s.equip[victim], evid)
			trades(&kills, killAt{tick: ev.Tick, killer: killer, victim: victim, side: s.side[killer], ev: evid}, tracked, s.rate)
			flashAssists(blinds, killer, victim, s.side, ev.Tick, tracked)
		case EventPlant:
			before := WinProbabilityCT(s.win(ev.Tick))
			s.planted, s.plantAt = true, ev.Tick
			after := WinProbabilityCT(s.win(ev.Tick))
			if draw {
				timeline.Points = append(timeline.Points, s.point(ev.Tick, string(EventPlant), ev, after))
			}
			if ev.ActorSlot != nil {
				credit(*ev.ActorSlot, SwingPlant, ev.Tick, 1-before, 1-after)
			}
		case EventDefuse:
			before := WinProbabilityCT(s.win(ev.Tick))
			if draw {
				timeline.Points = append(timeline.Points, s.point(ev.Tick, string(EventDefuse), ev, 1))
			}
			if ev.ActorSlot != nil {
				credit(*ev.ActorSlot, SwingDefuse, ev.Tick, before, 1)
			}
		case EventExplode:
			if draw {
				timeline.Points = append(timeline.Points, s.point(ev.Tick, string(EventExplode), ev, 0))
			}
		case EventFlash, EventHE, EventMolotov:
			if ev.ActorSlot == nil {
				continue
			}
			key := grenadeKey{kind: ev.Kind, slot: *ev.ActorSlot, tick: ev.Tick}
			if seenUtility[key] {
				// The scan can record one detonation twice; it counts once.
				continue
			}
			seenUtility[key] = true
			blinds = run.utility(tracked[*ev.ActorSlot], ev, evid, blinds, s.rate)
		}
	}
	if timeline != nil {
		end := r.TickEnd
		p := WinProbabilityCT(s.win(end))
		switch r.Winner {
		case SideCT:
			p = 1
		case SideT:
			p = 0
		}
		timeline.Points = append(timeline.Points, s.point(end, WinPointEnd, nil, p))
		report.Timelines = append(report.Timelines, *timeline)
	}

	slots := make([]uint8, 0, len(tracked))
	for slot := range tracked {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
	for _, slot := range slots {
		a := tracked[slot]
		for _, pr := range r.Players {
			if pr.Slot == slot {
				position(a, m, r, pr, roundWPA[slot])
			}
		}
	}
	return len(tracked) > 0
}

func sideProbability(ct float64, side Side) float64 {
	if side == SideT {
		return 1 - ct
	}
	return ct
}

type grenadeKey struct {
	kind EventKind
	slot uint8
	tick int
}

func death(a *playerAcc) {
	if a != nil {
		a.m.Deaths++
		a.current().Deaths++
	}
}

// current returns the line of the match being read. A tracked player's line is
// opened when the round starts and matches are read one at a time, so it is last.
func (a *playerAcc) current() *PlayerMatchLine {
	return &a.m.MatchLines[len(a.m.MatchLines)-1]
}

func duelBucket(d *DuelMetrics, own, other int) *DuelBucket {
	ratio := float64(own) / float64(other)
	switch {
	case ratio > MetricDuelEvenRatio:
		return &d.Advantage
	case ratio < 1/MetricDuelEvenRatio:
		return &d.Disadvantage
	default:
		return &d.Even
	}
}

func (run *metricsRun) duel(killer, victim *playerAcc, killerEquip, victimEquip int, e Evidence) {
	value := 2 * float64(victimEquip) / float64(killerEquip+victimEquip)
	if killer != nil {
		killer.m.Kills++
		line := killer.current()
		line.Kills++
		line.EcoKills += value
		killer.m.Eco.Kills += value
		duelBucket(&killer.m.Duels, killerEquip, victimEquip).Won.add(e)
		run.order++
		killer.ecoKills = append(killer.ecoKills, ecoAt{order: run.order, kill: EcoKill{
			Evidence: e, Value: value, KillerEquip: killerEquip, VictimEquip: victimEquip,
		}})
	}
	if victim != nil {
		victim.m.Deaths++
		victim.current().Deaths++
		victim.m.Eco.Deaths += value
		duelBucket(&victim.m.Duels, victimEquip, killerEquip).Lost.add(e)
	}
}

// trades credits a kill that avenges a teammate inside the trade window: the
// killer gets one trade kill and every avenged teammate a traded death.
func trades(kills *[]killAt, k killAt, tracked map[uint8]*playerAcc, rate float64) {
	window := int(MetricTradeSeconds * rate)
	traded := false
	for i := range *kills {
		prev := (*kills)[i]
		if prev.traded || prev.killer != k.victim || prev.side == k.side || k.tick-prev.tick > window {
			continue
		}
		traded = true
		(*kills)[i].traded = true
		if a := tracked[prev.victim]; a != nil {
			a.m.Trades.TradedDeaths.add(prev.ev)
		}
	}
	if traded {
		if a := tracked[k.killer]; a != nil {
			a.m.Trades.TradeKills.add(k.ev)
		}
	}
	*kills = append(*kills, k)
}

func flashAssists(blinds []blindWindow, killer, victim uint8, side map[uint8]Side, tick int, tracked map[uint8]*playerAcc) {
	credited := map[uint8]bool{}
	for _, b := range blinds {
		if b.victim != victim || b.thrower == killer || side[b.thrower] != side[killer] || credited[b.thrower] {
			continue
		}
		if tick < b.from || tick > b.to {
			continue
		}
		credited[b.thrower] = true
		if a := tracked[b.thrower]; a != nil {
			a.m.Utility.Flash.Assists.add(Evidence{MatchID: b.ev.MatchID, Round: b.ev.Round, Tick: tick})
		}
	}
}

func (run *metricsRun) utility(a *playerAcc, ev *Event, e Evidence, blinds []blindWindow, rate float64) []blindWindow {
	if ev.Kind == EventFlash && ev.Effect != nil && ev.ActorSlot != nil {
		for _, b := range ev.Effect.Blinded {
			if b.Enemy {
				blinds = append(blinds, blindWindow{
					thrower: *ev.ActorSlot, victim: b.Slot, from: ev.Tick, to: ev.Tick + int(b.Seconds*rate), ev: e,
				})
			}
		}
	}
	if a == nil {
		return blinds
	}
	run.order++
	switch ev.Kind {
	case EventFlash:
		f := &a.m.Utility.Flash
		f.Thrown++
		if ev.Effect == nil {
			return blinds
		}
		enemies, team := 0, 0
		for _, b := range ev.Effect.Blinded {
			if b.Enemy {
				f.EnemyBlindSeconds += b.Seconds
			}
			if b.Seconds < MetricFlashMinSeconds {
				continue
			}
			if b.Enemy {
				enemies++
			} else {
				team++
			}
		}
		addCount(&f.EnemiesBlinded, enemies, e)
		addCount(&f.TeamBlinded, team, e)
	case EventHE:
		a.he = append(a.he, grenadeDamage(&a.m.Utility.HE, ev, e, run.order))
	case EventMolotov:
		a.molotov = append(a.molotov, grenadeDamage(&a.m.Utility.Molotov, ev, e, run.order))
	}
	return blinds
}

func addCount(m *MetricCount, n int, e Evidence) {
	if n <= 0 {
		return
	}
	m.Count += n
	if len(m.Examples) < MetricExampleLimit {
		m.Examples = append(m.Examples, e)
	}
}

func grenadeDamage(g *GrenadeDamage, ev *Event, e Evidence, order int) damageAt {
	g.Thrown++
	d := damageAt{order: order, ev: e}
	if ev.Effect != nil {
		g.EnemyDamage += ev.Effect.EnemyDamage
		g.TeamDamage += ev.Effect.TeamDamage
		d.damage = ev.Effect.EnemyDamage
	}
	return d
}

func position(a *playerAcc, m MetricsMatch, r Round, pr PlayerRound, wpa float64) {
	frame, ok := m.Setup[r.Number]
	if !ok {
		return
	}
	for _, sample := range frame.Samples {
		if sample.Slot != pr.Slot || !sample.Flags.Has(FlagAlive) {
			continue
		}
		callout := NearestCallout(m.Document.Geometry, [3]float64{sample.X, sample.Y, sample.Z})
		if callout == "" {
			return
		}
		key := positionKey{side: pr.Side, callout: callout}
		line := a.positions[key]
		if line == nil {
			line = &PositionLine{Side: pr.Side, Callout: callout, Examples: []Evidence{}}
			a.positions[key] = line
		}
		line.Rounds++
		if r.Winner == pr.Side {
			line.Won++
		}
		line.Kills += pr.Kills
		line.Deaths += pr.Deaths
		line.WPA += wpa
		if len(line.Examples) < MetricExampleLimit {
			line.Examples = append(line.Examples, Evidence{MatchID: m.MatchID, Round: r.Number, Tick: frame.Tick})
		}
		return
	}
}

func (run *metricsRun) finish() []PlayerMetrics {
	out := make([]PlayerMetrics, 0, len(run.players))
	for _, a := range run.players {
		p := a.m
		p.Utility.Measured = a.utilityHit
		if p.Rounds > 0 {
			p.Swing.PerRound = round4(p.Swing.WPA / float64(p.Rounds))
		}
		p.Swing.WPA = round4(p.Swing.WPA)
		p.Swing.Best, p.Swing.Worst = topSwings(a.swings)
		p.Eco.Kills, p.Eco.Deaths = round4(p.Eco.Kills), round4(p.Eco.Deaths)
		p.Eco.Examples = topEcoKills(a.ecoKills)
		p.Utility.Flash.EnemyBlindSeconds = round4(p.Utility.Flash.EnemyBlindSeconds)
		p.Utility.HE.Examples = topDamage(a.he)
		p.Utility.Molotov.Examples = topDamage(a.molotov)
		p.Trades.UntradedDeaths = untraded(p)
		for i := range p.MatchLines {
			p.MatchLines[i].EcoKills = round4(p.MatchLines[i].EcoKills)
			p.MatchLines[i].WPA = round4(p.MatchLines[i].WPA)
		}
		for _, line := range a.positions {
			line.WPA = round4(line.WPA)
			p.Positions = append(p.Positions, *line)
		}
		sort.Slice(p.Positions, func(i, j int) bool {
			x, y := p.Positions[i], p.Positions[j]
			if x.Side != y.Side {
				return x.Side < y.Side
			}
			if x.Rounds != y.Rounds {
				return x.Rounds > y.Rounds
			}
			return x.Callout < y.Callout
		})
		normalizeCounts(&p)
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Swing.WPA != out[j].Swing.WPA {
			return out[i].Swing.WPA > out[j].Swing.WPA
		}
		return out[i].SteamID64 < out[j].SteamID64
	})
	return out
}

// untraded is the deaths no teammate traded. Its examples are left empty: the
// deaths are the player's swing debits, which already link to their rounds.
func untraded(p PlayerMetrics) MetricCount {
	n := max(p.Deaths-p.Trades.TradedDeaths.Count, 0)
	return MetricCount{Count: n, Examples: []Evidence{}}
}

// normalizeCounts turns nil example lists into empty ones, so the JSON always
// carries arrays.
func normalizeCounts(p *PlayerMetrics) {
	counts := []*MetricCount{
		&p.Duels.Even.Won, &p.Duels.Even.Lost, &p.Duels.Advantage.Won, &p.Duels.Advantage.Lost,
		&p.Duels.Disadvantage.Won, &p.Duels.Disadvantage.Lost,
		&p.Trades.TradeKills, &p.Trades.TradedDeaths, &p.Trades.UntradedDeaths,
		&p.Utility.Flash.EnemiesBlinded, &p.Utility.Flash.TeamBlinded, &p.Utility.Flash.Assists,
	}
	for _, c := range counts {
		if c.Examples == nil {
			c.Examples = []Evidence{}
		}
	}
}

func topSwings(all []swingAt) ([]SwingEvent, []SwingEvent) {
	best, worst := []swingAt{}, []swingAt{}
	for _, s := range all {
		if s.ev.Delta > 0 {
			best = append(best, s)
		} else if s.ev.Delta < 0 {
			worst = append(worst, s)
		}
	}
	sort.SliceStable(best, func(i, j int) bool {
		if best[i].ev.Delta != best[j].ev.Delta {
			return best[i].ev.Delta > best[j].ev.Delta
		}
		return best[i].order < best[j].order
	})
	sort.SliceStable(worst, func(i, j int) bool {
		if worst[i].ev.Delta != worst[j].ev.Delta {
			return worst[i].ev.Delta < worst[j].ev.Delta
		}
		return worst[i].order < worst[j].order
	})
	return swingList(best), swingList(worst)
}

func swingList(list []swingAt) []SwingEvent {
	out := make([]SwingEvent, 0, metricSwingLimit)
	for i := 0; i < len(list) && i < metricSwingLimit; i++ {
		ev := list[i].ev
		ev.Delta, ev.Before, ev.After = round4(ev.Delta), round4(ev.Before), round4(ev.After)
		out = append(out, ev)
	}
	return out
}

func topEcoKills(all []ecoAt) []EcoKill {
	sorted := append([]ecoAt(nil), all...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].kill.Value != sorted[j].kill.Value {
			return sorted[i].kill.Value > sorted[j].kill.Value
		}
		return sorted[i].order < sorted[j].order
	})
	out := make([]EcoKill, 0, MetricExampleLimit)
	for i := 0; i < len(sorted) && i < MetricExampleLimit; i++ {
		k := sorted[i].kill
		k.Value = round4(k.Value)
		out = append(out, k)
	}
	return out
}

func topDamage(all []damageAt) []Evidence {
	sorted := []damageAt{}
	for _, d := range all {
		if d.damage > 0 {
			sorted = append(sorted, d)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].damage != sorted[j].damage {
			return sorted[i].damage > sorted[j].damage
		}
		return sorted[i].order < sorted[j].order
	})
	out := make([]Evidence, 0, MetricExampleLimit)
	for i := 0; i < len(sorted) && i < MetricExampleLimit; i++ {
		out = append(out, sorted[i].ev)
	}
	return out
}

func roleLines(players []PlayerMetrics) []RoleLine {
	byRole := map[string]*RoleLine{}
	for _, p := range players {
		line := byRole[p.Role]
		if line == nil {
			line = &RoleLine{Role: p.Role, Players: []string{}}
			byRole[p.Role] = line
		}
		line.Players = append(line.Players, p.SteamID64)
		line.Rounds += p.Rounds
		line.Kills += p.Kills
		line.Deaths += p.Deaths
		line.EcoKills += p.Eco.Kills
		line.EcoDeaths += p.Eco.Deaths
		line.WPA += p.Swing.WPA
		line.FlashAssists += p.Utility.Flash.Assists.Count
		line.UtilityDamage += p.Utility.HE.EnemyDamage + p.Utility.Molotov.EnemyDamage
	}
	out := make([]RoleLine, 0, len(byRole))
	for _, line := range byRole {
		sort.Strings(line.Players)
		line.EcoKills, line.EcoDeaths, line.WPA = round4(line.EcoKills), round4(line.EcoDeaths), round4(line.WPA)
		out = append(out, *line)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Role < out[j].Role })
	return out
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
