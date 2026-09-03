package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vodfetch"
)

type MemoryJobRepository struct {
	mu   sync.RWMutex
	jobs map[uuid.UUID]job.Job
}

func NewMemoryJobRepository() *MemoryJobRepository {
	return &MemoryJobRepository{jobs: map[uuid.UUID]job.Job{}}
}

func (r *MemoryJobRepository) Create(ctx context.Context, j *job.Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	} else if _, exists := r.jobs[j.ID]; exists {
		// Same contract as the SQLite primary key: a job id is minted once.
		return fmt.Errorf("create job %s: id already exists", j.ID)
	}
	now := time.Now().UTC()
	j.CreatedAt = now
	j.UpdatedAt = now
	stored := cloneJob(*j)
	r.jobs[stored.ID] = stored
	*j = cloneJob(stored)
	return nil
}

func (r *MemoryJobRepository) Get(ctx context.Context, id uuid.UUID) (job.Job, error) {
	if err := ctx.Err(); err != nil {
		return job.Job{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[id]
	if !ok {
		return job.Job{}, job.ErrNotFound
	}
	return cloneJob(j), nil
}

func (r *MemoryJobRepository) GetMeta(ctx context.Context, id uuid.UUID) (job.Job, error) {
	if err := ctx.Err(); err != nil {
		return job.Job{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[id]
	if !ok {
		return job.Job{}, job.ErrNotFound
	}
	j = cloneJob(j)
	j.KillPlan = nil
	return j, nil
}

// GetStatus mirrors the SQLite projection: the failure reason is only
// meaningful on a failed job and the segment count only while recording.
func (r *MemoryJobRepository) GetStatus(ctx context.Context, id uuid.UUID) (job.Status, string, int, error) {
	if err := ctx.Err(); err != nil {
		return 0, "", 0, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[id]
	if !ok {
		return 0, "", 0, job.ErrNotFound
	}
	failureReason := ""
	if j.Status == job.StatusFailed {
		failureReason = j.FailureReason
	}
	segmentCount := 0
	if j.Status == job.StatusRecording && j.KillPlan != nil {
		segmentCount = len(j.KillPlan.Segments)
	}
	return j.Status, failureReason, segmentCount, nil
}

func (r *MemoryJobRepository) List(ctx context.Context, limit int) ([]job.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]job.Job, 0, len(r.jobs))
	for _, j := range r.jobs {
		j = cloneJob(j)
		j.KillPlan = nil
		out = append(out, j)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ListBySeries returns the metadata-only jobs of one upload series ordered by
// creation time ascending, with the id as a deterministic tie-break when two
// jobs share a CreatedAt (the same ordering the SQLite repo uses). The kill
// plan is stripped and the result is capped at 100 jobs, matching List: a
// series is a handful of demos, so the cap only guards against a pathological
// job set.
func (r *MemoryJobRepository) ListBySeries(ctx context.Context, seriesID string) ([]job.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []job.Job{}
	for _, j := range r.jobs {
		if j.SeriesID == seriesID {
			j.KillPlan = nil
			out = append(out, j)
		}
	}
	sort.Slice(out, func(i, k int) bool {
		if out[i].CreatedAt.Equal(out[k].CreatedAt) {
			return out[i].ID.String() < out[k].ID.String()
		}
		return out[i].CreatedAt.Before(out[k].CreatedAt)
	})
	if len(out) > 100 {
		out = out[:100]
	}
	return out, nil
}

// ListByStatus returns metadata-only jobs in the same order as List; the
// startup sweep re-sorts by id, but any other consumer sees the SQLite order.
func (r *MemoryJobRepository) ListByStatus(ctx context.Context, status job.Status) ([]job.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []job.Job{}
	for _, j := range r.jobs {
		if j.Status == status {
			j = cloneJob(j)
			j.KillPlan = nil
			out = append(out, j)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (r *MemoryJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status job.Status, failureReason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return job.ErrNotFound
	}
	j.Status = status
	j.FailureReason = failureReason
	j.FailureCode = obs.ClassOf(failureReason)
	j.UpdatedAt = time.Now().UTC()
	r.jobs[id] = j
	return nil
}

func (r *MemoryJobRepository) SetParseInputs(ctx context.Context, id uuid.UUID, steamID string, rl rules.Rules) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return job.ErrNotFound
	}
	if j.Status != job.StatusScanned && j.Status != job.StatusParsed {
		return job.ErrConflict
	}
	j.TargetSteamID = steamID
	j.Rules = rl
	j.Status = job.StatusParsing
	j.UpdatedAt = time.Now().UTC()
	r.jobs[id] = j
	return nil
}

// Delete removes the job from the map. A missing id is not an error, so deletes
// are idempotent and safe to retry after a failed artifact cleanup.
func (r *MemoryJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.jobs, id)
	return nil
}

func (r *MemoryJobRepository) SetKillPlan(ctx context.Context, id uuid.UUID, plan killplan.Plan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return job.ErrNotFound
	}
	// Same contract as the SQLite row: a plan that cannot be encoded (NaN or
	// Inf in a float) is refused at write time instead of panicking every
	// later clone.
	if _, err := json.Marshal(plan); err != nil {
		return fmt.Errorf("marshal kill plan: %w", err)
	}
	planCopy := plan
	j.KillPlan = &planCopy
	j.UpdatedAt = time.Now().UTC()
	r.jobs[id] = j
	return nil
}

// cloneJob is a deep copy through the same JSON the SQLite repository stores,
// so a caller mutating the returned plan, segments or rules cannot corrupt the
// map, exactly as it cannot corrupt a row.
func cloneJob(j job.Job) job.Job {
	raw, err := json.Marshal(j)
	if err != nil {
		panic(fmt.Sprintf("memory job repository: marshal job %s: %v", j.ID, err))
	}
	var out job.Job
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(fmt.Sprintf("memory job repository: unmarshal job %s: %v", j.ID, err))
	}
	return out
}

// MemoryStreamJobRepository is the in-memory equivalent of
// streamclips.Repository, used when ZV_DATABASE_URL=memory (Local Studio)
// so the streamer-clips flow, including acquisition-by-URL, works without
// Postgres.
type MemoryStreamJobRepository struct {
	mu   sync.RWMutex
	jobs map[uuid.UUID]streamclips.Job
}

func NewMemoryStreamJobRepository() *MemoryStreamJobRepository {
	return &MemoryStreamJobRepository{jobs: map[uuid.UUID]streamclips.Job{}}
}

func (r *MemoryStreamJobRepository) Create(ctx context.Context, j *streamclips.Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	now := time.Now().UTC()
	j.CreatedAt = now
	j.UpdatedAt = now
	if j.SourceURL != "" {
		source, err := vodfetch.ValidateSource(j.SourceURL)
		if err != nil {
			return err
		}
		j.SourceURL = source.AcquisitionURL
		j.PublicSourceURL = source.PublicURL
	}
	stored := cloneStreamJob(*j)
	r.jobs[stored.ID] = stored
	*j = cloneStreamJob(stored)
	return nil
}

func (r *MemoryStreamJobRepository) Get(ctx context.Context, id uuid.UUID) (streamclips.Job, error) {
	if err := ctx.Err(); err != nil {
		return streamclips.Job{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[id]
	if !ok {
		return streamclips.Job{}, streamclips.ErrNotFound
	}
	return cloneStreamJob(j), nil
}

func (r *MemoryStreamJobRepository) List(ctx context.Context, limit int) ([]streamclips.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]streamclips.Job, 0, len(r.jobs))
	for _, j := range r.jobs {
		out = append(out, cloneStreamJob(j))
	}
	sort.Slice(out, func(i, k int) bool {
		if out[i].UpdatedAt.Equal(out[k].UpdatedAt) {
			return out[i].CreatedAt.After(out[k].CreatedAt)
		}
		return out[i].UpdatedAt.After(out[k].UpdatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *MemoryStreamJobRepository) ListByStatus(ctx context.Context, status streamclips.Status) ([]streamclips.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []streamclips.Job{}
	for _, j := range r.jobs {
		if j.Status == status {
			out = append(out, cloneStreamJob(j))
		}
	}
	sort.Slice(out, func(i, k int) bool {
		if out[i].UpdatedAt.Equal(out[k].UpdatedAt) {
			return out[i].CreatedAt.After(out[k].CreatedAt)
		}
		return out[i].UpdatedAt.After(out[k].UpdatedAt)
	})
	return out, nil
}

func (r *MemoryStreamJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status streamclips.Status, failureReason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return streamclips.ErrNotFound
	}
	j.Status = status
	j.FailureReason = failureReason
	if code := streamclips.CodeFromReason(failureReason); code != "" {
		j.FailureCode = code
	} else {
		j.FailureCode = obs.ClassOf(failureReason)
	}
	if status == streamclips.StatusFailed {
		j.SourceURL = ""
	}
	j.UpdatedAt = time.Now().UTC()
	r.jobs[id] = j
	return nil
}

func (r *MemoryStreamJobRepository) SetEditPlan(ctx context.Context, id uuid.UUID, plan streamclips.EditPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	plan = streamclips.NormalizeEditPlan(plan)
	if err := plan.Validate(); err != nil {
		return err
	}
	b, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return streamclips.ErrNotFound
	}
	j.EditPlan = append(json.RawMessage(nil), b...)
	j.Status = streamclips.StatusReady
	j.FailureReason = ""
	j.FailureCode = ""
	j.UpdatedAt = time.Now().UTC()
	r.jobs[id] = j
	return nil
}

func (r *MemoryStreamJobRepository) SetAcquired(ctx context.Context, id uuid.UUID, probe streamclips.SourceProbe, sha256, discoveredTitle string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return streamclips.ErrNotFound
	}
	j.Probe = probe
	j.SourceSHA256 = sha256
	if strings.TrimSpace(j.Title) == "" {
		j.Title = strings.TrimSpace(discoveredTitle)
	}
	j.Status = streamclips.StatusReady
	j.FailureReason = ""
	j.FailureCode = ""
	j.SourceURL = ""
	j.UpdatedAt = time.Now().UTC()
	r.jobs[id] = j
	return nil
}

// Delete removes the stream job; a missing id is not an error.
func (r *MemoryStreamJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.jobs, id)
	return nil
}

func cloneStreamJob(j streamclips.Job) streamclips.Job {
	if j.EditPlan != nil {
		j.EditPlan = append(json.RawMessage(nil), j.EditPlan...)
	}
	if j.SourceURL == "" {
		j.SourceURL = j.PublicSourceURL
	}
	return j
}
