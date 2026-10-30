package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tactical"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// libraryClassWriteRecord is the failure class of a scan whose result the
// library could not store.
const libraryClassWriteRecord = "write_library"

// LibraryRepository is the slice of the library store the library worker uses.
type LibraryRepository interface {
	GetDemo(ctx context.Context, id uuid.UUID) (library.Demo, error)
	SetDemoState(ctx context.Context, id uuid.UUID, state library.ParseState, errorCode, message string) error
	SaveAnalysis(ctx context.Context, analysis library.Analysis) (library.Match, error)
}

// libraryTreeDeleter is the storage capability that removes a demo's folder
// when the demo was deleted while its scan ran.
type libraryTreeDeleter interface {
	DeleteTree(key string) error
}

// LibraryWorker handles "analyze:library-demo": it runs the tactical scan over
// an imported demo, stores the document and position blob next to the demo,
// and records the match, its teams and its players in the library.
type LibraryWorker struct {
	repo    LibraryRepository
	storage storage.Storage
}

// NewLibraryWorker returns a worker that processes library demo scans.
func NewLibraryWorker(repo LibraryRepository, store storage.Storage) *LibraryWorker {
	return &LibraryWorker{repo: repo, storage: store}
}

// HandleAnalyzeLibraryDemo is the Asynq handler signature.
func (w *LibraryWorker) HandleAnalyzeLibraryDemo(ctx context.Context, t *asynq.Task) error {
	var payload tasks.AnalyzeLibraryDemoPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return w.ProcessAnalyzeLibraryDemo(ctx, payload.DemoID)
}

// ProcessAnalyzeLibraryDemo scans one library demo. A demo that is already
// ready is skipped, and a demo deleted before or during the scan is dropped
// without an error, since there is nothing left to retry for.
func (w *LibraryWorker) ProcessAnalyzeLibraryDemo(ctx context.Context, id uuid.UUID) error {
	d, err := w.repo.GetDemo(ctx, id)
	if errors.Is(err, library.ErrNotFound) {
		log.Printf("worker demo=%s task=%s skip=deleted", id, tasks.TypeAnalyzeLibraryDemo)
		return nil
	}
	if err != nil {
		return fmt.Errorf("load library demo %s: %w", id, err)
	}
	if d.State == library.ParseReady {
		logWorkerSkip(id, tasks.TypeAnalyzeLibraryDemo, nil)
		return nil
	}
	if err := w.repo.SetDemoState(ctx, id, library.ParseParsing, "", ""); err != nil {
		return fmt.Errorf("mark library demo parsing: %w", err)
	}

	analysis, scanErr := w.analyze(ctx, d)
	if scanErr == nil {
		_, err := w.repo.SaveAnalysis(ctx, analysis)
		if errors.Is(err, library.ErrNotFound) {
			w.discard(id)
			return nil
		}
		if err != nil {
			scanErr = newTacticalFailure(libraryClassWriteRecord, fmt.Errorf("save library analysis: %w", err))
		}
	}
	if scanErr != nil {
		class := tacticalFailureClass(scanErr)
		if err := w.repo.SetDemoState(ctx, id, library.ParseFailed, class, scanErr.Error()); err != nil && !errors.Is(err, library.ErrNotFound) {
			logWorkerError(id, "write library demo state", err)
		}
		if !taskIsTerminal(ctx) {
			logWorkerError(id, tasks.TypeAnalyzeLibraryDemo+" will retry", scanErr)
			return scanErr
		}
		recordStageFailure(id, obs.StageTactical, tasks.TypeAnalyzeLibraryDemo, class, scanErr)
		logWorkerError(id, tasks.TypeAnalyzeLibraryDemo+" failed", scanErr)
		return scanErr
	}
	logWorkerArtifacts(id, tasks.TypeAnalyzeLibraryDemo, []string{artifacts.LibraryTacticalIndexKey(id), artifacts.LibraryTacticalPositionsKey(id)})
	return nil
}

// analyze scans the demo, publishes the tactical artifacts and summarizes the
// document for the library.
func (w *LibraryWorker) analyze(ctx context.Context, d library.Demo) (library.Analysis, error) {
	demoPath, cleanup, err := localDemoFile(w.storage, d.Path, d.ID, "library")
	if err != nil {
		return library.Analysis{}, newTacticalFailure(tacticalClassDemoUnreadable, err)
	}
	defer cleanup()

	result, err := tactical.ScanFile(ctx, demoPath, tactical.Options{SHA256: d.SHA256})
	if err != nil {
		return library.Analysis{}, fmt.Errorf("scan tactical: %w", err)
	}
	if !result.Document.Geometry.Calibration.Valid() {
		return library.Analysis{}, newTacticalFailure(tacticalClassMapUncalibrated, fmt.Errorf("map %q has no usable radar calibration", result.Document.Demo.Map))
	}
	// The stored document names the demo by its library key, not by the
	// temporary or resolved path the scan happened to read.
	result.Document.Demo.Path = d.Path
	b, err := json.MarshalIndent(result.Document, "", "  ")
	if err != nil {
		return library.Analysis{}, newTacticalFailure(tacticalClassWriteArtifact, fmt.Errorf("encode tactical document: %w", err))
	}
	if err := w.storage.Put(artifacts.LibraryTacticalPositionsKey(d.ID), bytes.NewReader(result.Positions.Data)); err != nil {
		return library.Analysis{}, newTacticalFailure(tacticalClassWriteArtifact, fmt.Errorf("store tactical positions: %w", err))
	}
	if err := w.storage.Put(artifacts.LibraryTacticalIndexKey(d.ID), bytes.NewReader(append(b, '\n'))); err != nil {
		return library.Analysis{}, newTacticalFailure(tacticalClassWriteArtifact, fmt.Errorf("store tactical document: %w", err))
	}

	analysis := library.AnalysisFromDocument(d.ID, result.Document)
	analysis.Source = library.DetectSource(result.Header.ServerName, d.FileName)
	analysis.CS2Build = result.Header.BuildNum
	analysis.PatchVersion = result.Header.PatchVersion
	return analysis, nil
}

// discard removes the folder of a demo deleted while its scan ran, so the
// artifacts the scan wrote do not outlive the record.
func (w *LibraryWorker) discard(id uuid.UUID) {
	deleter, ok := w.storage.(libraryTreeDeleter)
	if !ok {
		return
	}
	if err := deleter.DeleteTree(artifacts.LibraryDemoPrefix(id)); err != nil {
		logWorkerError(id, "remove deleted library demo", err)
	}
}
