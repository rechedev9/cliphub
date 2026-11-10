// Package patterndetect keeps every saved pattern's detection over the library: one stored
// read per pattern, match and team, refreshed when a demo is analysed or a pattern changes,
// so a list, a pattern's rounds or the Replay never rerun the whole library.
package patterndetect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// LibraryMatches caps a library-wide pattern at the newest matches of its map, to bound a pass.
const LibraryMatches = 200

// Repository is the slice of the library store detection reads and writes.
type Repository interface {
	ListPatterns(ctx context.Context, filter library.PatternFilter) ([]library.Pattern, error)
	ListMatches(ctx context.Context, filter library.MatchFilter) ([]library.Match, error)
	SavePatternMatches(ctx context.Context, set library.PatternMatchSet) error
	ListPatternMatches(ctx context.Context, filter library.PatternMatchFilter) ([]library.PatternMatchRow, error)
}

// Detector reads saved patterns over analysed matches and stores what it found.
type Detector struct {
	repo  Repository
	files storage.Storage
}

// New returns a detector over the library store and the files its analyses live in.
func New(repo Repository, files storage.Storage) *Detector {
	return &Detector{repo: repo, files: files}
}

// passMu keeps one detection pass at a time in the process: a pass holds one match in memory.
var passMu sync.Mutex

// fresh reports whether stored rows still answer for the pattern and the match as they are now.
func fresh(rows []library.PatternMatchRow, p library.Pattern, m library.Match) bool {
	if len(rows) == 0 {
		return false
	}
	revision := library.PatternRevision(p)
	for _, r := range rows {
		if r.Revision != revision || r.MatchUpdatedAt.UnixMilli() != m.UpdatedAt.UnixMilli() {
			return false
		}
	}
	return true
}

// byMatch groups rows by match, each match's rows by team key.
func byMatch(rows []library.PatternMatchRow) map[uuid.UUID][]library.PatternMatchRow {
	out := map[uuid.UUID][]library.PatternMatchRow{}
	for _, r := range rows {
		out[r.MatchID] = append(out[r.MatchID], r)
	}
	for id := range out {
		slices.SortFunc(out[id], func(a, b library.PatternMatchRow) int { return strings.Compare(a.TeamKey, b.TeamKey) })
	}
	return out
}

// scopeMatches is the newest matches of the map a pattern reads, and the cap when it cut the list.
func (d *Detector) scopeMatches(ctx context.Context, p library.Pattern) ([]library.Match, int, error) {
	limit := LibraryMatches
	if p.Scope.Matches > 0 {
		limit = min(limit, p.Scope.Matches)
	}
	listed, err := d.repo.ListMatches(ctx, library.MatchFilter{Map: p.Map, Limit: limit})
	if err != nil {
		return nil, 0, err
	}
	if p.Scope.Matches > 0 || len(listed) < limit {
		return listed, 0, nil
	}
	more, err := d.repo.ListMatches(ctx, library.MatchFilter{Map: p.Map, Limit: 1, Offset: limit})
	if err != nil {
		return nil, 0, err
	}
	if len(more) > 0 {
		return listed, limit, nil
	}
	return listed, 0, nil
}

// patternScope is one pattern's matches and its stored rows by match.
type patternScope struct {
	matches []library.Match
	capped  int
	rows    map[uuid.UUID][]library.PatternMatchRow
}

// Results reads each pattern over the newest matches of its map from the stored rows,
// detecting first the matches whose rows are missing or stale.
func (d *Detector) Results(ctx context.Context, patterns []library.Pattern) ([]tacticalplan.PatternResult, error) {
	scopes := make([]patternScope, len(patterns))
	stale := map[uuid.UUID][]int{}
	matches := map[uuid.UUID]library.Match{}
	var order []uuid.UUID
	for i, p := range patterns {
		listed, capped, err := d.scopeMatches(ctx, p)
		if err != nil {
			return nil, err
		}
		id := p.ID
		rows, err := d.repo.ListPatternMatches(ctx, library.PatternMatchFilter{PatternID: &id})
		if err != nil {
			return nil, err
		}
		scopes[i] = patternScope{matches: listed, capped: capped, rows: byMatch(rows)}
		for _, m := range listed {
			if fresh(scopes[i].rows[m.ID], p, m) {
				continue
			}
			if _, seen := stale[m.ID]; !seen {
				order = append(order, m.ID)
				matches[m.ID] = m
			}
			stale[m.ID] = append(stale[m.ID], i)
		}
	}
	if len(order) > 0 {
		passMu.Lock()
		for _, id := range order {
			if err := ctx.Err(); err != nil {
				passMu.Unlock()
				return nil, err
			}
			members := stale[id]
			chosen := make([]library.Pattern, len(members))
			for j, i := range members {
				chosen[j] = patterns[i]
			}
			found := d.evaluate(ctx, matches[id], chosen)
			for _, i := range members {
				scopes[i].rows[id] = found[patterns[i].ID]
			}
		}
		passMu.Unlock()
	}
	out := make([]tacticalplan.PatternResult, len(patterns))
	now := time.Now()
	for i, p := range patterns {
		var reads []tacticalplan.PatternMatchRead
		for _, m := range scopes[i].matches {
			for _, r := range scopes[i].rows[m.ID] {
				reads = append(reads, r.Read)
			}
		}
		out[i] = tacticalplan.CombinePatternReads(tacticalplan.PatternCombine{Reads: reads, Blocks: len(p.Pattern.Blocks), Now: now})
		out[i].CappedAt = scopes[i].capped
	}
	return out, nil
}

// DetectPattern brings a pattern's stored rows up to date over the newest matches of its map.
func (d *Detector) DetectPattern(ctx context.Context, p library.Pattern) error {
	_, err := d.Results(ctx, []library.Pattern{p})
	return err
}

// DetectMatch reads every saved pattern of the match's map over it, as a scan finishes.
func (d *Detector) DetectMatch(ctx context.Context, m library.Match) error {
	patterns, err := d.repo.ListPatterns(ctx, library.PatternFilter{Map: m.Map})
	if err != nil || len(patterns) == 0 {
		return err
	}
	passMu.Lock()
	defer passMu.Unlock()
	d.evaluate(ctx, m, patterns)
	return nil
}

// MatchHit is a saved pattern that holds in some rounds of a match, with the rows that hold.
type MatchHit struct {
	Pattern library.Pattern
	Rows    []library.PatternMatchRow
}

// MatchHits is every saved pattern of the match's map that holds in one of its rounds, and how
// many patterns the map has. A team's pattern only counts from that team's side.
func (d *Detector) MatchHits(ctx context.Context, m library.Match) ([]MatchHit, int, error) {
	patterns, err := d.repo.ListPatterns(ctx, library.PatternFilter{Map: m.Map})
	if err != nil {
		return nil, 0, err
	}
	id := m.ID
	stored, err := d.repo.ListPatternMatches(ctx, library.PatternMatchFilter{MatchID: &id})
	if err != nil {
		return nil, 0, err
	}
	rows := map[uuid.UUID][]library.PatternMatchRow{}
	for _, r := range stored {
		rows[r.PatternID] = append(rows[r.PatternID], r)
	}
	var stale []library.Pattern
	for _, p := range patterns {
		if !fresh(rows[p.ID], p, m) {
			stale = append(stale, p)
		}
	}
	if len(stale) > 0 {
		passMu.Lock()
		for pid, found := range d.evaluate(ctx, m, stale) {
			rows[pid] = found
		}
		passMu.Unlock()
	}
	teams := map[string]*uuid.UUID{}
	for _, t := range m.Teams {
		teams[t.Key] = t.TeamID
	}
	hits := []MatchHit{}
	for _, p := range patterns {
		hit := MatchHit{Pattern: p}
		for _, r := range rows[p.ID] {
			team := teams[r.TeamKey]
			if r.Matched() == 0 || (p.TeamID != nil && (team == nil || *team != *p.TeamID)) {
				continue
			}
			hit.Rows = append(hit.Rows, r)
		}
		if len(hit.Rows) > 0 {
			hits = append(hits, hit)
		}
	}
	return hits, len(patterns), nil
}

// buyScope is the part of a saved scope that changes how a match is read.
type buyScope struct{ buy, opponentBuy tacticalplan.BuyType }

func scopeBuys(b tacticalplan.BuyType) []tacticalplan.BuyType {
	switch b {
	case "":
		return nil
	case library.ScopeBuyHalf:
		return []tacticalplan.BuyType{tacticalplan.BuySemi, tacticalplan.BuyForce}
	default:
		return []tacticalplan.BuyType{b}
	}
}

// evaluate reads patterns over one match from each team's side and stores the rows, by pattern.
// The caller holds passMu.
func (d *Detector) evaluate(ctx context.Context, m library.Match, patterns []library.Pattern) map[uuid.UUID][]library.PatternMatchRow {
	doc, docErr := readDocument(d.files, m.DemoID)
	if docErr != nil {
		log.Printf("patterndetect: match %s unreadable: %v", m.ID, docErr)
	}
	blob := onceBlob(positionsReader(d.files, m.DemoID))
	groups := map[buyScope][]int{}
	var order []buyScope
	for i, p := range patterns {
		key := buyScope{p.Scope.Buy, p.Scope.OpponentBuy}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], i)
	}
	out := map[uuid.UUID][]library.PatternMatchRow{}
	for _, key := range order {
		members := groups[key]
		defs := make([]tacticalplan.Pattern, len(members))
		for j, i := range members {
			defs[j] = patterns[i].Pattern
		}
		runner := tacticalplan.NewPatternRunner(defs, time.Now())
		inputs := teamInputs(teamRead{match: m, doc: doc, blob: blob, scope: key})
		if docErr != nil {
			// One unreadable row per match, so the result counts the match once.
			runner.Unreadable(inputs[0])
		} else {
			for _, in := range inputs {
				runner.Add(in)
			}
		}
		reads := runner.Reads()
		for j, i := range members {
			p := patterns[i]
			rows := make([]library.PatternMatchRow, 0, len(reads[j]))
			for _, rd := range reads[j] {
				rows = append(rows, library.PatternMatchRow{PatternID: p.ID, MatchID: m.ID, TeamKey: rd.Match.TeamKey,
					Revision: library.PatternRevision(p), MatchUpdatedAt: m.UpdatedAt, PlayedAt: m.PlayedAt, Read: rd})
			}
			slices.SortFunc(rows, func(a, b library.PatternMatchRow) int { return strings.Compare(a.TeamKey, b.TeamKey) })
			err := d.repo.SavePatternMatches(ctx, library.PatternMatchSet{PatternID: p.ID, MatchID: m.ID, Rows: rows})
			if err != nil && !errors.Is(err, library.ErrNotFound) {
				log.Printf("patterndetect: store pattern %s over match %s: %v", p.ID, m.ID, err)
			}
			out[p.ID] = rows
		}
	}
	return out
}

func teamName(t library.MatchTeam) string {
	if t.TeamName != "" {
		return t.TeamName
	}
	return t.ClanName
}

// teamRead is one match as a group of patterns sharing a buy scope reads it.
type teamRead struct {
	match library.Match
	doc   tacticalplan.Document
	blob  func() ([]byte, error)
	scope buyScope
}

// teamInputs is the match from each team's side; a match without teams is read once with none.
func teamInputs(read teamRead) []tacticalplan.PatternInput {
	m, doc, blob, scope := read.match, read.doc, read.blob, read.scope
	filter := tacticalplan.Filter{Buys: scopeBuys(scope.buy), OpponentBuys: scopeBuys(scope.opponentBuy)}
	if len(m.Teams) == 0 {
		return []tacticalplan.PatternInput{{Match: tacticalplan.SpatialMatch{MatchID: m.ID, Document: doc, Filter: filter, Blob: blob}, PlayedAt: m.PlayedAt}}
	}
	out := make([]tacticalplan.PatternInput, 0, len(m.Teams))
	for _, t := range m.Teams {
		f := filter
		f.TeamKey = t.Key
		in := tacticalplan.PatternInput{Match: tacticalplan.SpatialMatch{MatchID: m.ID, Document: doc, Filter: f, Blob: blob},
			PlayedAt: m.PlayedAt, Team: teamName(t)}
		for _, other := range m.Teams {
			if other.Key != t.Key {
				in.Opponent = teamName(other)
			}
		}
		out = append(out, in)
	}
	return out
}

// readDocument reads a library demo's tactical document, as the HTTP API does.
func readDocument(files storage.Storage, demoID uuid.UUID) (tacticalplan.Document, error) {
	rc, err := files.Open(artifacts.LibraryTacticalIndexKey(demoID))
	if err != nil {
		return tacticalplan.Document{}, err
	}
	defer rc.Close()
	var doc tacticalplan.Document
	if err := json.NewDecoder(rc).Decode(&doc); err != nil {
		return tacticalplan.Document{}, fmt.Errorf("decode tactical document: %w", err)
	}
	doc.DropRepeatedUtility()
	return doc, nil
}

// positionsReader reads a demo's positions blob; a missing blob reads as none.
func positionsReader(files storage.Storage, demoID uuid.UUID) func() ([]byte, error) {
	return func() ([]byte, error) {
		rc, err := files.Open(artifacts.LibraryTacticalPositionsKey(demoID))
		if err != nil {
			if !storage.IsNotExist(err) {
				log.Printf("patterndetect: open positions of demo %s: %v", demoID, err)
			}
			return nil, nil
		}
		defer rc.Close()
		blob, err := io.ReadAll(rc)
		if err != nil {
			log.Printf("patterndetect: read positions of demo %s: %v", demoID, err)
			return nil, nil
		}
		return blob, nil
	}
}

// onceBlob reads a match's blob once for every pattern group of the pass.
func onceBlob(read func() ([]byte, error)) func() ([]byte, error) {
	var once sync.Once
	var blob []byte
	var err error
	return func() ([]byte, error) {
		once.Do(func() { blob, err = read() })
		return blob, err
	}
}
