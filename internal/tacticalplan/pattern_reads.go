package tacticalplan

import (
	"math"
	"slices"
	"time"
)

// PatternMatchRead is one pattern over one match from one team's side: what a run keeps per
// match, so a stored read and a fresh one add up to the same result.
type PatternMatchRead struct {
	// Match holds the rounds in scope and the matched ones; Rounds 0 is a match out of scope.
	Match  PatternMatch   `json:"match"`
	Blocks []PatternCount `json:"blocks"`
	// NoThrow counts the grenades a throw-position block could not place.
	NoThrow int `json:"no_throw,omitempty"`
	// MissingPositions marks a match without saved positions that a player block needs.
	MissingPositions bool `json:"missing_positions,omitempty"`
	Unreadable       bool `json:"unreadable,omitempty"`
}

// PatternRunner reads patterns over matches added one at a time, so a caller
// can drop each match's document and positions before reading the next.
type PatternRunner struct {
	patterns []Pattern
	needs    []bool
	reads    [][]PatternMatchRead
	now      time.Time
}

// NewPatternRunner starts a run; now anchors the recent window of the trend.
func NewPatternRunner(patterns []Pattern, now time.Time) *PatternRunner {
	r := &PatternRunner{patterns: patterns, needs: make([]bool, len(patterns)), reads: make([][]PatternMatchRead, len(patterns)), now: now}
	for i, p := range patterns {
		r.needs[i] = slices.ContainsFunc(p.Blocks, PatternBlock.needsFrames)
	}
	return r
}

// emptyMatch is a match of the result before any round is read.
func emptyMatch(in PatternInput) PatternMatch {
	return PatternMatch{MatchID: in.Match.MatchID, PlayedAt: in.PlayedAt, TeamKey: in.Match.Filter.TeamKey, Team: in.Team,
		Opponent: in.Opponent, Matched: []PatternRound{}}
}

// Unreadable records a match the caller could not read, for every pattern.
func (r *PatternRunner) Unreadable(in PatternInput) {
	for i := range r.patterns {
		r.reads[i] = append(r.reads[i], PatternMatchRead{Match: emptyMatch(in), Unreadable: true})
	}
}

func (r *PatternRunner) addEmpty(in PatternInput, i int) {
	r.reads[i] = append(r.reads[i], PatternMatchRead{Match: emptyMatch(in)})
}

// Add reads one match, newest first: the order of the result's matches. Every pattern
// gets a read, an empty one when the match is out of its scope.
func (r *PatternRunner) Add(in PatternInput) {
	m := in.Match
	scoped := make([]bool, len(r.patterns))
	frames := false
	for i, p := range r.patterns {
		scoped[i] = playersInTeam(m, p)
		frames = frames || (scoped[i] && r.needs[i])
	}
	if !slices.Contains(scoped, true) {
		for i := range r.patterns {
			r.addEmpty(in, i)
		}
		return
	}
	rounds := m.Filter.Apply(m.Document)
	byRound := map[int][]Frame{}
	hasFrames := false
	if frames {
		blob, header, ok, err := m.positions()
		if err != nil {
			r.Unreadable(in)
			return
		}
		for _, rd := range rounds {
			if !ok {
				break
			}
			decoded, err := roundFrames(blob, header, m.Document, rd.Number)
			if err != nil {
				r.Unreadable(in)
				return
			}
			byRound[rd.Number] = decoded
		}
		hasFrames = ok
	}
	for i, p := range r.patterns {
		switch {
		case !scoped[i]:
			r.addEmpty(in, i)
		case r.needs[i] && !hasFrames:
			r.reads[i] = append(r.reads[i], PatternMatchRead{Match: emptyMatch(in), MissingPositions: true})
		default:
			r.reads[i] = append(r.reads[i], matchRead(addMatch{in: in, rounds: rounds, frames: byRound, needs: r.needs[i], pattern: p}))
		}
	}
}

type addMatch struct {
	in      PatternInput
	rounds  []Round
	frames  map[int][]Frame
	needs   bool
	pattern Pattern
}

func matchRead(a addMatch) PatternMatchRead {
	m := a.in.Match
	read := PatternMatchRead{Match: emptyMatch(a.in), Blocks: make([]PatternCount, len(a.pattern.Blocks))}
	for _, rd := range a.rounds {
		side := m.Filter.Perspective(m.Document, rd)
		// A pattern reads one side, so the other side's rounds are not in its scope.
		if a.pattern.Side != "" && side != a.pattern.Side {
			continue
		}
		pr := patternRound{m: m, r: rd, frames: a.frames[rd.Number]}
		if a.needs && len(pr.frames) == 0 {
			continue
		}
		v := matchRound(pr, a.pattern)
		if v.out {
			continue
		}
		read.Match.Rounds++
		read.NoThrow += v.noThrow
		for i, hit := range v.row.Hits {
			read.Blocks[i].Rounds++
			if hit.Matched {
				read.Blocks[i].Matched++
			}
		}
		if !v.matched {
			continue
		}
		row := v.row
		row.Side, row.Won = side, side != "" && rd.Winner == side
		if side != "" {
			row.Buy, row.OpponentBuy = rd.Economy.Buy(side), rd.Economy.Buy(side.Opponent())
		}
		row.PlantSeconds = plantSeconds(rd, m.Document.Demo.Tickrate)
		read.Match.Matched = append(read.Match.Matched, row)
	}
	return read
}

// Reads is each pattern's read of every match added, in order: what a library run stores.
func (r *PatternRunner) Reads() [][]PatternMatchRead {
	return r.reads
}

// Finish closes the run: one result per pattern, in order.
func (r *PatternRunner) Finish() []PatternResult {
	out := make([]PatternResult, len(r.patterns))
	for i, p := range r.patterns {
		out[i] = CombinePatternReads(PatternCombine{Reads: r.reads[i], Blocks: len(p.Blocks), Now: r.now})
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

// PatternCombine is one pattern's reads, newest match first, with its block count and the
// moment the recent window of the trend ends.
type PatternCombine struct {
	Reads  []PatternMatchRead
	Blocks int
	Now    time.Time
}

// CombinePatternReads adds up a pattern's reads into its result.
func CombinePatternReads(c PatternCombine) PatternResult {
	res := PatternResult{Blocks: make([]PatternCount, c.Blocks), Months: []PatternMonth{}, Matches: []PatternMatch{}}
	recentFrom := c.Now.AddDate(0, 0, -PatternRecentDays)
	months := map[string]*PatternMonth{}
	var earlier PatternCount
	for _, rd := range c.Reads {
		switch {
		case rd.Unreadable:
			res.Unreadable++
			continue
		case rd.MissingPositions:
			res.MissingPositions++
			continue
		}
		res.NoThrow += rd.NoThrow
		m := rd.Match
		if m.Rounds == 0 {
			continue
		}
		matched := len(m.Matched)
		res.Rounds += m.Rounds
		res.Matched += matched
		for i, b := range rd.Blocks {
			if i < len(res.Blocks) {
				res.Blocks[i].Rounds += b.Rounds
				res.Blocks[i].Matched += b.Matched
			}
		}
		for _, row := range m.Matched {
			if row.Won {
				res.Won++
			}
		}
		if m.PlayedAt.Before(recentFrom) {
			earlier.Rounds += m.Rounds
			earlier.Matched += matched
		} else {
			res.RecentRounds += m.Rounds
			res.RecentMatched += matched
		}
		key := m.PlayedAt.UTC().Format("2006-01")
		month, ok := months[key]
		if !ok {
			month = &PatternMonth{Month: key}
			months[key] = month
		}
		month.Rounds += m.Rounds
		month.Matched += matched
		res.Matches = append(res.Matches, m)
	}
	if res.Matched > 0 {
		res.WinRate = math.Round(float64(res.Won)/float64(res.Matched)*1000) / 1000
	}
	res.Trend = PatternTrend(PatternCount{Matched: res.RecentMatched, Rounds: res.RecentRounds}, earlier)
	res.Months = monthSeries(months)
	return res
}
