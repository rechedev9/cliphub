package store

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

type proMapKey struct {
	match int
	entry string
}

type memoryPro struct {
	maps    map[proMapKey]library.ProMap
	follows map[int]library.ProFollow
}

// proMapOut drops the demo of a record whose demo was deleted, as ON DELETE SET NULL does.
func (r *MemoryLibraryRepository) proMapOut(m library.ProMap) library.ProMap {
	m = proMapCopy(m)
	if m.DemoID != nil {
		if _, ok := r.demos[*m.DemoID]; !ok {
			m.DemoID = nil
		}
	}
	return m
}

func (r *MemoryLibraryRepository) SaveProMap(ctx context.Context, m *library.ProMap) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pro.maps == nil {
		r.pro.maps = map[proMapKey]library.ProMap{}
	}
	m.ImportedAt = fromMillis(millis(libraryNow()))
	m.PlayedAt = fromMillis(millis(m.PlayedAt))
	r.pro.maps[proMapKey{m.MatchID, m.Entry}] = proMapCopy(*m)
	return nil
}

func (r *MemoryLibraryRepository) GetProMap(ctx context.Context, matchID int, entry string) (library.ProMap, error) {
	if err := ctx.Err(); err != nil {
		return library.ProMap{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.pro.maps[proMapKey{matchID, entry}]
	if !ok {
		return library.ProMap{}, library.ErrNotFound
	}
	return r.proMapOut(m), nil
}

func (r *MemoryLibraryRepository) ProMapForDemo(ctx context.Context, demoID uuid.UUID) (library.ProMap, error) {
	maps, err := r.ListProMaps(ctx, 0)
	if err != nil {
		return library.ProMap{}, err
	}
	for _, m := range maps {
		if m.DemoID != nil && *m.DemoID == demoID {
			return m, nil
		}
	}
	return library.ProMap{}, library.ErrNotFound
}

func (r *MemoryLibraryRepository) ListProMaps(ctx context.Context, matchID int) ([]library.ProMap, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.ProMap{}
	for _, m := range r.pro.maps {
		if matchID <= 0 || m.MatchID == matchID {
			out = append(out, r.proMapOut(m))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if matchID <= 0 && !a.PlayedAt.Equal(b.PlayedAt) {
			return a.PlayedAt.After(b.PlayedAt)
		}
		if matchID <= 0 && a.MatchID != b.MatchID {
			return a.MatchID > b.MatchID
		}
		return a.Entry < b.Entry
	})
	return out, nil
}

func (r *MemoryLibraryRepository) FollowProTeam(ctx context.Context, f *library.ProFollow) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pro.follows == nil {
		r.pro.follows = map[int]library.ProFollow{}
	}
	stored, ok := r.pro.follows[f.TeamID]
	if !ok {
		stored = library.ProFollow{TeamID: f.TeamID, FollowedAt: fromMillis(millis(libraryNow()))}
	}
	stored.Name = f.Name
	r.pro.follows[f.TeamID] = stored
	*f = proFollowCopy(stored)
	return nil
}

func proFollowCopy(f library.ProFollow) library.ProFollow {
	if f.SyncedAt != nil {
		t := *f.SyncedAt
		f.SyncedAt = &t
	}
	return f
}

func (r *MemoryLibraryRepository) UnfollowProTeam(ctx context.Context, teamID int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.pro.follows[teamID]; !ok {
		return library.ErrNotFound
	}
	delete(r.pro.follows, teamID)
	return nil
}

func (r *MemoryLibraryRepository) ListProFollows(ctx context.Context) ([]library.ProFollow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.ProFollow{}
	for _, f := range r.pro.follows {
		out = append(out, proFollowCopy(f))
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].TeamID < out[j].TeamID
	})
	return out, nil
}

func (r *MemoryLibraryRepository) MarkProFollowSynced(ctx context.Context, teamID int, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.pro.follows[teamID]
	if !ok {
		return library.ErrNotFound
	}
	t := fromMillis(millis(at))
	f.SyncedAt = &t
	r.pro.follows[teamID] = f
	return nil
}
