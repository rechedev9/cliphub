package store

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func (r *MemoryLibraryRepository) SetTeamProfile(ctx context.Context, p *library.TeamProfile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.teams[p.TeamID]; !ok {
		return library.ErrNotFound
	}
	if r.libreta.teamProfiles == nil {
		r.libreta.teamProfiles = map[uuid.UUID]library.TeamProfile{}
	}
	p.UpdatedAt = libraryNow()
	if p.Own {
		for id, other := range r.libreta.teamProfiles {
			if other.Own && id != p.TeamID {
				other.Own, other.UpdatedAt = false, p.UpdatedAt
				r.libreta.teamProfiles[id] = other
			}
		}
	}
	r.libreta.teamProfiles[p.TeamID] = *p
	return nil
}

func (r *MemoryLibraryRepository) ListTeamProfiles(ctx context.Context) ([]library.TeamProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	out := []library.TeamProfile{}
	for id, p := range r.libreta.teamProfiles {
		if _, ok := r.teams[id]; ok {
			out = append(out, p)
		}
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].TeamID.String() < out[j].TeamID.String() })
	return out, nil
}

// moveTeamProfile hands from's profile to into, which keeps its own rank when it
// has a profile; into is the own team after the merge when either one was.
func (l *memoryLibreta) moveTeamProfile(from, into uuid.UUID) {
	p, ok := l.teamProfiles[from]
	if !ok {
		return
	}
	delete(l.teamProfiles, from)
	kept, taken := l.teamProfiles[into]
	if !taken {
		p.TeamID = into
		l.teamProfiles[into] = p
		return
	}
	kept.Own = kept.Own || p.Own
	l.teamProfiles[into] = kept
}
