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

	"github.com/rechedev9/fragforge/internal/anticheat"
	"github.com/rechedev9/fragforge/internal/artifacts"
	"github.com/rechedev9/fragforge/internal/job"
	"github.com/rechedev9/fragforge/internal/tasks"
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
	j, err := w.repo.GetMeta(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load job %s: %w", jobID, err)
	}

	doc := anticheat.NewRunningDocument(j.ID.String(), time.Now())
	if err := w.putAnticheatDocument(j.ID, doc); err != nil {
		return err
	}

	report, analyzeErr := w.analyzeAnticheat(ctx, j)
	if analyzeErr != nil {
		// A cancelled context means the process is going away, not that the
		// demo is unscreenable; leave the document running so a restart can
		// pick it up instead of recording a false failure.
		if ctx.Err() != nil {
			return analyzeErr
		}
		recordWorkerFailure(j.ID, tasks.TypeAnalyzeAnticheat, analyzeErr)
		logWorkerError(j.ID, "anticheat", analyzeErr)
		return w.putAnticheatDocument(j.ID, doc.Fail(analyzeErr.Error(), time.Now()))
	}

	if err := w.putAnticheatDocument(j.ID, doc.Complete(report, time.Now())); err != nil {
		return err
	}
	logWorkerArtifacts(j.ID, tasks.TypeAnalyzeAnticheat, []string{artifacts.AnticheatKey(j.ID)})
	return nil
}

func (w *ParserWorker) analyzeAnticheat(ctx context.Context, j job.Job) (anticheat.Report, error) {
	demo, cleanup, err := w.openDemo(j.DemoPath)
	if err != nil {
		return anticheat.Report{}, err
	}
	defer cleanup()

	p := demoinfocs.NewParser(demo)
	defer p.Close()

	return anticheat.AnalyzeWithContext(ctx, p, anticheat.Options{
		DemoPath: j.DemoFileName,
		SHA256:   j.DemoSHA256,
	})
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
