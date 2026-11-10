package tacticalplan

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

// Pattern finder limits and the rules its trend reads.
const (
	PatternMaxBlocks  = 6
	PatternMaxPlayers = 10
	// PatternMaxSeconds bounds a block's clock window: a round, its bomb and overtime.
	PatternMaxSeconds = 300
	// PatternRecentDays is the recent window the trend compares with everything before it.
	PatternRecentDays = 30
	// PatternTrendMargin is how far the recent share must move from the earlier one to count as a trend.
	PatternTrendMargin = 0.05
	// PatternTrendMinRounds is the rounds each side of the cut needs before a trend is read.
	PatternTrendMinRounds = 10
)

// PatternBlockKind is what a block looks for in a round.
type PatternBlockKind string

// Block kinds: a player who passes through an area, a grenade that lands in
// (or is thrown from) an area, and a plant in an area.
const (
	PatternPlayer  PatternBlockKind = "player"
	PatternUtility PatternBlockKind = "utility"
	PatternPlant   PatternBlockKind = "plant"
)

// Pattern trends: the recent share of rounds against the earlier one.
const (
	TrendUp   = "up"
	TrendDown = "down"
	TrendFlat = "flat"
)

// PatternArea is a box on the map in world units. Level keeps it to one floor
// of a two-floor map; empty reads both.
type PatternArea struct {
	MinX  float64 `json:"min_x"`
	MinY  float64 `json:"min_y"`
	MaxX  float64 `json:"max_x"`
	MaxY  float64 `json:"max_y"`
	Level string  `json:"level,omitempty"`
}

// Contains reports whether a world position is inside the box and on its floor.
func (a PatternArea) Contains(cal radarmap.Calibration, pos [3]float64) bool {
	if pos[0] < a.MinX || pos[0] > a.MaxX || pos[1] < a.MinY || pos[1] > a.MaxY {
		return false
	}
	return a.Level == "" || !cal.MultiLevel() || cal.Level(pos[2]) == a.Level
}

// PatternBlock is one condition of a round. Players are SteamID64s (empty is anyone of the side);
// From and To are seconds after freeze end, To 0 the end of the round.
type PatternBlock struct {
	Kind    PatternBlockKind `json:"kind"`
	Players []string         `json:"players,omitempty"`
	// Rival reads the other team's players.
	Rival   bool      `json:"rival,omitempty"`
	Grenade EventKind `json:"grenade,omitempty"`
	// FromThrow places a grenade where it was thrown instead of where it landed.
	FromThrow bool         `json:"from_throw,omitempty"`
	Area      *PatternArea `json:"area,omitempty"`
	From      float64      `json:"from"`
	To        float64      `json:"to,omitempty"`
	// Invert keeps the rounds in which the block does not hold.
	Invert bool `json:"invert,omitempty"`
}

// Pattern is a set of blocks that must all hold in a round, read from one side.
type Pattern struct {
	Side   Side           `json:"side,omitempty"`
	Blocks []PatternBlock `json:"blocks"`
}

func validSteamID64(id string) bool {
	if len(id) != 17 {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Check validates a pattern the way the API and the store accept it.
func (p Pattern) Check() error {
	if p.Side != "" && p.Side != SideCT && p.Side != SideT {
		return errors.New("side must be CT or T")
	}
	if len(p.Blocks) == 0 || len(p.Blocks) > PatternMaxBlocks {
		return fmt.Errorf("a pattern takes 1 to %d blocks", PatternMaxBlocks)
	}
	for i, b := range p.Blocks {
		if err := b.check(); err != nil {
			return fmt.Errorf("block %d: %w", i+1, err)
		}
	}
	return nil
}

func (b PatternBlock) check() error {
	switch b.Kind {
	case PatternPlayer, PatternUtility, PatternPlant:
	default:
		return errors.New("kind must be player, utility or plant")
	}
	if len(b.Players) > PatternMaxPlayers {
		return fmt.Errorf("at most %d players", PatternMaxPlayers)
	}
	for _, id := range b.Players {
		if !validSteamID64(id) {
			return errors.New("players must be SteamID64s")
		}
	}
	if b.Grenade != "" && (b.Kind != PatternUtility || !slices.Contains(GrenadeKinds, b.Grenade)) {
		return errors.New("grenade needs kind utility and one of smoke, flash, he, molotov or decoy")
	}
	if b.FromThrow && b.Kind != PatternUtility {
		return errors.New("from_throw only applies to utility")
	}
	if a := b.Area; a != nil {
		if a.MinX >= a.MaxX || a.MinY >= a.MaxY || math.IsNaN(a.MinX+a.MinY+a.MaxX+a.MaxY) {
			return errors.New("area needs min_x below max_x and min_y below max_y")
		}
		if a.Level != "" && a.Level != radarmap.LevelDefault && a.Level != radarmap.LevelLower {
			return errors.New("area level must be default or lower")
		}
	}
	if b.From < 0 || b.To < 0 || b.From > PatternMaxSeconds || b.To > PatternMaxSeconds {
		return fmt.Errorf("from and to must be from 0 to %d seconds", PatternMaxSeconds)
	}
	if b.To > 0 && b.To < b.From {
		return errors.New("to must not be before from")
	}
	return nil
}

func (b PatternBlock) inWindow(seconds float64) bool {
	return seconds >= b.From && (b.To <= 0 || seconds <= b.To)
}

func (b PatternBlock) inArea(cal radarmap.Calibration, pos [3]float64) bool {
	return b.Area == nil || b.Area.Contains(cal, pos)
}

// needsFrames reports whether the block reads positions rather than events.
func (b PatternBlock) needsFrames() bool { return b.Kind == PatternPlayer }

// timed is whether the block's first moment means something: a player block without an area
// holds from the first frame of every round it plays.
func (b PatternBlock) timed() bool { return b.Kind != PatternPlayer || b.Area != nil }

// PatternInput is one match a pattern reads, with what its result lists names.
type PatternInput struct {
	Match    SpatialMatch
	PlayedAt time.Time
	Team     string
	Opponent string
}

// PatternHit is one block's answer in a round. Seconds is when it first held,
// after freeze end; an inverted block or one that never held has none.
type PatternHit struct {
	Matched bool     `json:"matched"`
	Seconds *float64 `json:"seconds,omitempty"`
}

// PatternRound is a round in which every block held.
type PatternRound struct {
	Round       int          `json:"round"`
	Side        Side         `json:"side"`
	Won         bool         `json:"won"`
	Buy         BuyType      `json:"buy,omitempty"`
	OpponentBuy BuyType      `json:"opponent_buy,omitempty"`
	Hits        []PatternHit `json:"hits"`
	// Seconds and Tick are when the first timed block first held: when the player enters
	// the area, the grenade leaves the hand or the bomb goes down. A player block without an area has none.
	Seconds *float64 `json:"seconds,omitempty"`
	Tick    int      `json:"tick,omitempty"`
	// PlantSeconds is when the bomb went down, so a moment past it reads as time after the plant.
	PlantSeconds *float64 `json:"plant_seconds,omitempty"`
}

// PatternMatch is one match of the result, with its rounds in scope and the matched ones.
type PatternMatch struct {
	MatchID  uuid.UUID      `json:"match_id"`
	PlayedAt time.Time      `json:"played_at"`
	TeamKey  string         `json:"team_key,omitempty"`
	Team     string         `json:"team,omitempty"`
	Opponent string         `json:"opponent,omitempty"`
	Rounds   int            `json:"rounds"`
	Matched  []PatternRound `json:"matched"`
}

// PatternCount is how many of the rounds in scope one block keeps on its own.
type PatternCount struct {
	Matched int `json:"matched"`
	Rounds  int `json:"rounds"`
}

// PatternMonth is one calendar month (UTC) of the trend.
type PatternMonth struct {
	Month   string `json:"month"`
	Rounds  int    `json:"rounds"`
	Matched int    `json:"matched"`
}

// PatternResult is what a pattern found over the rounds in scope.
type PatternResult struct {
	Rounds  int            `json:"rounds"`
	Matched int            `json:"matched"`
	Won     int            `json:"won"`
	WinRate float64        `json:"win_rate"`
	Blocks  []PatternCount `json:"blocks"`
	// MissingPositions counts the matches without saved positions; a player block leaves them out of scope.
	MissingPositions int `json:"missing_positions"`
	// Unreadable counts the matches whose analysis or positions could not be read.
	Unreadable int `json:"unreadable_matches,omitempty"`
	// NoThrow counts the grenades a throw-position block could not place, for lack of throw data.
	NoThrow int `json:"grenades_without_throw,omitempty"`
	// CappedAt is set when a library run read only the newest CappedAt matches of the map.
	CappedAt int `json:"capped_at,omitempty"`
	// RecentRounds and RecentMatched cover the last PatternRecentDays; Trend
	// compares their share with the earlier rounds', empty when either is missing.
	RecentRounds  int            `json:"recent_rounds"`
	RecentMatched int            `json:"recent_matched"`
	Trend         string         `json:"trend"`
	Months        []PatternMonth `json:"months"`
	Matches       []PatternMatch `json:"matches"`
}

// patternRound is one round as the blocks read it.
type patternRound struct {
	m      SpatialMatch
	r      Round
	frames []Frame
}

// blockSlots is who a block reads in a round: the side the filter looks from
// (the other one for Rival), narrowed to the block's players.
func blockSlots(in patternRound, b PatternBlock) map[uint8]bool {
	side := in.m.Filter.Perspective(in.m.Document, in.r)
	if b.Rival {
		side = side.Opponent()
	}
	out := map[uint8]bool{}
	for _, pr := range in.r.Players {
		if side != "" && pr.Side != side {
			continue
		}
		if len(b.Players) > 0 {
			p, ok := in.m.Document.PlayerBySlot(pr.Slot)
			if !ok || !slices.Contains(b.Players, p.SteamID64) {
				continue
			}
		}
		out[pr.Slot] = true
	}
	return out
}

// blockAnswer is one block in one round before inversion. Out marks a round the
// block cannot speak for: none of its named players was on the side.
type blockAnswer struct {
	held, out bool
	seconds   float64
	tick      int
	noThrow   int
}

func evalBlock(in patternRound, b PatternBlock) blockAnswer {
	doc := in.m.Document
	cal, tickrate := doc.Geometry.Calibration, doc.Demo.Tickrate
	slots := blockSlots(in, b)
	if len(slots) == 0 {
		return blockAnswer{out: len(b.Players) > 0}
	}
	if b.Kind == PatternPlayer {
		end := roundEndTick(in.r)
		for _, frame := range in.frames {
			if end > 0 && frame.Tick > end {
				break
			}
			seconds, ok := secondsSince(in.r, frame.Tick, tickrate)
			if !ok || !b.inWindow(seconds) {
				continue
			}
			for _, s := range frame.Samples {
				if slots[s.Slot] && s.Flags.Has(FlagAlive) && b.inArea(cal, [3]float64{s.X, s.Y, s.Z}) {
					return blockAnswer{held: true, seconds: seconds, tick: frame.Tick}
				}
			}
		}
		return blockAnswer{}
	}
	var a blockAnswer
	for _, ev := range in.r.Events {
		if ev.ActorSlot == nil || !slots[*ev.ActorSlot] {
			continue
		}
		tick, pos := ev.Tick, ev.Pos
		switch b.Kind {
		case PatternPlant:
			if ev.Kind != EventPlant {
				continue
			}
		case PatternUtility:
			if !slices.Contains(GrenadeKinds, ev.Kind) || (b.Grenade != "" && ev.Kind != b.Grenade) {
				continue
			}
			if ev.Throw != nil {
				tick = ev.Throw.Tick
			}
			if b.FromThrow {
				if ev.Throw == nil {
					a.noThrow++
					continue
				}
				pos = ev.Throw.Pos
			}
		}
		seconds, ok := secondsSince(in.r, tick, tickrate)
		if ok && b.inWindow(seconds) && b.inArea(cal, pos) {
			a.held, a.seconds, a.tick = true, seconds, tick
			return a
		}
	}
	return a
}

// roundVerdict is a round against a whole pattern.
type roundVerdict struct {
	row     PatternRound
	out     bool
	matched bool
	noThrow int
}

// matchRound answers every block of a pattern in a round. The round matches when all hold.
func matchRound(in patternRound, p Pattern) roundVerdict {
	v := roundVerdict{row: PatternRound{Round: in.r.Number, Hits: make([]PatternHit, len(p.Blocks))}, matched: true}
	for i, b := range p.Blocks {
		a := evalBlock(in, b)
		if a.out {
			return roundVerdict{out: true}
		}
		v.noThrow += a.noThrow
		if b.Invert {
			v.row.Hits[i] = PatternHit{Matched: !a.held}
		} else {
			v.row.Hits[i] = PatternHit{Matched: a.held}
			if a.held {
				at := math.Round(a.seconds*100) / 100
				v.row.Hits[i].Seconds = &at
				if v.row.Seconds == nil && b.timed() {
					v.row.Seconds, v.row.Tick = &at, a.tick
				}
			}
		}
		v.matched = v.matched && v.row.Hits[i].Matched
	}
	return v
}

// playersInTeam reports whether every block that names players finds one of them
// on its team in the match; a match without them is out of the pattern's scope.
func playersInTeam(m SpatialMatch, p Pattern) bool {
	for _, b := range p.Blocks {
		if len(b.Players) == 0 {
			continue
		}
		key := m.Filter.TeamKey
		if b.Rival {
			key = ""
			for _, t := range m.Document.Teams {
				if t.Key != m.Filter.TeamKey {
					key = t.Key
				}
			}
		}
		if key == "" {
			continue
		}
		if !slices.ContainsFunc(m.Document.Players, func(pl Player) bool { return pl.TeamKey == key && slices.Contains(b.Players, pl.SteamID64) }) {
			return false
		}
	}
	return true
}

func plantSeconds(r Round, tickrate float64) *float64 {
	for _, ev := range r.Events {
		if ev.Kind != EventPlant {
			continue
		}
		if s, ok := secondsSince(r, ev.Tick, tickrate); ok {
			at := math.Round(s*100) / 100
			return &at
		}
	}
	return nil
}

type patternAcc struct {
	result PatternResult
	months map[string]*PatternMonth
	recent struct{ earlierRounds, earlierMatched int }
}

// PatternRunner reads patterns over matches added one at a time, so a caller
// can drop each match's document and positions before reading the next.
type PatternRunner struct {
	patterns   []Pattern
	needs      []bool
	accs       []*patternAcc
	recentFrom time.Time
}

// NewPatternRunner starts a run; now anchors the recent window of the trend.
func NewPatternRunner(patterns []Pattern, now time.Time) *PatternRunner {
	r := &PatternRunner{patterns: patterns, needs: make([]bool, len(patterns)), accs: make([]*patternAcc, len(patterns)),
		recentFrom: now.AddDate(0, 0, -PatternRecentDays)}
	for i, p := range patterns {
		r.needs[i] = slices.ContainsFunc(p.Blocks, PatternBlock.needsFrames)
		r.accs[i] = &patternAcc{months: map[string]*PatternMonth{}, result: PatternResult{
			Blocks: make([]PatternCount, len(p.Blocks)), Months: []PatternMonth{}, Matches: []PatternMatch{},
		}}
	}
	return r
}

// Unreadable counts a match the caller could not read.
func (r *PatternRunner) Unreadable() {
	for _, acc := range r.accs {
		acc.result.Unreadable++
	}
}

// Add reads one match, newest first: the order of the result's matches.
func (r *PatternRunner) Add(in PatternInput) {
	m := in.Match
	scoped := make([]bool, len(r.patterns))
	frames := false
	for i, p := range r.patterns {
		scoped[i] = playersInTeam(m, p)
		frames = frames || (scoped[i] && r.needs[i])
	}
	if !slices.Contains(scoped, true) {
		return
	}
	rounds := m.Filter.Apply(m.Document)
	byRound := map[int][]Frame{}
	hasFrames := false
	if frames {
		blob, header, ok, err := m.positions()
		if err != nil {
			r.Unreadable()
			return
		}
		for _, rd := range rounds {
			if !ok {
				break
			}
			decoded, err := roundFrames(blob, header, m.Document, rd.Number)
			if err != nil {
				r.Unreadable()
				return
			}
			byRound[rd.Number] = decoded
		}
		hasFrames = ok
	}
	for i, p := range r.patterns {
		if !scoped[i] {
			continue
		}
		if r.needs[i] && !hasFrames {
			r.accs[i].result.MissingPositions++
			continue
		}
		r.addMatch(addMatch{in: in, rounds: rounds, frames: byRound, index: i, pattern: p})
	}
}

type addMatch struct {
	in      PatternInput
	rounds  []Round
	frames  map[int][]Frame
	index   int
	pattern Pattern
}

func (r *PatternRunner) addMatch(a addMatch) {
	m, acc, needs := a.in.Match, r.accs[a.index], r.needs[a.index]
	match := PatternMatch{MatchID: m.MatchID, PlayedAt: a.in.PlayedAt, TeamKey: m.Filter.TeamKey, Team: a.in.Team,
		Opponent: a.in.Opponent, Matched: []PatternRound{}}
	earlier := a.in.PlayedAt.Before(r.recentFrom)
	for _, rd := range a.rounds {
		side := m.Filter.Perspective(m.Document, rd)
		// A pattern reads one side, so the other side's rounds are not in its scope.
		if a.pattern.Side != "" && side != a.pattern.Side {
			continue
		}
		pr := patternRound{m: m, r: rd, frames: a.frames[rd.Number]}
		if needs && len(pr.frames) == 0 {
			continue
		}
		v := matchRound(pr, a.pattern)
		if v.out {
			continue
		}
		match.Rounds++
		acc.result.NoThrow += v.noThrow
		acc.month(a.in.PlayedAt, false)
		acc.count(countedRound{row: v.row, matched: v.matched, earlier: earlier})
		if !v.matched {
			continue
		}
		row := v.row
		row.Side, row.Won = side, side != "" && rd.Winner == side
		if side != "" {
			row.Buy, row.OpponentBuy = rd.Economy.Buy(side), rd.Economy.Buy(side.Opponent())
		}
		row.PlantSeconds = plantSeconds(rd, m.Document.Demo.Tickrate)
		match.Matched = append(match.Matched, row)
		if row.Won {
			acc.result.Won++
		}
		acc.month(a.in.PlayedAt, true)
	}
	if match.Rounds > 0 {
		acc.result.Matches = append(acc.result.Matches, match)
	}
}

// Finish closes the run: one result per pattern, in order.
func (r *PatternRunner) Finish() []PatternResult {
	out := make([]PatternResult, len(r.accs))
	for i, acc := range r.accs {
		out[i] = acc.finish()
	}
	return out
}

// RunPatterns reads every pattern over inputs already in memory, newest first.
func RunPatterns(inputs []PatternInput, patterns []Pattern, now time.Time) []PatternResult {
	r := NewPatternRunner(patterns, now)
	for _, in := range inputs {
		r.Add(in)
	}
	return r.Finish()
}

// countedRound is one round in a pattern's scope, as the counts read it.
type countedRound struct {
	row     PatternRound
	matched bool
	earlier bool
}

func (a *patternAcc) count(c countedRound) {
	a.result.Rounds++
	for i, hit := range c.row.Hits {
		a.result.Blocks[i].Rounds++
		if hit.Matched {
			a.result.Blocks[i].Matched++
		}
	}
	if c.earlier {
		a.recent.earlierRounds++
	} else {
		a.result.RecentRounds++
	}
	if !c.matched {
		return
	}
	a.result.Matched++
	if c.earlier {
		a.recent.earlierMatched++
	} else {
		a.result.RecentMatched++
	}
}

// month adds a round to its calendar month: matched rounds once more as matched.
func (a *patternAcc) month(at time.Time, matched bool) {
	key := at.UTC().Format("2006-01")
	m, ok := a.months[key]
	if !ok {
		m = &PatternMonth{Month: key}
		a.months[key] = m
	}
	if matched {
		m.Matched++
	} else {
		m.Rounds++
	}
}

func (a *patternAcc) finish() PatternResult {
	r := a.result
	if r.Matched > 0 {
		r.WinRate = math.Round(float64(r.Won)/float64(r.Matched)*1000) / 1000
	}
	r.Trend = PatternTrend(PatternCount{Matched: r.RecentMatched, Rounds: r.RecentRounds},
		PatternCount{Matched: a.recent.earlierMatched, Rounds: a.recent.earlierRounds})
	r.Months = monthSeries(a.months)
	return r
}

// monthSeries is every month from the first to the last with rounds, the empty ones at zero.
func monthSeries(months map[string]*PatternMonth) []PatternMonth {
	out := []PatternMonth{}
	if len(months) == 0 {
		return out
	}
	keys := make([]string, 0, len(months))
	for k := range months {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	at, err := time.Parse("2006-01", keys[0])
	if err != nil {
		return out
	}
	last := keys[len(keys)-1]
	for key := keys[0]; key <= last; key = at.Format("2006-01") {
		if m, ok := months[key]; ok {
			out = append(out, *m)
		} else {
			out = append(out, PatternMonth{Month: key})
		}
		at = at.AddDate(0, 1, 0)
	}
	return out
}

// PatternTrend compares the recent share of matched rounds with the earlier one: up or down past
// PatternTrendMargin, flat inside it, empty under PatternTrendMinRounds on either side.
func PatternTrend(recent, earlier PatternCount) string {
	if recent.Rounds < PatternTrendMinRounds || earlier.Rounds < PatternTrendMinRounds {
		return ""
	}
	delta := float64(recent.Matched)/float64(recent.Rounds) - float64(earlier.Matched)/float64(earlier.Rounds)
	switch {
	case delta > PatternTrendMargin:
		return TrendUp
	case delta < -PatternTrendMargin:
		return TrendDown
	default:
		return TrendFlat
	}
}

// OnlyMatched narrows each match to the rounds a result kept, in the inputs'
// order, so a heatmap or an overlay draws just those rounds.
func OnlyMatched(inputs []PatternInput, result PatternResult) []SpatialMatch {
	type key struct {
		id   uuid.UUID
		team string
	}
	kept := map[key]map[int]bool{}
	for _, m := range result.Matches {
		for _, r := range m.Matched {
			k := key{m.MatchID, m.TeamKey}
			if kept[k] == nil {
				kept[k] = map[int]bool{}
			}
			kept[k][r.Round] = true
		}
	}
	out := make([]SpatialMatch, 0, len(kept))
	for _, in := range inputs {
		rounds := kept[key{in.Match.MatchID, in.Match.Filter.TeamKey}]
		if len(rounds) == 0 {
			continue
		}
		m := in.Match
		m.Document.Rounds = slices.DeleteFunc(slices.Clone(m.Document.Rounds), func(r Round) bool { return !rounds[r.Number] })
		out = append(out, m)
	}
	return out
}
