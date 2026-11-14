package store

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

type memoryTeamDocument struct {
	doc     library.TeamDocument
	content []byte
}

func (r *MemoryLibraryRepository) teamHasDocuments(teamID uuid.UUID) bool {
	for _, d := range r.documents {
		if d.doc.TeamID == teamID {
			return true
		}
	}
	return false
}

func (r *MemoryLibraryRepository) documentView(d memoryTeamDocument) library.TeamDocument {
	doc := d.doc
	doc.TeamName = r.teams[doc.TeamID].name
	return doc
}

func (r *MemoryLibraryRepository) CreateTeamDocument(ctx context.Context, d *library.TeamDocument, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.teams[d.TeamID]; !ok {
		return library.ErrNotFound
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	now := libraryNow()
	stored := *d
	stored.SizeBytes = int64(len(content))
	stored.CreatedAt, stored.UpdatedAt = now, now
	entry := memoryTeamDocument{doc: stored, content: append([]byte{}, content...)}
	r.documents[stored.ID] = entry
	*d = r.documentView(entry)
	return nil
}

func (r *MemoryLibraryRepository) GetTeamDocument(ctx context.Context, id uuid.UUID) (library.TeamDocument, error) {
	if err := ctx.Err(); err != nil {
		return library.TeamDocument{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.documents[id]
	if !ok {
		return library.TeamDocument{}, library.ErrNotFound
	}
	return r.documentView(d), nil
}

func (r *MemoryLibraryRepository) TeamDocumentContent(ctx context.Context, id uuid.UUID) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.documents[id]
	if !ok {
		return nil, library.ErrNotFound
	}
	return append([]byte{}, d.content...), nil
}

func (r *MemoryLibraryRepository) ListTeamDocuments(ctx context.Context, filter library.TeamDocumentFilter) ([]library.TeamDocument, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.TeamDocument{}
	for _, d := range r.documents {
		if filter.TeamID != nil && d.doc.TeamID != *filter.TeamID {
			continue
		}
		out = append(out, r.documentView(d))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return libraryPage(out, filter.Limit, filter.Offset), nil
}

func (r *MemoryLibraryRepository) DeleteTeamDocument(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.documents[id]; !ok {
		return library.ErrNotFound
	}
	delete(r.documents, id)
	return nil
}
