package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/timelineplan"
)

type JobRepository interface {
	Create(context.Context, *job.Job) error
	Get(context.Context, uuid.UUID) (job.Job, error)
	GetMeta(context.Context, uuid.UUID) (job.Job, error)
	GetStatus(context.Context, uuid.UUID) (job.Status, string, int, error)
	List(context.Context, int) ([]job.Job, error)
	ListBySeries(context.Context, string) ([]job.Job, error)
	ListByStatus(context.Context, job.Status) ([]job.Job, error)
	UpdateStatus(context.Context, uuid.UUID, job.Status, string) error
	SetParseInputs(context.Context, uuid.UUID, string, rules.Rules) error
	SetKillPlan(context.Context, uuid.UUID, killplan.Plan) error
	Delete(context.Context, uuid.UUID) error
}

type StreamJobRepository interface {
	Create(ctx context.Context, j *streamclips.Job) error
	Get(ctx context.Context, id uuid.UUID) (streamclips.Job, error)
	List(ctx context.Context, limit int) ([]streamclips.Job, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, s streamclips.Status, failureReason string) error
	SetEditPlan(ctx context.Context, id uuid.UUID, plan streamclips.EditPlan) error
	SetAcquired(ctx context.Context, id uuid.UUID, probe streamclips.SourceProbe, sha256, discoveredTitle string) error
	ListByStatus(ctx context.Context, status streamclips.Status) ([]streamclips.Job, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type EditorAssetRepository interface {
	Create(ctx context.Context, a *mediaassets.Asset) error
	Get(ctx context.Context, id uuid.UUID) (mediaassets.Asset, error)
	GetBySHA256(ctx context.Context, sha256 string) (mediaassets.Asset, error)
	List(ctx context.Context, limit int) ([]mediaassets.Asset, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type EditorProjectRepository interface {
	Create(ctx context.Context, p *timelineplan.Project) error
	Get(ctx context.Context, id uuid.UUID) (timelineplan.Project, error)
	List(ctx context.Context, limit int) ([]timelineplan.Project, error)
	ListByStatus(ctx context.Context, status timelineplan.Status) ([]timelineplan.Project, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, s timelineplan.Status, failureReason string) error
	SetPlan(ctx context.Context, id uuid.UUID, plan timelineplan.Document) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Repositories struct {
	Jobs           JobRepository
	Streams        StreamJobRepository
	EditorAssets   EditorAssetRepository
	EditorProjects EditorProjectRepository
	db             *sql.DB
}

func (r *Repositories) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	err := r.db.Close()
	r.db = nil
	return err
}

func OpenSQLite(path string) (*Repositories, error) {
	jobs, err := NewSQLiteJobRepository(path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: %w", err)
	}
	streams, err := NewSQLiteStreamJobRepository(jobs.db)
	if err != nil {
		_ = jobs.Close()
		return nil, fmt.Errorf("sqlite stream jobs: %w", err)
	}
	assets, err := NewSQLiteEditorAssetRepository(jobs.db)
	if err != nil {
		_ = jobs.Close()
		return nil, fmt.Errorf("sqlite editor assets: %w", err)
	}
	projects, err := NewSQLiteEditorProjectRepository(jobs.db)
	if err != nil {
		_ = jobs.Close()
		return nil, fmt.Errorf("sqlite editor projects: %w", err)
	}
	return &Repositories{
		Jobs:           jobs,
		Streams:        streams,
		EditorAssets:   assets,
		EditorProjects: projects,
		db:             jobs.db,
	}, nil
}

func NewMemory() *Repositories {
	return &Repositories{
		Jobs:           NewMemoryJobRepository(),
		Streams:        NewMemoryStreamJobRepository(),
		EditorAssets:   NewMemoryEditorAssetRepository(),
		EditorProjects: NewMemoryEditorProjectRepository(),
	}
}
