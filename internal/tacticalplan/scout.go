package tacticalplan

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Scouting rules. They ship inside every report, so a reader always sees the
// rule next to the number it produced.
const (
	// ScoutExampleLimit caps the example rounds kept for each row.
	ScoutExampleLimit = 8
	// ScoutEarlyUtilitySeconds is how soon after freeze end a grenade counts as
	// part of the default (T) or the setup (CT).
	ScoutEarlyUtilitySeconds = 20
	// ScoutExecuteWindowSeconds is how close before the plant, or before the
	// first contact when there was no plant, a grenade counts as the execute.
	ScoutExecuteWindowSeconds = 20
	// ScoutSetupSeconds is when the CT positions of a setup are read.
	ScoutSetupSeconds = 15
	// ScoutUtilityRowLimit caps the grenade rows of a section, most frequent first.
	ScoutUtilityRowLimit = 12
)

// Economy contexts of the rounds that follow a lost round.
const (
	ScoutAfterPistolLoss = "after_pistol_loss"
	ScoutAfterLoss       = "after_loss"
)

// awpWeapon is the weapon name the scan gives AWP kills.
const awpWeapon = "awp"

// ScoutMatch is one match of the scouted team: its document, the key the team
// played under in it, and the match-level facts the filter reads.
type ScoutMatch struct {
	MatchID    uuid.UUID
	TeamKey    string
	PlayedAt   time.Time
	OpponentID string
	Opponent   string
	Document   Document
	// Setup holds, per round number, the frame read ScoutSetupSeconds after
	// freeze end. It is nil when the match has no position blob.
	Setup map[int]Frame
}

// ScoutFilter selects matches by map, date and rival, and rounds by the side
// the scouted team played. Zero fields do not filter; From and To are inclusive.
type ScoutFilter struct {
	Maps      []string   `json:"maps,omitempty"`
	Side      Side       `json:"side,omitempty"`
	From      *time.Time `json:"from,omitempty"`
	To        *time.Time `json:"to,omitempty"`
	Opponents []string   `json:"opponents,omitempty"`
}

// Select returns the matches the filter keeps, most recent first. The order is
// the order every example list follows, so the newest rounds come first.
func (f ScoutFilter) Select(matches []ScoutMatch) []ScoutMatch {
	out := make([]ScoutMatch, 0, len(matches))
	for _, m := range matches {
		if len(f.Maps) > 0 && !containsString(f.Maps, m.Document.Demo.Map) {
			continue
		}
		if f.From != nil && m.PlayedAt.Before(*f.From) {
			continue
		}
		if f.To != nil && m.PlayedAt.After(*f.To) {
			continue
		}
		if len(f.Opponents) > 0 && !containsString(f.Opponents, m.OpponentID) {
			continue
		}
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].PlayedAt.Equal(out[j].PlayedAt) {
			return out[i].PlayedAt.After(out[j].PlayedAt)
		}
		return out[i].MatchID.String() < out[j].MatchID.String()
	})
	return out
}

func (f ScoutFilter) rounds(m ScoutMatch) []Round {
	return f.roundFilter(m).Apply(m.Document)
}

func (f ScoutFilter) roundFilter(m ScoutMatch) Filter {
	return Filter{TeamKey: m.TeamKey, Side: f.Side}
}

// AggregateMatches computes Tendencies over the rounds the scouted team played
// in the selected matches. Player lines are keyed by SteamID across matches.
func AggregateMatches(matches []ScoutMatch, f ScoutFilter) Tendencies {
	selected := f.Select(matches)
	a := newAggregator(Filter{Side: f.Side})
	for _, m := range selected {
		filter := f.roundFilter(m)
		a.add(m.Document, filter, filter.Apply(m.Document))
	}
	t := a.finish()
	t.MatchCount = len(selected)
	return t
}

// Evidence is one round a number counted, by match, round and tick.
type Evidence struct {
	MatchID uuid.UUID `json:"match_id"`
	Round   int       `json:"round"`
	Tick    int       `json:"tick"`
}

// ScoutRounds is a set of rounds: how many, their share of the section, how
// many were won, and the first rounds of the set.
type ScoutRounds struct {
	Rounds   int        `json:"rounds"`
	Share    Rate       `json:"share"`
	WinRate  Rate       `json:"win_rate"`
	Examples []Evidence `json:"examples"`
}

// ScoutThresholds are the rules the report applied.
type ScoutThresholds struct {
	MinReliableSample    int `json:"min_reliable_sample"`
	ExampleLimit         int `json:"example_limit"`
	EarlyUtilitySeconds  int `json:"early_utility_seconds"`
	ExecuteWindowSeconds int `json:"execute_window_seconds"`
	SetupSeconds         int `json:"setup_seconds"`
	UtilityRowLimit      int `json:"utility_row_limit"`
}

// ScoutMatchSummary is one match the report read.
type ScoutMatchSummary struct {
	MatchID       uuid.UUID `json:"match_id"`
	Map           string    `json:"map"`
	PlayedAt      time.Time `json:"played_at"`
	TeamKey       string    `json:"team_key"`
	OpponentID    string    `json:"opponent_id,omitempty"`
	Opponent      string    `json:"opponent"`
	Score         int       `json:"score"`
	OpponentScore int       `json:"opponent_score"`
	// Rounds and Wins count the rounds the side filter kept.
	Rounds int `json:"rounds"`
	Wins   int `json:"wins"`
}

// ScoutMapSummary is the team's record on one map.
type ScoutMapSummary struct {
	Map       string `json:"map"`
	Matches   int    `json:"matches"`
	MatchWins int    `json:"match_wins"`
	Rounds    int    `json:"rounds"`
	WinRate   Rate   `json:"win_rate"`
	CTWinRate Rate   `json:"ct_win_rate"`
	TWinRate  Rate   `json:"t_win_rate"`
}

// ScoutPatternRow is one round shape on one site, for one side.
type ScoutPatternRow struct {
	Side    Side   `json:"side"`
	Pattern string `json:"pattern"`
	Site    Site   `json:"site"`
	ScoutRounds
}

// ScoutSide is a section's rounds on one side.
type ScoutSide struct {
	Side Side `json:"side"`
	ScoutRounds
}

// ScoutPlayerCount is how often one player did something.
type ScoutPlayerCount struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	Count     int    `json:"count"`
}

// ScoutUtilityRow is one grenade type landing at one callout: in how many of
// the section's rounds it was thrown, when, and by whom.
type ScoutUtilityRow struct {
	Kind    EventKind `json:"kind"`
	Callout string    `json:"callout"`
	Rounds  int       `json:"rounds"`
	Share   Rate      `json:"share"`
	Count   int       `json:"count"`
	// MedianSeconds is when it landed, in seconds from freeze end.
	MedianSeconds float64            `json:"median_seconds"`
	Throwers      []ScoutPlayerCount `json:"throwers"`
	Examples      []Evidence         `json:"examples"`
}

// ScoutPlaceRow is one place where the round's opening duel was fought.
type ScoutPlaceRow struct {
	Place    string     `json:"place"`
	Rounds   int        `json:"rounds"`
	Share    Rate       `json:"share"`
	Won      Rate       `json:"won"`
	Examples []Evidence `json:"examples"`
}

// ScoutPistols is the pistol rounds, per side and per round shape.
type ScoutPistols struct {
	Sides []ScoutSide       `json:"sides"`
	Rows  []ScoutPatternRow `json:"rows"`
}

// ScoutDefaults is the T rounds on a full buy: their shape, the early
// utility, and where the first duel happens.
type ScoutDefaults struct {
	ScoutRounds
	Patterns          []ScoutPatternRow `json:"patterns"`
	Utility           []ScoutUtilityRow `json:"utility"`
	OpeningPlaces     []ScoutPlaceRow   `json:"opening_places"`
	FirstContact      Histogram         `json:"first_contact"`
	UtilityRoundTotal int               `json:"utility_round_total"`
}

// ScoutSetups is the CT rounds on a full buy: their shape, the early utility,
// and where each player stands ScoutSetupSeconds into the round.
type ScoutSetups struct {
	ScoutRounds
	Patterns []ScoutPatternRow `json:"patterns"`
	Utility  []ScoutUtilityRow `json:"utility"`
	// Sampled counts the rounds with a position frame to read the setup from.
	Sampled   int                `json:"sampled"`
	Positions []ScoutPositionRow `json:"positions"`
}

// ScoutPositionRow is where one player stood at setup time.
type ScoutPositionRow struct {
	SteamID64 string      `json:"steamid64"`
	Name      string      `json:"name"`
	Rounds    int         `json:"rounds"`
	Spots     []ScoutSpot `json:"spots"`
}

// ScoutSpot is one callout a player held, over the rounds they were read in.
type ScoutSpot struct {
	Callout  string     `json:"callout"`
	Share    Rate       `json:"share"`
	Examples []Evidence `json:"examples"`
}

// ScoutExecute is the executes and splits on one bombsite.
type ScoutExecute struct {
	Site Site `json:"site"`
	ScoutRounds
	Patterns     []ScoutPatternRow `json:"patterns"`
	Plants       Rate              `json:"plants"`
	PlantTime    Histogram         `json:"plant_time"`
	FirstContact Histogram         `json:"first_contact"`
	// Mean grenades of each kind the team threw inside the execute window.
	MeanUtility map[EventKind]float64 `json:"mean_utility"`
	Utility     []ScoutUtilityRow     `json:"utility"`
}

// ScoutEconomyRow is one buy the team made in one context.
type ScoutEconomyRow struct {
	Context string  `json:"context"`
	Side    Side    `json:"side"`
	Buy     BuyType `json:"buy"`
	ScoutRounds
}

// ScoutEconomyContext is every round of one context on one side.
type ScoutEconomyContext struct {
	Context string `json:"context"`
	Side    Side   `json:"side"`
	ScoutRounds
}

// ScoutEconomy is what the team buys after losing a round.
type ScoutEconomy struct {
	Contexts []ScoutEconomyContext `json:"contexts"`
	Rows     []ScoutEconomyRow     `json:"rows"`
}

// ScoutAWPSide is the team's AWP use on one side.
type ScoutAWPSide struct {
	Side         Side `json:"side"`
	Rounds       int  `json:"rounds"`
	KillRounds   Rate `json:"kill_rounds"`
	Kills        int  `json:"kills"`
	OpeningKills int  `json:"opening_kills"`
}

// ScoutAWPRow is the AWP kills made from one place on one side.
type ScoutAWPRow struct {
	Side         Side               `json:"side"`
	Place        string             `json:"place"`
	Kills        int                `json:"kills"`
	Share        Rate               `json:"share"`
	OpeningKills int                `json:"opening_kills"`
	Players      []ScoutPlayerCount `json:"players"`
	Examples     []Evidence         `json:"examples"`
}

// ScoutAWP is where the team's AWPs get their kills.
type ScoutAWP struct {
	Sides []ScoutAWPSide `json:"sides"`
	Rows  []ScoutAWPRow  `json:"rows"`
}

// ScoutOpening is one player's opening duels on one side.
type ScoutOpening struct {
	SteamID64    string           `json:"steamid64"`
	Name         string           `json:"name"`
	Side         Side             `json:"side"`
	Rounds       int              `json:"rounds"`
	Attempts     Rate             `json:"attempts"`
	Won          Rate             `json:"won"`
	TradedOnLoss Rate             `json:"traded_on_loss"`
	Places       []ScoutPlaceUsed `json:"places"`
	Examples     []Evidence       `json:"examples"`
}

// ScoutPlaceUsed is how many duels were fought at one place.
type ScoutPlaceUsed struct {
	Place string `json:"place"`
	Count int    `json:"count"`
}

// ScoutReport is the anti-strat reading of one team over several matches.
// It is a pure function of its input: the same matches give the same report.
type ScoutReport struct {
	Filter     ScoutFilter         `json:"filter"`
	Thresholds ScoutThresholds     `json:"thresholds"`
	Matches    []ScoutMatchSummary `json:"matches"`
	Maps       []ScoutMapSummary   `json:"maps"`
	Tendencies Tendencies          `json:"tendencies"`
	Pistols    ScoutPistols        `json:"pistols"`
	TDefaults  ScoutDefaults       `json:"t_defaults"`
	CTSetups   ScoutSetups         `json:"ct_setups"`
	Executes   []ScoutExecute      `json:"executes"`
	Economy    ScoutEconomy        `json:"economy"`
	AWP        ScoutAWP            `json:"awp"`
	Openings   []ScoutOpening      `json:"openings"`
}

// Scout builds the scouting report of a team over its matches.
func Scout(matches []ScoutMatch, f ScoutFilter) ScoutReport {
	selected := f.Select(matches)
	report := ScoutReport{
		Filter: f,
		Thresholds: ScoutThresholds{
			MinReliableSample: MinReliableSample, ExampleLimit: ScoutExampleLimit,
			EarlyUtilitySeconds: ScoutEarlyUtilitySeconds, ExecuteWindowSeconds: ScoutExecuteWindowSeconds,
			SetupSeconds: ScoutSetupSeconds, UtilityRowLimit: ScoutUtilityRowLimit,
		},
		Matches:    make([]ScoutMatchSummary, 0, len(selected)),
		Tendencies: AggregateMatches(selected, f),
	}
	maps := newMapRecords()
	pistols := newPatternGroups()
	defaults := newSectionAcc()
	setups := newSectionAcc()
	executes := map[Site]*executeAcc{}
	economy := newEconomyAcc()
	awp := newAWPAcc()
	openings := newOpeningAcc()

	for _, m := range selected {
		doc := m.Document
		summary := matchSummary(m)
		for _, r := range f.rounds(m) {
			side := doc.TeamSide(m.TeamKey, r)
			won := r.Winner == side
			summary.Rounds++
			if won {
				summary.Wins++
			}
			maps.addRound(doc.Demo.Map, side, won)
			team := teamSlots(r, side)
			start := evidence(m, r, 0)

			if r.Class.HasTag(TagPistol) {
				pistols.add(side, sidePattern(r, side), r.Class.Site, won, start)
			}
			if !r.Class.HasTag(TagPistol) && r.Economy.Buy(side) == BuyFull {
				switch side {
				case SideT:
					defaults.add(m, r, side, won, team)
				case SideCT:
					setups.add(m, r, side, won, team)
					setups.addSetup(m, r, team)
				}
			}
			if side == SideT && (r.Class.TSide == TExecute || r.Class.TSide == TSplit) &&
				(r.Class.Site == SiteA || r.Class.Site == SiteB) {
				acc, ok := executes[r.Class.Site]
				if !ok {
					acc = newExecuteAcc()
					executes[r.Class.Site] = acc
				}
				acc.add(m, r, won, team)
			}
			awp.add(m, r, side, team)
			openings.add(m, r, side, team)
		}
		economy.addMatch(m, f)
		maps.addMatch(doc.Demo.Map, summary.Score > summary.OpponentScore)
		report.Matches = append(report.Matches, summary)
	}

	report.Maps = maps.finish()
	report.Pistols = ScoutPistols{Sides: pistols.sides(), Rows: pistols.rows()}
	report.TDefaults = defaults.defaults()
	report.CTSetups = setups.setups()
	report.Executes = finishExecutes(executes)
	report.Economy = economy.finish()
	report.AWP = awp.finish()
	report.Openings = openings.finish()
	return report
}

func matchSummary(m ScoutMatch) ScoutMatchSummary {
	s := ScoutMatchSummary{
		MatchID: m.MatchID, Map: m.Document.Demo.Map, PlayedAt: m.PlayedAt, TeamKey: m.TeamKey,
		OpponentID: m.OpponentID, Opponent: m.Opponent,
	}
	for _, r := range m.Document.Rounds {
		side := m.Document.TeamSide(m.TeamKey, r)
		switch {
		case r.Winner == "" || side == "":
		case r.Winner == side:
			s.Score++
		default:
			s.OpponentScore++
		}
	}
	return s
}

func evidence(m ScoutMatch, r Round, tick int) Evidence {
	if tick <= 0 {
		tick = r.TickFreezeEnd
	}
	return Evidence{MatchID: m.MatchID, Round: r.Number, Tick: tick}
}

func appendExample(list []Evidence, e Evidence) []Evidence {
	if len(list) >= ScoutExampleLimit {
		return list
	}
	return append(list, e)
}

func sidePattern(r Round, side Side) string {
	if side == SideCT {
		return string(r.Class.CTSide)
	}
	return string(r.Class.TSide)
}

// teamSlots is the team's slots in a round, read from the round's own player
// list so a mid-match slot change cannot misattribute an event.
func teamSlots(r Round, side Side) map[uint8]bool {
	slots := map[uint8]bool{}
	for _, pr := range r.Players {
		if pr.Side == side {
			slots[pr.Slot] = true
		}
	}
	return slots
}

func secondsSince(r Round, tick int, tickrate float64) (float64, bool) {
	if tickrate <= 0 || r.TickFreezeEnd <= 0 || tick < r.TickFreezeEnd {
		return 0, false
	}
	return float64(tick-r.TickFreezeEnd) / tickrate, true
}

func isScoutUtility(kind EventKind) bool {
	switch kind {
	case EventSmoke, EventFlash, EventHE, EventMolotov:
		return true
	default:
		return false
	}
}

var utilityOrder = map[EventKind]int{EventSmoke: 0, EventFlash: 1, EventMolotov: 2, EventHE: 3}

// NearestCallout names the place CS2 reported most in the grid cell of a world
// position; off the grid, the callout whose centre of play is closest on the
// same level, or "" when the geometry has no callouts.
func NearestCallout(g MapGeometry, pos [3]float64) string {
	level := g.Calibration.Level(pos[2])
	if name := calloutAt(g, level, pos[0], pos[1], pos[2]); name != "" {
		return name
	}
	for _, sameLevel := range []bool{true, false} {
		best, bestDist := "", math.Inf(1)
		for _, c := range g.Callouts {
			if sameLevel && c.Level != level {
				continue
			}
			dx, dy := c.Center[0]-pos[0], c.Center[1]-pos[1]
			d := dx*dx + dy*dy
			if d < bestDist || (d == bestDist && c.Name < best) {
				best, bestDist = c.Name, d
			}
		}
		if best != "" {
			return best
		}
	}
	return ""
}

func playerIdentity(d Document, slot uint8) (string, string) {
	p, ok := d.PlayerBySlot(slot)
	if !ok {
		return fmt.Sprintf("slot:%d", slot), ""
	}
	if p.SteamID64 == "" {
		return fmt.Sprintf("slot:%d", slot), p.Name
	}
	return p.SteamID64, p.Name
}

// playerCounts accumulates per-player counts; the first name seen wins, which
// is the most recent match's name because matches arrive newest first.
type playerCounts struct {
	counts map[string]int
	names  map[string]string
}

func newPlayerCounts() *playerCounts {
	return &playerCounts{counts: map[string]int{}, names: map[string]string{}}
}

func (p *playerCounts) add(d Document, slot uint8) {
	id, name := playerIdentity(d, slot)
	p.counts[id]++
	if _, ok := p.names[id]; !ok {
		p.names[id] = name
	}
}

func (p *playerCounts) list() []ScoutPlayerCount {
	out := make([]ScoutPlayerCount, 0, len(p.counts))
	for id, n := range p.counts {
		out = append(out, ScoutPlayerCount{SteamID64: id, Name: p.names[id], Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].SteamID64 < out[j].SteamID64
	})
	return out
}

// roundGroup counts a set of rounds and keeps the first examples.
type roundGroup struct {
	rounds, wins int
	examples     []Evidence
}

func (g *roundGroup) add(won bool, e Evidence) {
	g.rounds++
	if won {
		g.wins++
	}
	g.examples = appendExample(g.examples, e)
}

func (g *roundGroup) result(total int) ScoutRounds {
	examples := g.examples
	if examples == nil {
		examples = []Evidence{}
	}
	return ScoutRounds{Rounds: g.rounds, Share: newRate(g.rounds, total), WinRate: newRate(g.wins, g.rounds), Examples: examples}
}

type patternKey struct {
	side    Side
	pattern string
	site    Site
}

type patternGroups struct {
	groups map[patternKey]*roundGroup
	bySide map[Side]*roundGroup
}

func newPatternGroups() *patternGroups {
	return &patternGroups{groups: map[patternKey]*roundGroup{}, bySide: map[Side]*roundGroup{}}
}

func (p *patternGroups) add(side Side, pattern string, site Site, won bool, e Evidence) {
	key := patternKey{side: side, pattern: pattern, site: site}
	g, ok := p.groups[key]
	if !ok {
		g = &roundGroup{}
		p.groups[key] = g
	}
	g.add(won, e)
	s, ok := p.bySide[side]
	if !ok {
		s = &roundGroup{}
		p.bySide[side] = s
	}
	s.add(won, e)
}

func (p *patternGroups) sides() []ScoutSide {
	total := 0
	for _, g := range p.bySide {
		total += g.rounds
	}
	out := []ScoutSide{}
	for _, side := range []Side{SideT, SideCT} {
		if g, ok := p.bySide[side]; ok {
			out = append(out, ScoutSide{Side: side, ScoutRounds: g.result(total)})
		}
	}
	return out
}

func (p *patternGroups) rows() []ScoutPatternRow {
	out := make([]ScoutPatternRow, 0, len(p.groups))
	for key, g := range p.groups {
		total := 0
		if s, ok := p.bySide[key.side]; ok {
			total = s.rounds
		}
		out = append(out, ScoutPatternRow{Side: key.side, Pattern: key.pattern, Site: key.site, ScoutRounds: g.result(total)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Side != out[j].Side {
			return out[i].Side == SideT
		}
		if out[i].Rounds != out[j].Rounds {
			return out[i].Rounds > out[j].Rounds
		}
		if out[i].Pattern != out[j].Pattern {
			return out[i].Pattern < out[j].Pattern
		}
		return out[i].Site < out[j].Site
	})
	return out
}

type utilityKey struct {
	kind    EventKind
	callout string
}

type utilityAcc struct {
	rounds   int
	count    int
	seconds  []float64
	throwers *playerCounts
	examples []Evidence
}

// utilityTable counts grenades per kind and landing callout, once per round
// for the share and once per grenade for the count.
type utilityTable struct {
	rows map[utilityKey]*utilityAcc
}

func newUtilityTable() *utilityTable {
	return &utilityTable{rows: map[utilityKey]*utilityAcc{}}
}

// addRound records the team's grenades of one round that landed inside
// [fromTick, toTick].
func (u *utilityTable) addRound(m ScoutMatch, r Round, team map[uint8]bool, fromTick, toTick int) map[EventKind]int {
	seen := map[utilityKey]bool{}
	kinds := map[EventKind]int{}
	for _, ev := range r.Events {
		if !isScoutUtility(ev.Kind) || ev.ActorSlot == nil || !team[*ev.ActorSlot] {
			continue
		}
		if ev.Tick < fromTick || ev.Tick > toTick {
			continue
		}
		kinds[ev.Kind]++
		key := utilityKey{kind: ev.Kind, callout: NearestCallout(m.Document.Geometry, ev.Pos)}
		acc, ok := u.rows[key]
		if !ok {
			acc = &utilityAcc{throwers: newPlayerCounts()}
			u.rows[key] = acc
		}
		acc.count++
		acc.throwers.add(m.Document, *ev.ActorSlot)
		if secs, ok := secondsSince(r, ev.Tick, m.Document.Demo.Tickrate); ok {
			acc.seconds = append(acc.seconds, secs)
		}
		if !seen[key] {
			seen[key] = true
			acc.rounds++
			acc.examples = appendExample(acc.examples, evidence(m, r, ev.Tick))
		}
	}
	return kinds
}

func (u *utilityTable) result(total int) []ScoutUtilityRow {
	out := make([]ScoutUtilityRow, 0, len(u.rows))
	for key, acc := range u.rows {
		out = append(out, ScoutUtilityRow{
			Kind: key.kind, Callout: key.callout, Rounds: acc.rounds, Share: newRate(acc.rounds, total),
			Count: acc.count, MedianSeconds: median(acc.seconds), Throwers: acc.throwers.list(), Examples: acc.examples,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rounds != out[j].Rounds {
			return out[i].Rounds > out[j].Rounds
		}
		if out[i].Kind != out[j].Kind {
			return utilityOrder[out[i].Kind] < utilityOrder[out[j].Kind]
		}
		return out[i].Callout < out[j].Callout
	})
	if len(out) > ScoutUtilityRowLimit {
		out = out[:ScoutUtilityRowLimit]
	}
	return out
}

// sectionAcc is a set of full-buy rounds on one side: the T defaults or the
// CT setups.
type sectionAcc struct {
	all          roundGroup
	patterns     *patternGroups
	utility      *utilityTable
	firstContact []float64
	places       map[string]*placeAcc
	sampled      int
	positions    map[string]*positionAcc
}

type placeAcc struct {
	rounds, won int
	examples    []Evidence
}

type positionAcc struct {
	name   string
	rounds int
	spots  map[string]*roundGroup
}

func newSectionAcc() *sectionAcc {
	return &sectionAcc{
		patterns: newPatternGroups(), utility: newUtilityTable(),
		places: map[string]*placeAcc{}, positions: map[string]*positionAcc{},
	}
}

func (s *sectionAcc) add(m ScoutMatch, r Round, side Side, won bool, team map[uint8]bool) {
	doc := m.Document
	start := evidence(m, r, 0)
	s.all.add(won, start)
	s.patterns.add(side, sidePattern(r, side), r.Class.Site, won, start)
	if doc.Demo.Tickrate > 0 {
		s.utility.addRound(m, r, team, r.TickFreezeEnd, r.TickFreezeEnd+int(ScoutEarlyUtilitySeconds*doc.Demo.Tickrate))
	}
	if secs, ok := secondsSince(r, r.Class.FirstContactTick, doc.Demo.Tickrate); ok && r.Class.FirstContactTick > 0 {
		s.firstContact = append(s.firstContact, secs)
	}
	ev, ok := openingEvent(r)
	place := ""
	if ok {
		place = duelPlace(doc, ev, ev.ActorSlot != nil && team[*ev.ActorSlot])
	}
	if place != "" {
		acc, found := s.places[place]
		if !found {
			acc = &placeAcc{}
			s.places[place] = acc
		}
		acc.rounds++
		if r.Class.OpeningSide == side {
			acc.won++
		}
		acc.examples = appendExample(acc.examples, evidence(m, r, ev.Tick))
	}
}

func (s *sectionAcc) addSetup(m ScoutMatch, r Round, team map[uint8]bool) {
	frame, ok := m.Setup[r.Number]
	if !ok {
		return
	}
	s.sampled++
	for _, sample := range frame.Samples {
		if !team[sample.Slot] || !sample.Flags.Has(FlagAlive) {
			continue
		}
		callout := NearestCallout(m.Document.Geometry, [3]float64{sample.X, sample.Y, sample.Z})
		if callout == "" {
			continue
		}
		id, name := playerIdentity(m.Document, sample.Slot)
		acc, found := s.positions[id]
		if !found {
			acc = &positionAcc{name: name, spots: map[string]*roundGroup{}}
			s.positions[id] = acc
		}
		acc.rounds++
		spot, found := acc.spots[callout]
		if !found {
			spot = &roundGroup{}
			acc.spots[callout] = spot
		}
		spot.add(false, evidence(m, r, frame.Tick))
	}
}

func (s *sectionAcc) base() ScoutRounds {
	return s.all.result(s.all.rounds)
}

func (s *sectionAcc) defaults() ScoutDefaults {
	places := make([]ScoutPlaceRow, 0, len(s.places))
	total := 0
	for _, acc := range s.places {
		total += acc.rounds
	}
	for place, acc := range s.places {
		examples := acc.examples
		if examples == nil {
			examples = []Evidence{}
		}
		places = append(places, ScoutPlaceRow{
			Place: place, Rounds: acc.rounds, Share: newRate(acc.rounds, total), Won: newRate(acc.won, acc.rounds), Examples: examples,
		})
	}
	sort.Slice(places, func(i, j int) bool {
		if places[i].Rounds != places[j].Rounds {
			return places[i].Rounds > places[j].Rounds
		}
		return places[i].Place < places[j].Place
	})
	return ScoutDefaults{
		ScoutRounds:       s.base(),
		Patterns:          s.patterns.rows(),
		Utility:           s.utility.result(s.all.rounds),
		OpeningPlaces:     places,
		FirstContact:      newHistogram(s.firstContact),
		UtilityRoundTotal: s.all.rounds,
	}
}

func (s *sectionAcc) setups() ScoutSetups {
	positions := make([]ScoutPositionRow, 0, len(s.positions))
	for id, acc := range s.positions {
		spots := make([]ScoutSpot, 0, len(acc.spots))
		for callout, g := range acc.spots {
			spots = append(spots, ScoutSpot{Callout: callout, Share: newRate(g.rounds, acc.rounds), Examples: g.examples})
		}
		sort.Slice(spots, func(i, j int) bool {
			if spots[i].Share.Count != spots[j].Share.Count {
				return spots[i].Share.Count > spots[j].Share.Count
			}
			return spots[i].Callout < spots[j].Callout
		})
		positions = append(positions, ScoutPositionRow{SteamID64: id, Name: acc.name, Rounds: acc.rounds, Spots: spots})
	}
	sort.Slice(positions, func(i, j int) bool {
		if positions[i].Rounds != positions[j].Rounds {
			return positions[i].Rounds > positions[j].Rounds
		}
		return positions[i].SteamID64 < positions[j].SteamID64
	})
	return ScoutSetups{
		ScoutRounds: s.base(),
		Patterns:    s.patterns.rows(),
		Utility:     s.utility.result(s.all.rounds),
		Sampled:     s.sampled,
		Positions:   positions,
	}
}

// duelPlace is where the scouted team's player stood in a duel: the killer's
// place when the team won it, the victim's callout when the team lost it.
func duelPlace(doc Document, ev Event, teamKilled bool) string {
	if teamKilled {
		if ev.Place != "" {
			return ev.Place
		}
		return NearestCallout(doc.Geometry, ev.Pos)
	}
	if callout := NearestCallout(doc.Geometry, ev.TargetPos); callout != "" {
		return callout
	}
	return ev.Place
}

// openingEvent is the kill that opened the round.
func openingEvent(r Round) (Event, bool) {
	if r.Class.OpeningSide == "" {
		return Event{}, false
	}
	for _, ev := range r.Events {
		if ev.Kind == EventKill && (ev.Opening || ev.Tick == r.Class.OpeningTick) {
			return ev, true
		}
	}
	return Event{}, false
}

type executeAcc struct {
	all          roundGroup
	patterns     *patternGroups
	plants       int
	plantTime    []float64
	firstContact []float64
	utility      *utilityTable
	kindTotals   map[EventKind]int
}

func newExecuteAcc() *executeAcc {
	return &executeAcc{patterns: newPatternGroups(), utility: newUtilityTable(), kindTotals: map[EventKind]int{}}
}

func (e *executeAcc) add(m ScoutMatch, r Round, won bool, team map[uint8]bool) {
	doc := m.Document
	anchor := r.Class.FirstContactTick
	if r.Bomb != nil && r.Bomb.PlantTick > 0 {
		anchor = r.Bomb.PlantTick
		e.plants++
		if secs, ok := secondsSince(r, anchor, doc.Demo.Tickrate); ok {
			e.plantTime = append(e.plantTime, secs)
		}
	}
	if secs, ok := secondsSince(r, r.Class.FirstContactTick, doc.Demo.Tickrate); ok && r.Class.FirstContactTick > 0 {
		e.firstContact = append(e.firstContact, secs)
	}
	example := evidence(m, r, anchor)
	e.all.add(won, example)
	e.patterns.add(SideT, string(r.Class.TSide), r.Class.Site, won, example)
	if anchor <= 0 {
		anchor = r.TickEnd
	}
	if doc.Demo.Tickrate > 0 {
		from := max(anchor-int(ScoutExecuteWindowSeconds*doc.Demo.Tickrate), r.TickFreezeEnd)
		for kind, n := range e.utility.addRound(m, r, team, from, anchor) {
			e.kindTotals[kind] += n
		}
	}
}

func finishExecutes(sites map[Site]*executeAcc) []ScoutExecute {
	total := 0
	for _, acc := range sites {
		total += acc.all.rounds
	}
	out := []ScoutExecute{}
	for _, site := range []Site{SiteA, SiteB} {
		acc, ok := sites[site]
		if !ok {
			continue
		}
		mean := map[EventKind]float64{}
		for _, kind := range []EventKind{EventSmoke, EventFlash, EventMolotov, EventHE} {
			mean[kind] = math.Round(float64(acc.kindTotals[kind])/float64(acc.all.rounds)*10) / 10
		}
		out = append(out, ScoutExecute{
			Site:         site,
			ScoutRounds:  acc.all.result(total),
			Patterns:     acc.patterns.rows(),
			Plants:       newRate(acc.plants, acc.all.rounds),
			PlantTime:    newHistogram(acc.plantTime),
			FirstContact: newHistogram(acc.firstContact),
			MeanUtility:  mean,
			Utility:      acc.utility.result(acc.all.rounds),
		})
	}
	return out
}

type economyKey struct {
	context string
	side    Side
	buy     BuyType
}

type economyContextKey struct {
	context string
	side    Side
}

type economyAcc struct {
	rows     map[economyKey]*roundGroup
	contexts map[economyContextKey]*roundGroup
}

func newEconomyAcc() *economyAcc {
	return &economyAcc{rows: map[economyKey]*roundGroup{}, contexts: map[economyContextKey]*roundGroup{}}
}

// addMatch reads the buy of every round that follows a lost round of the same
// half. The first round of a half is a pistol or an overtime reset, so it never
// counts as a reaction to the round before it.
func (e *economyAcc) addMatch(m ScoutMatch, f ScoutFilter) {
	doc := m.Document
	filter := f.roundFilter(m)
	for i := 1; i < len(doc.Rounds); i++ {
		r, prev := doc.Rounds[i], doc.Rounds[i-1]
		if prev.Number != r.Number-1 || prev.Half != r.Half || r.Class.HasTag(TagPistol) {
			continue
		}
		if !filter.Match(doc, r) {
			continue
		}
		prevSide := doc.TeamSide(m.TeamKey, prev)
		if prev.Winner == "" || prevSide == "" || prev.Winner == prevSide {
			continue
		}
		context := ScoutAfterLoss
		if prev.Class.HasTag(TagPistol) {
			context = ScoutAfterPistolLoss
		}
		side := doc.TeamSide(m.TeamKey, r)
		won := r.Winner == side
		example := evidence(m, r, r.Economy.SampleTick)
		key := economyKey{context: context, side: side, buy: r.Economy.Buy(side)}
		g, ok := e.rows[key]
		if !ok {
			g = &roundGroup{}
			e.rows[key] = g
		}
		g.add(won, example)
		ck := economyContextKey{context: context, side: side}
		c, ok := e.contexts[ck]
		if !ok {
			c = &roundGroup{}
			e.contexts[ck] = c
		}
		c.add(won, example)
	}
}

var buyOrder = map[BuyType]int{BuyPistol: 0, BuyEco: 1, BuySemi: 2, BuyForce: 3, BuyFull: 4, BuyUnknown: 5}

func (e *economyAcc) finish() ScoutEconomy {
	out := ScoutEconomy{Contexts: []ScoutEconomyContext{}, Rows: []ScoutEconomyRow{}}
	for _, context := range []string{ScoutAfterPistolLoss, ScoutAfterLoss} {
		for _, side := range []Side{SideT, SideCT} {
			c, ok := e.contexts[economyContextKey{context: context, side: side}]
			if !ok {
				continue
			}
			out.Contexts = append(out.Contexts, ScoutEconomyContext{Context: context, Side: side, ScoutRounds: c.result(c.rounds)})
		}
	}
	for key, g := range e.rows {
		total := e.contexts[economyContextKey{context: key.context, side: key.side}].rounds
		out.Rows = append(out.Rows, ScoutEconomyRow{Context: key.context, Side: key.side, Buy: key.buy, ScoutRounds: g.result(total)})
	}
	contextOrder := map[string]int{ScoutAfterPistolLoss: 0, ScoutAfterLoss: 1}
	sort.Slice(out.Rows, func(i, j int) bool {
		a, b := out.Rows[i], out.Rows[j]
		if a.Context != b.Context {
			return contextOrder[a.Context] < contextOrder[b.Context]
		}
		if a.Side != b.Side {
			return a.Side == SideT
		}
		return buyOrder[a.Buy] < buyOrder[b.Buy]
	})
	return out
}

type awpRowKey struct {
	side  Side
	place string
}

type awpRowAcc struct {
	kills, openings int
	players         *playerCounts
	examples        []Evidence
}

type awpSideAcc struct {
	rounds, killRounds, kills, openings int
}

type awpAcc struct {
	rows  map[awpRowKey]*awpRowAcc
	sides map[Side]*awpSideAcc
}

func newAWPAcc() *awpAcc {
	return &awpAcc{rows: map[awpRowKey]*awpRowAcc{}, sides: map[Side]*awpSideAcc{}}
}

func (a *awpAcc) add(m ScoutMatch, r Round, side Side, team map[uint8]bool) {
	s, ok := a.sides[side]
	if !ok {
		s = &awpSideAcc{}
		a.sides[side] = s
	}
	s.rounds++
	killed := false
	for _, ev := range r.Events {
		if ev.Kind != EventKill || ev.Weapon != awpWeapon || ev.ActorSlot == nil || !team[*ev.ActorSlot] {
			continue
		}
		if ev.TargetSlot != nil && team[*ev.TargetSlot] {
			continue
		}
		killed = true
		place := ev.Place
		if place == "" {
			place = NearestCallout(m.Document.Geometry, ev.Pos)
		}
		key := awpRowKey{side: side, place: place}
		acc, found := a.rows[key]
		if !found {
			acc = &awpRowAcc{players: newPlayerCounts()}
			a.rows[key] = acc
		}
		acc.kills++
		s.kills++
		if ev.Opening {
			acc.openings++
			s.openings++
		}
		acc.players.add(m.Document, *ev.ActorSlot)
		acc.examples = appendExample(acc.examples, evidence(m, r, ev.Tick))
	}
	if killed {
		s.killRounds++
	}
}

func (a *awpAcc) finish() ScoutAWP {
	out := ScoutAWP{Sides: []ScoutAWPSide{}, Rows: []ScoutAWPRow{}}
	for _, side := range []Side{SideT, SideCT} {
		s, ok := a.sides[side]
		if !ok {
			continue
		}
		out.Sides = append(out.Sides, ScoutAWPSide{
			Side: side, Rounds: s.rounds, KillRounds: newRate(s.killRounds, s.rounds), Kills: s.kills, OpeningKills: s.openings,
		})
	}
	for key, acc := range a.rows {
		out.Rows = append(out.Rows, ScoutAWPRow{
			Side: key.side, Place: key.place, Kills: acc.kills, Share: newRate(acc.kills, a.sides[key.side].kills),
			OpeningKills: acc.openings, Players: acc.players.list(), Examples: acc.examples,
		})
	}
	sort.Slice(out.Rows, func(i, j int) bool {
		a, b := out.Rows[i], out.Rows[j]
		if a.Side != b.Side {
			return a.Side == SideT
		}
		if a.Kills != b.Kills {
			return a.Kills > b.Kills
		}
		return a.Place < b.Place
	})
	return out
}

type openingKey struct {
	id   string
	side Side
}

type openingPlayerAcc struct {
	name                        string
	rounds, attempts, won, lost int
	tradedOnLoss                int
	places                      map[string]int
	examples                    []Evidence
}

type openingAcc struct {
	players map[openingKey]*openingPlayerAcc
}

func newOpeningAcc() *openingAcc {
	return &openingAcc{players: map[openingKey]*openingPlayerAcc{}}
}

func (o *openingAcc) add(m ScoutMatch, r Round, side Side, team map[uint8]bool) {
	doc := m.Document
	ev, hasEvent := openingEvent(r)
	for _, pr := range r.Players {
		if !team[pr.Slot] {
			continue
		}
		id, name := playerIdentity(doc, pr.Slot)
		key := openingKey{id: id, side: side}
		acc, ok := o.players[key]
		if !ok {
			acc = &openingPlayerAcc{name: name, places: map[string]int{}}
			o.players[key] = acc
		}
		acc.rounds++
		if !pr.OpeningKill && !pr.OpeningDeath {
			continue
		}
		acc.attempts++
		if pr.OpeningKill {
			acc.won++
		} else {
			acc.lost++
			if pr.Traded {
				acc.tradedOnLoss++
			}
		}
		if hasEvent {
			if place := duelPlace(doc, ev, pr.OpeningKill); place != "" {
				acc.places[place]++
			}
		}
		acc.examples = appendExample(acc.examples, evidence(m, r, r.Class.OpeningTick))
	}
}

func (o *openingAcc) finish() []ScoutOpening {
	out := make([]ScoutOpening, 0, len(o.players))
	for key, acc := range o.players {
		places := make([]ScoutPlaceUsed, 0, len(acc.places))
		for place, n := range acc.places {
			places = append(places, ScoutPlaceUsed{Place: place, Count: n})
		}
		sort.Slice(places, func(i, j int) bool {
			if places[i].Count != places[j].Count {
				return places[i].Count > places[j].Count
			}
			return places[i].Place < places[j].Place
		})
		examples := acc.examples
		if examples == nil {
			examples = []Evidence{}
		}
		out = append(out, ScoutOpening{
			SteamID64: key.id, Name: acc.name, Side: key.side, Rounds: acc.rounds,
			Attempts: newRate(acc.attempts, acc.rounds), Won: newRate(acc.won, acc.attempts),
			TradedOnLoss: newRate(acc.tradedOnLoss, acc.lost), Places: places, Examples: examples,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Side != b.Side {
			return a.Side == SideT
		}
		if a.Attempts.Count != b.Attempts.Count {
			return a.Attempts.Count > b.Attempts.Count
		}
		return a.SteamID64 < b.SteamID64
	})
	return out
}

type mapRecord struct {
	matches, matchWins int
	rounds, wins       int
	ctRounds, ctWins   int
	tRounds, tWins     int
}

type mapRecords struct {
	maps map[string]*mapRecord
}

func newMapRecords() *mapRecords {
	return &mapRecords{maps: map[string]*mapRecord{}}
}

func (m *mapRecords) get(name string) *mapRecord {
	rec, ok := m.maps[name]
	if !ok {
		rec = &mapRecord{}
		m.maps[name] = rec
	}
	return rec
}

func (m *mapRecords) addRound(name string, side Side, won bool) {
	rec := m.get(name)
	rec.rounds++
	if won {
		rec.wins++
	}
	switch side {
	case SideCT:
		rec.ctRounds++
		if won {
			rec.ctWins++
		}
	case SideT:
		rec.tRounds++
		if won {
			rec.tWins++
		}
	}
}

func (m *mapRecords) addMatch(name string, won bool) {
	rec := m.get(name)
	rec.matches++
	if won {
		rec.matchWins++
	}
}

func (m *mapRecords) finish() []ScoutMapSummary {
	out := make([]ScoutMapSummary, 0, len(m.maps))
	for name, rec := range m.maps {
		out = append(out, ScoutMapSummary{
			Map: name, Matches: rec.matches, MatchWins: rec.matchWins, Rounds: rec.rounds,
			WinRate: newRate(rec.wins, rec.rounds), CTWinRate: newRate(rec.ctWins, rec.ctRounds), TWinRate: newRate(rec.tWins, rec.tRounds),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Matches != out[j].Matches {
			return out[i].Matches > out[j].Matches
		}
		return out[i].Map < out[j].Map
	})
	return out
}

// SetupFrames reads, for every round of a document, the first position frame
// at or after ScoutSetupSeconds past freeze end. A round that ended earlier,
// or has no frames, is left out.
func SetupFrames(blob []byte, doc Document) (map[int]Frame, error) {
	header, err := DecodeHeader(blob)
	if err != nil {
		return nil, err
	}
	out := map[int]Frame{}
	if doc.Demo.Tickrate <= 0 {
		return out, nil
	}
	for _, r := range doc.Rounds {
		offset, ok := doc.Positions.Offset(r.Number)
		if !ok || offset.FrameCount == 0 {
			continue
		}
		frames, err := DecodeFrames(blob, offset.ByteOffset, offset.FrameCount, header)
		if err != nil {
			return nil, fmt.Errorf("round %d: %w", r.Number, err)
		}
		target := r.TickFreezeEnd + int(ScoutSetupSeconds*doc.Demo.Tickrate)
		for _, frame := range frames {
			if frame.Tick < target {
				continue
			}
			if r.TickEnd > 0 && frame.Tick > r.TickEnd {
				break
			}
			out[r.Number] = frame
			break
		}
	}
	return out, nil
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// median is the middle sample, rounded to a tenth; 0 for no samples.
func median(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	value := sorted[mid]
	if len(sorted)%2 == 0 {
		value = (sorted[mid-1] + sorted[mid]) / 2
	}
	return math.Round(value*10) / 10
}
