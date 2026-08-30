package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"github.com/rechedev9/cliphub/internal/anticheat"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/jobprogress"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// HandleAnalyzeAnticheat is the Asynq handler for analyze:anticheat.
func (w *ParserWorker) HandleAnalyzeAnticheat(ctx context.Context, t *asynq.Task) error {
	var payload tasks.AnalyzeAnticheatPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return w.ProcessAnalyzeAnticheat(ctx, payload.JobID)
}

// ProcessAnalyzeAnticheat screens one job's demo for cheat-suspicion signals.
//
// The result lands in the job's anticheat document and nowhere else: the job
// status is untouched, so a screening can run beside a clip render and a
// screening failure never makes a healthy job look broken. A failure is
// therefore reported into the document (and the obs journal) rather than
// returned, because there is no job state for the queue to retry into.
func (w *ParserWorker) ProcessAnalyzeAnticheat(ctx context.Context, jobID uuid.UUID) error {
	doc := anticheat.NewRunningDocument(jobID.String(), time.Now())
	if err := w.putAnticheatDocument(jobID, doc); err != nil {
		return err
	}

	j, err := w.repo.GetMeta(ctx, jobID)
	if err != nil {
		// The document is already claimed at this point, so a job that cannot
		// be loaded has to be recorded as a failure; returning would leave the
		// lane reading "running" with nothing left to finish it.
		return w.failAnticheat(ctx, jobID, doc, fmt.Errorf("load job %s: %w", jobID, err))
	}

	report, analyzeErr := w.analyzeAnticheat(ctx, j)
	if analyzeErr != nil {
		return w.failAnticheat(ctx, j.ID, doc, analyzeErr)
	}

	if err := w.putAnticheatDocument(j.ID, doc.Complete(report, time.Now())); err != nil {
		return err
	}
	logWorkerArtifacts(j.ID, tasks.TypeAnalyzeAnticheat, []string{artifacts.AnticheatKey(j.ID)})
	return nil
}

// failAnticheat records cause in the job's analysis document. An intermediate
// cancellation remains running for the queue's retry, but the final attempt
// must become failed: otherwise the UI polls an abandoned running document
// until the claim TTL expires.
func (w *ParserWorker) failAnticheat(ctx context.Context, jobID uuid.UUID, doc anticheat.Document, cause error) error {
	if ctx.Err() != nil && !taskIsTerminal(ctx) {
		return cause
	}
	recordStageFailure(jobID, obs.StageWorker, tasks.TypeAnalyzeAnticheat, cause)
	logWorkerError(jobID, "anticheat", cause)
	return w.putAnticheatDocument(jobID, doc.Fail(cause.Error(), time.Now()))
}

func (w *ParserWorker) analyzeAnticheat(ctx context.Context, j job.Job) (anticheat.Report, error) {
	demo, cleanup, err := w.openDemo(j.DemoPath)
	if err != nil {
		return anticheat.Report{}, err
	}
	defer cleanup()

	p := demoinfocs.NewParser(demo)
	defer p.Close()

	rep := jobprogress.NewKeyedReporter(w.storage, artifacts.AnticheatProgressKey(j.ID), jobprogress.StageAnticheat, jobprogress.UnitTicks, "ticks")
	_ = rep.Update(0, 0)
	parser.AttachTickProgress(p, func(done, total int) {
		_ = rep.Update(int64(done), int64(total))
	})

	report, err := anticheat.AnalyzeWithContext(ctx, p, anticheat.Options{
		DemoPath: j.DemoFileName,
		SHA256:   j.DemoSHA256,
	})
	if err != nil {
		return report, err
	}
	_ = rep.Complete(int64(p.CurrentFrame()))
	return report, nil
}

func (w *ParserWorker) putAnticheatDocument(id uuid.UUID, doc anticheat.Document) error {
	var buf bytes.Buffer
	if err := doc.Encode(&buf); err != nil {
		return err
	}
	if err := w.storage.Put(artifacts.AnticheatKey(id), bytes.NewReader(buf.Bytes())); err != nil {
		return fmt.Errorf("store anticheat document: %w", err)
	}
	return nil
}
