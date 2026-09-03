package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vodfetch"
)

// SQLiteStreamJobRepository persists streamer-clip jobs (internal/streamclips)
// in the same local SQLite database as SQLiteJobRepository, so the
// stream-jobs API works on the local desktop studio, which has no Postgres.
// It shares the *sql.DB opened by NewSQLiteJobRepository (see main.go)
// instead of opening the database file a second time: a single connection
// (db.SetMaxOpenConns(1) is set once, by the job repository) serializes all
// writers across both tables.
type SQLiteStreamJobRepository struct {
	db *sql.DB
}

// NewSQLiteStreamJobRepository returns a repository over the stream_jobs table
// of db, which openSQLite has already migrated to the current schema.
func NewSQLiteStreamJobRepository(db *sql.DB) (*SQLiteStreamJobRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("stream job repository requires an open database")
	}
	return &SQLiteStreamJobRepository{db: db}, nil
}

func (r *SQLiteStreamJobRepository) Create(ctx context.Context, j *streamclips.Job) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	now := time.Now().UTC()
	j.CreatedAt = now
	j.UpdatedAt = now
	probeJSON, err := json.Marshal(j.Probe)
	if err != nil {
		return fmt.Errorf("marshal probe: %w", err)
	}
	if j.SourceURL != "" {
		source, err := vodfetch.ValidateSource(j.SourceURL)
		if err != nil {
			return fmt.Errorf("validate source url: %w", err)
		}
		j.SourceURL = source.AcquisitionURL
		j.PublicSourceURL = source.PublicURL
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO stream_jobs (id, status, failure_reason, failure_code, source_path, source_sha256, source_url, public_source_url, title, probe, edit_plan, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID.String(), string(j.Status), nullableText(j.FailureReason), nullableText(streamFailureCode(j.FailureReason, j.FailureCode)), j.SourcePath, j.SourceSHA256,
		nullableText(j.SourceURL), nullableText(j.PublicSourceURL), nullableText(j.Title), probeJSON, nullableEditPlan(j.EditPlan),
		now.UnixNano(), now.UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("insert stream job: %w", err)
	}
	return nil
}

func (r *SQLiteStreamJobRepository) Get(ctx context.Context, id uuid.UUID) (streamclips.Job, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, status, COALESCE(failure_reason,''), COALESCE(failure_code,''), source_path, source_sha256,
		        COALESCE(source_url,''), COALESCE(public_source_url,''), COALESCE(title,''), probe, edit_plan, created_at, updated_at
		 FROM stream_jobs WHERE id = ?`, id.String())
	return scanSQLiteStreamJob(row)
}

func (r *SQLiteStreamJobRepository) List(ctx context.Context, limit int) ([]streamclips.Job, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, status, COALESCE(failure_reason,''), COALESCE(failure_code,''), source_path, source_sha256,
		        COALESCE(source_url,''), COALESCE(public_source_url,''), COALESCE(title,''), probe, edit_plan, created_at, updated_at
		 FROM stream_jobs ORDER BY updated_at DESC, created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query stream jobs: %w", err)
	}
	defer rows.Close()

	out := []streamclips.Job{}
	for rows.Next() {
		j, err := scanSQLiteStreamJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stream jobs: %w", err)
	}
	return out, nil
}

func (r *SQLiteStreamJobRepository) ListByStatus(ctx context.Context, status streamclips.Status) ([]streamclips.Job, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, status, COALESCE(failure_reason,''), COALESCE(failure_code,''), source_path, source_sha256,
		        COALESCE(source_url,''), COALESCE(public_source_url,''), COALESCE(title,''), probe, edit_plan, created_at, updated_at
		 FROM stream_jobs WHERE status = ? ORDER BY updated_at DESC, created_at DESC`, string(status))
	if err != nil {
		return nil, fmt.Errorf("query stream jobs by status: %w", err)
	}
	defer rows.Close()

	out := []streamclips.Job{}
	for rows.Next() {
		j, err := scanSQLiteStreamJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stream jobs by status: %w", err)
	}
	return out, nil
}

func (r *SQLiteStreamJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status streamclips.Status, failureReason string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE stream_jobs SET status = ?, failure_reason = ?, failure_code = ?,
		 source_url = CASE WHEN ? THEN NULL ELSE source_url END,
		 updated_at = ? WHERE id = ?`,
		string(status), nullableText(failureReason), nullableText(streamFailureCode(failureReason, "")),
		status == streamclips.StatusFailed,
		time.Now().UTC().UnixNano(), id.String(),
	)
	if err != nil {
		return fmt.Errorf("update stream job status: %w", err)
	}
	return checkStreamJobRowsAffected(res)
}

// SetAcquired records a successful acquire-by-URL download: the probed source
// metadata and sha256, moving the job to "ready". It clears any prior failure
// reason so a retried acquire does not leave a stale message behind.
func (r *SQLiteStreamJobRepository) SetAcquired(ctx context.Context, id uuid.UUID, probe streamclips.SourceProbe, sha256, discoveredTitle string) error {
	probeJSON, err := json.Marshal(probe)
	if err != nil {
		return fmt.Errorf("marshal probe: %w", err)
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE stream_jobs SET probe = ?, source_sha256 = ?, source_url = NULL, title = CASE WHEN COALESCE(trim(title), '') = '' THEN ? ELSE title END, status = ?, failure_reason = NULL, failure_code = NULL, updated_at = ? WHERE id = ?`,
		probeJSON, sha256, discoveredTitle, string(streamclips.StatusReady), time.Now().UTC().UnixNano(), id.String(),
	)
	if err != nil {
		return fmt.Errorf("update stream job acquired: %w", err)
	}
	return checkStreamJobRowsAffected(res)
}

func (r *SQLiteStreamJobRepository) SetEditPlan(ctx context.Context, id uuid.UUID, plan streamclips.EditPlan) error {
	plan = streamclips.NormalizeEditPlan(plan)
	if err := plan.Validate(); err != nil {
		return err
	}
	b, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal edit plan: %w", err)
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE stream_jobs SET edit_plan = ?, status = ?, failure_reason = NULL, failure_code = NULL, updated_at = ? WHERE id = ?`,
		b, string(streamclips.StatusReady), time.Now().UTC().UnixNano(), id.String(),
	)
	if err != nil {
		return fmt.Errorf("update stream edit plan: %w", err)
	}
	return checkStreamJobRowsAffected(res)
}

// sqlScanner is satisfied by both *sql.Row and *sql.Rows, so scanSQLiteStreamJob
// works for Get (one row) and List (many rows) alike.
type sqlScanner interface {
	Scan(dest ...any) error
}


// Delete removes the stream job row. Idempotent: a missing id is not an error,
// so a retried delete after a partial artifact cleanup converges.
func (r *SQLiteStreamJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM stream_jobs WHERE id = ?`, id.String()); err != nil {
		return fmt.Errorf("delete stream job: %w", err)
	}
	return nil
}
func scanSQLiteStreamJob(row sqlScanner) (streamclips.Job, error) {
	var j streamclips.Job
	var idStr, statusRaw string
	var probeJSON, planJSON []byte
	var createdNano, updatedNano int64
	err := row.Scan(&idStr, &statusRaw, &j.FailureReason, &j.FailureCode, &j.SourcePath, &j.SourceSHA256,
		&j.SourceURL, &j.PublicSourceURL, &j.Title, &probeJSON, &planJSON, &createdNano, &updatedNano)
	if errors.Is(err, sql.ErrNoRows) {
		return streamclips.Job{}, streamclips.ErrNotFound
	}
	if err != nil {
		return streamclips.Job{}, fmt.Errorf("scan stream job: %w", err)
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return streamclips.Job{}, fmt.Errorf("parse stream job id: %w", err)
	}
	j.ID = id
	status, err := streamclips.ParseStatus(statusRaw)
	if err != nil {
		return streamclips.Job{}, err
	}
	j.Status = status
	if j.SourceURL == "" {
		// Downstream render metadata historically reads SourceURL. Once the
		// acquisition secret is cleared, the safe public value preserves that
		// behavior without reintroducing secret material.
		j.SourceURL = j.PublicSourceURL
	}
	if len(probeJSON) > 0 {
		if err := json.Unmarshal(probeJSON, &j.Probe); err != nil {
			return streamclips.Job{}, fmt.Errorf("unmarshal probe: %w", err)
		}
	}
	if len(planJSON) > 0 {
		j.EditPlan = append(json.RawMessage(nil), planJSON...)
	}
	j.CreatedAt = time.Unix(0, createdNano).UTC()
	j.UpdatedAt = time.Unix(0, updatedNano).UTC()
	j.FailureCode = streamFailureCode(j.FailureReason, j.FailureCode)
	return j, nil
}

func streamFailureCode(reason, stored string) string {
	if stored != "" {
		return stored
	}
	if code := streamclips.CodeFromReason(reason); code != "" {
		return code
	}
	return obs.ClassOf(reason)
}

// checkStreamJobRowsAffected turns a zero-row UPDATE into streamclips.ErrNotFound,
// matching the postgres repository's RowsAffected() == 0 semantics.
func checkStreamJobRowsAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return streamclips.ErrNotFound
	}
	return nil
}

// nullableText maps an empty string to SQL NULL so COALESCE(...,”) in the
// SELECTs above round-trips "unset" the same way the postgres repository's
// nullable TEXT columns do.
func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableEditPlan(plan json.RawMessage) any {
	if len(plan) == 0 {
		return nil
	}
	return []byte(plan)
}
