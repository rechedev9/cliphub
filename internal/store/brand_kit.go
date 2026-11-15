package store

import (
	"context"

	"github.com/rechedev9/cliphub/internal/brandkit"
)

// BrandKitRepository stores the creator's one brand kit. It stays on the
// device: watermarks and banners are Studio renders, so the library sync leaves it out.
type BrandKitRepository interface {
	// GetBrandKit returns the saved kit, or brandkit.Default before the first save.
	GetBrandKit(ctx context.Context) (brandkit.Kit, error)
	// SaveBrandKit validates and replaces the kit and returns it with its new UpdatedAt.
	SaveBrandKit(ctx context.Context, kit brandkit.Kit) (brandkit.Kit, error)
}
