package store

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func patternMatches(p library.Pattern, f library.PatternFilter) bool {
	if (f.Map != "" && p.Map != f.Map) || (f.Category != "" && p.Category != f.Category) {
		return false
	}
	if f.PlayID != nil && (p.PlayID == nil || *p.PlayID != *f.PlayID) {
		return false
	}
	return f.TeamID == nil || p.TeamID == nil || *p.TeamID == *f.TeamID
}

// playExists reports whether a linked play is there; no link is fine. The caller holds r.mu.
func (r *MemoryLibraryRepository) playExists(id *uuid.UUID) bool {
	if id == nil {
		return true
	}
	_, ok := r.plays[*id]
	return ok
}

func (r *MemoryLibraryRepository) CreatePattern(ctx context.Context, p *library.Pattern) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if _, taken := r.libreta.patterns[p.ID]; taken {
		return library.ErrConflict
	}
	if !r.playExists(p.PlayID) {
		return library.ErrNotFound
	}
	if r.libreta.patterns == nil {
		r.libreta.patterns = map[uuid.UUID]library.Pattern{}
	}
	stored := library.PatternCopy(*p)
	stored.CreatedAt = libraryNow()
	stored.UpdatedAt = stored.CreatedAt
	r.libreta.patterns[p.ID] = stored
	*p = library.PatternCopy(stored)
	return nil
}

func (r *MemoryLibraryRepository) GetPattern(ctx context.Context, id uuid.UUID) (library.Pattern, error) {
	if err := ctx.Err(); err != nil {
		return library.Pattern{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.libreta.patterns[id]
	if !ok {
		return library.Pattern{}, library.ErrNotFound
	}
	return library.PatternCopy(p), nil
}

func (r *MemoryLibraryRepository) ListPatterns(ctx context.Context, filter library.PatternFilter) ([]library.Pattern, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	out := []library.Pattern{}
	for _, p := range r.libreta.patterns {
		if patternMatches(p, filter) {
			out = append(out, library.PatternCopy(p))
		}
	}
	r.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	start := min(max(filter.Offset, 0), len(out))
	return out[start:min(start+clampPatternLimit(filter.Limit), len(out))], nil
}

func (r *MemoryLibraryRepository) UpdatePattern(ctx context.Context, id uuid.UUID, patch library.PatternPatch) (library.Pattern, error) {
	if err := ctx.Err(); err != nil {
		return library.Pattern{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.libreta.patterns[id]
	if !ok {
		return library.Pattern{}, library.ErrNotFound
	}
	if !r.playExists(patch.PlayID) {
		return library.Pattern{}, library.ErrNotFound
	}
	next := library.ApplyPatternPatch(p, patch)
	next.UpdatedAt = libraryNow()
	r.libreta.patterns[id] = next
	return library.PatternCopy(next), nil
}

func (r *MemoryLibraryRepository) DeletePattern(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.libreta.patterns[id]; !ok {
		return library.ErrNotFound
	}
	delete(r.libreta.patterns, id)
	r.libreta.dropPatternRows(func(key patternRowKey) bool { return key.pattern != id })
	return nil
}

func (l *memoryLibreta) moveTeamPatterns(from, into uuid.UUID) {
	for id, p := range l.patterns {
		if p.TeamID != nil && *p.TeamID == from {
			p.TeamID = &into
			l.patterns[id] = p
		}
	}
}
