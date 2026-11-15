package store

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/hudthemes"
)

type MemoryHUDThemeRepository struct {
	mu     sync.RWMutex
	themes []hudthemes.Theme
}

func NewMemoryHUDThemeRepository() *MemoryHUDThemeRepository {
	return &MemoryHUDThemeRepository{}
}

func (r *MemoryHUDThemeRepository) ListHUDThemes(ctx context.Context) ([]hudthemes.Theme, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]hudthemes.Theme, 0, len(r.themes))
	for _, t := range r.themes {
		out = append(out, t.Copy())
	}
	return out, nil
}

func (r *MemoryHUDThemeRepository) GetHUDTheme(ctx context.Context, id uuid.UUID) (hudthemes.Theme, error) {
	if err := ctx.Err(); err != nil {
		return hudthemes.Theme{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.themes {
		if t.ID == id {
			return t.Copy(), nil
		}
	}
	return hudthemes.Theme{}, hudthemes.ErrNotFound
}

func (r *MemoryHUDThemeRepository) CreateHUDTheme(ctx context.Context, t *hudthemes.Theme) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.themes) >= hudthemes.MaxThemes {
		return hudthemes.ErrLimit
	}
	stored := t.Copy()
	newHUDTheme(&stored)
	r.themes = append(r.themes, stored)
	*t = stored.Copy()
	return nil
}

func (r *MemoryHUDThemeRepository) UpdateHUDTheme(ctx context.Context, id uuid.UUID, style hudthemes.Style) (hudthemes.Theme, error) {
	if err := ctx.Err(); err != nil {
		return hudthemes.Theme{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, t := range r.themes {
		if t.ID == id {
			t.Style = style
			t.UpdatedAt = time.Now().UTC().Truncate(time.Millisecond)
			r.themes[i] = t.Copy()
			return t.Copy(), nil
		}
	}
	return hudthemes.Theme{}, hudthemes.ErrNotFound
}

func (r *MemoryHUDThemeRepository) DeleteHUDTheme(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	i := slices.IndexFunc(r.themes, func(t hudthemes.Theme) bool { return t.ID == id })
	if i < 0 {
		return hudthemes.ErrNotFound
	}
	r.themes = slices.Delete(r.themes, i, i+1)
	return nil
}
