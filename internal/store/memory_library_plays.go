package store

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// fillPlayLists gives a play's nil lists empty ones, the shape both
// repositories return.
func fillPlayLists(p library.Play) library.Play {
	if p.Grenades == nil {
		p.Grenades = []tacticalplan.PlayGrenade{}
	}
	if p.Positions == nil {
		p.Positions = []tacticalplan.PlayPosition{}
	}
	if p.Examples == nil {
		p.Examples = []tacticalplan.Evidence{}
	}
	return p
}

func clampPlayLimit(limit int) int {
	if limit <= 0 || limit > library.MaxPlayList {
		return library.MaxPlayList
	}
	return limit
}

// applyPlayPatch is the edit both repositories apply to a stored play.
func applyPlayPatch(p library.Play, patch library.PlayPatch) library.Play {
	if patch.Title != nil {
		p.Title = *patch.Title
	}
	if patch.ClearTeam {
		p.TeamID = nil
	}
	if patch.TeamID != nil {
		p.TeamID = uuidPtr(*patch.TeamID)
	}
	if patch.Kind != nil {
		p.Kind = *patch.Kind
	}
	if patch.Site != nil {
		p.Site = *patch.Site
	}
	if patch.Notes != nil {
		p.Notes = *patch.Notes
	}
	if patch.Grenades != nil {
		p.Grenades = append([]tacticalplan.PlayGrenade{}, *patch.Grenades...)
	}
	if patch.Positions != nil {
		p.Positions = append([]tacticalplan.PlayPosition{}, *patch.Positions...)
	}
	if patch.Examples != nil {
		p.Examples = append([]tacticalplan.Evidence{}, *patch.Examples...)
	}
	return p
}

func (r *MemoryLibraryRepository) teamHasPlays(teamID uuid.UUID) bool {
	for _, p := range r.plays {
		if p.TeamID != nil && *p.TeamID == teamID {
			return true
		}
	}
	return false
}

// playView fills the team name and copies the lists, so the caller owns what
// it reads. A play whose team is gone reads without one, like SQLite's SET NULL.
func (r *MemoryLibraryRepository) playView(p library.Play) library.Play {
	p.TeamName = ""
	if p.TeamID != nil {
		if team, ok := r.teams[*p.TeamID]; ok {
			p.TeamName = team.name
			p.TeamID = uuidPtr(*p.TeamID)
		} else {
			p.TeamID = nil
		}
	}
	p = applyPlayPatch(p, library.PlayPatch{Grenades: &p.Grenades, Positions: &p.Positions, Examples: &p.Examples})
	return fillPlayLists(p)
}

func (r *MemoryLibraryRepository) CreatePlay(ctx context.Context, p *library.Play) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.TeamID != nil {
		if _, ok := r.teams[*p.TeamID]; !ok {
			return library.ErrNotFound
		}
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if _, ok := r.plays[p.ID]; ok {
		return library.ErrConflict
	}
	now := libraryNow()
	stored := r.playView(*p)
	stored.CreatedAt, stored.UpdatedAt = now, now
	r.plays[stored.ID] = stored
	*p = r.playView(stored)
	return nil
}

func (r *MemoryLibraryRepository) GetPlay(ctx context.Context, id uuid.UUID) (library.Play, error) {
	if err := ctx.Err(); err != nil {
		return library.Play{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plays[id]
	if !ok {
		return library.Play{}, library.ErrNotFound
	}
	return r.playView(p), nil
}

func (r *MemoryLibraryRepository) ListPlays(ctx context.Context, filter library.PlayFilter) ([]library.Play, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.Play{}
	for _, stored := range r.plays {
		p := r.playView(stored)
		if filter.TeamID != nil && (p.TeamID == nil || *p.TeamID != *filter.TeamID) {
			continue
		}
		if (filter.Map != "" && p.Map != filter.Map) || (filter.Side != "" && p.Side != filter.Side) ||
			(filter.Kind != "" && p.Kind != filter.Kind) {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	offset := max(filter.Offset, 0)
	if offset >= len(out) {
		return []library.Play{}, nil
	}
	return out[offset:min(offset+clampPlayLimit(filter.Limit), len(out))], nil
}

func (r *MemoryLibraryRepository) UpdatePlay(ctx context.Context, id uuid.UUID, patch library.PlayPatch) (library.Play, error) {
	if err := ctx.Err(); err != nil {
		return library.Play{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.plays[id]
	if !ok {
		return library.Play{}, library.ErrNotFound
	}
	if patch.TeamID != nil {
		if _, ok := r.teams[*patch.TeamID]; !ok {
			return library.Play{}, library.ErrNotFound
		}
	}
	p = applyPlayPatch(r.playView(p), patch)
	p.UpdatedAt = libraryNow()
	r.plays[id] = p
	return r.playView(p), nil
}

func (r *MemoryLibraryRepository) DeletePlay(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.plays[id]; !ok {
		return library.ErrNotFound
	}
	delete(r.plays, id)
	return nil
}
