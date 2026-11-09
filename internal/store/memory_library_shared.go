package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func (r *MemoryLibraryRepository) AttachDemoFile(ctx context.Context, id uuid.UUID, file library.DemoFile) (library.Demo, error) {
	if err := ctx.Err(); err != nil {
		return library.Demo{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.demos[id]
	if !ok {
		return library.Demo{}, library.ErrNotFound
	}
	d.Path, d.FileName, d.SizeBytes = file.Path, file.FileName, file.SizeBytes
	d.Source, d.SourceManual = attachedSource(file.Source), false
	d.UpdatedAt = libraryNow()
	r.demos[id] = d
	return r.demoCopy(d), nil
}

// attachedSource keeps a demo that now has its file from staying shared.
func attachedSource(s library.Source) library.Source {
	if s.WithoutDemo() || !library.ValidSource(s) {
		return library.SourceUnknown
	}
	return s
}
