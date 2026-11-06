package store

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// collectionView copies the items, so the caller owns what it reads.
func collectionView(c library.Collection) library.Collection {
	c.Items = append([]library.CollectionItem{}, c.Items...)
	return c
}

func applyCollectionPatch(c library.Collection, patch library.CollectionPatch) library.Collection {
	if patch.Name != nil {
		c.Name = *patch.Name
	}
	if patch.Items != nil {
		c.Items = append([]library.CollectionItem{}, *patch.Items...)
	}
	return c
}

func (r *MemoryLibraryRepository) CreateCollection(ctx context.Context, c *library.Collection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.collections == nil {
		r.collections = map[uuid.UUID]library.Collection{}
	}
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if _, ok := r.collections[c.ID]; ok {
		return library.ErrConflict
	}
	now := libraryNow()
	stored := collectionView(*c)
	stored.CreatedAt, stored.UpdatedAt = now, now
	r.collections[stored.ID] = stored
	*c = collectionView(stored)
	return nil
}

func (r *MemoryLibraryRepository) GetCollection(ctx context.Context, id uuid.UUID) (library.Collection, error) {
	if err := ctx.Err(); err != nil {
		return library.Collection{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.collections[id]
	if !ok {
		return library.Collection{}, library.ErrNotFound
	}
	return collectionView(c), nil
}

func (r *MemoryLibraryRepository) ListCollections(ctx context.Context) ([]library.Collection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.Collection{}
	for _, c := range r.collections {
		out = append(out, collectionView(c))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return out[:min(len(out), library.MaxCollectionList)], nil
}

func (r *MemoryLibraryRepository) UpdateCollection(ctx context.Context, id uuid.UUID, patch library.CollectionPatch) (library.Collection, error) {
	if err := ctx.Err(); err != nil {
		return library.Collection{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.collections[id]
	if !ok {
		return library.Collection{}, library.ErrNotFound
	}
	c = applyCollectionPatch(c, patch)
	c.UpdatedAt = libraryNow()
	r.collections[id] = c
	return collectionView(c), nil
}

func (r *MemoryLibraryRepository) DeleteCollection(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.collections[id]; !ok {
		return library.ErrNotFound
	}
	delete(r.collections, id)
	return nil
}
