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

// passMu keeps one match's evaluation at a time in the process: it holds the match in memory.
// It is taken per match, so a long pass never holds back a read of another match for long.
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

// scopeKey is what decides the matches a pattern reads: patterns that share it list them once.
type scopeKey struct {
	mapName string
	matches int
}

// scopes lists each group's matches once and reads its stored rows over them in one query.
func (d *Detector) scopes(ctx context.Context, patterns []library.Pattern) ([]patternScope, error) {
	groups := map[scopeKey][]int{}
	var keys []scopeKey
	for i, p := range patterns {
		key := scopeKey{p.Map, p.Scope.Matches}
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], i)
	}
	out := make([]patternScope, len(patterns))
	for _, key := range keys {
		members := groups[key]
		listed, capped, err := d.scopeMatches(ctx, patterns[members[0]])
		if err != nil {
			return nil, err
		}
		byPattern := map[uuid.UUID][]library.PatternMatchRow{}
		if len(listed) > 0 {
			filter := library.PatternMatchFilter{MatchIDs: make([]uuid.UUID, 0, len(listed))}
			for _, m := range listed {
				filter.MatchIDs = append(filter.MatchIDs, m.ID)
			}
			for _, i := range members {
				filter.PatternIDs = append(filter.PatternIDs, patterns[i].ID)
			}
			rows, err := d.repo.ListPatternMatches(ctx, filter)
			if err != nil {
				return nil, err
			}
			for _, r := range rows {
				byPattern[r.PatternID] = append(byPattern[r.PatternID], r)
			}
		}
		for _, i := range members {
			out[i] = patternScope{matches: listed, capped: capped, rows: byMatch(byPattern[patterns[i].ID])}
		}
	}
	return out, nil
}

// Results reads each pattern over the newest matches of its map from the stored rows,
// detecting first the matches whose rows are missing or stale.
func (d *Detector) Results(ctx context.Context, patterns []library.Pattern) ([]tacticalplan.PatternResult, error) {
	scopes, err := d.scopes(ctx, patterns)
	if err != nil {
		return nil, err
	}
	stale := map[uuid.UUID][]int{}
	matches := map[uuid.UUID]library.Match{}
	var order []uuid.UUID
	for i, p := range patterns {
		for _, m := range scopes[i].matches {
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
	for _, id := range order {
		if err := ctx.Err(); err != nil {
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
	out := make([]tacticalplan.PatternResult, len(patterns))
	now := time.Now()
	for i, p := range patterns {
		var reads []tacticalplan.PatternMatchRead
		for _, m := range scopes[i].matches {
			for _, r := range scopes[i].rows[m.ID] {
				reads = append(reads, named(r.Read, m))
			}
		}
		out[i] = tacticalplan.CombinePatternReads(tacticalplan.PatternCombine{Reads: reads, Blocks: len(p.Pattern.Blocks), Now: now})
		out[i].CappedAt = scopes[i].capped
	}
	return out, nil
}

// named gives a stored read the match's team names as they are now: a rename or a merge of
// teams does not touch the match's updated_at, so the stored names can be old.
func named(read tacticalplan.PatternMatchRead, m library.Match) tacticalplan.PatternMatchRead {
	if read.Match.TeamKey == "" {
		return read
	}
	for _, t := range m.Teams {
		if t.Key == read.Match.TeamKey {
			read.Match.Team = teamName(t)
		} else {
			read.Match.Opponent = teamName(t)
		}
	}
	return read
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
	d.evaluate(ctx, m, patterns)
	return nil
}

// MatchHit is a saved pattern that holds in some rounds of a match, with the rows that hold.
type MatchHit struct {
	Pattern library.Pattern
	Rows    []library.PatternMatchRow
}

// MatchHits is every saved pattern of the match's map that holds in one of its rounds, and how
// many of the map's patterns apply to it: the library-wide ones and those of its two teams.
// A team's pattern only counts from that team's side.
func (d *Detector) MatchHits(ctx context.Context, m library.Match) ([]MatchHit, int, error) {
	patterns, err := d.repo.ListPatterns(ctx, library.PatternFilter{Map: m.Map})
	if err != nil {
		return nil, 0, err
	}
	stored, err := d.repo.ListPatternMatches(ctx, library.PatternMatchFilter{MatchIDs: []uuid.UUID{m.ID}})
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
		for pid, found := range d.evaluate(ctx, m, stale) {
			rows[pid] = found
		}
	}
	teams := map[string]*uuid.UUID{}
	for _, t := range m.Teams {
		teams[t.Key] = t.TeamID
	}
	hits := []MatchHit{}
	total := 0
	for _, p := range patterns {
		if !ownedByMatch(p, m) {
			continue
		}
		total++
		hit := MatchHit{Pattern: p}
		for _, r := range rows[p.ID] {
			team := teams[r.TeamKey]
			if r.Matched() == 0 || (p.TeamID != nil && (team == nil || *team != *p.TeamID)) {
				continue
			}
			r.Read = named(r.Read, m)
			hit.Rows = append(hit.Rows, r)
		}
		if len(hit.Rows) > 0 {
			hits = append(hits, hit)
		}
	}
	return hits, total, nil
}

// ownedByMatch reports whether a pattern reads this match: library-wide, or of one of its teams.
func ownedByMatch(p library.Pattern, m library.Match) bool {
	if p.TeamID == nil {
		return true
	}
	for _, t := range m.Teams {
		if t.TeamID != nil && *t.TeamID == *p.TeamID {
			return true
		}
	}
	return false
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
// A match whose files are missing is stored as unreadable or without positions; a file that
// failed to read for another reason gives the rows but stores nothing, so the next read retries.
func (d *Detector) evaluate(ctx context.Context, m library.Match, patterns []library.Pattern) map[uuid.UUID][]library.PatternMatchRow {
	passMu.Lock()
	defer passMu.Unlock()
	doc, docErr := readDocument(d.files, m.DemoID)
	if docErr != nil {
		log.Printf("patterndetect: match %s unreadable: %v", m.ID, docErr)
	}
	pos := &positions{files: d.files, demoID: m.DemoID}
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
		inputs := teamInputs(teamRead{match: m, doc: doc, blob: pos.read, scope: key})
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
			out[p.ID] = rows
		}
	}
	if (docErr != nil && !storage.IsNotExist(docErr)) || pos.failed {
		return out
	}
	for _, p := range patterns {
		err := d.repo.SavePatternMatches(ctx, library.PatternMatchSet{PatternID: p.ID, MatchID: m.ID, Rows: out[p.ID]})
		if err != nil && !errors.Is(err, library.ErrNotFound) {
			log.Printf("patterndetect: store pattern %s over match %s: %v", p.ID, m.ID, err)
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

// positions reads a demo's positions blob once per evaluation. A missing blob reads as none;
// failed marks any other error, so what was read without positions is not stored.
type positions struct {
	files  storage.Storage
	demoID uuid.UUID
	once   sync.Once
	blob   []byte
	failed bool
}

func (p *positions) read() ([]byte, error) {
	p.once.Do(func() {
		rc, err := p.files.Open(artifacts.LibraryTacticalPositionsKey(p.demoID))
		if err != nil {
			if !storage.IsNotExist(err) {
				log.Printf("patterndetect: open positions of demo %s: %v", p.demoID, err)
				p.failed = true
			}
			return
		}
		defer rc.Close()
		blob, err := io.ReadAll(rc)
		if err != nil {
			log.Printf("patterndetect: read positions of demo %s: %v", p.demoID, err)
			p.failed = true
			return
		}
		p.blob = blob
	})
	return p.blob, nil
}
