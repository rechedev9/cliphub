package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/library"
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
	// GetStatuses is GetStatus for many ids in one query, so a batched poll
	// costs one read instead of one per item. An id with no job is absent from
	// the map: the same "not found" a single GetStatus reports, never an error.
	GetStatuses(context.Context, []uuid.UUID) (map[uuid.UUID]job.StatusRow, error)
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

// LibraryRepository persists the shared demo library. Every read returns a
// copy the caller owns; every missing record is library.ErrNotFound and every
// broken uniqueness rule is library.ErrConflict.
type LibraryRepository interface {
	// CreateDemo stores a new demo, or, when a demo with the same SHA-256 is
	// already in the library, fills d with that demo and reports false.
	CreateDemo(ctx context.Context, d *library.Demo) (created bool, err error)
	GetDemo(ctx context.Context, id uuid.UUID) (library.Demo, error)
	ListDemos(ctx context.Context, filter library.DemoFilter) ([]library.Demo, error)
	UpdateDemo(ctx context.Context, id uuid.UUID, patch library.DemoPatch) (library.Demo, error)
	SetDemoState(ctx context.Context, id uuid.UUID, state library.ParseState, errorCode, message string) error
	// DeleteDemo removes the demo with its match; a missing id is not an error.
	DeleteDemo(ctx context.Context, id uuid.UUID) error
	// LinkDemoJob records that a Crear job was made from a demo. Linking a job
	// twice is a no-op; a missing demo is ErrNotFound.
	LinkDemoJob(ctx context.Context, demoID, jobID uuid.UUID) error
	// DemoForJob returns the demo a Crear job is linked to, or ErrNotFound.
	DemoForJob(ctx context.Context, jobID uuid.UUID) (library.Demo, error)
	// SaveAnalysis creates or refreshes the match of a demo and marks the demo
	// ready. Series, date and manual team assignments survive a refresh.
	SaveAnalysis(ctx context.Context, analysis library.Analysis) (library.Match, error)
	GetMatch(ctx context.Context, id uuid.UUID) (library.Match, error)
	ListMatches(ctx context.Context, filter library.MatchFilter) ([]library.Match, error)
	UpdateMatch(ctx context.Context, id uuid.UUID, patch library.MatchPatch) (library.Match, error)
	AssignMatchTeam(ctx context.Context, assignment library.TeamAssignment) (library.Match, error)
	CreateTeam(ctx context.Context, t *library.Team) error
	GetTeam(ctx context.Context, id uuid.UUID) (library.Team, error)
	ListTeams(ctx context.Context, filter library.TeamFilter) ([]library.Team, error)
	UpdateTeam(ctx context.Context, id uuid.UUID, patch library.TeamPatch) (library.Team, error)
	// MergeTeams moves every alias and match of from onto into and deletes from.
	MergeTeams(ctx context.Context, from, into uuid.UUID) (library.Team, error)
	GetPlayer(ctx context.Context, steamID string) (library.Player, error)
	ListPlayers(ctx context.Context, filter library.PlayerFilter) ([]library.Player, error)
	SetPlayerFaceit(ctx context.Context, steamID, faceitPlayerID, faceitNickname string) (library.Player, error)
	CreateSeries(ctx context.Context, s *library.Series) error
	GetSeries(ctx context.Context, id uuid.UUID) (library.Series, error)
	ListSeries(ctx context.Context, filter library.SeriesFilter) ([]library.Series, error)
	// UpdateSeries stores a cleaned edit; a missing series or team is ErrNotFound.
	UpdateSeries(ctx context.Context, id uuid.UUID, s library.Series) (library.Series, error)
	// DeleteSeries removes a series and detaches its matches.
	DeleteSeries(ctx context.Context, id uuid.UUID) error
	// CreateReport stores a scouting report; a missing team is ErrNotFound.
	CreateReport(ctx context.Context, r *library.Report) error
	GetReport(ctx context.Context, id uuid.UUID) (library.Report, error)
	ListReports(ctx context.Context, filter library.ReportFilter) ([]library.Report, error)
	UpdateReport(ctx context.Context, id uuid.UUID, patch library.ReportPatch) (library.Report, error)
	// DeleteReport removes a report; a missing id is ErrNotFound.
	DeleteReport(ctx context.Context, id uuid.UUID) error
	// CreateAnnotation stores a replay note; a missing match is ErrNotFound.
	CreateAnnotation(ctx context.Context, a *library.Annotation) error
	GetAnnotation(ctx context.Context, id uuid.UUID) (library.Annotation, error)
	// ListAnnotations returns the notes of the given matches by match, round and tick.
	ListAnnotations(ctx context.Context, filter library.AnnotationFilter) ([]library.Annotation, error)
	UpdateAnnotation(ctx context.Context, id uuid.UUID, patch library.AnnotationPatch) (library.Annotation, error)
	// DeleteAnnotation removes a note; a missing id is ErrNotFound.
	DeleteAnnotation(ctx context.Context, id uuid.UUID) error
	// CreatePlay stores a stratbook play; a missing team is ErrNotFound and an
	// id that is already stored is ErrConflict.
	CreatePlay(ctx context.Context, p *library.Play) error
	GetPlay(ctx context.Context, id uuid.UUID) (library.Play, error)
	// ListPlays returns plays by most recent edit.
	ListPlays(ctx context.Context, filter library.PlayFilter) ([]library.Play, error)
	UpdatePlay(ctx context.Context, id uuid.UUID, patch library.PlayPatch) (library.Play, error)
	// DeletePlay removes a play; a missing id is ErrNotFound.
	DeletePlay(ctx context.Context, id uuid.UUID) error
}

type Repositories struct {
	Jobs           JobRepository
	Streams        StreamJobRepository
	EditorAssets   EditorAssetRepository
	EditorProjects EditorProjectRepository
	Library        LibraryRepository
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
		Library:        NewSQLiteLibraryRepository(jobs.db),
		db:             jobs.db,
	}, nil
}

func NewMemory() *Repositories {
	return &Repositories{
		Jobs:           NewMemoryJobRepository(),
		Streams:        NewMemoryStreamJobRepository(),
		EditorAssets:   NewMemoryEditorAssetRepository(),
		EditorProjects: NewMemoryEditorProjectRepository(),
		Library:        NewMemoryLibraryRepository(),
	}
}
