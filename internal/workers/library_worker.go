package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/patterndetect"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tactical"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// libraryClassWriteRecord is the failure class of a scan whose result the
// library could not store.
const libraryClassWriteRecord = "write_library"

// libraryScanCrashLimit is how many scans of one demo may die in a fatal error (a parser out of memory)
// before the demo is failed instead of resumed at the next start.
const libraryScanCrashLimit = 2

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
	repo     LibraryRepository
	storage  storage.Storage
	seriesMu sync.Mutex
	// scanMu lets scans run together, except a demo suspected of a crash, which scans alone.
	scanMu  sync.RWMutex
	boot    string
	crashed func(boot string) bool
}

// NewLibraryWorker returns a worker that processes library demo scans.
func NewLibraryWorker(repo LibraryRepository, store storage.Storage) *LibraryWorker {
	return &LibraryWorker{repo: repo, storage: store, boot: uuid.NewString()}
}

// UseBoot names this process for the scans it runs and tells whether an earlier boot died of a
// fatal error, so a scan that boot left running counts against its demo. Call it before the queue starts.
func (w *LibraryWorker) UseBoot(boot string, crashed func(boot string) bool) {
	w.boot = boot
	w.crashed = crashed
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
	crashes := w.settleScan(id)
	if crashes >= libraryScanCrashLimit {
		w.failCrashingDemo(ctx, id)
		return nil
	}
	unlock, err := w.lockScan(ctx, crashes > 0)
	if err != nil {
		return err
	}
	defer unlock()
	if err := w.repo.SetDemoState(ctx, id, library.ParseParsing, "", ""); err != nil {
		return fmt.Errorf("mark library demo parsing: %w", err)
	}

	analysis, scanErr := w.analyze(ctx, d)
	// Not deferred: a scan that panics leaves its mark, as the panic stops the process.
	w.endScan(id)
	var match library.Match
	if scanErr == nil {
		var err error
		match, err = w.repo.SaveAnalysis(ctx, analysis)
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
		if err := w.repo.SetDemoState(ctx, id, library.ParseFailed, class, scanErr.Error()); errors.Is(err, library.ErrNotFound) {
			w.discard(id)
			return nil
		} else if err != nil {
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
	w.groupFaceitSeries(ctx, d)
	w.detectPatterns(ctx, match)
	return nil
}

// detectPatterns reads the saved patterns of the map over the match just analysed. It never
// fails the scan: a pattern read later detects the match itself.
func (w *LibraryWorker) detectPatterns(ctx context.Context, m library.Match) {
	repo, ok := w.repo.(patterndetect.Repository)
	if !ok {
		return
	}
	if err := patterndetect.New(repo, w.storage).DetectMatch(ctx, m); err != nil {
		logWorkerError(m.DemoID, "detect saved patterns", err)
	}
}

// analyze scans the demo, publishes the tactical artifacts and summarizes the
// document for the library.
func (w *LibraryWorker) analyze(ctx context.Context, d library.Demo) (library.Analysis, error) {
	demoPath, cleanup, err := localDemoFile(w.storage, d.Path, d.ID, "library")
	if err != nil {
		return library.Analysis{}, newTacticalFailure(tacticalClassDemoUnreadable, err)
	}
	defer cleanup()
	w.putScanText(artifacts.LibraryScanRunningKey(d.ID), w.boot)
	if err := checkLibraryDemo(ctx, demoPath); err != nil {
		class := tacticalClassDemoUnreadable
		if errors.Is(err, parser.ErrTruncatedDemo) {
			class = obs.ClassTruncatedDemo
		}
		return library.Analysis{}, newTacticalFailure(class, err)
	}

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

// checkLibraryDemo refuses a cut demo before the tactical scan, which reads it
// without complaint. A recording CS2 never closed gets a roster pass, the parse
// that tells whether it was cut before the first round.
func checkLibraryDemo(ctx context.Context, path string) error {
	// #nosec G304 -- the path is the worker's own local copy of a stored demo.
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open demo: %w", err)
	}
	defer f.Close()
	header, err := checkDemoFileLength(f)
	if err != nil || !parser.DemoRecordingUnclosed(header) {
		return err
	}
	p := demoinfocs.NewParser(f)
	defer p.Close()
	// Any other failure is the tactical scan's to report.
	if _, err := parser.RosterScanWithContext(ctx, p); errors.Is(err, parser.ErrTruncatedDemo) {
		return err
	}
	return ctx.Err()
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

// failCrashingDemo fails a demo whose scans kept dying in a fatal error; Analizar runs it again on request.
func (w *LibraryWorker) failCrashingDemo(ctx context.Context, id uuid.UUID) {
	err := fmt.Errorf("the scan of this demo stopped Studio %d times; the demo may be damaged", libraryScanCrashLimit)
	stateErr := w.repo.SetDemoState(ctx, id, library.ParseFailed, tacticalClassDemoUnreadable, err.Error())
	if errors.Is(stateErr, library.ErrNotFound) {
		w.discard(id)
		return
	}
	if stateErr != nil {
		logWorkerError(id, "write library demo state", stateErr)
	}
	w.putScanText(artifacts.LibraryScanCrashesKey(id), "0")
	recordStageFailure(id, obs.StageTactical, tasks.TypeAnalyzeLibraryDemo, tacticalClassDemoUnreadable, err)
	logWorkerError(id, tasks.TypeAnalyzeLibraryDemo+" failed", err)
}

// settleScan returns the demo's crash count. A scan an earlier process left running counts
// only when that process died of a fatal error: closing Studio kills it too, without one.
func (w *LibraryWorker) settleScan(id uuid.UUID) int {
	crashes, _ := strconv.Atoi(w.scanText(artifacts.LibraryScanCrashesKey(id)))
	running := w.scanText(artifacts.LibraryScanRunningKey(id))
	if running == "" || running == w.boot {
		return crashes
	}
	w.putScanText(artifacts.LibraryScanRunningKey(id), "")
	if w.crashed == nil || !w.crashed(running) {
		return crashes
	}
	crashes++
	w.putScanText(artifacts.LibraryScanCrashesKey(id), strconv.Itoa(crashes))
	return crashes
}

// endScan records that the scan returned, so the demo is no longer suspected of a crash.
func (w *LibraryWorker) endScan(id uuid.UUID) {
	if w.scanText(artifacts.LibraryScanRunningKey(id)) != "" {
		w.putScanText(artifacts.LibraryScanRunningKey(id), "")
	}
	if c := w.scanText(artifacts.LibraryScanCrashesKey(id)); c != "" && c != "0" {
		w.putScanText(artifacts.LibraryScanCrashesKey(id), "0")
	}
}

// lockScan waits for its turn to scan, alone when the demo is suspected of a crash, or until ctx ends.
func (w *LibraryWorker) lockScan(ctx context.Context, alone bool) (func(), error) {
	try, unlock := w.scanMu.TryRLock, w.scanMu.RUnlock
	if alone {
		try, unlock = w.scanMu.TryLock, w.scanMu.Unlock
	}
	for !try() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return unlock, nil
}

// scanText reads a small scan record; a missing or unreadable one is empty.
func (w *LibraryWorker) scanText(key string) string {
	rc, err := w.storage.Open(key)
	if err != nil {
		return ""
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, 64))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (w *LibraryWorker) putScanText(key, text string) {
	if err := w.storage.Put(key, strings.NewReader(text)); err != nil {
		log.Printf("worker: write %s: %v", key, err)
	}
}
