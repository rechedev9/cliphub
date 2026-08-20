package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

// sqliteJobRepository persists jobs in a local SQLite file so job state survives
// an orchestrator restart, unlike the in-memory repository. It is the default
// for the local desktop studio, which has no Postgres: job metadata lives in
// the `data` JSON document, and the kill plan is a sibling `kill_plan` blob so
// GetMeta/List/UpdateStatus never parse it. status/created_at/updated_at are
// mirrored into columns for List ordering. modernc.org/sqlite is a pure-Go
// driver, so no CGO or C toolchain is needed on Windows or in the static build.
type sqliteJobRepository struct {
	db *sql.DB
}

// newSQLiteJobRepository opens (creating if needed) the SQLite database at path
// and ensures the jobs table exists. A single connection fully serializes
// access, which for a local single-user studio removes all "database is locked"
// contention; WAL keeps that durable and fast.
func newSQLiteJobRepository(path string) (*sqliteJobRepository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("sqlite %s: %w", pragma, err)
		}
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS jobs (
		id         TEXT PRIMARY KEY,
		data       BLOB    NOT NULL,
		kill_plan  BLOB,
		status     TEXT    NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create jobs table: %w", err)
	}
	if err := ensureJobsKillPlanColumn(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrateEmbeddedKillPlans(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &sqliteJobRepository{db: db}, nil
}

func ensureJobsKillPlanColumn(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(jobs)`)
	if err != nil {
		return fmt.Errorf("inspect jobs columns: %w", err)
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan jobs column: %w", err)
		}
		found = found || name == "kill_plan"
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close jobs column rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate jobs columns: %w", err)
	}
	if found {
		return nil
	}
	if _, err := db.Exec(`ALTER TABLE jobs ADD COLUMN kill_plan BLOB`); err != nil {
		return fmt.Errorf("add jobs kill_plan: %w", err)
	}
	return nil
}

// migrateEmbeddedKillPlans moves a kill plan that still lives inside `data` into
// the dedicated column and strips it from the document. Idempotent: rows that
// already store the plan in the column keep that copy, and a second open is a
// no-op once `data` has no `$.kill_plan`.
func migrateEmbeddedKillPlans(db *sql.DB) error {
	if _, err := db.Exec(`
		UPDATE jobs
		SET kill_plan = json_extract(data, '$.kill_plan')
		WHERE kill_plan IS NULL AND json_type(data, '$.kill_plan') IS NOT NULL`); err != nil {
		return fmt.Errorf("extract embedded kill plan: %w", err)
	}
	if _, err := db.Exec(`
		UPDATE jobs
		SET data = json_remove(data, '$.kill_plan')
		WHERE json_type(data, '$.kill_plan') IS NOT NULL`); err != nil {
		return fmt.Errorf("strip embedded kill plan: %w", err)
	}
	return nil
}

// Close releases the underlying database handle.
func (r *sqliteJobRepository) Close() error { return r.db.Close() }

func (r *sqliteJobRepository) Create(ctx context.Context, j *job.Job) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	now := time.Now().UTC()
	j.CreatedAt = now
	j.UpdatedAt = now
	data, planJSON, err := marshalJobDocuments(j)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO jobs (id, data, kill_plan, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		j.ID.String(), data, nullableBlob(planJSON), j.Status.String(), now.UnixNano(), now.UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("insert job: %w", err)
	}
	return nil
}

func (r *sqliteJobRepository) Get(ctx context.Context, id uuid.UUID) (job.Job, error) {
	var data, planJSON []byte
	err := r.db.QueryRowContext(ctx, `SELECT data, kill_plan FROM jobs WHERE id = ?`, id.String()).Scan(&data, &planJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return job.Job{}, job.ErrNotFound
	}
	if err != nil {
		return job.Job{}, fmt.Errorf("query job: %w", err)
	}
	j, err := decodeJobRow(data, planJSON)
	if err != nil {
		return job.Job{}, fmt.Errorf("unmarshal job: %w", err)
	}
	return j, nil
}

func (r *sqliteJobRepository) GetMeta(ctx context.Context, id uuid.UUID) (job.Job, error) {
	var data []byte
	err := r.db.QueryRowContext(ctx, `SELECT data FROM jobs WHERE id = ?`, id.String()).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return job.Job{}, job.ErrNotFound
	}
	if err != nil {
		return job.Job{}, fmt.Errorf("query job metadata: %w", err)
	}
	j, err := decodeJobMeta(data)
	if err != nil {
		return job.Job{}, fmt.Errorf("unmarshal job metadata: %w", err)
	}
	return j, nil
}

func (r *sqliteJobRepository) GetStatus(ctx context.Context, id uuid.UUID) (job.Status, string, int, error) {
	var rawStatus, failureReason string
	var segmentCount int
	err := r.db.QueryRowContext(ctx, `
		SELECT status,
		       CASE WHEN status = ? THEN COALESCE(json_extract(data, '$.failure_reason'), '') ELSE '' END,
		       CASE WHEN status = ? THEN COALESCE(json_array_length(kill_plan, '$.segments'), 0) ELSE 0 END
		FROM jobs WHERE id = ?`,
		job.StatusFailed.String(), job.StatusRecording.String(), id.String(),
	).Scan(&rawStatus, &failureReason, &segmentCount)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", 0, job.ErrNotFound
	}
	if err != nil {
		return 0, "", 0, fmt.Errorf("query job status: %w", err)
	}
	status, err := job.ParseStatus(rawStatus)
	if err != nil {
		return 0, "", 0, fmt.Errorf("parse stored job status: %w", err)
	}
	return status, failureReason, segmentCount, nil
}

func (r *sqliteJobRepository) List(ctx context.Context, limit int) ([]job.Job, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return r.scanJobs(ctx,
		`SELECT data FROM jobs ORDER BY updated_at DESC, created_at DESC LIMIT ?`, limit,
	)
}

// ListBySeries returns the metadata-only jobs of one upload series ordered by
// creation time ascending, with the id as a deterministic tie-break when two
// jobs share a created_at (the same ordering the memory repo uses). created_at
// is the UnixNano mirror column. The kill plan is not selected and the result
// is capped at 100 jobs, matching List: a series is a handful of demos, so the
// cap only guards against a pathological document set.
func (r *sqliteJobRepository) ListBySeries(ctx context.Context, seriesID string) ([]job.Job, error) {
	return r.scanJobs(ctx,
		`SELECT data FROM jobs WHERE json_extract(data, '$.series_id') = ? ORDER BY created_at ASC, id ASC LIMIT 100`,
		seriesID,
	)
}

func (r *sqliteJobRepository) ListByStatus(ctx context.Context, status job.Status) ([]job.Job, error) {
	return r.scanJobs(ctx,
		`SELECT data FROM jobs WHERE status = ? ORDER BY updated_at DESC, created_at DESC`,
		status.String(),
	)
}

func (r *sqliteJobRepository) scanJobs(ctx context.Context, query string, args ...any) ([]job.Job, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	out := []job.Job{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		j, err := decodeJobMeta(data)
		if err != nil {
			return nil, fmt.Errorf("unmarshal job: %w", err)
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	return out, nil
}

func (r *sqliteJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status job.Status, failureReason string) error {
	now := time.Now().UTC()
	var result sql.Result
	var err error
	if failureReason == "" {
		result, err = r.db.ExecContext(ctx, `
			UPDATE jobs
			SET data = json_remove(
					json_set(data, '$.status', ?, '$.updated_at', ?),
					'$.failure_reason'
				),
				status = ?,
				updated_at = ?
			WHERE id = ?`,
			status.String(), now.Format(time.RFC3339Nano), status.String(), now.UnixNano(), id.String(),
		)
	} else {
		result, err = r.db.ExecContext(ctx, `
			UPDATE jobs
			SET data = json_set(
					data,
					'$.status', ?,
					'$.failure_reason', ?,
					'$.updated_at', ?
				),
				status = ?,
				updated_at = ?
			WHERE id = ?`,
			status.String(), failureReason, now.Format(time.RFC3339Nano), status.String(), now.UnixNano(), id.String(),
		)
	}
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated jobs: %w", err)
	}
	if updated == 0 {
		return job.ErrNotFound
	}
	return nil
}

func (r *sqliteJobRepository) SetParseInputs(ctx context.Context, id uuid.UUID, steamID string, rl rules.Rules) error {
	return r.mutate(ctx, id, func(j *job.Job) error {
		// Same status guard as the memory/Postgres repos: only a scanned or
		// already-parsed job can be (re)claimed for a parse.
		if j.Status != job.StatusScanned && j.Status != job.StatusParsed {
			return job.ErrConflict
		}
		j.TargetSteamID = steamID
		j.Rules = rl
		j.Status = job.StatusParsing
		return nil
	})
}

// Delete removes the job row. A missing row is not an error, so deletes are
// idempotent and safe to retry after a failed artifact cleanup.
func (r *sqliteJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id.String()); err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	return nil
}

func (r *sqliteJobRepository) SetKillPlan(ctx context.Context, id uuid.UUID, plan killplan.Plan) error {
	now := time.Now().UTC()
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal kill plan: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE jobs
		SET kill_plan = ?,
		    data = json_set(data, '$.updated_at', ?),
		    updated_at = ?
		WHERE id = ?`,
		planJSON, now.Format(time.RFC3339Nano), now.UnixNano(), id.String(),
	)
	if err != nil {
		return fmt.Errorf("update kill plan: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated kill plans: %w", err)
	}
	if updated == 0 {
		return job.ErrNotFound
	}
	return nil
}

// mutate loads job metadata inside a transaction, applies fn, bumps UpdatedAt,
// and writes the metadata document back. The kill_plan column is not selected
// or written: SetKillPlan is the only writer for that blob. The
// single-connection pool serializes writers, so the read-modify-write is
// race-free. fn's error (e.g. job.ErrConflict) is returned verbatim so callers
// can errors.Is on it.
func (r *sqliteJobRepository) mutate(ctx context.Context, id uuid.UUID, fn func(*job.Job) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var data []byte
	err = tx.QueryRowContext(ctx, `SELECT data FROM jobs WHERE id = ?`, id.String()).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return job.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("query job: %w", err)
	}
	j, err := decodeJobMeta(data)
	if err != nil {
		return fmt.Errorf("unmarshal job: %w", err)
	}
	if err := fn(&j); err != nil {
		return err
	}
	j.UpdatedAt = time.Now().UTC()
	j.KillPlan = nil
	updated, err := json.Marshal(&j)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE jobs SET data = ?, status = ?, updated_at = ? WHERE id = ?`,
		updated, j.Status.String(), j.UpdatedAt.UnixNano(), id.String(),
	); err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	return tx.Commit()
}

func marshalJobDocuments(j *job.Job) (data, planJSON []byte, err error) {
	if j.KillPlan != nil {
		planJSON, err = json.Marshal(j.KillPlan)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal kill plan: %w", err)
		}
	}
	meta := *j
	meta.KillPlan = nil
	data, err = json.Marshal(&meta)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal job: %w", err)
	}
	return data, planJSON, nil
}

func decodeJobRow(data, planJSON []byte) (job.Job, error) {
	var j job.Job
	if err := json.Unmarshal(data, &j); err != nil {
		return job.Job{}, err
	}
	if len(planJSON) == 0 {
		return j, nil
	}
	var plan killplan.Plan
	if err := json.Unmarshal(planJSON, &plan); err != nil {
		return job.Job{}, err
	}
	j.KillPlan = &plan
	return j, nil
}

func decodeJobMeta(data []byte) (job.Job, error) {
	var j job.Job
	if err := json.Unmarshal(data, &j); err != nil {
		return job.Job{}, err
	}
	j.KillPlan = nil
	return j, nil
}

func nullableBlob(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
