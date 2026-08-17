package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tactical"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// Tactical failure classes recorded under obs.StageTactical. They double as the
// `class` metric label, so keep them short and stable.
const (
	tacticalClassDemoUnreadable   = "demo_unreadable"
	tacticalClassDemoIncompatible = "demo_incompatible"
	tacticalClassMapUncalibrated  = "map_uncalibrated"
	tacticalClassWriteArtifact    = "write_artifact"
)

// TacticalRepository is the subset of *job.Repository the tactical worker needs.
// The scan only ever reads the demo: the analysis is an orthogonal artifact
// whose readiness lives in its own status document, so the worker never writes
// the job's status.
type TacticalRepository interface {
	GetMeta(ctx context.Context, id uuid.UUID) (job.Job, error)
}

// demoPathResolver is the optional storage capability that hands back an
// artifact's local filesystem path, so the scan reads a multi-hundred-megabyte
// demo in place instead of copying it through Open first.
type demoPathResolver interface {
	ResolvePath(key string) (string, error)
}

// TacticalWorker handles the "analyze:tactical" Asynq task: it scans a job's
// demo into the durable tactical document and its sidecar position blob.
//
// Unlike recording, the scan is pure CPU over an immutable demo and touches no
// external process, so it is safe to retry and it stays on the default queue
// lane rather than the serial capture lane that exists for cs2.exe. A retry
// skips the work entirely once both durable artifacts exist.
type TacticalWorker struct {
	repo    TacticalRepository
	storage storage.Storage
}

// NewTacticalWorker returns a worker that processes analyze:tactical tasks.
func NewTacticalWorker(repo TacticalRepository, store storage.Storage) *TacticalWorker {
	return &TacticalWorker{repo: repo, storage: store}
}

// HandleAnalyzeTactical is the Asynq handler signature.
func (w *TacticalWorker) HandleAnalyzeTactical(ctx context.Context, t *asynq.Task) error {
	var payload tasks.AnalyzeTacticalPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return w.ProcessAnalyzeTactical(ctx, payload.JobID, payload.SampleHZ)
}

// ProcessAnalyzeTactical runs the tactical scan for one job, independent of any
// queue. Readiness is modelled by artifact presence plus the status document,
// so the job's own status is only touched when the scan fails.
func (w *TacticalWorker) ProcessAnalyzeTactical(ctx context.Context, jobID uuid.UUID, sampleHZ float64) error {
	if sampleHZ == 0 {
		sampleHZ = tactical.DefaultSampleHZ
	}
	if sampleHZ != tactical.DefaultSampleHZ {
		return fmt.Errorf("job tactical sample_hz %.3f is not canonical %.3f", sampleHZ, float64(tactical.DefaultSampleHZ))
	}
	j, err := w.repo.GetMeta(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load job %s: %w", jobID, err)
	}
	indexKey := artifacts.TacticalIndexKey(j.ID)
	positionsKey := artifacts.TacticalPositionsKey(j.ID)
	statusKey := artifacts.TacticalStatusKey(j.ID)

	ready, err := w.artifactsReady(statusKey, sampleHZ, indexKey, positionsKey)
	if err != nil {
		return fmt.Errorf("check tactical artifacts: %w", err)
	}
	if ready {
		logWorkerSkip(j.ID, tasks.TypeAnalyzeTactical, []string{indexKey, positionsKey, statusKey})
		return nil
	}
	if err := w.writeStatus(j.ID, sampleHZ, artifacts.TacticalStateRunning, ""); err != nil {
		return fmt.Errorf("write tactical status: %w", err)
	}

	if scanErr := w.analyze(ctx, j, sampleHZ, indexKey, positionsKey); scanErr != nil {
		// The status document records every attempt so a reader learns why the
		// analysis is missing. The job's own status is deliberately left alone:
		// a tactical scan is an optional artifact, and failing one must not
		// flip a job whose video already shipped to failed.
		if err := w.writeStatus(j.ID, sampleHZ, artifacts.TacticalStateFailed, scanErr.Error()); err != nil {
			logWorkerError(j.ID, "write tactical status", err)
		}
		if !taskIsTerminal(ctx) {
			logWorkerError(j.ID, tasks.TypeAnalyzeTactical+" will retry", scanErr)
			return scanErr
		}
		recordStageFailure(j.ID, obs.StageTactical, tacticalFailureClass(scanErr), scanErr)
		logWorkerError(j.ID, tasks.TypeAnalyzeTactical+" failed", scanErr)
		return scanErr
	}

	logWorkerArtifacts(j.ID, tasks.TypeAnalyzeTactical, []string{indexKey, positionsKey})
	if err := w.writeStatus(j.ID, sampleHZ, artifacts.TacticalStateReady, ""); err != nil {
		return fmt.Errorf("write tactical status: %w", err)
	}
	return nil
}

// analyze scans the demo and publishes both durable artifacts.
func (w *TacticalWorker) analyze(ctx context.Context, j job.Job, sampleHZ float64, indexKey, positionsKey string) error {
	demoPath, cleanup, err := w.demoFile(j)
	if err != nil {
		return newTacticalFailure(tacticalClassDemoUnreadable, err)
	}
	defer cleanup()

	result, err := tactical.ScanFile(ctx, demoPath, tactical.Options{
		SHA256:   j.DemoSHA256,
		JobID:    j.ID,
		SampleHZ: sampleHZ,
	})
	if err != nil {
		return fmt.Errorf("scan tactical: %w", err)
	}
	// Without a calibration no position can be placed on a radar, so an
	// undrawable document is a failure rather than a half-usable artifact.
	if !result.Document.Geometry.Calibration.Valid() {
		return newTacticalFailure(tacticalClassMapUncalibrated, fmt.Errorf("map %q has no usable radar calibration", result.Document.Demo.Map))
	}

	b, err := json.MarshalIndent(result.Document, "", "  ")
	if err != nil {
		return newTacticalFailure(tacticalClassWriteArtifact, fmt.Errorf("encode tactical document: %w", err))
	}
	// The blob lands first: the document is the index into it, so a reader that
	// sees the document always finds the bytes it describes.
	if err := w.storage.Put(positionsKey, bytes.NewReader(result.Positions.Data)); err != nil {
		return newTacticalFailure(tacticalClassWriteArtifact, fmt.Errorf("store tactical positions: %w", err))
	}
	if err := w.storage.Put(indexKey, bytes.NewReader(append(b, '\n'))); err != nil {
		return newTacticalFailure(tacticalClassWriteArtifact, fmt.Errorf("store tactical document: %w", err))
	}
	return nil
}

// artifactsReady reports whether a previous run published the two durable
// artifacts under a ready status for the current schema and canonical sampling
// rate. Presence alone is insufficient: older output at another rate cannot be
// reused as if it satisfied today's contract.
func (w *TacticalWorker) artifactsReady(
	statusKey string,
	sampleHZ float64,
	keys ...string,
) (bool, error) {
	for _, key := range keys {
		exists, err := w.storage.Exists(key)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}
	exists, err := w.storage.Exists(statusKey)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	r, err := w.storage.Open(statusKey)
	if err != nil {
		return false, err
	}
	defer r.Close()
	var status artifacts.TacticalStatus
	if err := json.NewDecoder(r).Decode(&status); err != nil {
		return false, fmt.Errorf("decode %s: %w", statusKey, err)
	}
	return status.State == artifacts.TacticalStateReady &&
		status.SchemaVersion == tacticalplan.SchemaVersion &&
		status.SampleHZ == sampleHZ, nil
}

func (w *TacticalWorker) writeStatus(id uuid.UUID, sampleHZ float64, state, failure string) error {
	return putJSONToStorage(w.storage, artifacts.TacticalStatusKey(id), artifacts.TacticalStatus{
		State:         state,
		GeneratedAt:   time.Now().UTC(),
		SchemaVersion: tacticalplan.SchemaVersion,
		SampleHZ:      sampleHZ,
		Error:         failure,
	})
}

// demoFile materializes the job's demo as a local file the scan can open. Local
// storage resolves the stored demo in place; any other backend is copied to a
// temporary file, mirroring ParserWorker.openDemo. A resolved path that is not
// readable falls through to the copy so the caller sees one clear error instead
// of a backend-specific one. The returned cleanup must be deferred.
func (w *TacticalWorker) demoFile(j job.Job) (string, func(), error) {
	if resolver, ok := w.storage.(demoPathResolver); ok {
		if path, err := resolver.ResolvePath(j.DemoPath); err == nil {
			if _, err := os.Stat(path); err == nil {
				return path, func() {}, nil
			}
		}
	}
	dir, cleanup, err := prepareStageDir("", j.ID, "tactical")
	if err != nil {
		return "", nil, fmt.Errorf("create tactical work dir: %w", err)
	}
	local := filepath.Join(dir, "demo.dem")
	if err := copyStorageToFile(w.storage, j.DemoPath, local); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("open demo %q: %w", j.DemoPath, err)
	}
	return local, cleanup, nil
}

// tacticalFailure carries the obs class of a scan failure while preserving the
// original error for logs and tests.
type tacticalFailure struct {
	class string
	err   error
}

func (f *tacticalFailure) Error() string { return f.err.Error() }

func (f *tacticalFailure) Unwrap() error { return f.err }

func newTacticalFailure(class string, err error) error {
	return &tacticalFailure{class: class, err: err}
}

// tacticalFailureClass reports the obs class for a scan failure. A demo the
// parser refuses outright is incompatible; anything else that goes wrong while
// reading it is reported as unreadable, since the demo is the only input.
func tacticalFailureClass(err error) string {
	if f, ok := errors.AsType[*tacticalFailure](err); ok {
		return f.class
	}
	if errors.Is(err, demoinfocs.ErrInvalidFileType) {
		return tacticalClassDemoIncompatible
	}
	return tacticalClassDemoUnreadable
}
