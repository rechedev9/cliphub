package store

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

type patternRowKey struct {
	pattern, match uuid.UUID
	team           string
}

// copyPatternRow gives a row its own rounds, the way a SQLite read decodes fresh ones.
func copyPatternRow(row library.PatternMatchRow) library.PatternMatchRow {
	raw, err := json.Marshal(row.Read)
	if err != nil {
		return row
	}
	var read tacticalplan.PatternMatchRead
	if json.Unmarshal(raw, &read) == nil {
		row.Read = read
	}
	return row
}

func (r *MemoryLibraryRepository) SavePatternMatches(ctx context.Context, set library.PatternMatchSet) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, pattern := r.libreta.patterns[set.PatternID]
	_, match := r.matches[set.MatchID]
	if !pattern || !match {
		return library.ErrNotFound
	}
	for key := range r.libreta.patternRows {
		if key.pattern == set.PatternID && key.match == set.MatchID {
			delete(r.libreta.patternRows, key)
		}
	}
	if r.libreta.patternRows == nil {
		r.libreta.patternRows = map[patternRowKey]library.PatternMatchRow{}
	}
	for _, row := range set.Rows {
		row.PatternID, row.MatchID = set.PatternID, set.MatchID
		r.libreta.patternRows[patternRowKey{set.PatternID, set.MatchID, row.TeamKey}] = copyPatternRow(row)
	}
	return nil
}

func (r *MemoryLibraryRepository) ListPatternMatches(ctx context.Context, filter library.PatternMatchFilter) ([]library.PatternMatchRow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	out := []library.PatternMatchRow{}
	for key, row := range r.libreta.patternRows {
		if (filter.PatternID != nil && key.pattern != *filter.PatternID) || (filter.MatchID != nil && key.match != *filter.MatchID) ||
			(filter.MatchedOnly && row.Matched() == 0) {
			continue
		}
		out = append(out, copyPatternRow(row))
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case !a.PlayedAt.Equal(b.PlayedAt):
			return a.PlayedAt.After(b.PlayedAt)
		case a.MatchID != b.MatchID:
			return a.MatchID.String() < b.MatchID.String()
		case a.TeamKey != b.TeamKey:
			return a.TeamKey < b.TeamKey
		default:
			return a.PatternID.String() < b.PatternID.String()
		}
	})
	return out, nil
}

// dropPatternRows forgets the stored detection of a deleted pattern or match, as SQLite's cascade does.
func (l *memoryLibreta) dropPatternRows(keep func(patternRowKey) bool) {
	for key := range l.patternRows {
		if !keep(key) {
			delete(l.patternRows, key)
		}
	}
}

// unlinkPlay clears a deleted play from its patterns, as SQLite's ON DELETE SET NULL does.
func (l *memoryLibreta) unlinkPlay(id uuid.UUID) {
	for pid, p := range l.patterns {
		if p.PlayID != nil && *p.PlayID == id {
			p.PlayID = nil
			l.patterns[pid] = p
		}
	}
}
