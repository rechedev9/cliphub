package main

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/timelineplan"
)

type memoryEditorAssetRepository struct {
	mu     sync.RWMutex
	assets map[uuid.UUID]mediaassets.Asset
}

func newMemoryEditorAssetRepository() *memoryEditorAssetRepository {
	return &memoryEditorAssetRepository{assets: map[uuid.UUID]mediaassets.Asset{}}
}

func (r *memoryEditorAssetRepository) Create(ctx context.Context, a *mediaassets.Asset) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	stored := *a
	r.assets[stored.ID] = stored
	*a = stored
	return nil
}

func (r *memoryEditorAssetRepository) Get(ctx context.Context, id uuid.UUID) (mediaassets.Asset, error) {
	if err := ctx.Err(); err != nil {
		return mediaassets.Asset{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.assets[id]
	if !ok {
		return mediaassets.Asset{}, mediaassets.ErrNotFound
	}
	return a, nil
}

func (r *memoryEditorAssetRepository) GetBySHA256(ctx context.Context, digest string) (mediaassets.Asset, error) {
	if err := ctx.Err(); err != nil {
		return mediaassets.Asset{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, a := range r.assets {
		if a.SHA256 == digest {
			return a, nil
		}
	}
	return mediaassets.Asset{}, mediaassets.ErrNotFound
}

func (r *memoryEditorAssetRepository) List(ctx context.Context, limit int) ([]mediaassets.Asset, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]mediaassets.Asset, 0, len(r.assets))
	for _, a := range r.assets {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type memoryEditorProjectRepository struct {
	mu       sync.RWMutex
	projects map[uuid.UUID]timelineplan.Project
}

func newMemoryEditorProjectRepository() *memoryEditorProjectRepository {
	return &memoryEditorProjectRepository{projects: map[uuid.UUID]timelineplan.Project{}}
}

func (r *memoryEditorProjectRepository) Create(ctx context.Context, p *timelineplan.Project) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	stored := cloneEditorProject(*p)
	r.projects[stored.ID] = stored
	*p = cloneEditorProject(stored)
	return nil
}

func (r *memoryEditorProjectRepository) Get(ctx context.Context, id uuid.UUID) (timelineplan.Project, error) {
	if err := ctx.Err(); err != nil {
		return timelineplan.Project{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.projects[id]
	if !ok {
		return timelineplan.Project{}, timelineplan.ErrNotFound
	}
	return cloneEditorProject(p), nil
}

func (r *memoryEditorProjectRepository) List(ctx context.Context, limit int) ([]timelineplan.Project, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]timelineplan.Project, 0, len(r.projects))
	for _, p := range r.projects {
		out = append(out, cloneEditorProject(p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *memoryEditorProjectRepository) UpdateStatus(ctx context.Context, id uuid.UUID, s timelineplan.Status, failureReason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.projects[id]
	if !ok {
		return timelineplan.ErrNotFound
	}
	p.Status = s
	p.FailureReason = failureReason
	p.UpdatedAt = time.Now().UTC()
	r.projects[id] = p
	return nil
}

func (r *memoryEditorProjectRepository) SetPlan(ctx context.Context, id uuid.UUID, plan timelineplan.Document) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.projects[id]
	if !ok {
		return timelineplan.ErrNotFound
	}
	p.Plan = raw
	p.UpdatedAt = time.Now().UTC()
	r.projects[id] = p
	return nil
}

func cloneEditorProject(p timelineplan.Project) timelineplan.Project {
	if len(p.Plan) > 0 {
		p.Plan = append([]byte(nil), p.Plan...)
	}
	return p
}
