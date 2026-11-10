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

// PatternBlock is one condition of a pattern. Players are SteamID64s (empty is
// anyone of the side); Rival reads the other team's players. From and To are
// seconds after freeze end, To 0 runs to the end of the round. Invert keeps
// the rounds in which the block does not hold.
type PatternBlock struct {
	Kind    PatternBlockKind `json:"kind"`
	Players []string         `json:"players,omitempty"`
	Rival   bool             `json:"rival,omitempty"`
	Grenade EventKind        `json:"grenade,omitempty"`
	// FromThrow places a grenade where it was thrown instead of where it landed.
	FromThrow bool         `json:"from_throw,omitempty"`
	Area      *PatternArea `json:"area,omitempty"`
	From      float64      `json:"from"`
	To        float64      `json:"to,omitempty"`
	Invert    bool         `json:"invert,omitempty"`
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
	// Seconds and Tick are when the first timed block first held: when the player
	// enters the area, the grenade leaves the hand or the bomb goes down.
	Seconds *float64 `json:"seconds,omitempty"`
	Tick    int      `json:"tick,omitempty"`
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
	// MissingPositions counts the matches with no position blob, where a player block never holds.
	MissingPositions int `json:"missing_positions"`
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

// evalBlock answers one block in one round, before inversion: whether it held,
// and the earliest moment and tick it did.
func evalBlock(in patternRound, b PatternBlock) (bool, float64, int) {
	doc := in.m.Document
	cal, tickrate := doc.Geometry.Calibration, doc.Demo.Tickrate
	slots := blockSlots(in, b)
	if len(slots) == 0 {
		return false, 0, 0
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
					return true, seconds, frame.Tick
				}
			}
		}
		return false, 0, 0
	}
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
					continue
				}
				pos = ev.Throw.Pos
			}
		}
		seconds, ok := secondsSince(in.r, tick, tickrate)
		if ok && b.inWindow(seconds) && b.inArea(cal, pos) {
			return true, seconds, tick
		}
	}
	return false, 0, 0
}

// matchRound answers every block of a pattern in a round. The round matches when all hold.
func matchRound(in patternRound, p Pattern) (PatternRound, bool) {
	row := PatternRound{Round: in.r.Number, Hits: make([]PatternHit, len(p.Blocks))}
	all := true
	for i, b := range p.Blocks {
		held, seconds, tick := evalBlock(in, b)
		if b.Invert {
			row.Hits[i] = PatternHit{Matched: !held}
		} else {
			row.Hits[i] = PatternHit{Matched: held}
			if held {
				at := math.Round(seconds*100) / 100
				row.Hits[i].Seconds = &at
				if row.Seconds == nil {
					row.Seconds, row.Tick = &at, tick
				}
			}
		}
		all = all && row.Hits[i].Matched
	}
	return row, all
}

func patternsNeedFrames(patterns []Pattern) bool {
	for _, p := range patterns {
		if slices.ContainsFunc(p.Blocks, PatternBlock.needsFrames) {
			return true
		}
	}
	return false
}

type patternAcc struct {
	result PatternResult
	months map[string]*PatternMonth
	recent struct{ earlierRounds, earlierMatched int }
}

// RunPatterns reads every pattern over the same rounds in one pass, so each
// round's frames are decoded once. Inputs come newest first, the order of the
// result's matches; now anchors the recent window of the trend.
func RunPatterns(inputs []PatternInput, patterns []Pattern, now time.Time) ([]PatternResult, error) {
	accs := make([]*patternAcc, len(patterns))
	for i, p := range patterns {
		accs[i] = &patternAcc{months: map[string]*PatternMonth{}, result: PatternResult{
			Blocks: make([]PatternCount, len(p.Blocks)), Months: []PatternMonth{}, Matches: []PatternMatch{},
		}}
	}
	frames := patternsNeedFrames(patterns)
	recentFrom := now.AddDate(0, 0, -PatternRecentDays)
	for _, in := range inputs {
		m := in.Match
		var blob []byte
		var header Positions
		hasBlob := false
		if frames {
			var err error
			if blob, header, hasBlob, err = m.positions(); err != nil {
				return nil, err
			}
		}
		rounds := m.Filter.Apply(m.Document)
		matches := make([]PatternMatch, len(patterns))
		for i := range patterns {
			matches[i] = PatternMatch{MatchID: m.MatchID, PlayedAt: in.PlayedAt, TeamKey: m.Filter.TeamKey, Team: in.Team,
				Opponent: in.Opponent, Matched: []PatternRound{}}
		}
		for _, r := range rounds {
			pr := patternRound{m: m, r: r}
			if hasBlob {
				decoded, err := roundFrames(blob, header, m.Document, r.Number)
				if err != nil {
					return nil, fmt.Errorf("match %s round %d: %w", m.MatchID, r.Number, err)
				}
				pr.frames = decoded
			}
			side := m.Filter.Perspective(m.Document, r)
			for i, p := range patterns {
				if p.Side != "" && side != p.Side {
					// A pattern reads one side, so the other side's rounds are not in its scope.
					continue
				}
				row, ok := matchRound(pr, p)
				matches[i].Rounds++
				accs[i].month(in.PlayedAt, false)
				accs[i].count(row, ok, in.PlayedAt.Before(recentFrom))
				if !ok {
					continue
				}
				row.Side, row.Won = side, side != "" && r.Winner == side
				if side != "" {
					row.Buy, row.OpponentBuy = r.Economy.Buy(side), r.Economy.Buy(side.Opponent())
				}
				matches[i].Matched = append(matches[i].Matched, row)
				if row.Won {
					accs[i].result.Won++
				}
				accs[i].month(in.PlayedAt, true)
			}
		}
		for i := range patterns {
			if frames && !hasBlob {
				accs[i].result.MissingPositions++
			}
			if matches[i].Rounds > 0 {
				accs[i].result.Matches = append(accs[i].result.Matches, matches[i])
			}
		}
	}
	out := make([]PatternResult, len(patterns))
	for i, acc := range accs {
		out[i] = acc.finish()
	}
	return out, nil
}

func (a *patternAcc) count(row PatternRound, matched, earlier bool) {
	a.result.Rounds++
	for i, hit := range row.Hits {
		a.result.Blocks[i].Rounds++
		if hit.Matched {
			a.result.Blocks[i].Matched++
		}
	}
	if earlier {
		a.recent.earlierRounds++
	} else {
		a.result.RecentRounds++
	}
	if !matched {
		return
	}
	a.result.Matched++
	if earlier {
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
	for _, m := range a.months {
		r.Months = append(r.Months, *m)
	}
	slices.SortFunc(r.Months, func(x, y PatternMonth) int {
		switch {
		case x.Month < y.Month:
			return -1
		case x.Month > y.Month:
			return 1
		}
		return 0
	})
	return r
}

// PatternTrend compares the recent share of matched rounds with the earlier
// one: up or down past PatternTrendMargin, flat inside it, empty without both.
func PatternTrend(recent, earlier PatternCount) string {
	if recent.Rounds == 0 || earlier.Rounds == 0 {
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
