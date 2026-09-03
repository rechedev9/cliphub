package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/timelineplan"
)

type SQLiteEditorAssetRepository struct {
	db *sql.DB
}

// NewSQLiteEditorAssetRepository returns a repository over the editor_assets
// table of db, which openSQLite has already migrated to the current schema.
func NewSQLiteEditorAssetRepository(db *sql.DB) (*SQLiteEditorAssetRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("editor asset repository requires an open database")
	}
	return &SQLiteEditorAssetRepository{db: db}, nil
}

func (r *SQLiteEditorAssetRepository) Create(ctx context.Context, a *mediaassets.Asset) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	probe, err := json.Marshal(a.Probe)
	if err != nil {
		return fmt.Errorf("marshal asset probe: %w", err)
	}
	var originJob any
	if a.OriginJobID != nil {
		originJob = a.OriginJobID.String()
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO editor_assets (id, sha256, file_name, origin, origin_job_id, origin_variant, origin_name, probe, media_key, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID.String(), a.SHA256, a.FileName, string(a.Origin), originJob, nullableText(a.OriginVariant), nullableText(a.OriginName),
		probe, a.MediaKey, a.CreatedAt.UnixMilli(),
	)
	return err
}

func (r *SQLiteEditorAssetRepository) Get(ctx context.Context, id uuid.UUID) (mediaassets.Asset, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, sha256, file_name, origin, origin_job_id, origin_variant, origin_name, probe, media_key, created_at
		 FROM editor_assets WHERE id = ?`, id.String())
	return scanEditorAsset(row)
}

// GetBySHA256 returns the oldest asset with this digest: sha256 is not UNIQUE
// (historic duplicates may exist), so the choice must be stable across calls.
func (r *SQLiteEditorAssetRepository) GetBySHA256(ctx context.Context, digest string) (mediaassets.Asset, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, sha256, file_name, origin, origin_job_id, origin_variant, origin_name, probe, media_key, created_at
		 FROM editor_assets WHERE sha256 = ? ORDER BY created_at ASC, id ASC LIMIT 1`, digest)
	return scanEditorAsset(row)
}

// Delete removes the asset row; a missing id is not an error.
func (r *SQLiteEditorAssetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM editor_assets WHERE id = ?`, id.String()); err != nil {
		return fmt.Errorf("delete editor asset: %w", err)
	}
	return nil
}

func (r *SQLiteEditorAssetRepository) List(ctx context.Context, limit int) ([]mediaassets.Asset, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, sha256, file_name, origin, origin_job_id, origin_variant, origin_name, probe, media_key, created_at
		 FROM editor_assets ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mediaassets.Asset
	for rows.Next() {
		a, err := scanEditorAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func scanEditorAsset(row sqlScanner) (mediaassets.Asset, error) {
	var (
		id, digest, fileName, origin, mediaKey string
		originJob, originVariant, originName   sql.NullString
		probeRaw                               []byte
		created                                int64
	)
	if err := row.Scan(&id, &digest, &fileName, &origin, &originJob, &originVariant, &originName, &probeRaw, &mediaKey, &created); err != nil {
		if err == sql.ErrNoRows {
			return mediaassets.Asset{}, mediaassets.ErrNotFound
		}
		return mediaassets.Asset{}, err
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return mediaassets.Asset{}, err
	}
	var probe mediaassets.Probe
	if err := json.Unmarshal(probeRaw, &probe); err != nil {
		return mediaassets.Asset{}, fmt.Errorf("decode asset probe: %w", err)
	}
	a := mediaassets.Asset{
		ID:            parsed,
		SHA256:        digest,
		FileName:      fileName,
		Origin:        mediaassets.Origin(origin),
		OriginVariant: originVariant.String,
		OriginName:    originName.String,
		Probe:         probe,
		MediaKey:      mediaKey,
		CreatedAt:     time.UnixMilli(created).UTC(),
	}
	if originJob.Valid && originJob.String != "" {
		jobID, err := uuid.Parse(originJob.String)
		if err != nil {
			return mediaassets.Asset{}, err
		}
		a.OriginJobID = &jobID
	}
	return a, nil
}

type SQLiteEditorProjectRepository struct {
	db *sql.DB
}

// NewSQLiteEditorProjectRepository returns a repository over the
// editor_projects table of db, which openSQLite has already migrated.
func NewSQLiteEditorProjectRepository(db *sql.DB) (*SQLiteEditorProjectRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("editor project repository requires an open database")
	}
	return &SQLiteEditorProjectRepository{db: db}, nil
}

func (r *SQLiteEditorProjectRepository) Create(ctx context.Context, p *timelineplan.Project) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO editor_projects (id, title, status, failure_reason, plan_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID.String(), p.Title, string(p.Status), p.FailureReason, nullableBytes(p.Plan), p.CreatedAt.UnixMilli(), p.UpdatedAt.UnixMilli(),
	)
	return err
}

func (r *SQLiteEditorProjectRepository) Get(ctx context.Context, id uuid.UUID) (timelineplan.Project, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, title, status, failure_reason, plan_json, created_at, updated_at FROM editor_projects WHERE id = ?`,
		id.String())
	return scanEditorProject(row)
}

func (r *SQLiteEditorProjectRepository) List(ctx context.Context, limit int) ([]timelineplan.Project, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, title, status, failure_reason, plan_json, created_at, updated_at
		 FROM editor_projects ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []timelineplan.Project
	for rows.Next() {
		p, err := scanEditorProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListByStatus is uncapped: the startup sweep must see every stranded project,
// not the HTTP page size.
func (r *SQLiteEditorProjectRepository) ListByStatus(ctx context.Context, status timelineplan.Status) ([]timelineplan.Project, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, title, status, failure_reason, plan_json, created_at, updated_at
		 FROM editor_projects WHERE status = ? ORDER BY updated_at DESC, id ASC`, string(status))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []timelineplan.Project{}
	for rows.Next() {
		p, err := scanEditorProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Delete removes the project row; a missing id is not an error.
func (r *SQLiteEditorProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM editor_projects WHERE id = ?`, id.String()); err != nil {
		return fmt.Errorf("delete editor project: %w", err)
	}
	return nil
}

func (r *SQLiteEditorProjectRepository) UpdateStatus(ctx context.Context, id uuid.UUID, s timelineplan.Status, failureReason string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE editor_projects SET status = ?, failure_reason = ?, updated_at = ? WHERE id = ?`,
		string(s), failureReason, time.Now().UTC().UnixMilli(), id.String())
	if err != nil {
		return err
	}
	return checkEditorRowsAffected(res)
}

func (r *SQLiteEditorProjectRepository) SetPlan(ctx context.Context, id uuid.UUID, plan timelineplan.Document) error {
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE editor_projects SET plan_json = ?, updated_at = ? WHERE id = ?`,
		string(raw), time.Now().UTC().UnixMilli(), id.String())
	if err != nil {
		return err
	}
	return checkEditorRowsAffected(res)
}

func scanEditorProject(row sqlScanner) (timelineplan.Project, error) {
	var (
		id, title, status, failure string
		plan                       sql.NullString
		created, updated           int64
	)
	if err := row.Scan(&id, &title, &status, &failure, &plan, &created, &updated); err != nil {
		if err == sql.ErrNoRows {
			return timelineplan.Project{}, timelineplan.ErrNotFound
		}
		return timelineplan.Project{}, err
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return timelineplan.Project{}, err
	}
	p := timelineplan.Project{
		ID:            parsed,
		Title:         title,
		Status:        timelineplan.Status(status),
		FailureReason: failure,
		CreatedAt:     time.UnixMilli(created).UTC(),
		UpdatedAt:     time.UnixMilli(updated).UTC(),
	}
	if plan.Valid {
		p.Plan = []byte(plan.String)
	}
	return p, nil
}

func nullableBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

func checkEditorRowsAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return timelineplan.ErrNotFound
	}
	return nil
}
