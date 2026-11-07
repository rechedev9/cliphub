package store

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

type calloutKey struct{ mapName, token string }

func callMatches(c library.Call, f library.CallFilter) bool {
	switch {
	case f.Side != "" && c.Side != f.Side && c.Side != library.CallSideBoth, f.Status != "" && c.Status != f.Status,
		f.ExcludeArchived && c.Status == library.StatusArchived:
		return false
	case f.Tag == "":
		return true
	}
	for _, tag := range c.Tags {
		if strings.EqualFold(tag, f.Tag) {
			return true
		}
	}
	return false
}

func (r *MemoryLibraryRepository) CreateCall(ctx context.Context, c *library.Call) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if _, taken := r.libreta.calls[c.ID]; taken {
		return library.ErrConflict
	}
	if r.libreta.calls == nil {
		r.libreta.calls = map[uuid.UUID]library.Call{}
	}
	stored := callCopy(*c)
	stored.CreatedAt = libraryNow()
	stored.UpdatedAt = stored.CreatedAt
	r.libreta.calls[c.ID] = stored
	*c = callCopy(stored)
	return nil
}

func (r *MemoryLibraryRepository) GetCall(ctx context.Context, id uuid.UUID) (library.Call, error) {
	if err := ctx.Err(); err != nil {
		return library.Call{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.libreta.calls[id]
	if !ok {
		return library.Call{}, library.ErrNotFound
	}
	return callCopy(c), nil
}

func (r *MemoryLibraryRepository) ListCalls(ctx context.Context, filter library.CallFilter) ([]library.Call, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	out := []library.Call{}
	for _, c := range r.libreta.calls {
		if callMatches(c, filter) {
			out = append(out, callCopy(c))
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
	return out[start:min(start+clampCallLimit(filter.Limit), len(out))], nil
}

func (r *MemoryLibraryRepository) UpdateCall(ctx context.Context, id uuid.UUID, patch library.CallPatch) (library.Call, error) {
	if err := ctx.Err(); err != nil {
		return library.Call{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.libreta.calls[id]
	if !ok {
		return library.Call{}, library.ErrNotFound
	}
	next := callCopy(library.ApplyCallPatch(c, patch))
	next.UpdatedAt = libraryNow()
	r.libreta.calls[id] = next
	return callCopy(next), nil
}

func (r *MemoryLibraryRepository) DeleteCall(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.libreta.calls[id]; !ok {
		return library.ErrNotFound
	}
	delete(r.libreta.calls, id)
	return nil
}

func (r *MemoryLibraryRepository) ListCalloutNames(ctx context.Context, mapName string) ([]library.CalloutName, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	out := []library.CalloutName{}
	for key, c := range r.libreta.calloutNames {
		if mapName == "" || key.mapName == mapName {
			out = append(out, c)
		}
	}
	r.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Map != out[j].Map {
			return out[i].Map < out[j].Map
		}
		return out[i].Token < out[j].Token
	})
	return out, nil
}

func (r *MemoryLibraryRepository) SetCalloutName(ctx context.Context, c *library.CalloutName) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.libreta.calloutNames == nil {
		r.libreta.calloutNames = map[calloutKey]library.CalloutName{}
	}
	c.UpdatedAt = libraryNow()
	r.libreta.calloutNames[calloutKey{c.Map, c.Token}] = *c
	return nil
}

func (r *MemoryLibraryRepository) DeleteCalloutName(ctx context.Context, mapName, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := calloutKey{mapName, token}
	if _, ok := r.libreta.calloutNames[key]; !ok {
		return library.ErrNotFound
	}
	delete(r.libreta.calloutNames, key)
	return nil
}
