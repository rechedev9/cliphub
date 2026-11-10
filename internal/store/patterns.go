package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// PatternRepository stores the pattern finder's saved patterns.
type PatternRepository interface {
	// CreatePattern stores a pattern; an id that is already stored is ErrConflict.
	CreatePattern(ctx context.Context, p *library.Pattern) error
	GetPattern(ctx context.Context, id uuid.UUID) (library.Pattern, error)
	// ListPatterns returns patterns by name; a team filter also keeps the library-wide ones.
	ListPatterns(ctx context.Context, filter library.PatternFilter) ([]library.Pattern, error)
	UpdatePattern(ctx context.Context, id uuid.UUID, patch library.PatternPatch) (library.Pattern, error)
	// DeletePattern removes a pattern and its stored detection; a missing id is ErrNotFound.
	DeletePattern(ctx context.Context, id uuid.UUID) error
	// SavePatternMatches replaces a pattern's stored detection over one match; a deleted
	// pattern or match is ErrNotFound. Deleting either drops its rows.
	SavePatternMatches(ctx context.Context, set library.PatternMatchSet) error
	// ListPatternMatches returns stored detection rows, newest match first.
	ListPatternMatches(ctx context.Context, filter library.PatternMatchFilter) ([]library.PatternMatchRow, error)
}

func clampPatternLimit(limit int) int {
	if limit <= 0 || limit > library.MaxPatternList {
		return library.MaxPatternList
	}
	return limit
}
