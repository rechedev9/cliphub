package store

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// memoryLibreta holds the Libreta's records of the memory repository, under
// its lock. Maps are made on first write.
type memoryLibreta struct {
	lineups      map[uuid.UUID]library.Lineup
	teamRoles    map[teamRolesKey]library.TeamRoleSheet
	calls        map[uuid.UUID]library.Call
	calloutNames map[calloutKey]library.CalloutName
	teamProfiles map[uuid.UUID]library.TeamProfile
	patterns     map[uuid.UUID]library.Pattern
}

func lineupMatches(l library.Lineup, f library.LineupFilter) bool {
	switch {
	case f.Map != "" && l.Map != f.Map, f.Side != "" && l.Side != f.Side, f.Grenade != "" && l.Grenade != f.Grenade,
		f.Status != "" && l.Status != f.Status, f.ExcludeArchived && l.Status == library.StatusArchived:
		return false
	case f.Tag == "":
		return true
	}
	for _, tag := range l.Tags {
		if strings.EqualFold(tag, f.Tag) {
			return true
		}
	}
	return false
}

func (r *MemoryLibraryRepository) CreateLineup(ctx context.Context, l *library.Lineup) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.libreta.lineups == nil {
		r.libreta.lineups = map[uuid.UUID]library.Lineup{}
	}
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	if _, ok := r.libreta.lineups[l.ID]; ok {
		return library.ErrConflict
	}
	stored := lineupCopy(*l)
	now := libraryNow()
	stored.CreatedAt, stored.UpdatedAt = now, now
	r.libreta.lineups[stored.ID] = stored
	*l = lineupCopy(stored)
	return nil
}

func (r *MemoryLibraryRepository) GetLineup(ctx context.Context, id uuid.UUID) (library.Lineup, error) {
	if err := ctx.Err(); err != nil {
		return library.Lineup{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	l, ok := r.libreta.lineups[id]
	if !ok {
		return library.Lineup{}, library.ErrNotFound
	}
	return lineupCopy(l), nil
}

func (r *MemoryLibraryRepository) ListLineups(ctx context.Context, filter library.LineupFilter) ([]library.Lineup, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.Lineup{}
	for _, l := range r.libreta.lineups {
		if lineupMatches(l, filter) {
			out = append(out, lineupCopy(l))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	offset := max(filter.Offset, 0)
	if offset >= len(out) {
		return []library.Lineup{}, nil
	}
	return out[offset:min(offset+clampLineupLimit(filter.Limit), len(out))], nil
}

func (r *MemoryLibraryRepository) UpdateLineup(ctx context.Context, id uuid.UUID, patch library.LineupPatch) (library.Lineup, error) {
	if err := ctx.Err(); err != nil {
		return library.Lineup{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.libreta.lineups[id]
	if !ok {
		return library.Lineup{}, library.ErrNotFound
	}
	l = library.ApplyLineupPatch(lineupCopy(l), patch)
	l.UpdatedAt = libraryNow()
	r.libreta.lineups[id] = l
	return lineupCopy(l), nil
}

func (r *MemoryLibraryRepository) DeleteLineup(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.libreta.lineups[id]; !ok {
		return library.ErrNotFound
	}
	delete(r.libreta.lineups, id)
	return nil
}
