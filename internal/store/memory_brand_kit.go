package store

import (
	"context"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/brandkit"
)

type MemoryBrandKitRepository struct {
	mu    sync.RWMutex
	kit   brandkit.Kit
	saved bool
}

func NewMemoryBrandKitRepository() *MemoryBrandKitRepository {
	return &MemoryBrandKitRepository{}
}

func (r *MemoryBrandKitRepository) GetBrandKit(ctx context.Context) (brandkit.Kit, error) {
	if err := ctx.Err(); err != nil {
		return brandkit.Kit{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.saved {
		return brandkit.Default(), nil
	}
	return r.kit.Copy(), nil
}

func (r *MemoryBrandKitRepository) SaveBrandKit(ctx context.Context, kit brandkit.Kit) (brandkit.Kit, error) {
	if err := ctx.Err(); err != nil {
		return brandkit.Kit{}, err
	}
	if err := kit.Validate(); err != nil {
		return brandkit.Kit{}, err
	}
	saved := kit.Copy()
	saved.UpdatedAt = time.Now().UTC().Truncate(time.Millisecond)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kit, r.saved = saved, true
	return saved.Copy(), nil
}
