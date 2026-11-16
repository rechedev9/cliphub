package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/hudthemes"
)

// HUDThemeRepository stores the user's own Full Demo HUD themes. They stay on
// the device: Full Demo is a Studio feature, so the library sync leaves them out.
type HUDThemeRepository interface {
	// ListHUDThemes returns every theme, oldest first.
	ListHUDThemes(ctx context.Context) ([]hudthemes.Theme, error)
	// GetHUDTheme returns one theme; a missing id is hudthemes.ErrNotFound.
	GetHUDTheme(ctx context.Context, id uuid.UUID) (hudthemes.Theme, error)
	// CreateHUDTheme stores a new theme; past hudthemes.MaxThemes it is hudthemes.ErrLimit.
	CreateHUDTheme(ctx context.Context, t *hudthemes.Theme) error
	// UpdateHUDTheme replaces a theme's style; a missing id is hudthemes.ErrNotFound.
	UpdateHUDTheme(ctx context.Context, id uuid.UUID, style hudthemes.Style) (hudthemes.Theme, error)
	// DeleteHUDTheme removes a theme; a missing id is hudthemes.ErrNotFound.
	DeleteHUDTheme(ctx context.Context, id uuid.UUID) error
}

// newHUDTheme fills the id and timestamps a create leaves to the store.
func newHUDTheme(t *hudthemes.Theme) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	at := time.Now().UTC().Truncate(time.Millisecond)
	t.CreatedAt, t.UpdatedAt = at, at
}
