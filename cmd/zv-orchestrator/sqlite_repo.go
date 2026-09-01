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
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/rules"
)

// sqliteJobRepository persists jobs in a local SQLite file so job state survives
// an orchestrator restart, unlike the in-memory repository. It is the default
// for the local desktop studio, which has no Postgres: job metadata lives in
// the `data` JSON document, and the kill plan is a sibling `job_kill_plans`
// row so GetMeta/List/GetStatus/UpdateStatus never load it. status/created_at/
// updated_at are mirrored into columns for List ordering. modernc.org/sqlite is
// a pure-Go driver, so no CGO or C toolchain is needed on Windows or in the
// static build.
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
		status     TEXT    NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create jobs table: %w", err)
	}
	if err := ensureJobKillPlansTable(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrateKillPlansOffJobsRow(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &sqliteJobRepository{db: db}, nil
}

func ensureJobKillPlansTable(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS job_kill_plans (
		job_id TEXT PRIMARY KEY,
		plan   BLOB NOT NULL
	)`); err != nil {
		return fmt.Errorf("create job_kill_plans: %w", err)
	}
	return nil
}

func jobsColumnExists(db *sql.DB, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(jobs)`)
	if err != nil {
		return false, fmt.Errorf("inspect jobs columns: %w", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, fmt.Errorf("scan jobs column: %w", err)
		}
		found = found || name == column
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate jobs columns: %w", err)
	}
	return found, nil
}

// migrateKillPlansOffJobsRow moves a kill plan out of the jobs row — from a
// leftover `kill_plan` column or an embedded `data.kill_plan` — into
// job_kill_plans, then strips both sources. Idempotent: a second open is a
// no-op once the sibling table holds the plan and `data` has no `$.kill_plan`.
func migrateKillPlansOffJobsRow(db *sql.DB) error {
	hasColumn, err := jobsColumnExists(db, "kill_plan")
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin kill plan migrate: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if hasColumn {
		if _, err := tx.Exec(`
			INSERT INTO job_kill_plans (job_id, plan)
			SELECT id, kill_plan FROM jobs
			WHERE kill_plan IS NOT NULL
			  AND id NOT IN (SELECT job_id FROM job_kill_plans)`); err != nil {
			return fmt.Errorf("move kill_plan column: %w", err)
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO job_kill_plans (job_id, plan)
		SELECT id, json_extract(data, '$.kill_plan')
		FROM jobs
		WHERE json_extract(data, '$.kill_plan') IS NOT NULL
		  AND id NOT IN (SELECT job_id FROM job_kill_plans)`); err != nil {
		return fmt.Errorf("extract embedded kill plan: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE jobs
		SET data = json_remove(data, '$.kill_plan')
		WHERE json_type(data, '$.kill_plan') IS NOT NULL`); err != nil {
		return fmt.Errorf("strip embedded kill plan: %w", err)
	}
	if hasColumn {
		if _, err := tx.Exec(`UPDATE jobs SET kill_plan = NULL WHERE kill_plan IS NOT NULL`); err != nil {
			return fmt.Errorf("clear jobs kill_plan column: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit kill plan migrate: %w", err)
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
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin insert job: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO jobs (id, data, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		j.ID.String(), data, j.Status.String(), now.UnixNano(), now.UnixNano(),
	); err != nil {
		return fmt.Errorf("insert job: %w", err)
	}
	if len(planJSON) > 0 {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO job_kill_plans (job_id, plan) VALUES (?, ?)`,
			j.ID.String(), planJSON,
		); err != nil {
			return fmt.Errorf("insert kill plan: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit insert job: %w", err)
	}
	return nil
}

func (r *sqliteJobRepository) Get(ctx context.Context, id uuid.UUID) (job.Job, error) {
	var data, planJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT jobs.data, job_kill_plans.plan
		FROM jobs
		LEFT JOIN job_kill_plans ON job_kill_plans.job_id = jobs.id
		WHERE jobs.id = ?`, id.String()).Scan(&data, &planJSON)
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
		       CASE WHEN status = ? THEN COALESCE((SELECT json_array_length(plan, '$.segments') FROM job_kill_plans WHERE job_id = jobs.id), 0) ELSE 0 END
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
					'$.failure_reason',
					'$.failure_code'
				),
				status = ?,
				updated_at = ?
			WHERE id = ?`,
			status.String(), now.Format(time.RFC3339Nano), status.String(), now.UnixNano(), id.String(),
		)
	} else if code := obs.ClassOf(failureReason); code != "" {
		result, err = r.db.ExecContext(ctx, `
			UPDATE jobs
			SET data = json_set(
					data,
					'$.status', ?,
					'$.failure_reason', ?,
					'$.failure_code', ?,
					'$.updated_at', ?
				),
				status = ?,
				updated_at = ?
			WHERE id = ?`,
			status.String(), failureReason, code, now.Format(time.RFC3339Nano), status.String(), now.UnixNano(), id.String(),
		)
	} else {
		result, err = r.db.ExecContext(ctx, `
			UPDATE jobs
			SET data = json_remove(
					json_set(
						data,
						'$.status', ?,
						'$.failure_reason', ?,
						'$.updated_at', ?
					),
					'$.failure_code'
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

// Delete removes the job row and its kill plan. A missing row is not an error,
// so deletes are idempotent and safe to retry after a failed artifact cleanup.
func (r *sqliteJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete job: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM job_kill_plans WHERE job_id = ?`, id.String()); err != nil {
		return fmt.Errorf("delete kill plan: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id.String()); err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete job: %w", err)
	}
	return nil
}

func (r *sqliteJobRepository) SetKillPlan(ctx context.Context, id uuid.UUID, plan killplan.Plan) error {
	now := time.Now().UTC()
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal kill plan: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update kill plan: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
		UPDATE jobs
		SET data = json_set(data, '$.updated_at', ?),
		    updated_at = ?
		WHERE id = ?`,
		now.Format(time.RFC3339Nano), now.UnixNano(), id.String(),
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
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO job_kill_plans (job_id, plan) VALUES (?, ?)
		ON CONFLICT(job_id) DO UPDATE SET plan = excluded.plan`,
		id.String(), planJSON,
	); err != nil {
		return fmt.Errorf("upsert kill plan: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update kill plan: %w", err)
	}
	return nil
}

// mutate loads job metadata inside a transaction, applies fn, bumps UpdatedAt,
// and writes the metadata document back. job_kill_plans is not selected or
// written: SetKillPlan is the only writer for that blob. The single-connection
// pool serializes writers, so the read-modify-write is race-free. fn's error
// (e.g. job.ErrConflict) is returned verbatim so callers can errors.Is on it.
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
