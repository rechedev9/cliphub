package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/generateintent"
	"github.com/rechedev9/cliphub/internal/httpapi"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tasks"
	"github.com/rechedev9/cliphub/internal/workers"
	"github.com/rechedev9/cliphub/internal/youtubetrends"
)

type orchestratorStreamJobRepository interface {
	httpapi.StreamJobRepository
	streamInterruptSweeper
}

const gracefulShutdownTimeout = 10 * time.Second

const streamAcquireRecoveryDisabledReason = "interrupted: stream acquisition cannot resume because the acquisition worker is disabled"

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := clearLegacyCaptionCredentialsEnvironment(); err != nil {
		return fmt.Errorf("config: clear legacy caption credential from process environment: %w", err)
	}
	if err := clearSubprocessCredentialEnvironment(); err != nil {
		return fmt.Errorf("config: clear subprocess credentials from process environment: %w", err)
	}
	// Auto-detect HLAE/CS2/recorder/editor/ffmpeg on the host so capture and
	// rendering work without the user setting env vars; explicit env still wins.
	// Best-effort, never fatal.
	cfg, captureSource := detectCaptureTools(cfg)
	for _, name := range []string{"ZV_RECORDER_PATH", "ZV_HLAE_PATH", "ZV_CS2_PATH", "ZV_EDITOR_PATH", "ZV_FFMPEG_PATH", "ZV_FFPROBE_PATH"} {
		if captureSource[name] == "detected" {
			log.Printf("capture: auto-detected %s", name)
		}
	}
	log.Printf("capture: record worker enabled=%v", cfg.recordWorkerEnabled())
	log.Printf("capture: render worker enabled=%v", cfg.renderWorkerEnabled())
	if missing := cfg.missingRecordConfig(); len(missing) > 0 {
		log.Printf("capture: record worker disabled, missing after auto-detection: %v", missing)
	}
	if missing := cfg.missingRecordTools(); len(missing) > 0 {
		log.Printf("capture: configured record tool path(s) not found on disk: %v", missing)
	}

	dataLease, err := acquireDataDirLease(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("data directory lease: %w", err)
	}
	defer func() {
		if err := dataLease.Close(); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := storage.NewLocal(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	observability, err := obs.InitializeDefault()
	if err != nil {
		return fmt.Errorf("observability: %w", err)
	}
	generateIntents := generateintent.New(store)
	youtubeTrends, err := youtubetrends.New(youtubetrends.Options{APIKey: cfg.FirecrawlAPIKey})
	if err != nil {
		return fmt.Errorf("youtube trends client: %w", err)
	}
	log.Printf("publish assistant: firecrawl trends enabled=%v", cfg.firecrawlEnabled())
	log.Printf("faceit: data api enabled=%v", cfg.faceitEnabled())

	var faceitClient *faceit.Client
	if cfg.faceitEnabled() {
		faceitClient, err = faceit.New(faceit.Options{APIKey: cfg.FaceitAPIKey})
		if err != nil {
			return fmt.Errorf("faceit client: %w", err)
		}
	}
	faceitFollows, err := faceit.NewFollowStore(filepath.Join(cfg.DataDir, "faceit", "followed.json"), time.Now)
	if err != nil {
		return fmt.Errorf("faceit follow store: %w", err)
	}

	var repo orchestratorJobRepository
	var streamRepo orchestratorStreamJobRepository
	var editorAssets httpapi.EditorAssetRepository
	var editorProjects httpapi.EditorProjectRepository
	switch {
	case cfg.DatabaseURL == databaseURLMemory:
		repo = newMemoryJobRepository()
		streamRepo = newMemoryStreamJobRepository()
		editorAssets = newMemoryEditorAssetRepository()
		editorProjects = newMemoryEditorProjectRepository()
		log.Printf("jobs: using in-memory repository (state resets on restart)")
	case cfg.DatabaseURL == databaseURLSQLite || strings.HasPrefix(cfg.DatabaseURL, databaseURLSQLite+":"):
		path := sqlitePath(cfg.DatabaseURL, cfg.DataDir)
		sqliteRepo, err := newSQLiteJobRepository(path)
		if err != nil {
			return fmt.Errorf("sqlite: %w", err)
		}
		defer func() { _ = sqliteRepo.Close() }()
		repo = sqliteRepo
		sqliteStreamRepo, err := newSQLiteStreamJobRepository(sqliteRepo.db)
		if err != nil {
			return fmt.Errorf("sqlite stream jobs: %w", err)
		}
		streamRepo = sqliteStreamRepo
		sqliteAssets, err := newSQLiteEditorAssetRepository(sqliteRepo.db)
		if err != nil {
			return fmt.Errorf("sqlite editor assets: %w", err)
		}
		sqliteProjects, err := newSQLiteEditorProjectRepository(sqliteRepo.db)
		if err != nil {
			return fmt.Errorf("sqlite editor projects: %w", err)
		}
		editorAssets = sqliteAssets
		editorProjects = sqliteProjects
		log.Printf("jobs: using sqlite repository at %s", path)
	default:
		return fmt.Errorf("unsupported ZV_DATABASE_URL %q: cliphub desktop only supports %q or %q", cfg.DatabaseURL, databaseURLMemory, databaseURLSQLite)
	}

	// Reconcile durable state whose process-local work vanished with the previous
	// desktop process. Run every sweep before serving traffic so clients never
	// observe an active state with no queue owner capable of advancing it.
	reconciled, err := reconcileInterruptedWork(ctx, repo, streamRepo, store, observability)
	if err != nil {
		return fmt.Errorf("startup reconciliation: %w", err)
	}
	if reconciled.total() > 0 {
		log.Printf(
			"startup: reconciled interrupted work (demo_jobs=%d demo_renders=%d generate_runs=%d stream_jobs=%d stream_renders=%d stream_acquisitions=%d)",
			reconciled.DemoJobs,
			reconciled.DemoRenders,
			reconciled.GenerateRuns,
			reconciled.StreamJobs,
			reconciled.StreamRenderStates,
			len(reconciled.StreamAcquisitions),
		)
	}
	// HTTP plan mutations and stream render workers share this per-job
	// coordinator. It closes the Ready->Rendering claim and final pointer-commit
	// races without serializing unrelated stream jobs.
	streamJobLocks := streamclips.NewJobLocks()

	taskHandlers := map[string]taskHandler{}
	parserWorker := workers.NewParserWorker(repo, store)
	taskHandlers[tasks.TypeParseDemo] = parserWorker.HandleParseDemo
	taskHandlers[tasks.TypeScanRoster] = parserWorker.HandleScanRoster
	taskHandlers[tasks.TypeAnalyzeAnticheat] = parserWorker.HandleAnalyzeAnticheat
	tacticalWorker := workers.NewTacticalWorker(repo, store)
	taskHandlers[tasks.TypeAnalyzeTactical] = tacticalWorker.HandleAnalyzeTactical
	var recordWorker *workers.RecordWorker
	if cfg.recordWorkerEnabled() {
		recordWorker = workers.NewRecordWorker(repo, store, workers.RecordWorkerConfig{
			WorkDir:      cfg.MediaWorkDir,
			RecorderPath: cfg.RecorderPath,
			HLAEPath:     cfg.HLAEPath,
			CS2Path:      cfg.CS2Path,
			Timeout:      cfg.RecordTimeout,
			HUDMode:      cfg.RecordHUD,
		})
		taskHandlers[tasks.TypeRecordDemo] = recordWorker.HandleRecordDemo
		log.Printf("worker: record enabled")
	}
	if cfg.composeWorkerEnabled() {
		composeWorker := workers.NewComposeWorker(repo, store, workers.ComposeWorkerConfig{
			WorkDir:      cfg.MediaWorkDir,
			ComposerPath: cfg.ComposerPath,
			FFmpegPath:   cfg.FFmpegPath,
			Timeout:      cfg.ComposeTimeout,
		})
		taskHandlers[tasks.TypeComposeFinal] = composeWorker.HandleComposeFinal
		log.Printf("worker: compose enabled")
	}
	if cfg.renderWorkerEnabled() {
		renderWorker := workers.NewRenderWorker(repo, store, workers.RenderWorkerConfig{
			WorkDir:     cfg.MediaWorkDir,
			EditorPath:  cfg.EditorPath,
			FFmpegPath:  cfg.FFmpegPath,
			FFprobePath: cfg.FFprobePath,
			Timeout:     cfg.RenderTimeout,
			MusicDir:    cfg.MusicDir,
		})
		taskHandlers[tasks.TypeRenderVariant] = renderWorker.HandleRenderVariant
		log.Printf("worker: render enabled")
	}
	if cfg.streamRenderWorkerEnabled() && streamRepo != nil {
		streamWorker := workers.NewStreamRenderWorker(streamRepo, store, workers.StreamRenderWorkerConfig{
			WorkDir:    cfg.MediaWorkDir,
			FFmpegPath: cfg.FFmpegPath,
			Timeout:    cfg.RenderTimeout,
			JobLocks:   streamJobLocks,
			MusicDir:   cfg.MusicDir,
			// Studio only ever enqueues bound tasks, so an unbound one is a
			// stale or forged render and must never commit the canonical pointer.
			RequireImmutableEditPlanIntent: true,
		})
		taskHandlers[tasks.TypeRenderStreamClip] = streamWorker.HandleRenderStreamClip
		log.Printf("worker: stream render enabled")
	}
	streamAcquireEnabled := cfg.streamAcquireWorkerEnabled()
	if streamAcquireEnabled && streamRepo != nil {
		acquireWorker := workers.NewAcquireWorker(streamRepo, store, workers.AcquireWorkerConfig{
			WorkDir:     cfg.MediaWorkDir,
			YtdlpPath:   cfg.YtdlpPath,
			FFprobePath: cfg.FFprobePath,
			Timeout:     cfg.RenderTimeout,
		})
		taskHandlers[tasks.TypeStreamAcquire] = acquireWorker.HandleStreamAcquire
		log.Printf("worker: stream acquire enabled")
	}
	if cfg.streamRenderWorkerEnabled() && editorProjects != nil && editorAssets != nil {
		timelineWorker := workers.NewTimelineRenderWorker(editorProjects, store, workers.TimelineRenderWorkerConfig{
			WorkDir:     cfg.MediaWorkDir,
			FFmpegPath:  cfg.FFmpegPath,
			Timeout:     cfg.RenderTimeout,
			MusicDir:    cfg.MusicDir,
			AssetLookup: editorAssets,
		})
		taskHandlers[tasks.TypeRenderTimeline] = timelineWorker.HandleRenderTimeline
		log.Printf("worker: timeline render enabled")
	}

	var queue httpapi.Enqueuer
	inline := newInlineQueue(taskHandlers, cfg.WorkerConcurrency)
	queue = inline
	// Wire the chaining queue before processing starts so the record worker
	// never handles a task with a half-set enqueuer.
	if recordWorker != nil {
		recordWorker.UseGenerateIntentStore(generateIntents)
		recordWorker.UseEnqueuer(queue)
	}
	handlers := httpapi.NewHandlers(repo, store, queue,
		httpapi.WithMutationToken(cfg.MutationToken),
		httpapi.WithRequireReadAuth(true),
		// Remote binds are rejected. A per-IP limiter on loopback would give
		// every local process the same 127.0.0.1 bucket and let unauthenticated
		// traffic starve the authenticated desktop session.
		httpapi.WithRateLimit(0, 0),
		httpapi.WithUploadConcurrency(2),
		httpapi.WithStreamRepository(streamRepo),
		httpapi.WithEditorRepositories(editorAssets, editorProjects),
		httpapi.WithStreamJobLocks(streamJobLocks),
		httpapi.WithStreamProber(streamclips.FFprobeProber{Path: cfg.FFprobePath}),
		httpapi.WithMusicDir(cfg.MusicDir),
		httpapi.WithCapabilities(cfg.captureCapabilities(captureSource)),
		httpapi.WithGenerateIntentStore(generateIntents),
		httpapi.WithPublishAssistantTrends(youtubeTrends),
		httpapi.WithFaceit(faceitClient, faceitFollows),
	)
	srv := newOrchestratorHTTPServer(cfg.HTTPAddr, httpapi.Routes(handlers))
	httpRuntime, err := prepareHTTPServer(srv)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}

	// The address is reserved now, so workers cannot start behind a server that
	// already failed to bind.
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	inline.Start(workerCtx)
	if err := recoverStreamAcquisitions(
		ctx,
		reconciled.StreamAcquisitions,
		streamAcquireEnabled,
		streamRepo,
		inline,
		observability,
	); err != nil {
		return err
	}
	log.Printf("queue: inline mode enabled (concurrency=%d)", cfg.WorkerConcurrency)
	httpRuntime.Start()
	log.Printf("http: listening on %s", httpRuntime.Addr())

	serveErr := waitAndCancelOnHTTPFailure(ctx, stop, httpRuntime)
	if serveErr != nil {
		log.Printf("shutdown: http server failed, draining: %v", serveErr)
	} else {
		log.Print("shutdown: received signal, draining")
	}

	// Stop accepting mutations before canceling workers. Any request already in
	// flight can finish its atomic admission while the queue is still live;
	// shutdown then compensates accepted work that remains pending.
	httpShutdownCtx, cancelHTTPShutdown := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	if err := httpRuntime.Shutdown(httpShutdownCtx); err != nil {
		log.Printf("shutdown: HTTP drain failed: %v", err)
	}
	cancelHTTPShutdown()

	cancelWorkers()
	queueShutdownCtx, cancelQueueShutdown := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	if err := inline.Shutdown(queueShutdownCtx); err != nil {
		log.Printf("shutdown: queue drain failed: %v", err)
	}
	cancelQueueShutdown()
	log.Print("shutdown: done")
	if serveErr != nil {
		return fmt.Errorf("http: %w", serveErr)
	}
	return nil
}

func recoverStreamAcquisitions(
	ctx context.Context,
	ids []uuid.UUID,
	workerEnabled bool,
	repo orchestratorStreamJobRepository,
	queue *inlineQueue,
	rec *obs.Recorder,
) error {
	if !workerEnabled {
		for _, id := range ids {
			if err := repo.UpdateStatus(ctx, id, streamclips.StatusFailed, streamAcquireRecoveryDisabledReason); err != nil {
				return fmt.Errorf("fail unrecoverable stream acquisition %s: %w", id, err)
			}
			if rec != nil {
				_ = rec.RecordError(obs.Event{
					Stage:   obs.StageStreamAcquire,
					Class:   interruptedClass,
					Message: id.String() + ": " + streamAcquireRecoveryDisabledReason,
				})
			}
		}
		return nil
	}
	for _, id := range ids {
		task, taskErr := tasks.NewStreamAcquireTask(id)
		if taskErr != nil {
			return fmt.Errorf("build recovered stream acquisition %s: %w", id, taskErr)
		}
		if _, enqueueErr := queue.Enqueue(task, asynq.MaxRetry(0)); enqueueErr != nil {
			return fmt.Errorf("enqueue recovered stream acquisition %s: %w", id, enqueueErr)
		}
	}
	return nil
}
