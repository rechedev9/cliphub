// Package workers implements the Asynq task handlers that drive the
// orchestrator's pipeline. Each worker is a thin glue layer that pulls
// a job from the repository, delegates the heavy lifting to a domain
// package (parser, composer, ...), and writes the result back.
package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/moments"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// JobRepository is the subset of *job.Repository the worker needs.
type JobRepository interface {
	GetMeta(ctx context.Context, id uuid.UUID) (job.Job, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, s job.Status, failureReason string) error
	SetKillPlan(ctx context.Context, id uuid.UUID, plan killplan.Plan) error
}

// ParserWorker handles the "parse:demo" Asynq task.
type ParserWorker struct {
	repo    JobRepository
	storage storage.Storage
}

// NewParserWorker returns a worker that processes parse:demo tasks.
func NewParserWorker(repo JobRepository, store storage.Storage) *ParserWorker {
	return &ParserWorker{repo: repo, storage: store}
}

// HandleParseDemo is the Asynq handler signature.
func (w *ParserWorker) HandleParseDemo(ctx context.Context, t *asynq.Task) error {
	var payload tasks.ParseDemoPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return w.ProcessParseDemo(ctx, payload.JobID)
}

// ProcessParseDemo runs the parse stage for one job, independent of any queue.
func (w *ParserWorker) ProcessParseDemo(ctx context.Context, jobID uuid.UUID) error {
	j, err := w.repo.GetMeta(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load job %s: %w", jobID, err)
	}
	if err := w.repo.UpdateStatus(ctx, j.ID, job.StatusParsing, ""); err != nil {
		return fmt.Errorf("mark parsing: %w", err)
	}
	logWorkerTransition(j.ID, tasks.TypeParseDemo, job.StatusParsing)

	plan, parseErr := w.parse(ctx, j)
	if parseErr != nil {
		return w.recordParserFailure(ctx, j.ID, tasks.TypeParseDemo, parseErr)
	}
	if err := w.repo.SetKillPlan(ctx, j.ID, plan); err != nil {
		return fmt.Errorf("save plan: %w", err)
	}
	momentsKey, err := w.writeMoments(j.ID, plan)
	if err != nil {
		return fmt.Errorf("write moments: %w", err)
	}
	logWorkerArtifacts(j.ID, tasks.TypeParseDemo, []string{"kill_plan", momentsKey})
	if err := w.repo.UpdateStatus(ctx, j.ID, job.StatusParsed, ""); err != nil {
		return fmt.Errorf("mark parsed: %w", err)
	}
	logWorkerTransition(j.ID, tasks.TypeParseDemo, job.StatusParsed)
	return nil
}

// HandleScanRoster is the Asynq handler for scan:roster.
func (w *ParserWorker) HandleScanRoster(ctx context.Context, t *asynq.Task) error {
	var payload tasks.ScanRosterPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return w.ProcessScanRoster(ctx, payload.JobID)
}

// ProcessScanRoster runs the roster scan for one job, independent of any
// queue. It scans the demo's roster once so the user can pick a target
// before a full parse, mirroring ProcessParseDemo's status transitions and
// failure handling.
func (w *ParserWorker) ProcessScanRoster(ctx context.Context, jobID uuid.UUID) error {
	j, err := w.repo.GetMeta(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load job %s: %w", jobID, err)
	}
	if err := w.repo.UpdateStatus(ctx, j.ID, job.StatusScanning, ""); err != nil {
		return fmt.Errorf("mark scanning: %w", err)
	}
	logWorkerTransition(j.ID, tasks.TypeScanRoster, job.StatusScanning)

	rosterKey, scanErr := w.scanRoster(ctx, j)
	if scanErr != nil {
		return w.recordParserFailure(ctx, j.ID, tasks.TypeScanRoster, scanErr)
	}
	logWorkerArtifacts(j.ID, tasks.TypeScanRoster, []string{rosterKey})
	if err := w.repo.UpdateStatus(ctx, j.ID, job.StatusScanned, ""); err != nil {
		return fmt.Errorf("mark scanned: %w", err)
	}
	logWorkerTransition(j.ID, tasks.TypeScanRoster, job.StatusScanned)
	return nil
}

// recordParserFailure records a failed scan or parse. A POV demo, a demo the
// parser cannot read or a target missing from it fails the same way on every
// attempt, so it ends the task now.
func (w *ParserWorker) recordParserFailure(ctx context.Context, id uuid.UUID, taskType string, err error) error {
	class := obs.ClassOf(err.Error())
	final := class == obs.ClassPOVDemo || class == obs.ClassDemoIncompatible || class == obs.ClassTargetNotFound
	if final {
		ctx = tasks.WithTaskAttempt(ctx, 0, 0)
	}
	if statusErr := recordTaskFailure(ctx, w.repo, id, taskType, err); statusErr != nil {
		return errors.Join(err, fmt.Errorf("record %s failure status: %w", taskType, statusErr))
	}
	if final {
		return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
	}
	return err
}

func (w *ParserWorker) parse(ctx context.Context, j job.Job) (killplan.Plan, error) {
	demo, cleanup, err := w.openDemo(j.DemoPath)
	if err != nil {
		return killplan.Plan{}, err
	}
	defer cleanup()

	p := demoinfocs.NewParser(demo)
	defer p.Close()
	// The roster scan already logged a scanned job's demo facts.
	if scanned, err := w.storage.Exists(artifacts.RosterKey(j.ID)); err != nil || !scanned {
		header := parser.TrackDemoHeader(p)
		first := parser.TrackFirstPacketTick(p)
		defer func() { logDemoFacts(j.ID, tasks.TypeParseDemo, p, header, first) }()
	}

	meta := parser.PlanMeta{
		DemoPath: j.DemoPath,
		SHA256:   j.DemoSHA256,
	}
	dual, err := parser.RunKillsAndRecapWithContext(ctx, p, j.TargetSteamID, j.Rules, meta)
	if err != nil {
		if errors.Is(err, parser.ErrTargetNotFound) {
			return killplan.Plan{}, fmt.Errorf("target steamid %s not found in demo", j.TargetSteamID)
		}
		return killplan.Plan{}, err
	}
	if err := recapplan.Store(w.storage, j.ID, dual.Recap); err != nil {
		return killplan.Plan{}, err
	}
	if err := recapplan.StoreFacts(w.storage, j.ID, dual.Facts); err != nil {
		return killplan.Plan{}, fmt.Errorf("store full demo facts: %w", err)
	}
	return dual.Kills, nil
}

func (w *ParserWorker) scanRoster(ctx context.Context, j job.Job) (string, error) {
	demo, cleanup, err := w.openDemo(j.DemoPath)
	if err != nil {
		return "", err
	}
	defer cleanup()

	p := demoinfocs.NewParser(demo)
	defer p.Close()
	header := parser.TrackDemoHeader(p)
	first := parser.TrackFirstPacketTick(p)

	result, err := parser.RosterScanWithContext(ctx, p)
	logDemoFacts(j.ID, tasks.TypeScanRoster, p, header, first)
	if err != nil {
		return "", fmt.Errorf("scan roster: %w", err)
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	key := artifacts.RosterKey(j.ID)
	if err := w.storage.Put(key, bytes.NewReader(b)); err != nil {
		return "", fmt.Errorf("store roster: %w", err)
	}
	return key, nil
}

// openDemo returns a local demo file directly when storage already provides
// one. Other storage implementations are copied to a temporary file so the
// parser retains the same stable, file-backed input it has always received.
// The returned cleanup must be deferred.
func (w *ParserWorker) openDemo(demoPath string) (*os.File, func(), error) {
	rc, err := w.storage.Open(demoPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open demo: %w", err)
	}
	if file, ok := rc.(*os.File); ok {
		return file, func() { _ = file.Close() }, nil
	}
	defer rc.Close()

	tmp, err := os.CreateTemp("", "zv-demo-*.dem")
	if err != nil {
		return nil, nil, fmt.Errorf("temp file: %w", err)
	}
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}

	if _, err := io.Copy(tmp, rc); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("write temp demo: %w", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, nil, err
	}
	return tmp, cleanup, nil
}

func (w *ParserWorker) writeMoments(id uuid.UUID, plan killplan.Plan) (string, error) {
	doc := moments.Build(id, plan)
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	key := moments.ArtifactKey(id)
	if err := w.storage.Put(key, bytes.NewReader(b)); err != nil {
		return "", err
	}
	return key, nil
}
