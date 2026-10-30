package store

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// applyAnnotationPatch is the edit both repositories apply to a stored note.
func applyAnnotationPatch(a library.Annotation, patch library.AnnotationPatch) library.Annotation {
	if patch.Tick != nil {
		a.Tick = *patch.Tick
	}
	if patch.Text != nil {
		a.Text = *patch.Text
	}
	if patch.Strokes != nil {
		a.Strokes = *patch.Strokes
	}
	return a
}

// annotationView copies the drawing, so the caller owns what it reads.
func annotationView(a library.Annotation) library.Annotation {
	strokes := make([]library.Stroke, 0, len(a.Strokes))
	for _, s := range a.Strokes {
		s.Points = append([][2]float64{}, s.Points...)
		strokes = append(strokes, s)
	}
	a.Strokes = strokes
	return a
}

// dropMatchAnnotations is the cascade SQLite runs when a match is deleted.
func (r *MemoryLibraryRepository) dropMatchAnnotations(matchID uuid.UUID) {
	for id, a := range r.annotations {
		if a.MatchID == matchID {
			delete(r.annotations, id)
		}
	}
}

func (r *MemoryLibraryRepository) CreateAnnotation(ctx context.Context, a *library.Annotation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.matches[a.MatchID]; !ok {
		return library.ErrNotFound
	}
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	now := libraryNow()
	stored := annotationView(*a)
	stored.CreatedAt, stored.UpdatedAt = now, now
	r.annotations[stored.ID] = stored
	*a = annotationView(stored)
	return nil
}

func (r *MemoryLibraryRepository) GetAnnotation(ctx context.Context, id uuid.UUID) (library.Annotation, error) {
	if err := ctx.Err(); err != nil {
		return library.Annotation{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.annotations[id]
	if !ok {
		return library.Annotation{}, library.ErrNotFound
	}
	return annotationView(a), nil
}

func (r *MemoryLibraryRepository) ListAnnotations(ctx context.Context, filter library.AnnotationFilter) ([]library.Annotation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	wanted := map[uuid.UUID]bool{}
	for _, id := range filter.MatchIDs {
		wanted[id] = true
	}
	out := []library.Annotation{}
	for _, a := range r.annotations {
		if !wanted[a.MatchID] || (filter.Round > 0 && a.Round != filter.Round) {
			continue
		}
		out = append(out, annotationView(a))
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case a.MatchID != b.MatchID:
			return a.MatchID.String() < b.MatchID.String()
		case a.Round != b.Round:
			return a.Round < b.Round
		case a.Tick != b.Tick:
			return a.Tick < b.Tick
		case !a.CreatedAt.Equal(b.CreatedAt):
			return a.CreatedAt.Before(b.CreatedAt)
		default:
			return a.ID.String() < b.ID.String()
		}
	})
	if len(out) > library.MaxAnnotationList {
		out = out[:library.MaxAnnotationList]
	}
	return out, nil
}

func (r *MemoryLibraryRepository) UpdateAnnotation(ctx context.Context, id uuid.UUID, patch library.AnnotationPatch) (library.Annotation, error) {
	if err := ctx.Err(); err != nil {
		return library.Annotation{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.annotations[id]
	if !ok {
		return library.Annotation{}, library.ErrNotFound
	}
	a = annotationView(applyAnnotationPatch(a, patch))
	a.UpdatedAt = libraryNow()
	r.annotations[id] = a
	return annotationView(a), nil
}

func (r *MemoryLibraryRepository) DeleteAnnotation(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.annotations[id]; !ok {
		return library.ErrNotFound
	}
	delete(r.annotations, id)
	return nil
}
