package workers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/composition"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/generateintent"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/keydropbanner"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tasks"
	"github.com/rechedev9/cliphub/internal/voicecomms"
)

const defaultMediaWorkerTimeout = "20m"

// errStreamRenderParentPromotion marks the narrow failure window after every
// render artifact and the authoritative rendered state are durable, but before
// the parent stream job can be promoted. The completion state must survive so
// startup reconciliation can finish that promotion after a restart.
var errStreamRenderParentPromotion = errors.New("stream render completed but parent status promotion failed")

// errStreamRenderSuperseded marks a render whose admitted edit plan or variant
// state was replaced while the task was queued or running. It is recoverable:
// the parent job keeps its previous status so the user can render again.
var errStreamRenderSuperseded = errors.New("stream render was superseded")

var errStaleGenerateHandoff = errors.New("generate render handoff no longer owns the active run")

// Bounded fan-out for the render worker's per-short I/O. Probing and localizing
// run one external/IO op per short; doing them concurrently (capped) turns an
// N-short serial wait into roughly N/limit while keeping disk and subprocess
// pressure sane on a single BYO box.
const (
	probeConcurrency    = 4
	localizeConcurrency = 6
)

// failureWriteTimeout bounds the fresh-context status write performed when a
// task fails. The handler context is frequently already cancelled at that
// point (Asynq deadline or shutdown), so the terminal StatusFailed write needs
// its own context to land in the database.
const failureWriteTimeout = 5 * time.Second

// StatusRepository is the subset of *job.Repository needed by media workers.
type StatusRepository interface {
	Get(ctx context.Context, id uuid.UUID) (job.Job, error)
	GetMeta(ctx context.Context, id uuid.UUID) (job.Job, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, s job.Status, failureReason string) error
}

// statusUpdater is the single method markFailed needs; both StatusRepository
// and JobRepository satisfy it.
type statusUpdater interface {
	UpdateStatus(ctx context.Context, id uuid.UUID, s job.Status, failureReason string) error
}

// Enqueuer is the desktop queue contract the record worker uses to chain a
// render after successful capture. The transition runs atomically with queue
// admission and receives a later non-nil decision if shutdown discards pending
// work; a nil Enqueuer disables chaining for the manual record path.
type Enqueuer interface {
	Enqueue(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
	EnqueueWithTransition(*asynq.Task, func(error) error, ...asynq.Option) (*asynq.TaskInfo, error)
}

// chainedRenderUniqueTTL deduplicates a chained render against a render the user
// may also have launched manually for the same job and variant within the day.
const chainedRenderUniqueTTL = 24 * time.Hour

// markFailed records a job's terminal failure on a fresh, short-lived context
// so the write survives a handler context already cancelled by an Asynq
// deadline or shutdown (pgxpool.Exec refuses to run on a cancelled context).
// The secondary error is logged rather than discarded: a job stranded in a
// non-terminal status is otherwise invisible to operators.
func markFailed(repo statusUpdater, id uuid.UUID, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), failureWriteTimeout)
	defer cancel()
	if err := repo.UpdateStatus(ctx, id, job.StatusFailed, reason); err != nil {
		logWorkerError(id, "mark failed", err)
		return err
	}
	return nil
}

// recordTaskFailure records a job's failure, but only when the current Asynq
// attempt is terminal (returning the error now archives the task instead of
// scheduling another retry). For a retryable task an intermediate failure is
// left as the in-progress status so the job does not flap StatusFailed<->in
// progress across retries; the terminal failure is recorded once retries are
// exhausted.
func recordTaskFailure(ctx context.Context, repo statusUpdater, id uuid.UUID, taskType string, err error) error {
	return recordTaskFailureAs(ctx, repo, id, taskType, obs.StageWorker, errorClass(taskType, err), err)
}

// recordTaskFailureAs is recordTaskFailure with an explicit obs stage and class,
// so a stage with its own label vocabulary still records exactly once, on the
// same terminal-attempt rule as every other worker failure.
func recordTaskFailureAs(ctx context.Context, repo statusUpdater, id uuid.UUID, taskType, stage, class string, err error) error {
	if !taskIsTerminal(ctx) {
		logWorkerError(id, taskType+" will retry", err)
		return nil
	}
	if markErr := markFailed(repo, id, err.Error()); markErr != nil {
		return markErr
	}
	recordStageFailure(id, stage, taskType, class, err)
	logWorkerTransition(id, taskType, job.StatusFailed)
	return nil
}

// recordPreservedRecordingFailure closes a failed recording attempt without
// invalidating a prior committed capture that remained usable. The task still
// returns its error, so a guided generate never renders stale media as though
// the requested recapture had succeeded.
func recordPreservedRecordingFailure(ctx context.Context, repo statusUpdater, id uuid.UUID, err error) error {
	if !taskIsTerminal(ctx) {
		logWorkerError(id, tasks.TypeRecordDemo+" will retry with the previous recording preserved", err)
		return nil
	}
	statusCtx, cancel := context.WithTimeout(context.Background(), failureWriteTimeout)
	defer cancel()
	if statusErr := repo.UpdateStatus(statusCtx, id, job.StatusRecorded, ""); statusErr != nil {
		logWorkerError(id, "preserve recorded status after failed recapture", statusErr)
		return statusErr
	}
	recordStageFailure(id, obs.StageWorker, tasks.TypeRecordDemo, errorClass(tasks.TypeRecordDemo, err), err)
	logWorkerTransition(id, tasks.TypeRecordDemo, job.StatusRecorded)
	return nil
}

// taskIsTerminal reports whether the current Asynq attempt is the last one, so
// returning an error archives the task instead of retrying. Outside an Asynq
// task context (e.g. direct unit tests) it returns true so a failure is still
// recorded.
func taskIsTerminal(ctx context.Context) bool {
	if retried, maxRetry, ok := tasks.TaskAttempt(ctx); ok {
		return isTerminalAttempt(retried, maxRetry, true)
	}
	retried, ok1 := asynq.GetRetryCount(ctx)
	maxRetry, ok2 := asynq.GetMaxRetry(ctx)
	return isTerminalAttempt(retried, maxRetry, ok1 && ok2)
}

// isTerminalAttempt holds the retry arithmetic separately so it can be tested
// without an Asynq task context.
func isTerminalAttempt(retried, maxRetry int, inTask bool) bool {
	if !inTask {
		return true
	}
	return retried >= maxRetry
}

type commandRunner interface {
	Run(ctx context.Context, exe string, args ...string) ([]byte, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, exe string, args ...string) ([]byte, error) {
	// #nosec G204 -- media workers execute configured local binaries with argument slices, not shell strings.
	cmd := exec.CommandContext(ctx, exe, args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, nil
	}
	if text := strings.TrimSpace(string(out)); text != "" {
		return out, fmt.Errorf("%s failed: %w: %s", exe, err, text)
	}
	return out, fmt.Errorf("%s failed: %w", exe, err)
}

type RecordWorkerConfig struct {
	WorkDir      string
	RecorderPath string
	HLAEPath     string
	CS2Path      string
	Timeout      string
	// HUDMode is the in-game HUD the recorder captures with (gameplay, clean, or
	// deathnotices). The viral short wants a HUD-less POV with the deathnotices
	// killfeed, so it defaults to "deathnotices" (see withDefaults).
	HUDMode string
}

type ComposeWorkerConfig struct {
	WorkDir      string
	ComposerPath string
	FFmpegPath   string
	Timeout      string
}

type RenderWorkerConfig struct {
	WorkDir     string
	EditorPath  string
	FFmpegPath  string
	FFprobePath string
	Timeout     string
	// MusicDir holds music tracks named "<key>.<ext>" that a render can mix in
	// (see RenderVariantPayload.MusicKey). Empty disables music mixing.
	MusicDir string
	Faceit   *faceit.Client
}

// resolveMusicFile returns the first existing track file for key in dir, or ""
// when dir is unset, key is unsafe, or nothing matches. key is validated
// upstream; the separator check is defence in depth against path traversal.
func resolveMusicFile(dir, key string) string {
	if dir == "" || key == "" || strings.ContainsAny(key, `/\`) || strings.Contains(key, "..") {
		return ""
	}
	for _, ext := range []string{".m4a", ".mp3", ".ogg", ".opus", ".wav", ".aac"} {
		p := filepath.Join(dir, key+ext)
		if info, statErr := os.Stat(p); statErr == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

type StreamRenderRepository interface {
	Get(ctx context.Context, id uuid.UUID) (streamclips.Job, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, s streamclips.Status, failureReason string) error
}

type StreamRenderWorkerConfig struct {
	WorkDir    string
	FFmpegPath string
	Timeout    string
	// JobLocks must be shared with HTTP handlers in Studio so render claims and
	// final pointer commits serialize with edit-plan mutations for the same job.
	// CLI and tests may leave it nil to receive a private coordinator.
	JobLocks *streamclips.JobLocks
	// RequireImmutableEditPlanIntent refuses a render:stream-clip task that
	// carries no StreamRenderIntent. Every supersede check below treats a
	// missing intent as "nothing to compare", so without this gate an unbound
	// task would render whatever the edit plan happens to be when it executes
	// and commit the canonical pointer. Studio enables it because it only ever
	// enqueues bound tasks; the CLI leaves it off so a plain task still renders.
	RequireImmutableEditPlanIntent bool
	// MusicDir holds catalog tracks named "<key>.<ext>" that an edit plan's
	// MusicPlan can mix under the clip audio (same directory the songs API and
	// the reel render worker use).
	MusicDir string
}

// RecordWorker handles the "record:demo" Asynq task.
type RecordWorker struct {
	repo            StatusRepository
	storage         storage.Storage
	generateIntents *generateintent.Store
	cfg             RecordWorkerConfig
	runner          commandRunner
	enqueuer        Enqueuer
	// jobLocks serializes recording per job so two reels for the same job (each a
	// distinct, non-deduped task with different segment ids) never launch the
	// recorder concurrently or race on the job-level recording result. The
	// coordinator removes an entry after its final waiter releases it, so a
	// long-running orchestrator does not retain every completed job forever.
	jobLocks *streamclips.JobLocks
}

// lockJob acquires the per-job recording lock and returns its release func.
func (w *RecordWorker) lockJob(id uuid.UUID) func() {
	return w.jobLocks.Lock(id)
}

func NewRecordWorker(repo StatusRepository, store storage.Storage, cfg RecordWorkerConfig) *RecordWorker {
	return &RecordWorker{
		repo:            repo,
		storage:         store,
		generateIntents: generateintent.New(store),
		cfg:             cfg,
		runner:          execCommandRunner{},
		jobLocks:        streamclips.NewJobLocks(),
	}
}

// UseGenerateIntentStore shares guided-generate synchronization with the HTTP
// admission path. It is set once at startup before queue processing begins.
func (w *RecordWorker) UseGenerateIntentStore(store *generateintent.Store) {
	w.generateIntents = store
}

// UseEnqueuer wires the task queue the worker uses to chain a render after a
// successful capture. It is set once at startup, before the queue begins
// processing tasks, so no in-flight handler observes a half-set field.
func (w *RecordWorker) UseEnqueuer(e Enqueuer) {
	w.enqueuer = e
}

func (w *RecordWorker) HandleRecordDemo(ctx context.Context, t *asynq.Task) (retErr error) {
	var payload tasks.RecordDemoPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	var (
		generateIntent    renderplan.GenerateIntent
		hasGenerateIntent bool
		err               error
	)
	// Once the payload identifies the durable job, every terminal failure must
	// close that job's active generate/record lifecycle. In particular, early
	// repository and task-header errors happen before the recorder call and were
	// previously able to leave a guided generate request pending forever.
	defer func() {
		if retErr == nil {
			return
		}
		recordFailure := func() error {
			var preserved *recordingCommitPreservedError
			if errors.As(retErr, &preserved) {
				return recordPreservedRecordingFailure(ctx, w.repo, payload.JobID, retErr)
			}
			return recordTaskFailure(ctx, w.repo, payload.JobID, tasks.TypeRecordDemo, retErr)
		}
		if hasGenerateIntent && taskIsTerminal(ctx) {
			_, err := w.generateIntents.Finish(payload.JobID, generateIntent.ActiveRunID, func() error {
				return recordFailure()
			})
			if err != nil {
				logWorkerError(payload.JobID, "finish failed generate task", err)
			}
			return
		}
		_ = recordFailure()
	}()
	generateIntent, hasGenerateIntent, err = tasks.GenerateIntentFromTask(t)
	if err != nil {
		return fmt.Errorf("decode record task generate intent: %w", err)
	}

	j, err := w.repo.Get(ctx, payload.JobID)
	if err != nil {
		return fmt.Errorf("load job %s: %w", payload.JobID, err)
	}
	if j.KillPlan == nil {
		return fmt.Errorf("job %s has no kill plan", j.ID)
	}

	if err := w.record(ctx, j, payload.HUDMode, payload.SegmentIDs, payload.PortraitSafeKillfeed, payload.UseRecapPlan); err != nil {
		return err
	}
	if err := w.repo.UpdateStatus(ctx, j.ID, job.StatusRecorded, ""); err != nil {
		return fmt.Errorf("mark recorded: %w", err)
	}
	logWorkerTransition(j.ID, tasks.TypeRecordDemo, job.StatusRecorded)
	// A successful capture (including a readiness skip against a now-valid
	// result) invalidates prior "re-record required" render failures so the
	// Studio client re-drives render instead of looping on re-record forever.
	if err := w.clearNotReusableRenderFailures(j.ID); err != nil {
		logWorkerError(j.ID, "clear not-reusable render failures", err)
	}
	if payload.UseRecapPlan {
		if err := w.invalidateReadyRenders(j.ID); err != nil {
			logWorkerError(j.ID, "invalidate ready renders after recap capture", err)
		}
	}
	// A guided generate task carries its own immutable render intent, so another
	// accepted capture cannot change the treatment this capture chains. A
	// chaining failure must not fail capture; manual render remains a fallback.
	// Studio Full Demo retries POST /record (no generate header). Without this
	// recap chain, a successful 20-round capture stays recorded and never composes.
	if hasGenerateIntent {
		w.chainRender(j.ID, generateIntent, true)
	} else if payload.UseRecapPlan {
		w.chainRecapRender(j.ID, payload.DemoSource)
	}
	return nil
}

// chainRender enqueues a generate task's render intent after capture. It is
// best effort: every failure is logged and swallowed so successful capture is
// never reported as failed.
func (w *RecordWorker) chainRender(id uuid.UUID, intent renderplan.GenerateIntent, hasIntent bool) {
	if !hasIntent {
		return
	}
	if w.enqueuer == nil {
		w.failGenerateHandoff(id, intent, errors.New("render queue is not configured"))
		return
	}
	task, err := tasks.NewRenderVariantTask(id, intent.Variant, intent.MusicKey, 0, nil, intent.Edit)
	if err != nil {
		w.failGenerateHandoff(id, intent, fmt.Errorf("build chained render task: %w", err))
		return
	}
	admitted := false
	_, err = w.enqueuer.EnqueueWithTransition(task, func(decision error) error {
		switch {
		case decision == nil:
			// Publish Queued before the task is visible so a crash anywhere in the
			// handoff remains recoverable by the startup render-state sweep. If
			// completing the generate marker then fails, compensate Queued to
			// Failed before rejecting admission so the live UI is not stranded.
			owned, handoffErr := w.generateIntents.Finish(id, intent.ActiveRunID, func() error {
				return w.writeQueuedRenderState(id, intent.Variant)
			})
			if !owned {
				return errStaleGenerateHandoff
			}
			if handoffErr != nil {
				failedErr := w.writeFailedRenderState(id, intent.Variant, fmt.Sprintf("accept render handoff: %v", handoffErr))
				if failedErr != nil {
					return errors.Join(handoffErr, failedErr)
				}
				return errors.Join(handoffErr, w.completeGenerateIntent(id, intent.ActiveRunID))
			}
			admitted = true
			return nil
		case errors.Is(decision, asynq.ErrDuplicateTask):
			// Another task owns the render and its state; never downgrade it.
			_, err := w.generateIntents.Finish(id, intent.ActiveRunID, nil)
			return err
		default:
			if admitted {
				return w.writeFailedRenderState(id, intent.Variant, fmt.Sprintf("enqueue render: %v", decision))
			}
			_, err := w.generateIntents.Finish(id, intent.ActiveRunID, func() error {
				return w.writeFailedRenderState(id, intent.Variant, fmt.Sprintf("enqueue render: %v", decision))
			})
			return err
		}
	}, asynq.Unique(chainedRenderUniqueTTL))
	if err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			logWorkerTransition(id, tasks.TypeRenderVariant, job.StatusRecorded)
			return
		}
		logWorkerError(id, "enqueue chained render", err)
		return
	}
	logWorkerTransition(id, tasks.TypeRenderVariant, job.StatusRecorded)
}

// chainRecapRender enqueues the locked 16:9 Full Demo render after a recap
// capture that did not carry generate intent (Studio POST /record).
func (w *RecordWorker) chainRecapRender(id uuid.UUID, demoSource string) {
	if w.enqueuer == nil {
		logWorkerError(id, "enqueue recap render", errors.New("render queue is not configured"))
		return
	}
	if strings.TrimSpace(demoSource) == "" {
		demoSource = loadFullDemoSource(w.storage, id)
	}
	edit := renderplan.RecapEditRequestWithSource(demoSource)
	task, err := tasks.NewRenderVariantTask(id, editor.PresetGameplayPOV60, "", 0, nil, edit)
	if err != nil {
		logWorkerError(id, "build recap render task", err)
		return
	}
	_, err = w.enqueuer.EnqueueWithTransition(task, func(decision error) error {
		switch {
		case decision == nil:
			return w.writeQueuedRenderState(id, editor.PresetGameplayPOV60)
		case errors.Is(decision, asynq.ErrDuplicateTask):
			return nil
		default:
			return w.writeFailedRenderState(id, editor.PresetGameplayPOV60, fmt.Sprintf("enqueue render: %v", decision))
		}
	}, asynq.Unique(chainedRenderUniqueTTL))
	if err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			logWorkerTransition(id, tasks.TypeRenderVariant, job.StatusRecorded)
			return
		}
		logWorkerError(id, "enqueue recap render", err)
		return
	}
	logWorkerTransition(id, tasks.TypeRenderVariant, job.StatusRecorded)
}

func loadFullDemoSource(store storage.Storage, id uuid.UUID) string {
	if store == nil {
		return ""
	}
	rc, err := store.Open(artifacts.FullDemoSourceKey(id))
	if err != nil {
		return ""
	}
	defer rc.Close()
	var doc struct {
		Source string `json:"source"`
	}
	if err := json.NewDecoder(rc).Decode(&doc); err != nil {
		return ""
	}
	return demooverlay.NormalizeSource(doc.Source)
}

func (w *RecordWorker) failGenerateHandoff(id uuid.UUID, intent renderplan.GenerateIntent, cause error) {
	logWorkerError(id, "generate render handoff", cause)
	_, err := w.generateIntents.Finish(id, intent.ActiveRunID, func() error {
		return w.writeFailedRenderState(id, intent.Variant, cause.Error())
	})
	if err != nil {
		logWorkerError(id, "finish failed generate handoff", err)
	}
}

func (w *RecordWorker) completeGenerateIntent(id, runID uuid.UUID) error {
	return w.generateIntents.Complete(id, runID)
}

func (w *RecordWorker) writeQueuedRenderState(id uuid.UUID, variant string) error {
	return w.writeRenderState(id, variant, renderplan.RenderVariantStatusQueued, "")
}

func (w *RecordWorker) writeFailedRenderState(id uuid.UUID, variant, message string) error {
	return w.writeRenderState(id, variant, renderplan.RenderVariantStatusFailed, message)
}

func (w *RecordWorker) writeRenderState(id uuid.UUID, variant, status, message string) error {
	loadout, err := renderplan.LoadoutForVariant(variant)
	if err != nil {
		return err
	}
	previous, _, err := w.readRenderVariantState(id, variant)
	if err != nil {
		return fmt.Errorf("read previous render state: %w", err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:    id,
		Loadout:  loadout,
		Status:   status,
		Error:    message,
		Previous: previous,
	})
	if err != nil {
		return err
	}
	key, err := renderplan.RenderVariantStateKey(id, variant)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return w.storage.Put(key, bytes.NewReader(b))
}

func (w *RecordWorker) readRenderVariantState(id uuid.UUID, variant string) (*renderplan.RenderVariantState, bool, error) {
	key, err := renderplan.RenderVariantStateKey(id, variant)
	if err != nil {
		return nil, false, err
	}
	rc, err := w.storage.Open(key)
	if err != nil {
		if storage.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer rc.Close()
	var state renderplan.RenderVariantState
	if err := json.NewDecoder(rc).Decode(&state); err != nil {
		return nil, false, err
	}
	return &state, true, nil
}

// clearNotReusableRenderFailures drops failed render status documents whose
// error says the stored capture is not reusable under the current contract.
// After a successful re-record the client must see render status "none" so it
// drives render instead of looping on re-record against the same failure.
func (w *RecordWorker) clearNotReusableRenderFailures(id uuid.UUID) error {
	deleter, ok := w.storage.(interface{ Delete(string) error })
	if !ok {
		return fmt.Errorf("storage cannot delete not-reusable render failures")
	}
	var errs []error
	for _, variant := range editor.PresetNames() {
		state, exists, err := w.readRenderVariantState(id, variant)
		if err != nil {
			errs = append(errs, fmt.Errorf("read render state %q: %w", variant, err))
			continue
		}
		if !exists || state.Status != renderplan.RenderVariantStatusFailed {
			continue
		}
		if !recording.IsNotReusableMessage(state.Error) {
			continue
		}
		key, err := renderplan.RenderVariantStateKey(id, variant)
		if err != nil {
			errs = append(errs, fmt.Errorf("render state key %q: %w", variant, err))
			continue
		}
		if err := deleter.Delete(key); err != nil {
			errs = append(errs, fmt.Errorf("delete render state %q: %w", variant, err))
		}
	}
	return errors.Join(errs...)
}

// invalidateReadyRenders drops ready/review render status after a recap
// recapture so Studio does not treat the previous Shorts pack as this reel.
func (w *RecordWorker) invalidateReadyRenders(id uuid.UUID) error {
	deleter, ok := w.storage.(interface{ Delete(string) error })
	if !ok {
		return fmt.Errorf("storage cannot delete ready renders")
	}
	var errs []error
	for _, variant := range editor.PresetNames() {
		state, exists, err := w.readRenderVariantState(id, variant)
		if err != nil {
			errs = append(errs, fmt.Errorf("read render state %q: %w", variant, err))
			continue
		}
		if !exists {
			continue
		}
		if state.Status != renderplan.RenderVariantStatusReady && state.Status != renderplan.RenderVariantStatusReview {
			continue
		}
		key, err := renderplan.RenderVariantStateKey(id, variant)
		if err != nil {
			errs = append(errs, fmt.Errorf("render state key %q: %w", variant, err))
			continue
		}
		if err := deleter.Delete(key); err != nil {
			errs = append(errs, fmt.Errorf("delete render state %q: %w", variant, err))
		}
	}
	return errors.Join(errs...)
}

func (w *RecordWorker) record(ctx context.Context, j job.Job, hudMode string, segmentIDs []string, portraitSafeKillfeed, useRecapPlan bool) (retErr error) {
	sourcePlan, err := recordSourcePlan(w.storage, j, useRecapPlan)
	if err != nil {
		return err
	}
	// Serialize recording per job: two reels for the same job (each a distinct,
	// non-deduped task with different segment ids) must not launch the recorder
	// concurrently or race on the job-level recording result.
	defer w.lockJob(j.ID)()

	// A reel records only its selected segment(s); an empty selection means the
	// whole source plan (the CLI all-kills default, or every recap round).
	// Resolve to concrete ids so readiness, plan filtering, and accumulation
	// all agree on the same set.
	requested := segmentIDs
	if len(requested) == 0 {
		requested = killPlanSegmentIDs(sourcePlan)
	}
	// requestedPlan scopes only the identity/profile computation below; the
	// plan actually handed to the recorder is narrowed further to the missing
	// subset once readiness is known.
	requestedPlan, err := filterKillPlanSegments(sourcePlan, requested)
	if err != nil {
		return err
	}

	cfg := w.cfg.withDefaults()
	// A per-job preset HUD (e.g. "Clean POV") overrides the worker default.
	if hudMode != "" {
		cfg.HUDMode = hudMode
	}
	// The encoder is part of the recording identity: it must reach both the
	// recorder argv and the expected plan, or attempt validation rejects the
	// recorder's echoed plan after a successful capture.
	captureEncoder := studioCaptureEncoder(cfg.HLAEPath)
	expectedStream, err := normalizedRecordingStream(requestedPlan, cfg.HUDMode, portraitSafeKillfeed, captureEncoder)
	if err != nil {
		return fmt.Errorf("build recording profile: %w", err)
	}
	expectedProfile, err := recording.NewPlanFromKillPlan(*requestedPlan, "profile.dem", "profile", expectedStream)
	if err != nil {
		return fmt.Errorf("build expected recording identity: %w", err)
	}
	missing, reusedKeys, err := recordingOutputsReady(w.storage, j.ID, requested, expectedProfile)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		logWorkerSkip(j.ID, tasks.TypeRecordDemo, reusedKeys)
		return nil
	}
	// Capture only the segments that still lack a compatible durable clip.
	// Segments already covered by a compatible previous run merge back in via
	// mergeRecordingResults below instead of recapturing the whole selection.
	// Scope the plan handed to the recorder, so everything downstream (HLAE script,
	// take mapping, recording result, render) derives from the chosen segments
	// alone. j is a value copy, so j.KillPlan stays the full plan for ordering.
	recordPlan, err := filterKillPlanSegments(sourcePlan, missing)
	if err != nil {
		return err
	}

	// Establish whether an earlier recording is a complete durable commit before
	// starting any replacement work. A decoded result alone is insufficient: its
	// script or one of its clips may already be missing.
	prev, hasPrev, err := tryDecodeStoredRecordingResult(w.storage, j.ID)
	if err != nil {
		return err
	}
	previousCommitReady := false
	if hasPrev {
		previousCommitReady, err = recordingCommitReady(w.storage, j.ID, prev)
		if err != nil {
			return fmt.Errorf("verify previous recording commit: %w", err)
		}
	}

	var (
		rollbackCaptureAttempt func() error
		uploadEntered          bool
	)
	// Everything before uploadRecordingOutputs is attempt-local or UI-only. If
	// it fails while a complete earlier commit still exists, restore the prior
	// selection/progress documents and classify the error so the handler keeps
	// Recorded.
	// Upload owns its finer mutation boundary because a failure after replacing
	// an old clip can no longer make this promise.
	defer func() {
		if retErr == nil {
			return
		}
		var preserved *recordingCommitPreservedError
		commitPreserved := errors.As(retErr, &preserved)
		if !commitPreserved && (!previousCommitReady || uploadEntered) {
			return
		}
		if rollbackCaptureAttempt != nil {
			if rollbackErr := rollbackCaptureAttempt(); rollbackErr != nil {
				retErr = errors.Join(retErr, fmt.Errorf("rollback capture attempt documents: %w", rollbackErr))
			}
		}
		if !commitPreserved {
			retErr = &recordingCommitPreservedError{cause: retErr}
		}
	}()

	rollbackCaptureAttempt, err = prepareCaptureAttemptRollback(w.storage, j.ID)
	if err != nil {
		return fmt.Errorf("prepare capture attempt rollback: %w", err)
	}

	// Persist the ordered segment ids this reel captures so the job poll scopes
	// capture progress to this reel, not the whole kill plan. Overwritten at the
	// start of every record task (last writer wins - it is the in-flight reel).
	if err := putCaptureSelection(w.storage, j.ID, killPlanSegmentIDs(recordPlan)); err != nil {
		return fmt.Errorf("persist capture selection: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return err
	}

	workDir, cleanup, err := prepareStageDir(cfg.WorkDir, j.ID, "record")
	if err != nil {
		return err
	}
	defer cleanup()

	demoPath := filepath.Join(workDir, "demo.dem")
	killPlanPath := filepath.Join(workDir, "killplan.json")
	outDir := filepath.Join(workDir, "out")
	if err := materializeStorageFile(w.storage, j.DemoPath, demoPath); err != nil {
		return fmt.Errorf("materialize demo: %w", err)
	}
	if err := writeJSONFile(killPlanPath, recordPlan); err != nil {
		return fmt.Errorf("write kill plan: %w", err)
	}
	expectedPlan, err := recording.NewPlanFromKillPlan(*recordPlan, demoPath, outDir, expectedStream)
	if err != nil {
		return fmt.Errorf("build launched recording plan: %w", err)
	}

	recorderArgs := []string{
		"--killplan", killPlanPath,
		"--demo", demoPath,
		"--out", outDir,
		"--hlae", cfg.HLAEPath,
		"--cs2", cfg.CS2Path,
		"--hud", cfg.HUDMode,
		"--timeout", cfg.Timeout,
	}
	if portraitSafeKillfeed {
		recorderArgs = append(recorderArgs, "--portrait-safe-killfeed")
	}
	if captureEncoder != "" {
		recorderArgs = append(recorderArgs, "--encoder", captureEncoder)
	}
	progressAttemptID, err := startCaptureProgressAttempt(w.storage, j.ID, killPlanSegmentIDs(recordPlan))
	if err != nil {
		return fmt.Errorf("start capture progress attempt: %w", err)
	}
	if err := w.repo.UpdateStatus(ctx, j.ID, job.StatusRecording, ""); err != nil {
		rollbackErr := rollbackCaptureAttempt()
		rollbackCaptureAttempt = nil
		if rollbackErr != nil {
			rollbackErr = fmt.Errorf("rollback capture attempt documents: %w", rollbackErr)
		}
		return errors.Join(fmt.Errorf("mark recording: %w", err), rollbackErr)
	}
	logWorkerTransition(j.ID, tasks.TypeRecordDemo, job.StatusRecording)

	progressCtx, stopProgress := context.WithCancel(ctx)
	progressDone := make(chan struct{})
	progress := newCaptureProgressReporter(
		w.storage,
		j.ID,
		progressAttemptID,
		filepath.Join(outDir, "segments"),
		killPlanSegmentIDs(recordPlan),
	)
	progress.outDir = outDir
	progress.tickrate = recordPlan.Demo.Tickrate
	progress.ticks = segmentTickWeights(recordPlan)
	go func() {
		defer close(progressDone)
		progress.watch(progressCtx)
	}()
	_, runErr := w.runner.Run(ctx, cfg.RecorderPath, recorderArgs...)
	stopProgress()
	<-progressDone

	resultPath := filepath.Join(outDir, "recording-result.json")
	var result recording.RecordingResult
	if err := readJSONFile(resultPath, &result); err != nil {
		if runErr != nil {
			return newRecordFailure(runErr, result, requested)
		}
		return fmt.Errorf("read recording result: %w", err)
	}
	if runErr != nil {
		return newRecordFailure(runErr, result, requested)
	}
	if err := recording.ValidateRecordingAttempt(expectedPlan, outDir, result); err != nil {
		return err
	}
	result.CaptureRevision = uuid.NewString()
	durable := result
	if hasPrev && recordingProfilesCompatible(prev, result) {
		durable, err = mergeRecordingResults(prev, result, sourcePlan)
		if err != nil {
			return err
		}
		durable.CaptureRevision = result.CaptureRevision
	}
	if err := writeJSONFile(resultPath, durable); err != nil {
		return fmt.Errorf("write recording revision: %w", err)
	}
	uploadEntered = true
	keys, err := uploadRecordingOutputs(w.storage, j.ID, outDir, resultPath, result, durable, prev, hasPrev)
	if err != nil {
		return err
	}
	logWorkerArtifacts(j.ID, tasks.TypeRecordDemo, keys)
	return nil
}

func (c RecordWorkerConfig) withDefaults() RecordWorkerConfig {
	if c.Timeout == "" {
		c.Timeout = defaultMediaWorkerTimeout
	}
	if c.HUDMode == "" {
		// The viral short is a HUD-less POV with the in-game deathnotices killfeed
		// (the editor crops that killfeed into its overlay), not the full scoreboard
		// HUD the recorder defaults to.
		c.HUDMode = string(recording.HUDModeDeathnotices)
	}
	return c
}

func (c RecordWorkerConfig) validate() error {
	required := map[string]string{
		"recorder": c.RecorderPath,
		"hlae":     c.HLAEPath,
		"cs2":      c.CS2Path,
		"timeout":  c.Timeout,
	}
	for name, value := range required {
		if value == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	return nil
}

// ComposeWorker handles the "compose:final" Asynq task.
type ComposeWorker struct {
	repo     StatusRepository
	storage  storage.Storage
	cfg      ComposeWorkerConfig
	runner   commandRunner
	jobLocks *streamclips.JobLocks
}

func NewComposeWorker(repo StatusRepository, store storage.Storage, cfg ComposeWorkerConfig) *ComposeWorker {
	return &ComposeWorker{
		repo:     repo,
		storage:  store,
		cfg:      cfg,
		runner:   execCommandRunner{},
		jobLocks: streamclips.NewJobLocks(),
	}
}

func (w *ComposeWorker) HandleComposeFinal(ctx context.Context, t *asynq.Task) error {
	var payload tasks.ComposeFinalPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	release := w.jobLocks.Lock(payload.JobID)
	defer release()

	j, err := w.repo.GetMeta(ctx, payload.JobID)
	if err != nil {
		return fmt.Errorf("load job %s: %w", payload.JobID, err)
	}
	if err := w.repo.UpdateStatus(ctx, j.ID, job.StatusComposing, ""); err != nil {
		return fmt.Errorf("mark composing: %w", err)
	}
	logWorkerTransition(j.ID, tasks.TypeComposeFinal, job.StatusComposing)

	reviewRequired, err := w.compose(ctx, j)
	if err != nil {
		if statusErr := recordTaskFailure(ctx, w.repo, j.ID, tasks.TypeComposeFinal, err); statusErr != nil {
			return errors.Join(err, fmt.Errorf("record compose failure status: %w", statusErr))
		}
		return err
	}
	finalStatus := compositionCompletionStatus(reviewRequired)
	if err := w.repo.UpdateStatus(ctx, j.ID, finalStatus, ""); err != nil {
		return fmt.Errorf("mark composed: %w", err)
	}
	logWorkerTransition(j.ID, tasks.TypeComposeFinal, finalStatus)
	return nil
}

func compositionCompletionStatus(reviewRequired bool) job.Status {
	if reviewRequired {
		return job.StatusReviewRequired
	}
	return job.StatusComposed
}

func (w *ComposeWorker) compose(ctx context.Context, j job.Job) (bool, error) {
	ready, reviewRequired, keys, err := compositionOutputsReady(w.storage, j.ID)
	if err != nil {
		return false, err
	}
	if ready {
		logWorkerSkip(j.ID, tasks.TypeComposeFinal, keys)
		return reviewRequired, nil
	}

	cfg := w.cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return false, err
	}

	workDir, cleanup, err := prepareStageDir(cfg.WorkDir, j.ID, "compose")
	if err != nil {
		return false, err
	}
	defer cleanup()

	recordingResult, err := readStoredRecordingResult(w.storage, j.ID)
	if err != nil {
		return false, err
	}
	localRecordingResult := filepath.Join(workDir, "recording-result.json")
	if err := localizeSegmentClips(w.storage, j.ID, workDir, &recordingResult); err != nil {
		return false, err
	}
	if err := writeJSONFile(localRecordingResult, recordingResult); err != nil {
		return false, fmt.Errorf("write localized recording result: %w", err)
	}

	// The result key is the commit marker for the fixed-key composition pair.
	// Invalidate it before the composer can replace final.mp4, so a failed
	// result upload can never leave an old successful marker pointing at a new
	// MP4. Put is atomic per key, making this a small two-phase publication.
	if err := writeCompositionPendingMarker(w.storage, j.ID); err != nil {
		return false, err
	}

	finalPath := filepath.Join(workDir, "final.mp4")
	args := []string{
		"--recording-result", localRecordingResult,
		"--out", finalPath,
		"--timeout", cfg.Timeout,
	}
	if cfg.FFmpegPath != "" {
		args = append(args, "--ffmpeg", cfg.FFmpegPath)
	}
	_, runErr := w.runner.Run(ctx, cfg.ComposerPath, args...)

	resultPath := filepath.Join(workDir, "composition-result.json")
	var result composition.Result
	if err := readJSONFile(resultPath, &result); err != nil {
		return false, errors.Join(runErr, fmt.Errorf("read composition result: %w", err))
	}
	validationErr := composition.ValidateUploadResult(result)
	if result.Error != "" {
		// A structured composer failure is useful even when the executable also
		// exits non-zero. Persist it before returning the process error; failed
		// results are diagnostic artifacts because readiness rejects Error.
		uploadErr := uploadFile(w.storage, composition.ResultArtifactKey(j.ID), resultPath)
		if uploadErr != nil {
			uploadErr = fmt.Errorf("upload failed composition result: %w", uploadErr)
		}
		return false, errors.Join(runErr, validationErr, uploadErr)
	}
	if runErr != nil {
		return false, errors.Join(runErr, validationErr)
	}
	if validationErr != nil {
		return false, validationErr
	}
	if err := uploadFile(w.storage, composition.FinalArtifactKey(j.ID), finalPath); err != nil {
		return false, fmt.Errorf("upload final mp4: %w", err)
	}
	// The result is the commit marker and therefore lands after the MP4 it
	// references. A failed final upload cannot leave a reusable result.
	if err := uploadFile(w.storage, composition.ResultArtifactKey(j.ID), resultPath); err != nil {
		return false, fmt.Errorf("upload composition result: %w", err)
	}
	logWorkerArtifacts(j.ID, tasks.TypeComposeFinal, []string{
		composition.ResultArtifactKey(j.ID),
		composition.FinalArtifactKey(j.ID),
	})
	return len(result.Warnings) > 0, nil
}

func writeCompositionPendingMarker(store storage.Storage, id uuid.UUID) error {
	marker, err := json.Marshal(composition.Result{
		Error: "composition publication is incomplete; rebuild required",
	})
	if err != nil {
		return fmt.Errorf("encode pending composition marker: %w", err)
	}
	if err := store.Put(composition.ResultArtifactKey(id), bytes.NewReader(marker)); err != nil {
		return fmt.Errorf("invalidate composition result before rebuild: %w", err)
	}
	return nil
}

func (c ComposeWorkerConfig) withDefaults() ComposeWorkerConfig {
	if c.Timeout == "" {
		c.Timeout = defaultMediaWorkerTimeout
	}
	return c
}

func (c ComposeWorkerConfig) validate() error {
	required := map[string]string{
		"composer": c.ComposerPath,
		"timeout":  c.Timeout,
	}
	for name, value := range required {
		if value == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	return nil
}

// RenderWorker handles the "render:variant" Asynq task.
type voiceExtractFunc func(demoPath, target, dir string) (int, error)

type RenderWorker struct {
	repo         StatusRepository
	storage      storage.Storage
	cfg          RenderWorkerConfig
	runner       commandRunner
	voiceExtract voiceExtractFunc
}

func NewRenderWorker(repo StatusRepository, store storage.Storage, cfg RenderWorkerConfig) *RenderWorker {
	return &RenderWorker{
		repo:         repo,
		storage:      store,
		cfg:          cfg,
		runner:       execCommandRunner{},
		voiceExtract: extractVoiceTracks,
	}
}

// StreamRenderWorker handles "render:stream-clip" tasks.
type StreamRenderWorker struct {
	repo     StreamRenderRepository
	storage  storage.Storage
	cfg      StreamRenderWorkerConfig
	runner   commandRunner
	jobLocks *streamclips.JobLocks
}

func NewStreamRenderWorker(repo StreamRenderRepository, store storage.Storage, cfg StreamRenderWorkerConfig) *StreamRenderWorker {
	jobLocks := cfg.JobLocks
	if jobLocks == nil {
		jobLocks = streamclips.NewJobLocks()
	}
	return &StreamRenderWorker{
		repo:     repo,
		storage:  store,
		cfg:      cfg,
		runner:   execCommandRunner{},
		jobLocks: jobLocks,
	}
}

func (w *StreamRenderWorker) HandleRenderStreamClip(ctx context.Context, t *asynq.Task) error {
	var payload tasks.RenderStreamClipPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	intent, hasIntent, err := tasks.StreamRenderIntentFromTask(t)
	if err != nil {
		return fmt.Errorf("decode stream render intent: %w", err)
	}
	j, err := w.repo.Get(ctx, payload.JobID)
	if err != nil {
		return fmt.Errorf("load stream job %s: %w", payload.JobID, err)
	}
	claim := streamRenderClaim{}
	if w.cfg.RequireImmutableEditPlanIntent && !hasIntent {
		err = fmt.Errorf("%w: immutable edit-plan intent is missing", errStreamRenderSuperseded)
	} else {
		err = w.render(ctx, j, payload.Variant, intent, hasIntent, &claim)
	}
	if err != nil {
		if errors.Is(err, errStreamRenderParentPromotion) {
			logWorkerError(j.ID, tasks.TypeRenderStreamClip, err)
			return err
		}
		if errors.Is(err, errStreamRenderSuperseded) {
			message := "render cancelled because its admitted edit plan is no longer current; review the plan, then render again"
			release := w.jobLocks.Lock(j.ID)
			defer release()
			current, getErr := w.repo.Get(ctx, j.ID)
			if getErr != nil {
				return errors.Join(err, fmt.Errorf("reload recoverable stream render parent: %w", getErr))
			}
			owned, stateErr := w.writeRecoverableStreamRenderState(
				j.ID, payload.Variant, intent, hasIntent, message,
			)
			if !owned && stateErr == nil {
				// A newer attempt replaced this variant's mutable state. This old
				// task no longer owns either status or parent repair.
				return nil
			}
			var repairErr error
			if stateErr != nil {
				repairErr = fmt.Errorf("write recoverable stream render state: %w", stateErr)
			}
			if claim.claimed {
				if current.Status == streamclips.StatusRendering {
					if statusErr := updateStreamStatus(w.repo, j.ID, claim.previousStatus, ""); statusErr != nil {
						repairErr = errors.Join(repairErr, fmt.Errorf("restore recoverable stream render parent status: %w", statusErr))
					}
				}
			}
			if repairErr != nil {
				finalErr := errors.Join(err, repairErr)
				logWorkerError(j.ID, tasks.TypeRenderStreamClip, finalErr)
				return finalErr
			}
			return nil
		}
		release := w.jobLocks.Lock(j.ID)
		defer release()
		current, getErr := w.repo.Get(ctx, j.ID)
		if getErr != nil {
			return errors.Join(err, fmt.Errorf("reload failed stream render parent: %w", getErr))
		}
		owned, stateErr := w.writeOwnedStreamRenderAttempt(
			j.ID, payload.Variant, intent, hasIntent,
			streamclips.StatusFailed, nil, err.Error(), "",
		)
		if !owned && stateErr == nil {
			logWorkerError(j.ID, tasks.TypeRenderStreamClip, err)
			return nil
		}
		var repairErr error
		if stateErr != nil {
			repairErr = fmt.Errorf("write failed stream render state: %w", stateErr)
		}
		switch {
		case claim.claimed && current.Status == streamclips.StatusRendering && claim.previousStatus == streamclips.StatusRendered:
			if statusErr := updateStreamStatus(w.repo, j.ID, streamclips.StatusRendered, ""); statusErr != nil {
				repairErr = errors.Join(repairErr, fmt.Errorf("restore previously rendered stream parent: %w", statusErr))
			}
		case claim.claimed && current.Status == streamclips.StatusRendering:
			if statusErr := updateStreamStatus(w.repo, j.ID, streamclips.StatusFailed, err.Error()); statusErr != nil {
				repairErr = errors.Join(repairErr, fmt.Errorf("mark stream render parent failed: %w", statusErr))
			}
		case !claim.claimed && current.Status == streamclips.StatusReady:
			if statusErr := updateStreamStatus(w.repo, j.ID, streamclips.StatusFailed, err.Error()); statusErr != nil {
				repairErr = errors.Join(repairErr, fmt.Errorf("mark unclaimed stream render parent failed: %w", statusErr))
			}
		}
		finalErr := errors.Join(err, repairErr)
		recordStageFailure(j.ID, obs.StageWorker, tasks.TypeRenderStreamClip, errorClass(tasks.TypeRenderStreamClip, err), err)
		logWorkerError(j.ID, tasks.TypeRenderStreamClip, finalErr)
		return finalErr
	}
	return nil
}

type streamRenderClaim struct {
	previousStatus streamclips.Status
	claimed        bool
}

func (w *StreamRenderWorker) render(
	ctx context.Context,
	j streamclips.Job,
	variant string,
	intent tasks.StreamRenderIntent,
	hasIntent bool,
	claim *streamRenderClaim,
) error {
	if claim == nil {
		return fmt.Errorf("stream render claim is required")
	}
	if _, ok := streamclips.VariantByName(variant); !ok {
		return fmt.Errorf("unsupported stream render variant %q (valid variants: %s)", variant, strings.Join(streamclips.VariantNames(), ", "))
	}
	cfg := w.cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return err
	}

	// Claim the parent under the same per-job lock used by every HTTP plan
	// mutation. Reloading here makes a queued task validate the current plan,
	// not the snapshot HandleRenderStreamClip happened to read before locking.
	releaseClaim := w.jobLocks.Lock(j.ID)
	claimReleased := false
	defer func() {
		if !claimReleased {
			releaseClaim()
		}
	}()
	current, err := w.repo.Get(ctx, j.ID)
	if err != nil {
		return fmt.Errorf("reload stream job %s for render claim: %w", j.ID, err)
	}
	j = current
	if err := w.ensureStreamRenderAttemptCurrent(j.ID, variant, intent, hasIntent); err != nil {
		return err
	}
	if j.Status != streamclips.StatusReady && j.Status != streamclips.StatusRendered {
		return fmt.Errorf("%w: stream job is not claimable (status=%s)", errStreamRenderSuperseded, j.Status)
	}
	if len(j.EditPlan) == 0 {
		return fmt.Errorf("stream job %s has no edit plan", j.ID)
	}
	var plan streamclips.EditPlan
	if err := json.Unmarshal(j.EditPlan, &plan); err != nil {
		return fmt.Errorf("decode edit plan: %w", err)
	}
	plan = streamclips.NormalizeEditPlan(plan)
	if migrated, changed := streamclips.MigrateLegacySourceDuration(plan, j.Probe.DurationSeconds); changed {
		plan = migrated
	}
	if err := plan.ValidateForRender(j.Probe.DurationSeconds); err != nil {
		return err
	}
	if plan.Variant != variant {
		return fmt.Errorf(
			"%w: task variant %q does not match edit plan variant %q",
			errStreamRenderSuperseded, variant, plan.Variant,
		)
	}
	if err := validateStreamRenderIntent(plan, intent, hasIntent); err != nil {
		return err
	}
	bannerFontPath := ""
	if plan.StreamerBanner.Nick != "" || plan.HasTextOverlays() || plan.KeyDropBanner.Enabled() {
		bannerFontPath = streamclips.FindBannerFont()
		if bannerFontPath == "" {
			return fmt.Errorf("render banner or text overlays: embedded font unavailable and no supported fallback font found")
		}
	}

	previousStatus := j.Status
	if err := w.repo.UpdateStatus(ctx, j.ID, streamclips.StatusRendering, ""); err != nil {
		return fmt.Errorf("mark stream rendering: %w", err)
	}
	owned, stateErr := w.writeOwnedStreamRenderAttempt(
		j.ID, variant, intent, hasIntent,
		streamclips.StatusRendering, nil, "", "",
	)
	if stateErr != nil || !owned {
		rollbackErr := w.repo.UpdateStatus(ctx, j.ID, previousStatus, "")
		if stateErr == nil {
			stateErr = fmt.Errorf("%w: render attempt lost ownership during claim", errStreamRenderSuperseded)
		}
		return errors.Join(stateErr, rollbackErr)
	}
	claim.previousStatus = previousStatus
	claim.claimed = true
	releaseClaim()
	claimReleased = true

	workDir, cleanup, err := prepareStageDir(cfg.WorkDir, j.ID, "stream-render")
	if err != nil {
		return err
	}
	defer cleanup()

	// Burn the plan's sponsor code into the plate PNG once per render. The
	// filtergraph only scales and overlays this file, so a code change always
	// produces a new plate instead of relying on in-filter drawtext.
	keyDropImagePath := ""
	if plan.KeyDropBanner.Enabled() {
		platePath := filepath.Join(workDir, "keydrop-banner.png")
		if err := keydropbanner.CompositeWithCode(
			cfg.FFmpegPath,
			plan.KeyDropBanner.Family,
			plan.KeyDropBanner.Style,
			plan.KeyDropBanner.Code,
			bannerFontPath,
			platePath,
		); err != nil {
			return fmt.Errorf("composite keydrop banner code %q: %w", keydropbanner.EffectiveCode(plan.KeyDropBanner.Code), err)
		}
		keyDropImagePath = platePath
	}

	sourcePath := filepath.Join(workDir, "source.mp4")
	if err := materializeStorageFile(w.storage, j.SourcePath, sourcePath); err != nil {
		return fmt.Errorf("materialize stream source: %w", err)
	}
	outDir := filepath.Join(workDir, "out", "shortslistosparasubir")
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return err
	}

	runCtx, cancel := context.WithTimeout(ctx, cfg.timeoutDuration())
	defer cancel()
	type publishArtifact struct {
		key  string
		path string
	}
	revisionID := uuid.New()
	revisionPrefix, err := streamclips.RenderRevisionPrefix(j.ID, variant, revisionID)
	if err != nil {
		return err
	}
	revisionCommitted := false
	defer func() {
		if revisionCommitted {
			return
		}
		if deleteErr := w.deleteStreamRenderRevision(j.ID, variant, revisionPrefix); deleteErr != nil {
			logWorkerError(j.ID, "delete uncommitted stream render revision", deleteErr)
		}
	}()
	var videos []streamclips.VideoEntry
	var delivery []streamclips.DeliveryEntry
	var publishArtifacts []publishArtifact
	var warnings []string
	firstRenderedVideo := ""
	musicPath := ""
	if plan.Music.Key != "" {
		if musicPath = resolveMusicFile(cfg.MusicDir, plan.Music.Key); musicPath == "" {
			// Requested music is unavailable; render without it rather than fail.
			warnings = append(warnings, fmt.Sprintf("music %q not found, rendering without music", plan.Music.Key))
		}
	}
	for _, clip := range plan.Clips {
		textPaths, err := writeClipOverlayTexts(workDir, clip)
		if err != nil {
			return err
		}
		outPath := filepath.Join(outDir, clip.ID+".mp4")
		args, err := streamclips.BuildFFmpegArgs(streamclips.FFmpegInputs{
			SourcePath:       sourcePath,
			OutputPath:       outPath,
			MusicPath:        musicPath,
			BannerFontPath:   bannerFontPath,
			KeyDropImagePath: keyDropImagePath,
			SourceHasAudio:   j.Probe.AudioCodec != "",
			TextOverlayPaths: textPaths,
		}, plan, clip)
		if err != nil {
			return err
		}
		renderStarted := time.Now()
		if _, err := w.runner.Run(runCtx, cfg.FFmpegPath, args...); err != nil {
			return fmt.Errorf("render clip %s: %w", clip.ID, err)
		}
		renderElapsed := time.Since(renderStarted)
		var outputBytes int64
		if info, statErr := os.Stat(outPath); statErr == nil {
			outputBytes = info.Size()
		}

		key, err := streamclips.RenderRevisionVideoKey(j.ID, variant, revisionID, clip.ID)
		if err != nil {
			return err
		}
		publishArtifacts = append(publishArtifacts, publishArtifact{key: key, path: outPath})
		deliveryName := clip.ID + ".mp4"
		deliveryKey, err := streamclips.RenderRevisionDeliveryKey(j.ID, variant, revisionID, deliveryName)
		if err != nil {
			return err
		}
		publishArtifacts = append(publishArtifacts, publishArtifact{key: deliveryKey, path: outPath})
		if firstRenderedVideo == "" {
			firstRenderedVideo = outPath
		}
		delivery = append(delivery, streamclips.DeliveryEntry{Name: deliveryName, Kind: "video", Key: deliveryKey})
		video := streamclips.NewVideoEntry(clip, key)
		video.Performance = streamclips.NewVideoPerformance(renderElapsed, video.DurationSeconds, outputBytes)
		videos = append(videos, video)
	}

	if len(videos) > 0 {
		coverPath := filepath.Join(outDir, "cover.jpg")
		if err := w.writeStreamCover(runCtx, cfg.FFmpegPath, firstRenderedVideo, coverPath); err != nil {
			return fmt.Errorf("generate stream cover: %w", err)
		}
		coverKey, err := streamclips.RenderRevisionDeliveryKey(j.ID, variant, revisionID, "cover.jpg")
		if err != nil {
			return err
		}
		publishArtifacts = append(publishArtifacts, publishArtifact{key: coverKey, path: coverPath})
		delivery = append(delivery, streamclips.DeliveryEntry{Name: "cover.jpg", Kind: "cover", Key: coverKey})
	}
	planPath := filepath.Join(outDir, "edit-plan.json")
	planBytes, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(planPath, append(planBytes, '\n'), 0o600); err != nil {
		return err
	}
	planKey, err := streamclips.RenderRevisionDeliveryKey(j.ID, variant, revisionID, "edit-plan.json")
	if err != nil {
		return err
	}
	publishArtifacts = append(publishArtifacts, publishArtifact{key: planKey, path: planPath})
	delivery = append(delivery, streamclips.DeliveryEntry{Name: "edit-plan.json", Kind: "plan", Key: planKey})
	metadataPath := filepath.Join(outDir, "metadata.txt")
	metadata := fmt.Sprintf("Título: %s\nOrigen: %s\nFormato: %s\nClips: %d\n", strings.TrimSpace(j.Title), publicSourceURL(j.SourceURL), variant, len(videos))
	if err := os.WriteFile(metadataPath, []byte(metadata), 0o600); err != nil {
		return err
	}
	metadataKey, err := streamclips.RenderRevisionDeliveryKey(j.ID, variant, revisionID, "metadata.txt")
	if err != nil {
		return err
	}
	publishArtifacts = append(publishArtifacts, publishArtifact{key: metadataKey, path: metadataPath})
	delivery = append(delivery, streamclips.DeliveryEntry{Name: "metadata.txt", Kind: "metadata", Key: metadataKey})
	manifestPath := filepath.Join(outDir, "manifest.json")
	manifestBytes, err := json.MarshalIndent(struct {
		Folder    string                      `json:"folder"`
		JobID     uuid.UUID                   `json:"job_id"`
		Variant   string                      `json:"variant"`
		Artifacts []streamclips.DeliveryEntry `json:"artifacts"`
	}{Folder: "shortslistosparasubir", JobID: j.ID, Variant: variant, Artifacts: delivery}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(manifestPath, append(manifestBytes, '\n'), 0o600); err != nil {
		return err
	}
	manifestKey, err := streamclips.RenderRevisionDeliveryKey(j.ID, variant, revisionID, "manifest.json")
	if err != nil {
		return err
	}
	publishArtifacts = append(publishArtifacts, publishArtifact{key: manifestKey, path: manifestPath})
	delivery = append(delivery, streamclips.DeliveryEntry{Name: "manifest.json", Kind: "manifest", Key: manifestKey})

	// Media and sidecars are uploaded into a new immutable revision. Until the
	// final status.json pointer is replaced, a partial upload is unreachable and
	// cannot corrupt an older completed render.
	for _, artifact := range publishArtifacts {
		if err := uploadFile(w.storage, artifact.key, artifact.path); err != nil {
			return fmt.Errorf("publish stream render artifact %s: %w", artifact.key, err)
		}
	}

	result, err := streamclips.NewRenderResult(j.ID, variant, videos, time.Now())
	if err != nil {
		return err
	}
	result.Warnings = warnings
	resultKey, err := streamclips.RenderRevisionResultKey(j.ID, variant, revisionID)
	if err != nil {
		return err
	}
	if err := putJSONToStorage(w.storage, resultKey, result); err != nil {
		return fmt.Errorf("write stream render result: %w", err)
	}
	galleryKey, err := streamclips.RenderRevisionGalleryKey(j.ID, variant, revisionID)
	if err != nil {
		return err
	}
	if err := w.storage.Put(galleryKey, strings.NewReader(streamclips.RenderGalleryHTML(j, videos))); err != nil {
		return fmt.Errorf("write stream gallery: %w", err)
	}
	state, err := streamclips.NewRenderState(j.ID, variant, streamclips.StatusRendered, warnings, "", videos)
	if err != nil {
		return err
	}
	state.ResultKey = resultKey
	state.GalleryKey = galleryKey
	state.ArtifactDir = revisionPrefix
	state.Delivery = delivery
	if hasIntent {
		state.AttemptID = intent.AttemptID
	}

	// The shared job lock turns this revalidation plus atomic status pointer
	// replacement into one commit relative to every HTTP plan mutation and any
	// competing variant worker. Immutable artifacts may already exist, but no
	// client can resolve them until this commit succeeds.
	releaseCommit := w.jobLocks.Lock(j.ID)
	commitReleased := false
	defer func() {
		if !commitReleased {
			releaseCommit()
		}
	}()
	if err := w.ensureStreamRenderIntentCurrent(ctx, j.ID, intent, hasIntent); err != nil {
		return err
	}
	_, owned, err = w.ownedStreamRenderState(j.ID, variant, intent, hasIntent)
	if err != nil {
		return err
	}
	if !owned {
		return fmt.Errorf("%w: render attempt no longer owns variant state", errStreamRenderSuperseded)
	}
	if err := w.writeStreamRenderState(state); err != nil {
		return err
	}
	revisionCommitted = true
	// A request may have resolved the previous state before this pointer swap
	// and open its artifacts afterward. Retain every published revision until
	// the whole job is deleted; only the deferred uncommitted cleanup is safe.
	releaseCommit()
	commitReleased = true
	logWorkerArtifacts(j.ID, tasks.TypeRenderStreamClip, []string{resultKey, galleryKey})
	if err := w.repo.UpdateStatus(ctx, j.ID, streamclips.StatusRendered, ""); err != nil {
		return errors.Join(
			errStreamRenderParentPromotion,
			fmt.Errorf("mark stream rendered: %w", err),
		)
	}

	return nil
}

func (w *StreamRenderWorker) writeStreamCover(ctx context.Context, ffmpegPath, videoPath, filename string) error {
	if strings.TrimSpace(videoPath) == "" {
		return errors.New("rendered video is required for cover generation")
	}
	_, err := w.runner.Run(ctx, ffmpegPath,
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", videoPath,
		"-frames:v", "1", "-vf", "scale=720:-2", "-q:v", "2", filename,
	)
	if err != nil {
		return err
	}
	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("verify cover output: %w", err)
	}
	if info.Size() == 0 {
		return errors.New("cover output is empty")
	}
	return nil
}

func publicSourceURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	parsed.User = nil
	query := parsed.Query()
	publicQuery := url.Values{}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	if host == "youtube.com" || strings.HasSuffix(host, ".youtube.com") {
		for _, key := range []string{"v", "list", "index", "t", "start"} {
			for _, value := range query[key] {
				publicQuery.Add(key, value)
			}
		}
	}
	if host == "kick.com" {
		for _, value := range query["clip"] {
			publicQuery.Add("clip", value)
		}
	}
	parsed.RawQuery = publicQuery.Encode()
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}

func validateStreamRenderIntent(plan streamclips.EditPlan, intent tasks.StreamRenderIntent, hasIntent bool) error {
	if !hasIntent {
		return nil
	}
	fingerprint, err := streamclips.EditPlanFingerprint(plan)
	if err != nil {
		return fmt.Errorf("fingerprint stream edit plan: %w", err)
	}
	if fingerprint != intent.EditPlanFingerprint {
		return fmt.Errorf("%w: admitted edit plan revision changed", errStreamRenderSuperseded)
	}
	return nil
}

func (w *StreamRenderWorker) ensureStreamRenderIntentCurrent(
	ctx context.Context,
	jobID uuid.UUID,
	intent tasks.StreamRenderIntent,
	hasIntent bool,
) error {
	if !hasIntent {
		return nil
	}
	current, err := w.repo.Get(ctx, jobID)
	if err != nil {
		return fmt.Errorf("reload stream job before publish: %w", err)
	}
	if current.Status != streamclips.StatusRendering {
		return fmt.Errorf("%w: parent render lease is no longer active", errStreamRenderSuperseded)
	}
	if len(current.EditPlan) == 0 {
		return fmt.Errorf("%w: current edit plan is missing", errStreamRenderSuperseded)
	}
	var plan streamclips.EditPlan
	if err := json.Unmarshal(current.EditPlan, &plan); err != nil {
		return fmt.Errorf("decode current edit plan before publish: %w", err)
	}
	plan = streamclips.NormalizeEditPlan(plan)
	if migrated, changed := streamclips.MigrateLegacySourceDuration(plan, current.Probe.DurationSeconds); changed {
		plan = migrated
	}
	return validateStreamRenderIntent(plan, intent, true)
}

// writeClipOverlayTexts materializes one text file per overlay so drawtext can
// read the user's text verbatim (expansion=none) instead of embedding it in
// the filtergraph, where escaping arbitrary characters is unreliable.
func writeClipOverlayTexts(workDir string, clip streamclips.ClipRange) ([]string, error) {
	if clip.Edit == nil || len(clip.Edit.TextOverlays) == 0 {
		return nil, nil
	}
	dir := filepath.Join(workDir, "overlay-text", clip.ID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	paths := make([]string, len(clip.Edit.TextOverlays))
	for i, overlay := range clip.Edit.TextOverlays {
		textPath := filepath.Join(dir, fmt.Sprintf("overlay%d.txt", i))
		if err := os.WriteFile(textPath, []byte(overlay.Text), 0o600); err != nil {
			return nil, fmt.Errorf("write text overlay for clip %s overlay %d: %w", clip.ID, i, err)
		}
		paths[i] = textPath
	}
	return paths, nil
}

func (w *StreamRenderWorker) writeStreamRenderState(state streamclips.RenderState) error {
	if err := streamclips.ValidateRenderStateArtifacts(state); err != nil {
		return fmt.Errorf("validate stream render state artifacts: %w", err)
	}
	id := state.JobID
	variant := state.Variant
	key, err := streamclips.RenderStateKey(id, variant)
	if err != nil {
		return err
	}
	return putJSONToStorage(w.storage, key, state)
}

// deleteStreamRenderRevision removes only a validated immutable revision
// namespace. Legacy canonical render prefixes are deliberately retained.
func (w *StreamRenderWorker) deleteStreamRenderRevision(id uuid.UUID, variant, artifactDir string) error {
	base, err := streamclips.RenderPrefix(id, variant)
	if err != nil {
		return err
	}
	revisionText := strings.TrimPrefix(artifactDir, base+"/revisions/")
	if revisionText == artifactDir || revisionText == "" || strings.Contains(revisionText, "/") {
		return nil
	}
	revisionID, err := uuid.Parse(revisionText)
	if err != nil || revisionID == uuid.Nil {
		return nil
	}
	want, err := streamclips.RenderRevisionPrefix(id, variant, revisionID)
	if err != nil || want != artifactDir {
		return nil
	}
	deleter, ok := w.storage.(interface{ DeleteTree(string) error })
	if !ok {
		return nil
	}
	return deleter.DeleteTree(artifactDir)
}

func (c StreamRenderWorkerConfig) withDefaults() StreamRenderWorkerConfig {
	if c.Timeout == "" {
		c.Timeout = defaultMediaWorkerTimeout
	}
	return c
}

func (c StreamRenderWorkerConfig) validate() error {
	if c.FFmpegPath == "" {
		return fmt.Errorf("ffmpeg is required")
	}
	if _, err := time.ParseDuration(c.Timeout); err != nil {
		return fmt.Errorf("timeout must be a duration: %w", err)
	}
	return nil
}

func (c StreamRenderWorkerConfig) timeoutDuration() time.Duration {
	d, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return 20 * time.Minute
	}
	return d
}

// streamStatusUpdater is the single method markStreamFailed needs; every
// stream repository the workers use (render, acquire) satisfies it.
type streamStatusUpdater interface {
	UpdateStatus(ctx context.Context, id uuid.UUID, s streamclips.Status, failureReason string) error
}

func markStreamFailed(repo streamStatusUpdater, id uuid.UUID, reason string) {
	if err := updateStreamStatus(repo, id, streamclips.StatusFailed, reason); err != nil {
		logWorkerError(id, "mark stream failed", err)
	}
}

func updateStreamStatus(repo streamStatusUpdater, id uuid.UUID, status streamclips.Status, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), failureWriteTimeout)
	defer cancel()
	return repo.UpdateStatus(ctx, id, status, reason)
}

func (w *RenderWorker) HandleRenderVariant(ctx context.Context, t *asynq.Task) error {
	var payload tasks.RenderVariantPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	j, err := w.repo.Get(ctx, payload.JobID)
	if err != nil {
		return fmt.Errorf("load job %s: %w", payload.JobID, err)
	}
	variant := payload.Variant
	if variant == "" {
		variant = editor.DefaultPreset().Name
	}
	if err := w.render(ctx, j, variant, payload.MusicKey, payload.MusicVolume, payload.GameVolume, payload.Edit); err != nil {
		recordStageFailure(j.ID, obs.StageWorker, tasks.TypeRenderVariant, errorClass(tasks.TypeRenderVariant, err), err)
		logWorkerError(j.ID, tasks.TypeRenderVariant, err)
		return err
	}
	return nil
}

func (w *RenderWorker) render(ctx context.Context, j job.Job, variant, musicKey string, musicVolume float64, gameVolume *float64, edit renderplan.EditRequest) (err error) {
	edit = renderplan.NormalizeEditRequest(edit)
	if err := edit.Validate(); err != nil {
		return err
	}
	loadout, err := renderplan.LoadoutForVariant(variant)
	if err != nil {
		return err
	}
	if j.KillPlan == nil {
		return fmt.Errorf("job %s has no kill plan", j.ID)
	}
	recordingResult, err := readStoredRecordingResult(w.storage, j.ID)
	if err != nil {
		return err
	}
	cfg := w.cfg.withDefaults()
	if isFullDemoNativeMix(loadout.Preset, edit) {
		musicKey = ""
		musicVolume = 0
		gameVolume = nil
	}
	musicPath := resolveMusicFile(cfg.MusicDir, musicKey)
	effectiveMusic := &renderplan.MusicSnapshot{}
	effectiveMusicVolume := 0.0
	if musicPath != "" {
		effectiveMusicVolume = musicVolume
		if effectiveMusicVolume <= 0 {
			effectiveMusicVolume = 1
		}
		effectiveMusic = &renderplan.MusicSnapshot{Key: musicKey, Volume: effectiveMusicVolume, GameVolume: gameVolume}
	}
	inputFingerprint, err := renderInputFingerprint(recordingResult, j.KillPlan, variant, musicKey, musicPath, effectiveMusicVolume, gameVolume, edit)
	if err != nil {
		return fmt.Errorf("fingerprint render inputs: %w", err)
	}
	previousState, _, err := w.readRenderVariantState(j.ID, variant)
	if err != nil {
		return fmt.Errorf("read render state: %w", err)
	}
	ready, cachedWarnings, keys, err := renderVariantOutputsReady(w.storage, j.ID, variant, inputFingerprint, previousState)
	if err != nil {
		return err
	}
	if ready {
		cachedStatus := renderVariantCompletionStatus(editor.Result{Warnings: cachedWarnings})
		if previousState != nil && previousState.ReviewResolvedFor(cachedWarnings) {
			cachedStatus = renderplan.RenderVariantStatusReady
		}
		if previousState == nil || previousState.Status != cachedStatus || !reflect.DeepEqual(previousState.Warnings, cachedWarnings) {
			migratedState, stateErr := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
				JobID:    j.ID,
				Loadout:  loadout,
				Status:   cachedStatus,
				Warnings: cachedWarnings,
				Previous: previousState,
			})
			if stateErr != nil {
				return stateErr
			}
			preserveRenderArtifactPointer(&migratedState, previousState)
			if previousState != nil && previousState.ReviewResolvedFor(cachedWarnings) {
				migratedState.ReviewResolution = previousState.ReviewResolution
			}
			if stateErr := w.writeRenderVariantState(migratedState); stateErr != nil {
				return fmt.Errorf("write migrated cached render state: %w", stateErr)
			}
		}
		logWorkerSkip(j.ID, tasks.TypeRenderVariant, keys)
		return nil
	}
	if err := cfg.validate(); err != nil {
		return err
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:    j.ID,
		Loadout:  loadout,
		Status:   renderplan.RenderVariantStatusRendering,
		Previous: previousState,
	})
	if err != nil {
		return err
	}
	preserveRenderArtifactPointer(&state, previousState)
	if err := w.writeRenderVariantState(state); err != nil {
		return fmt.Errorf("write rendering state: %w", err)
	}
	currentState := &state
	var result editor.Result
	defer func() {
		if err == nil {
			return
		}
		failedState, stateErr := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
			JobID:    j.ID,
			Loadout:  loadout,
			Status:   renderplan.RenderVariantStatusFailed,
			Warnings: result.Warnings,
			Error:    renderplan.RenderVariantFailureMessage(result, err),
			Previous: currentState,
		})
		if stateErr != nil {
			err = fmt.Errorf("%w; build failed render state: %v", err, stateErr)
			return
		}
		preserveRenderArtifactPointer(&failedState, previousState)
		if writeErr := w.writeRenderVariantState(failedState); writeErr != nil {
			err = fmt.Errorf("%w; write failed render state: %v", err, writeErr)
		}
	}()

	workDir, cleanup, err := prepareStageDir(cfg.WorkDir, j.ID, "render")
	if err != nil {
		return err
	}
	defer cleanup()

	localRecordingResult := filepath.Join(workDir, "recording-result.json")
	if err := localizeSegmentClips(w.storage, j.ID, workDir, &recordingResult); err != nil {
		return err
	}
	if err := writeJSONFile(localRecordingResult, recordingResult); err != nil {
		return fmt.Errorf("write localized recording result: %w", err)
	}
	localKillPlan := filepath.Join(workDir, "killplan.json")
	if err := writeJSONFile(localKillPlan, j.KillPlan); err != nil {
		return fmt.Errorf("write kill plan: %w", err)
	}

	outDir := filepath.Join(workDir, "out")
	publishDir := filepath.Join(outDir, "shortslistosparasubir")
	if err := w.writeEditDocument(outDir, j.ID, loadout, recordingResult, edit, effectiveMusic); err != nil {
		return err
	}
	args := []string{
		"--recording-result", localRecordingResult,
		"--killplan", localKillPlan,
		"--out", outDir,
		"--publish-dir", publishDir,
		"--preset", loadout.Preset,
		"--output-format", edit.Format,
		"--kill-effect", edit.KillEffect,
		"--transition", edit.Transition,
		"--hook=" + strconv.FormatBool(edit.HookText),
		"--kill-counter=" + strconv.FormatBool(edit.KillCounter),
		"--intro=" + strconv.FormatBool(edit.Intro),
		"--outro=" + strconv.FormatBool(edit.Outro),
	}
	args = append(args, explicitCoverArgs(loadout, edit)...)
	args = append(args, compileSegmentsArgs(recording.SegmentIDs(recordingResult))...)
	if overlayPath, overlayErr := w.writeFullDemoOverlay(j, workDir, loadout.Preset, edit); overlayErr != nil {
		return overlayErr
	} else if overlayPath != "" {
		args = append(args, "--full-demo-overlay", overlayPath)
	}
	if plateDir := overlayAssetsPlatesDir(w.storage); plateDir != "" {
		args = append(args, "--overlay-assets", plateDir)
	}
	if edit.IntroText != "" {
		args = append(args, "--intro-text", edit.IntroText)
	}
	if edit.OutroText != "" {
		args = append(args, "--outro-text", edit.OutroText)
	}
	if style := strings.TrimSpace(edit.KeyDropStyle); style != "" {
		if family := strings.TrimSpace(edit.KeyDropFamily); family != "" {
			args = append(args, "--keydrop-family", family)
		}
		args = append(args, "--keydrop-style", style)
		if code := strings.TrimSpace(edit.KeyDropCode); code != "" {
			args = append(args, "--keydrop-code", code)
		}
		if y := edit.KeyDropPositionY; y != nil {
			args = append(args, "--keydrop-position-y", strconv.FormatFloat(*y, 'f', 6, 64))
		}
		if s := edit.KeyDropStartSeconds; s != nil {
			args = append(args, "--keydrop-start", strconv.FormatFloat(*s, 'f', 6, 64))
		}
		if e := edit.KeyDropEndSeconds; e != nil {
			args = append(args, "--keydrop-end", strconv.FormatFloat(*e, 'f', 6, 64))
		}
	}
	if cfg.FFmpegPath != "" {
		args = append(args, "--ffmpeg", cfg.FFmpegPath)
	}
	if encoder := studioRenderVideoEncoder(cfg.FFmpegPath); encoder != "" {
		args = append(args, "--video-encoder", encoder)
	}
	if cfg.FFprobePath != "" {
		args = append(args, "--ffprobe", cfg.FFprobePath)
	}
	if musicPath != "" {
		args = append(
			args,
			"--music", musicPath,
			"--music-volume", strconv.FormatFloat(effectiveMusicVolume, 'f', -1, 64),
		)
		if gameVolume != nil {
			args = append(args, "--game-volume", strconv.FormatFloat(*gameVolume, 'f', -1, 64))
		}
	} else if musicKey != "" {
		// Requested music is unavailable; render without it rather than fail.
		logWorkerError(j.ID, tasks.TypeRenderVariant, fmt.Errorf("music %q not found in %q; rendering without music", musicKey, cfg.MusicDir))
	}
	if edit.VoiceComms {
		voiceDir, err := w.prepareVoiceDir(j, workDir)
		if err != nil {
			return err
		}
		if voiceDir != "" {
			args = append(args, "--voice-dir", voiceDir)
			if edit.VoiceVolume != nil {
				args = append(args, "--voice-volume", strconv.FormatFloat(*edit.VoiceVolume, 'f', -1, 64))
			}
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, cfg.timeoutDuration())
	defer cancel()

	progressPath := filepath.Join(workDir, "editor-progress.json")
	args = append(args, "--progress-out", progressPath)
	progressCtx, progressCancel := context.WithCancel(ctx)
	reporter := newRenderProgressReporter(w.storage, j.ID, progressPath)
	go reporter.watch(progressCtx)

	_, runErr := w.runner.Run(runCtx, cfg.EditorPath, args...)
	progressCancel()

	resultPath := filepath.Join(outDir, "shorts-result.json")
	if err := readJSONFile(resultPath, &result); err != nil {
		if runErr != nil {
			return runErr
		}
		return fmt.Errorf("read render result: %w", err)
	}
	result.InputFingerprint = inputFingerprint
	if cfg.FFprobePath != "" {
		if err := probeRenderResult(runCtx, w.runner, cfg.FFprobePath, &result); err != nil {
			result.Warnings = append(result.Warnings, "ffprobe quality metadata: "+err.Error())
		}
	}
	result.Warnings = renderplan.CompleteRenderWarnings(result)
	if err := writeJSONFile(resultPath, result); err != nil {
		return fmt.Errorf("write fingerprinted render result: %w", err)
	}
	if runErr != nil {
		return runErr
	}
	if err := renderplan.ValidateRenderVariantRunResult(result); err != nil {
		return err
	}
	result.GalleryPath = filepath.Join(publishDir, "index.html")
	result.SummaryPath = filepath.Join(publishDir, "publish-summary.md")
	revisionID := uuid.New()
	revisionPrefix, err := renderplan.RenderVariantRevisionPrefix(j.ID, variant, revisionID)
	if err != nil {
		return err
	}
	revisionCommitted := false
	defer func() {
		if revisionCommitted {
			return
		}
		if deleteErr := deleteRenderVariantRevision(w.storage, j.ID, variant, revisionPrefix); deleteErr != nil {
			logWorkerError(j.ID, "delete uncommitted render revision", deleteErr)
		}
	}()
	if err := writeDurableRenderDocuments(j.ID, variant, revisionID, outDir, publishDir, resultPath, result); err != nil {
		return err
	}
	keys, err = uploadRenderVariantOutputs(w.storage, j.ID, variant, revisionID, outDir, publishDir, resultPath, result)
	if err != nil {
		return err
	}
	logWorkerArtifacts(j.ID, tasks.TypeRenderVariant, keys)
	readyStatus := renderVariantCompletionStatus(result)
	readyState, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:      j.ID,
		Loadout:    loadout,
		Status:     readyStatus,
		Warnings:   result.Warnings,
		Previous:   currentState,
		RevisionID: revisionID,
	})
	if err != nil {
		return err
	}
	if err := w.writeRenderVariantState(readyState); err != nil {
		return fmt.Errorf("write ready render state: %w", err)
	}
	revisionCommitted = true
	// A request may have resolved the previous state before this pointer swap
	// and open its artifacts afterward. Retain every published revision until
	// the whole job is deleted; only the deferred uncommitted cleanup is safe.
	return nil
}

func renderVariantCompletionStatus(result editor.Result) string {
	if len(result.Warnings) > 0 {
		return renderplan.RenderVariantStatusReview
	}
	return renderplan.RenderVariantStatusReady
}

func explicitCoverArgs(loadout renderplan.Loadout, edit renderplan.EditRequest) []string {
	return []string{
		"--cover-sheets=" + strconv.FormatBool(loadout.CoverSheets),
		"--cover-first-frame=" + strconv.FormatBool(edit.CoverFirstFrame),
		"--covers=" + strconv.FormatBool(edit.CoverStrategy != renderplan.CoverStrategyNone),
	}
}

func preserveRenderArtifactPointer(state *renderplan.RenderVariantState, previous *renderplan.RenderVariantState) {
	if state == nil || previous == nil {
		return
	}
	if previous.Status != renderplan.RenderVariantStatusReady && previous.Status != renderplan.RenderVariantStatusReview {
		return
	}
	state.EditDocumentKey = previous.EditDocumentKey
	state.EditManifestKey = previous.EditManifestKey
	state.RenderResultKey = previous.RenderResultKey
	state.PackManifestKey = previous.PackManifestKey
	state.GalleryKey = previous.GalleryKey
	state.PublishSummaryKey = previous.PublishSummaryKey
	if previous.ArtifactPrefix != "" {
		state.ArtifactPrefix = previous.ArtifactPrefix
	}
}

type renderRevisionDeleter interface {
	DeleteTree(string) error
}

// deleteRenderVariantRevision removes only the exact immutable revision
// namespace owned by id and variant. Canonical, foreign, and malformed
// prefixes are rejected before the storage backend sees them.
func deleteRenderVariantRevision(store storage.Storage, id uuid.UUID, variant, prefix string) error {
	base, err := artifacts.RenderVariantPrefix(id, variant)
	if err != nil {
		return err
	}
	revisionText := strings.TrimPrefix(prefix, base+"/revisions/")
	if revisionText == prefix || revisionText == "" || strings.Contains(revisionText, "/") {
		return fmt.Errorf("render revision prefix is outside the expected job and variant namespace")
	}
	revisionID, err := uuid.Parse(revisionText)
	if err != nil || revisionID == uuid.Nil {
		return fmt.Errorf("render revision prefix has an invalid revision id")
	}
	want, err := renderplan.RenderVariantRevisionPrefix(id, variant, revisionID)
	if err != nil {
		return err
	}
	if want != prefix {
		return fmt.Errorf("render revision prefix does not match its canonical revision namespace")
	}
	deleter, ok := store.(renderRevisionDeleter)
	if !ok {
		return nil
	}
	return deleter.DeleteTree(prefix)
}

func writeDurableRenderDocuments(id uuid.UUID, variant string, revisionID uuid.UUID, outDir, publishDir, resultPath string, local editor.Result) error {
	refs, err := renderplan.NewRenderVariantRevisionArtifactRef(id, variant, revisionID, renderplan.RenderVariantArtifactResult, "")
	if err != nil {
		return err
	}
	if err := renderplan.ValidateRenderVariantRunResult(local); err != nil {
		return fmt.Errorf("validate render result for durable projection: %w", err)
	}
	local.GalleryPath = filepath.Join(publishDir, "index.html")
	local.SummaryPath = filepath.Join(publishDir, "publish-summary.md")
	prefix := strings.TrimSuffix(refs.Key, "/render-result.json")
	durable := local
	durable.Shorts = append([]editor.ShortResult(nil), local.Shorts...)
	for i := range durable.Shorts {
		durable.Shorts[i].Parts = append([]editor.ShortPart(nil), local.Shorts[i].Parts...)
	}
	durable.RecordingResult = recording.ResultArtifactKey(id)
	durable.KillPlan = ""
	durable.OutputDir = prefix
	durable.PublishDir = prefix
	durable.GalleryPath = prefix + "/index.html"
	durable.SummaryPath = prefix + "/publish-summary.md"
	for i := range durable.Shorts {
		if err := projectDurableShort(id, variant, revisionID, &durable.Shorts[i]); err != nil {
			return fmt.Errorf("project render result short %d: %w", i, err)
		}
	}

	var document renderplan.EditDocument
	documentPath := filepath.Join(outDir, "edit-document.json")
	if err := readJSONFile(documentPath, &document); err != nil {
		return fmt.Errorf("read edit document for durable projection: %w", err)
	}

	var manifest editor.Manifest
	manifestPath := filepath.Join(outDir, "edit-manifest.json")
	if err := readJSONFile(manifestPath, &manifest); err != nil {
		return fmt.Errorf("read edit manifest for durable projection: %w", err)
	}

	var pack editor.PackManifest
	packPath := filepath.Join(publishDir, "pack-manifest.json")
	if err := readJSONFile(packPath, &pack); err != nil {
		return fmt.Errorf("read pack manifest for durable projection: %w", err)
	}
	resultSegmentIDs := make([]string, len(local.Shorts))
	for i := range local.Shorts {
		resultSegmentIDs[i] = local.Shorts[i].SegmentID
	}
	editSegmentIDs := make([]string, len(manifest.Shorts))
	for i := range manifest.Shorts {
		editSegmentIDs[i] = manifest.Shorts[i].SegmentID
	}
	if err := validateRenderManifestSegmentIDs("edit manifest", editSegmentIDs, resultSegmentIDs); err != nil {
		return err
	}
	packSegmentIDs := make([]string, len(pack.Items))
	for i := range pack.Items {
		packSegmentIDs[i] = pack.Items[i].SegmentID
	}
	if err := validateRenderManifestSegmentIDs("pack manifest", packSegmentIDs, resultSegmentIDs); err != nil {
		return err
	}

	document.Outputs.Prefix = prefix
	document.Outputs.RenderResult = prefix + "/render-result.json"
	document.Outputs.EditManifest = prefix + "/edit-manifest.json"
	document.Outputs.PackManifest = prefix + "/pack-manifest.json"
	document.Outputs.Gallery = prefix + "/index.html"
	document.Outputs.PublishSummary = prefix + "/publish-summary.md"
	document.Outputs.UploadReadyRoot = prefix
	if err := writeJSONFile(documentPath, document); err != nil {
		return fmt.Errorf("write durable edit document: %w", err)
	}

	manifest.RecordingResult = recording.ResultArtifactKey(id)
	manifest.KillPlan = ""
	manifest.OutputDir = prefix
	manifest.PublishDir = prefix
	manifest.GalleryPath = prefix + "/index.html"
	manifest.SummaryPath = prefix + "/publish-summary.md"
	for i := range manifest.Shorts {
		if err := projectDurableEdit(id, variant, revisionID, &manifest.Shorts[i]); err != nil {
			return fmt.Errorf("project edit manifest short %d: %w", i, err)
		}
	}
	if err := writeJSONFile(manifestPath, manifest); err != nil {
		return fmt.Errorf("write durable edit manifest: %w", err)
	}

	pack.RecordingResult = recording.ResultArtifactKey(id)
	pack.KillPlan = ""
	pack.PublishDir = prefix
	pack.GalleryPath = prefix + "/index.html"
	pack.SummaryPath = prefix + "/publish-summary.md"
	for i := range pack.Items {
		if err := projectDurablePublishItem(id, variant, revisionID, &pack.Items[i]); err != nil {
			return fmt.Errorf("project pack manifest item %d: %w", i, err)
		}
	}
	if err := writeJSONFile(packPath, pack); err != nil {
		return fmt.Errorf("write durable pack manifest: %w", err)
	}

	if err := os.WriteFile(filepath.Join(publishDir, "index.html"), []byte(durableRenderGallery(id, variant, revisionID, durable)), 0o600); err != nil {
		return fmt.Errorf("write durable render gallery: %w", err)
	}
	var summary strings.Builder
	fmt.Fprintf(&summary, "# ClipHub publish pack\n\nVariant: %s\n\n", variant)
	for _, short := range durable.Shorts {
		fmt.Fprintf(&summary, "- %s: %s\n", short.SegmentID, short.PublishPath)
	}
	if err := os.WriteFile(filepath.Join(publishDir, "publish-summary.md"), []byte(summary.String()), 0o600); err != nil {
		return fmt.Errorf("write durable publish summary: %w", err)
	}
	if err := writeJSONFile(resultPath, durable); err != nil {
		return fmt.Errorf("write durable render result: %w", err)
	}
	return nil
}

func validateRenderManifestSegmentIDs(label string, got, want []string) error {
	seen := make(map[string]struct{}, len(got))
	for i, segmentID := range got {
		if err := artifacts.ValidateArtifactToken(label+" segment id", segmentID); err != nil {
			return fmt.Errorf("%s entry %d: %w", label, i, err)
		}
		if _, duplicate := seen[segmentID]; duplicate {
			return fmt.Errorf("%s contains duplicate segment id %q", label, segmentID)
		}
		seen[segmentID] = struct{}{}
	}
	if len(got) != len(want) {
		return fmt.Errorf("%s contains %d segment ids, want %d from render result", label, len(got), len(want))
	}
	for _, segmentID := range want {
		if _, ok := seen[segmentID]; !ok {
			return fmt.Errorf("%s is missing render result segment id %q", label, segmentID)
		}
	}
	return nil
}

type durableSegmentArtifactRefs struct {
	video   renderplan.RenderVariantArtifactRef
	caption renderplan.RenderVariantArtifactRef
	cover   renderplan.RenderVariantArtifactRef
}

func newDurableSegmentArtifactRefs(id uuid.UUID, variant string, revisionID uuid.UUID, segmentID string) (durableSegmentArtifactRefs, error) {
	video, err := renderplan.NewRenderVariantRevisionArtifactRef(id, variant, revisionID, renderplan.RenderVariantArtifactVideo, segmentID)
	if err != nil {
		return durableSegmentArtifactRefs{}, fmt.Errorf("derive video artifact: %w", err)
	}
	caption, err := renderplan.NewRenderVariantRevisionArtifactRef(id, variant, revisionID, renderplan.RenderVariantArtifactCaption, segmentID)
	if err != nil {
		return durableSegmentArtifactRefs{}, fmt.Errorf("derive caption artifact: %w", err)
	}
	cover, err := renderplan.NewRenderVariantRevisionArtifactRef(id, variant, revisionID, renderplan.RenderVariantArtifactCover, segmentID)
	if err != nil {
		return durableSegmentArtifactRefs{}, fmt.Errorf("derive cover artifact: %w", err)
	}
	return durableSegmentArtifactRefs{video: video, caption: caption, cover: cover}, nil
}

func projectDurableShort(id uuid.UUID, variant string, revisionID uuid.UUID, short *editor.ShortResult) error {
	refs, err := newDurableSegmentArtifactRefs(id, variant, revisionID, short.SegmentID)
	if err != nil {
		return err
	}
	short.Input = ""
	short.Output = refs.video.Key
	short.PromptPath = ""
	short.PublishPath = refs.video.Key
	short.CaptionPath = refs.caption.Key
	if short.CoverPath != "" {
		short.CoverPath = refs.cover.Key
	}
	short.CoverSheetPath = ""
	short.RenderLogPath = ""
	short.QualityLogPath = ""
	short.OutputArtifact.Path = refs.video.Key
	short.PublishArtifact.Path = refs.video.Key
	if short.CoverArtifact.Path != "" {
		short.CoverArtifact.Path = refs.cover.Key
	}
	short.CoverSheetArtifact.Path = ""
	for i := range short.Parts {
		short.Parts[i].Input = ""
		short.Parts[i].SourceArtifact.Path = ""
	}
	return nil
}

func projectDurableEdit(id uuid.UUID, variant string, revisionID uuid.UUID, short *editor.ShortEdit) error {
	refs, err := newDurableSegmentArtifactRefs(id, variant, revisionID, short.SegmentID)
	if err != nil {
		return err
	}
	short.Input = ""
	short.Output = refs.video.Key
	short.PromptPath = ""
	short.PublishPath = refs.video.Key
	short.CaptionPath = refs.caption.Key
	if short.CoverPath != "" {
		short.CoverPath = refs.cover.Key
	}
	short.CoverSheetPath = ""
	short.RenderLogPath = ""
	short.QualityLogPath = ""
	short.FFmpegCommand = nil
	short.CoverCommand = nil
	short.CoverSheetCommand = nil
	short.QualityCommand = nil
	short.OutputArtifact.Path = refs.video.Key
	short.PublishArtifact.Path = refs.video.Key
	return nil
}

func projectDurablePublishItem(id uuid.UUID, variant string, revisionID uuid.UUID, item *editor.PublishItem) error {
	refs, err := newDurableSegmentArtifactRefs(id, variant, revisionID, item.SegmentID)
	if err != nil {
		return err
	}
	item.Source = ""
	item.Video = refs.video.Key
	item.CaptionPath = refs.caption.Key
	if item.CoverPath != "" {
		item.CoverPath = refs.cover.Key
	}
	item.CoverSheetPath = ""
	item.Artifact.Path = refs.video.Key
	if item.CoverArtifact.Path != "" {
		item.CoverArtifact.Path = refs.cover.Key
	}
	item.CoverSheetArtifact.Path = ""
	item.SourceArtifact.Path = ""
	return nil
}

func durableRenderGallery(id uuid.UUID, variant string, revisionID uuid.UUID, result editor.Result) string {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width\"><title>ClipHub Publish Pack</title></head><body><h1>ClipHub publish pack</h1>")
	base := fmt.Sprintf(
		"/api/jobs/%s/renders/%s/revisions/%s",
		id,
		url.PathEscape(variant),
		revisionID,
	)
	for _, short := range result.Shorts {
		name := url.PathEscape(short.SegmentID)
		fmt.Fprintf(&b, "<article><h2>%s</h2><video controls src=\"%s/videos/%s\"></video>", html.EscapeString(short.Title), base, name)
		if short.CoverPath != "" {
			fmt.Fprintf(&b, "<img alt=\"cover\" src=\"%s/covers/%s\">", base, name)
		}
		fmt.Fprintf(&b, "<p><a href=\"%s/captions/%s\">Caption</a></p></article>", base, name)
	}
	b.WriteString("</body></html>")
	return b.String()
}

func (w *RenderWorker) writeEditDocument(outDir string, id uuid.UUID, loadout renderplan.Loadout, result recording.RecordingResult, edit renderplan.EditRequest, music *renderplan.MusicSnapshot) error {
	doc, err := renderplan.NewEditDocumentForLoadout(renderplan.NewEditDocumentForLoadoutOptions{
		JobID:      id,
		Loadout:    loadout,
		SegmentIDs: recording.SegmentIDs(result),
		Edit:       edit,
		Music:      music,
	})
	if err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(outDir, "edit-document.json"), doc)
}

func (w *RenderWorker) readRenderVariantState(id uuid.UUID, variant string) (*renderplan.RenderVariantState, bool, error) {
	key, err := renderplan.RenderVariantStateKey(id, variant)
	if err != nil {
		return nil, false, err
	}
	rc, err := w.storage.Open(key)
	if err != nil {
		if !storage.IsNotExist(err) {
			return nil, false, err
		}
		return nil, false, nil
	}
	defer rc.Close()
	var state renderplan.RenderVariantState
	if err := json.NewDecoder(rc).Decode(&state); err != nil {
		return nil, false, err
	}
	return &state, true, nil
}

func (w *RenderWorker) writeRenderVariantState(state renderplan.RenderVariantState) error {
	key, err := renderplan.RenderVariantStateKey(state.JobID, state.Variant)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return w.storage.Put(key, bytes.NewReader(b))
}

func probeRenderResult(ctx context.Context, runner commandRunner, ffprobePath string, result *editor.Result) error {
	// Each short probes an independent file and writes only its own struct, so
	// the probes run concurrently (bounded) and the per-short writes never race.
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	sem := make(chan struct{}, probeConcurrency)
	for i := range result.Shorts {
		short := &result.Shorts[i]
		path := short.PublishPath
		role := "publish"
		if path == "" {
			path = short.Output
			role = "short"
		}
		if path == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			artifact, err := probeVideoArtifact(ctx, runner, ffprobePath, short.SegmentID, role, path)
			if err != nil {
				artifact = recording.RecordingArtifact{
					SegmentID:  short.SegmentID,
					Role:       role,
					Type:       "video",
					Path:       path,
					ProbeError: err.Error(),
				}
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
			if role == "publish" {
				short.PublishArtifact = artifact
			} else {
				short.OutputArtifact = artifact
			}
		}()
	}
	wg.Wait()
	return firstErr
}

func probeVideoArtifact(ctx context.Context, runner commandRunner, ffprobePath, segmentID, role, path string) (recording.RecordingArtifact, error) {
	out, err := runner.Run(ctx, ffprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name,width,height,r_frame_rate,duration:format=duration,size",
		"-of", "json",
		path,
	)
	artifact := recording.RecordingArtifact{
		SegmentID: segmentID,
		Role:      role,
		Type:      "video",
		Path:      path,
	}
	if stat, statErr := os.Stat(path); statErr == nil {
		artifact.SizeBytes = stat.Size()
	}
	if err != nil {
		artifact.ProbeError = err.Error()
		return artifact, err
	}
	if err := recording.ApplyProbeOutput(&artifact, out); err != nil {
		artifact.ProbeError = err.Error()
		return artifact, err
	}
	return artifact, nil
}

func (c RenderWorkerConfig) withDefaults() RenderWorkerConfig {
	if c.Timeout == "" {
		c.Timeout = defaultMediaWorkerTimeout
	}
	return c
}

func (c RenderWorkerConfig) validate() error {
	required := map[string]string{
		"editor":  c.EditorPath,
		"timeout": c.Timeout,
	}
	for name, value := range required {
		if value == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if _, err := time.ParseDuration(c.Timeout); err != nil {
		return fmt.Errorf("timeout must be a duration: %w", err)
	}
	return nil
}

func (c RenderWorkerConfig) timeoutDuration() time.Duration {
	d, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return 20 * time.Minute
	}
	return d
}

func prepareStageDir(root string, id uuid.UUID, stage string) (string, func(), error) {
	base := root
	cleanup := func() {}
	if base == "" {
		base = os.TempDir()
	}
	if err := os.MkdirAll(base, 0o750); err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp(base, fmt.Sprintf("zv-%s-%s-", stage, id))
	if err != nil {
		return "", nil, err
	}
	if root == "" {
		cleanup = func() { _ = os.RemoveAll(dir) }
	}
	return dir, cleanup, nil
}

func copyStorageToFile(store storage.Storage, key, path string) error {
	rc, err := store.Open(key)
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	// #nosec G304 -- path is constructed under the worker stage directory.
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, rc)
	return err
}

// storagePathResolver is satisfied by storage.Local (storage.Local.ResolvePath),
// used to materialize an artifact without copying its bytes.
type storagePathResolver interface {
	ResolvePath(key string) (string, error)
}

// materializeStorageFile places the artifact at key at path so an external
// tool (HLAE/CS2, ffmpeg) can read it as a plain file. When storage resolves
// to a real local path (storage.Local), this hardlinks path straight to the
// storage-owned file instead of copying multi-gigabyte demos/clips; it falls
// back to copyStorageToFile for other storage backends or when linking fails
// (e.g. path and the storage root are on different volumes).
//
// A hardlink shares its inode with the durable artifact in storage: every
// caller must treat path as read-only from here on. Nothing downstream of
// record/compose writes in place to a materialized demo, stream source, or
// segment clip - each stage always writes a new output file - so this holds
// today, but it must keep holding for any future writer of these paths.
func extractVoiceTracks(demoPath, target, dir string) (int, error) {
	index, _, err := voicecomms.ExtractFile(demoPath, target, dir)
	if err != nil {
		return 0, err
	}
	return len(index.Tracks), nil
}

func (w *RenderWorker) prepareVoiceDir(j job.Job, workDir string) (string, error) {
	target := strings.TrimSpace(j.TargetSteamID)
	if target == "" {
		return "", fmt.Errorf("voice comms requested but job %s has no target steamid", j.ID)
	}
	if strings.TrimSpace(j.DemoPath) == "" {
		return "", fmt.Errorf("voice comms requested but job %s has no demo", j.ID)
	}
	demoPath := filepath.Join(workDir, "demo.dem")
	if err := materializeStorageFile(w.storage, j.DemoPath, demoPath); err != nil {
		return "", fmt.Errorf("materialize demo: %w", err)
	}
	voiceDir := filepath.Join(workDir, "voice")
	extract := w.voiceExtract
	if extract == nil {
		extract = extractVoiceTracks
	}
	tracks, err := extract(demoPath, target, voiceDir)
	if err != nil {
		return "", fmt.Errorf("extract voice: %w", err)
	}
	if tracks == 0 {
		return "", nil
	}
	return voiceDir, nil
}

func materializeStorageFile(store storage.Storage, key, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if resolver, ok := store.(storagePathResolver); ok {
		if src, err := resolver.ResolvePath(key); err == nil {
			_ = os.Remove(path)
			if linkErr := os.Link(src, path); linkErr == nil {
				return nil
			}
		}
	}
	return copyStorageToFile(store, key, path)
}

func uploadFile(store storage.Storage, key, path string) error {
	// #nosec G304 -- path is produced by recorder/composer stage outputs before upload.
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return store.Put(key, f)
}

func uploadOptionalFile(store storage.Storage, key, path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, uploadFile(store, key, path)
}

// recordingCommitReady reports whether result names a complete, reusable
// durable recording commit. Structural validation alone is insufficient:
// downstream readers also require the canonical script and every planned clip.
func recordingCommitReady(store storage.Storage, id uuid.UUID, result recording.RecordingResult) (bool, error) {
	if result.Error != "" {
		return false, nil
	}
	if err := recording.ValidateUploadResult(result); err != nil {
		return false, nil
	}
	exists, err := store.Exists(recording.ScriptArtifactKey(id))
	if err != nil || !exists {
		return false, err
	}
	for _, segment := range result.Plan.Segments {
		key, err := recording.SegmentClipArtifactKey(id, segment.ID)
		if err != nil {
			return false, err
		}
		exists, err := store.Exists(key)
		if err != nil || !exists {
			return false, err
		}
	}
	return true, nil
}

func uploadRecordingOutputs(
	store storage.Storage,
	id uuid.UUID,
	outDir, resultPath string,
	attempt, durable, previous recording.RecordingResult,
	hasPrevious bool,
) ([]string, error) {
	previousCommitRecoverable := false
	previousClipKeys := make(map[string]struct{})
	var previousScript []byte
	if hasPrevious {
		var err error
		previousCommitRecoverable, err = recordingCommitReady(store, id, previous)
		if err != nil {
			return nil, fmt.Errorf("verify previous recording commit: %w", err)
		}
		if previousCommitRecoverable {
			previousScript, err = readRecordingScriptForRollback(store, id)
			if err != nil {
				return nil, fmt.Errorf("snapshot previous recording script: %w", err)
			}
			for _, segment := range previous.Plan.Segments {
				key, keyErr := recording.SegmentClipArtifactKey(id, segment.ID)
				if keyErr != nil {
					return nil, fmt.Errorf("derive previous recording clip key: %w", keyErr)
				}
				previousClipKeys[key] = struct{}{}
			}
		}
	}
	preserveUnchangedCommit := func(cause error) error {
		if !previousCommitRecoverable {
			return cause
		}
		return &recordingCommitPreservedError{cause: cause}
	}

	targets, err := recording.NewUploadTargets(recording.NewUploadTargetsOptions{
		JobID:      id,
		OutDir:     outDir,
		ResultPath: resultPath,
		Result:     attempt,
	})
	if err != nil {
		return nil, preserveUnchangedCommit(err)
	}
	if err := recording.ValidateUploadResult(durable); err != nil {
		return nil, preserveUnchangedCommit(err)
	}
	resultKey := recording.ResultArtifactKey(id)
	if len(targets) == 0 || targets[len(targets)-1].Key != resultKey {
		return nil, preserveUnchangedCommit(fmt.Errorf("recording upload plan does not end with the canonical result"))
	}
	previousClipsIntact := previousCommitRecoverable
	failPublication := func(cause error) error {
		if !previousClipsIntact {
			return cause
		}
		if restoreErr := restorePreviousRecordingCommit(store, id, previous, previousScript); restoreErr != nil {
			return errors.Join(cause, fmt.Errorf("restore previous recording commit: %w", restoreErr))
		}
		return &recordingCommitPreservedError{cause: cause}
	}

	pending := recording.RecordingResult{PublicationPending: true}
	if hasPrevious {
		pending = previous
		pending.PublicationPending = true
	}
	if err := putRecordingResult(store, id, pending); err != nil {
		return nil, preserveUnchangedCommit(fmt.Errorf("invalidate recording result before publishing outputs: %w", err))
	}

	keys := make([]string, 0, len(targets))
	for _, target := range targets[:len(targets)-1] {
		uploaded := false
		if target.Required {
			if err := uploadFile(store, target.Key, target.Path); err != nil {
				if target.MissingMessage != "" && os.IsNotExist(err) {
					return nil, failPublication(errors.New(target.MissingMessage))
				}
				return nil, failPublication(fmt.Errorf("upload %s: %w", target.Label, err))
			}
			uploaded = true
		} else if ok, err := uploadOptionalFile(store, target.Key, target.Path); err != nil {
			return nil, failPublication(fmt.Errorf("upload %s: %w", target.Label, err))
		} else {
			uploaded = ok
		}
		if !uploaded {
			continue
		}
		keys = append(keys, target.Key)
		if _, overwrotePreviousClip := previousClipKeys[target.Key]; overwrotePreviousClip {
			previousClipsIntact = false
		}
	}
	durable.PublicationPending = false
	if err := putRecordingResult(store, id, durable); err != nil {
		return nil, failPublication(fmt.Errorf("commit recording result after publishing outputs: %w", err))
	}
	keys = append(keys, resultKey)
	return keys, nil
}

// recordingCommitPreservedError means a recording attempt failed while the
// last committed script, result, and every referenced clip remained usable.
// Callers must surface the failed attempt while preserving Recorded state.
type recordingCommitPreservedError struct {
	cause error
}

func (e *recordingCommitPreservedError) Error() string {
	return e.cause.Error()
}

func (e *recordingCommitPreservedError) Unwrap() error {
	return e.cause
}

const maxRecordingRollbackScriptBytes = 8 << 20

func readRecordingScriptForRollback(store storage.Storage, id uuid.UUID) ([]byte, error) {
	rc, err := store.Open(recording.ScriptArtifactKey(id))
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(io.LimitReader(rc, maxRecordingRollbackScriptBytes+1))
	closeErr := rc.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, err
	}
	if len(body) > maxRecordingRollbackScriptBytes {
		return nil, fmt.Errorf("previous recording script exceeds %d bytes", maxRecordingRollbackScriptBytes)
	}
	return body, nil
}

func restorePreviousRecordingCommit(
	store storage.Storage,
	id uuid.UUID,
	previous recording.RecordingResult,
	script []byte,
) error {
	if err := store.Put(recording.ScriptArtifactKey(id), bytes.NewReader(script)); err != nil {
		return fmt.Errorf("restore script: %w", err)
	}
	previous.PublicationPending = false
	if err := putRecordingResult(store, id, previous); err != nil {
		return fmt.Errorf("restore result: %w", err)
	}
	return nil
}

func uploadRenderVariantOutputs(store storage.Storage, id uuid.UUID, variant string, revisionID uuid.UUID, outDir, publishDir, resultPath string, result editor.Result) ([]string, error) {
	targets, err := renderplan.NewRenderVariantUploadTargets(renderplan.NewRenderVariantUploadTargetsOptions{
		JobID:      id,
		Variant:    variant,
		OutDir:     outDir,
		PublishDir: publishDir,
		ResultPath: resultPath,
		Result:     result,
		RevisionID: revisionID,
	})
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(targets))
	for _, target := range targets {
		if target.Required {
			if err := uploadFile(store, target.Key, target.Path); err != nil {
				return nil, fmt.Errorf("upload %s: %w", target.Label, err)
			}
			keys = append(keys, target.Key)
			continue
		}
		if uploaded, err := uploadOptionalFile(store, target.Key, target.Path); err != nil {
			return nil, fmt.Errorf("upload %s: %w", target.Label, err)
		} else if uploaded {
			keys = append(keys, target.Key)
		}
	}

	if err := renderplan.ValidateRenderVariantUploadResult(result); err != nil {
		return nil, err
	}
	return keys, nil
}

func decodeStoredRecordingResult(store storage.Storage, id uuid.UUID) (recording.RecordingResult, error) {
	rc, err := store.Open(recording.ResultArtifactKey(id))
	if err != nil {
		return recording.RecordingResult{}, fmt.Errorf("open recording result: %w", err)
	}
	defer rc.Close()

	var result recording.RecordingResult
	if err := json.NewDecoder(rc).Decode(&result); err != nil {
		return recording.RecordingResult{}, fmt.Errorf("decode recording result: %w", err)
	}
	return result, nil
}

func readStoredRecordingResult(store storage.Storage, id uuid.UUID) (recording.RecordingResult, error) {
	result, err := decodeStoredRecordingResult(store, id)
	if err != nil {
		return recording.RecordingResult{}, err
	}
	if err := recording.ValidateRunResult(result); err != nil {
		// Prefix so compose/render failures are classified as "re-record
		// required" instead of a permanent render loop on a stale capture.
		return recording.RecordingResult{}, recording.MarkNotReusable(err)
	}
	return result, nil
}

func isFullDemoNativeMix(preset string, edit renderplan.EditRequest) bool {
	return edit.MatchRecap && edit.Format == renderplan.FormatLandscape16x9 && preset == editor.PresetGameplayPOV60
}

func overlayAssetsPlatesDir(store storage.Storage) string {
	resolver, ok := store.(interface {
		ResolvePath(string) (string, error)
	})
	if !ok {
		return ""
	}
	path, err := resolver.ResolvePath("overlay-assets/plates")
	if err != nil {
		return ""
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return ""
	}
	return path
}

func (w *RenderWorker) writeFullDemoOverlay(j job.Job, workDir, preset string, edit renderplan.EditRequest) (string, error) {
	if !isFullDemoNativeMix(preset, edit) {
		return "", nil
	}
	rc, err := w.storage.Open(artifacts.RosterKey(j.ID))
	if err != nil {
		if storage.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("open roster for full-demo overlay: %w", err)
	}
	defer rc.Close()
	var roster parser.RosterResult
	if err := json.NewDecoder(rc).Decode(&roster); err != nil {
		return "", fmt.Errorf("decode roster for full-demo overlay: %w", err)
	}
	target := ""
	if j.KillPlan != nil {
		target = j.KillPlan.Target.SteamID64
	}
	if target == "" {
		target = j.TargetSteamID
	}
	var enrichment map[string]demooverlay.Enrichment
	if demooverlay.UsesFACEITEnrichment(edit.DemoSource) {
		enrichment = overlayEnrichment(w, j.ID, roster)
	}
	doc := demooverlay.BuildForSource(demooverlay.FromRosterScan(roster, target), edit.DemoSource, enrichment)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = demooverlay.MaterializeAvatars(&doc, filepath.Join(workDir, "overlay-avatars"), func(raw string) ([]byte, error) {
		return faceit.FetchAvatar(ctx, nil, raw)
	})
	path := filepath.Join(workDir, "full-demo-overlay.json")
	if err := demooverlay.Write(path, doc); err != nil {
		return "", err
	}
	return path, nil
}

func overlayEnrichment(w *RenderWorker, jobID uuid.UUID, roster parser.RosterResult) map[string]demooverlay.Enrichment {
	if w != nil && w.cfg.Faceit != nil {
		ids := make([]string, 0, len(roster.Players))
		for _, p := range roster.Players {
			ids = append(ids, p.SteamID64)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		players := w.cfg.Faceit.OverlayPlayers(ctx, ids)
		out := make(map[string]demooverlay.Enrichment, len(players))
		for steamID, player := range players {
			en := demooverlay.Enrichment{
				Nickname:   player.Nickname,
				Country:    player.Country,
				ELO:        player.ELO,
				SkillLevel: player.SkillLevel,
				Ranking:    player.Ranking,
				AvatarURL:  player.Avatar,
			}
			if last := last20FromFACEIT(player.Recent); last != nil {
				en.Last20 = last
			}
			out[steamID] = en
		}
		if len(out) > 0 {
			return out
		}
	}
	return storedOverlayEnrichment(w, jobID)
}

func storedOverlayEnrichment(w *RenderWorker, jobID uuid.UUID) map[string]demooverlay.Enrichment {
	out := map[string]demooverlay.Enrichment{}
	if w == nil || w.storage == nil {
		return out
	}
	frc, err := w.storage.Open(artifacts.FullDemoFaceitKey(jobID))
	if err != nil {
		return out
	}
	defer frc.Close()
	if err := json.NewDecoder(frc).Decode(&out); err != nil {
		return map[string]demooverlay.Enrichment{}
	}
	return out
}

func last20FromFACEIT(src faceit.Last20) *demooverlay.Last20 {
	out := demooverlay.Last20{
		Matches: src.Matches,
		WinPct:  src.WinPct,
		Kills:   src.Kills,
		Deaths:  src.Deaths,
		Assists: src.Assists,
		KD:      src.KD,
		KR:      src.KR,
		ADR:     src.ADR,
	}
	if out.Matches == nil && out.WinPct == nil && out.Kills == nil && out.KD == nil && out.KR == nil && out.ADR == nil {
		return nil
	}
	return &out
}

// compileSegmentsArgs returns the zv-editor flags that compile a render's
// segments into one upload-ready Short. Per CLAUDE.md, a multi-segment
// selection renders as a single concatenated Short (matching the "zv short"
// CLI's --compile-segments behavior); a single segment keeps today's
// per-segment short unchanged.
func compileSegmentsArgs(segmentIDs []string) []string {
	if len(segmentIDs) < 2 {
		return nil
	}
	return []string{"--compile-segments", "--segments", strings.Join(segmentIDs, ",")}
}

// prepareCaptureAttemptRollback snapshots both UI-only documents before a new
// attempt publishes either one. Rollback restores each exact prior body, or
// deletes the attempt's document when that key did not previously exist.
func prepareCaptureAttemptRollback(store storage.Storage, id uuid.UUID) (func() error, error) {
	snapshot := func(key, label string) (func() error, error) {
		rc, err := store.Open(key)
		if err != nil {
			if !storage.IsNotExist(err) {
				return nil, err
			}
			deleter, ok := store.(interface{ Delete(string) error })
			if !ok {
				return nil, fmt.Errorf("storage cannot delete an uncommitted %s", label)
			}
			return func() error {
				return deleter.Delete(key)
			}, nil
		}
		body, readErr := io.ReadAll(rc)
		if err := errors.Join(readErr, rc.Close()); err != nil {
			return nil, err
		}
		return func() error {
			return store.Put(key, bytes.NewReader(body))
		}, nil
	}

	selectionRollback, err := snapshot(artifacts.CaptureSelectionKey(id), "capture selection")
	if err != nil {
		return nil, fmt.Errorf("snapshot capture selection: %w", err)
	}
	progressRollback, err := snapshot(artifacts.CaptureProgressKey(id), "capture progress")
	if err != nil {
		return nil, fmt.Errorf("snapshot capture progress: %w", err)
	}
	return func() error {
		return errors.Join(selectionRollback(), progressRollback())
	}, nil
}

// putCaptureSelection persists the ordered segment ids a record run will
// capture, so the job poll can scope capture progress to this reel.
func putCaptureSelection(store storage.Storage, id uuid.UUID, segmentIDs []string) error {
	b, err := json.Marshal(segmentIDs)
	if err != nil {
		return err
	}
	return store.Put(artifacts.CaptureSelectionKey(id), bytes.NewReader(b))
}

func recordSourcePlan(store storage.Storage, j job.Job, useRecapPlan bool) (*killplan.Plan, error) {
	if !useRecapPlan {
		if j.KillPlan == nil {
			return nil, fmt.Errorf("job %s has no kill plan", j.ID)
		}
		return j.KillPlan, nil
	}
	recap, ok, err := recapplan.Load(store, j.ID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("job %s has no recap plan", j.ID)
	}
	if len(recap.Segments) == 0 {
		return nil, fmt.Errorf("job %s recap plan has no rounds", j.ID)
	}
	return &recap, nil
}

// killPlanSegmentIDs lists every segment id in the plan, in plan order.
func segmentTickWeights(plan *killplan.Plan) []int {
	weights := make([]int, len(plan.Segments))
	for i, s := range plan.Segments {
		w := s.TickEnd - s.TickStart
		if w < 0 {
			w = 0
		}
		weights[i] = w
	}
	return weights
}

func killPlanSegmentIDs(plan *killplan.Plan) []string {
	ids := make([]string, 0, len(plan.Segments))
	for _, s := range plan.Segments {
		ids = append(ids, s.ID)
	}
	return ids
}

// filterKillPlanSegments returns a copy of plan containing only the segments
// whose ID is in ids, preserving the plan's segment order (never the request
// order). It errors when the selection matches no segment so a stale request
// never launches the recorder with an empty plan.
func filterKillPlanSegments(plan *killplan.Plan, ids []string) (*killplan.Plan, error) {
	keep := make(map[string]bool, len(ids))
	for _, id := range ids {
		keep[id] = true
	}
	out := *plan
	out.Segments = nil
	for _, s := range plan.Segments {
		if keep[s.ID] {
			out.Segments = append(out.Segments, s)
		}
	}
	if len(out.Segments) == 0 {
		return nil, fmt.Errorf("no kill-plan segments match requested ids %v", ids)
	}
	return &out, nil
}

func isSegmentClip(a recording.RecordingArtifact) bool {
	return a.Role == "segment" && a.Type == "video" && a.SegmentID != ""
}

// tryDecodeStoredRecordingResult returns the stored recording result, reporting
// whether one exists. A missing result is not an error (the first reel of a job).
func tryDecodeStoredRecordingResult(store storage.Storage, id uuid.UUID) (recording.RecordingResult, bool, error) {
	exists, err := store.Exists(recording.ResultArtifactKey(id))
	if err != nil || !exists {
		return recording.RecordingResult{}, false, err
	}
	result, err := decodeStoredRecordingResult(store, id)
	if err != nil {
		return recording.RecordingResult{}, false, err
	}
	return result, true, nil
}

// normalizedRecordingStream resolves the exact stream profile the recorder CLI
// will use for this task. NewPlanFromKillPlan owns default normalization (FPS,
// dimensions, CRF, deathnotice safe zone and lifetime), so worker idempotency
// changes automatically when any output-affecting recorder default changes.
func normalizedRecordingStream(plan *killplan.Plan, hudMode string, portraitSafeKillfeed bool, encoder string) (recording.StreamConfig, error) {
	stream := recording.DefaultStreamConfig()
	stream.HUDMode = recording.HUDMode(hudMode)
	stream.PortraitSafeKillfeed = portraitSafeKillfeed
	stream.Encoder = encoder
	normalized, err := recording.NewPlanFromKillPlan(*plan, "profile.dem", "profile", stream)
	if err != nil {
		return recording.StreamConfig{}, err
	}
	return normalized.Stream, nil
}

func recordingProfilesCompatible(a, b recording.RecordingResult) bool {
	if a.CaptureMode != recording.CaptureModeReal || b.CaptureMode != recording.CaptureModeReal ||
		!a.CaptureVerified || !b.CaptureVerified ||
		!captureProfilesCompatible(a.Plan, b.Plan) {
		return false
	}
	aFingerprint, err := recording.CaptureInputFingerprint(a.Plan)
	if err != nil || a.CaptureInputFingerprint != aFingerprint {
		return false
	}
	bFingerprint, err := recording.CaptureInputFingerprint(b.Plan)
	return err == nil && b.CaptureInputFingerprint == bFingerprint
}

func captureProfilesCompatible(a, b recording.RecordingPlan) bool {
	return a.CaptureContract == recording.CaptureContractVersion &&
		b.CaptureContract == recording.CaptureContractVersion &&
		a.KillPlanSchemaVersion == b.KillPlanSchemaVersion &&
		a.DemoSHA256 == b.DemoSHA256 &&
		a.DemoMap == b.DemoMap &&
		a.DemoDurationTicks == b.DemoDurationTicks &&
		a.TargetSteamID64 == b.TargetSteamID64 &&
		a.TargetNameInDemo == b.TargetNameInDemo &&
		a.TargetAccountID == b.TargetAccountID &&
		a.Tickrate == b.Tickrate &&
		a.Stream == b.Stream &&
		a.Runtime.Normalized() == b.Runtime.Normalized()
}

func legacyCaptureProfileCompatible(legacy, current recording.RecordingPlan) bool {
	return legacy.CaptureContract == recording.LegacyCaptureContractVersion &&
		current.CaptureContract == recording.CaptureContractVersion &&
		legacy.DemoMap == current.DemoMap &&
		legacy.TargetSteamID64 == current.TargetSteamID64 &&
		legacy.TargetNameInDemo == current.TargetNameInDemo &&
		legacy.TargetAccountID == current.TargetAccountID &&
		legacy.Tickrate == current.Tickrate &&
		legacy.Stream == current.Stream &&
		legacy.Runtime.Normalized() == current.Runtime.Normalized()
}

// mergeRecordingResults unions a freshly recorded result over a previously
// stored one so the job-level recording result accumulates every segment any
// reel has recorded. The new run wins for segments it covers; segments only in
// prev are carried forward (their clips still live in storage under their
// per-segment keys). Capture remains chronological; EditorialSegmentIDs keeps
// the user-selected order from fullPlan regardless of which reel recorded when.
func mergeRecordingResults(prev, next recording.RecordingResult, fullPlan *killplan.Plan) (recording.RecordingResult, error) {
	merged := next
	merged.Plan.Segments = append([]recording.RecordingSegment(nil), next.Plan.Segments...)
	merged.Artifacts = append([]recording.RecordingArtifact(nil), next.Artifacts...)
	merged.Performance = mergeRecordingPerformance(prev.Performance, next.Performance)
	haveSegment := make(map[string]bool, len(next.Plan.Segments))
	for _, s := range next.Plan.Segments {
		haveSegment[s.ID] = true
	}
	for _, s := range prev.Plan.Segments {
		if !haveSegment[s.ID] {
			merged.Plan.Segments = append(merged.Plan.Segments, s)
			haveSegment[s.ID] = true
		}
	}
	merged.Plan.EditorialSegmentIDs = make([]string, 0, len(merged.Plan.Segments))
	for _, segment := range fullPlan.Segments {
		if haveSegment[segment.ID] {
			merged.Plan.EditorialSegmentIDs = append(merged.Plan.EditorialSegmentIDs, segment.ID)
		}
	}
	sort.SliceStable(merged.Plan.Segments, func(a, b int) bool {
		if merged.Plan.Segments[a].TickStart != merged.Plan.Segments[b].TickStart {
			return merged.Plan.Segments[a].TickStart < merged.Plan.Segments[b].TickStart
		}
		return merged.Plan.Segments[a].ID < merged.Plan.Segments[b].ID
	})
	haveClip := make(map[string]bool, len(next.Artifacts))
	for _, a := range next.Artifacts {
		if isSegmentClip(a) {
			haveClip[a.SegmentID] = true
		}
	}
	for _, a := range prev.Artifacts {
		if isSegmentClip(a) && !haveClip[a.SegmentID] {
			merged.Artifacts = append(merged.Artifacts, a)
		}
	}
	if err := merged.Plan.Validate(); err != nil {
		return recording.RecordingResult{}, fmt.Errorf("merge recording plans: %w", err)
	}
	fingerprint, err := recording.CaptureInputFingerprint(merged.Plan)
	if err != nil {
		return recording.RecordingResult{}, fmt.Errorf("fingerprint merged recording plan: %w", err)
	}
	merged.CaptureInputFingerprint = fingerprint
	return merged, nil
}

func mergeRecordingPerformance(prev, next *recording.RecordingPerformance) *recording.RecordingPerformance {
	if prev == nil && next == nil {
		return nil
	}
	if prev == nil {
		return copyRecordingPerformance(next)
	}
	if next == nil {
		return copyRecordingPerformance(prev)
	}
	if prev.Version != next.Version {
		return copyRecordingPerformance(next)
	}
	merged := copyRecordingPerformance(prev)
	merged.Runs = append(merged.Runs, next.Runs...)
	return merged
}

func copyRecordingPerformance(source *recording.RecordingPerformance) *recording.RecordingPerformance {
	if source == nil {
		return nil
	}
	out := &recording.RecordingPerformance{Version: source.Version}
	out.Runs = append([]recording.RecordingRunPerformance(nil), source.Runs...)
	return out
}

// putRecordingResult overwrites the durable recording result with result.
func putRecordingResult(store storage.Storage, id uuid.UUID, result recording.RecordingResult) error {
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return store.Put(recording.ResultArtifactKey(id), bytes.NewReader(b))
}

// recordingOutputsReady reports which of requested's segments still lack a
// durable, profile-compatible clip and therefore need (re)capture. Callers
// resolve the effective segment ids (whole-demo mode passes every plan
// segment), so a reel scoped to one clip is never wrongly considered ready
// against the job-level result.json, which holds only the last run's
// segments until the accumulate step unions it.
//
// An empty return means every requested segment can be skipped entirely; a
// non-empty return is the subset the caller must actually record. Recapture
// is all-or-nothing only when the stored result itself is unusable as a base
// (missing, failed, pending publication, invalid, or built under an
// incompatible capture profile) - in that case every requested segment is
// returned as missing, since there is nothing per-segment left to trust.
// captureProfilesCompatible and the per-segment reflect.DeepEqual comparison
// stay authoritative: a clip is only reused when it was captured under the
// exact same profile and segment definition, so HUD modes are never mixed
// within one reel.
func recordingOutputsReady(store storage.Storage, id uuid.UUID, requested []string, expectedPlan recording.RecordingPlan) ([]string, []string, error) {
	if len(requested) == 0 {
		return nil, nil, nil
	}
	needAll := append([]string(nil), requested...)
	resultKey := recording.ResultArtifactKey(id)
	exists, err := store.Exists(resultKey)
	if err != nil || !exists {
		return needAll, nil, err
	}
	result, err := decodeStoredRecordingResult(store, id)
	if err != nil || result.Error != "" {
		return needAll, nil, err
	}
	if result.PublicationPending {
		return needAll, nil, nil
	}
	if err := recording.ValidateRunResult(result); err != nil {
		return needAll, nil, nil
	}
	if !captureProfilesCompatible(result.Plan, expectedPlan) &&
		!legacyCaptureProfileCompatible(result.Plan, expectedPlan) {
		return needAll, nil, nil
	}

	recorded := make(map[string]bool)
	recordedSegments := make(map[string]recording.RecordingSegment, len(result.Plan.Segments))
	for _, segment := range result.Plan.Segments {
		recordedSegments[segment.ID] = segment
	}
	for _, a := range result.Artifacts {
		if isSegmentClip(a) {
			recorded[a.SegmentID] = true
		}
	}
	scriptKey := recording.ScriptArtifactKey(id)
	if ok, err := store.Exists(scriptKey); err != nil || !ok {
		return needAll, nil, err
	}
	keys := []string{resultKey, scriptKey}
	expectedSegments := make(map[string]recording.RecordingSegment, len(expectedPlan.Segments))
	for _, segment := range expectedPlan.Segments {
		expectedSegments[segment.ID] = segment
	}

	var missing []string
	for _, segID := range requested {
		storedSegment, stored := recordedSegments[segID]
		expectedSegment, expected := expectedSegments[segID]
		if !recorded[segID] || !stored || !expected || !reflect.DeepEqual(storedSegment, expectedSegment) {
			missing = append(missing, segID)
			continue
		}
		clipKey, err := recording.SegmentClipArtifactKey(id, segID)
		if err != nil {
			return needAll, nil, err
		}
		ok, err := store.Exists(clipKey)
		if err != nil {
			return needAll, nil, err
		}
		if !ok {
			missing = append(missing, segID)
			continue
		}
		keys = append(keys, clipKey)
	}
	return missing, keys, nil
}

func compositionOutputsReady(store storage.Storage, id uuid.UUID) (bool, bool, []string, error) {
	resultKey := composition.ResultArtifactKey(id)
	resultExists, err := store.Exists(resultKey)
	if err != nil || !resultExists {
		return false, false, nil, err
	}

	rc, err := store.Open(resultKey)
	if err != nil {
		return false, false, nil, fmt.Errorf("open composition result: %w", err)
	}
	defer rc.Close()
	var result composition.Result
	if err := json.NewDecoder(rc).Decode(&result); err != nil {
		return false, false, nil, fmt.Errorf("decode composition result: %w", err)
	}
	if result.Error != "" {
		return false, false, nil, nil
	}
	readyArtifacts := composition.NewReadyArtifacts(id, result)
	keys := []string{readyArtifacts.ResultKey}
	for _, key := range readyArtifacts.RequiredKeys {
		exists, err := store.Exists(key)
		if err != nil || !exists {
			return false, false, nil, err
		}
		keys = append(keys, key)
	}
	return true, len(result.Warnings) > 0, keys, nil
}

type renderMusicInput struct {
	Key        string   `json:"key,omitempty"`
	Available  bool     `json:"available"`
	Volume     float64  `json:"volume,omitempty"`
	GameVolume *float64 `json:"game_volume,omitempty"`
	SHA256     string   `json:"sha256,omitempty"`
}

type renderFingerprintInput struct {
	SchemaVersion string                    `json:"schema_version"`
	Variant       string                    `json:"variant"`
	Edit          renderplan.EditRequest    `json:"edit"`
	Music         renderMusicInput          `json:"music"`
	Recording     recording.RecordingResult `json:"recording"`
	KillPlan      killplan.Plan             `json:"kill_plan"`
}

func renderInputFingerprint(result recording.RecordingResult, plan *killplan.Plan, variant, musicKey, musicPath string, effectiveMusicVolume float64, gameVolume *float64, edit renderplan.EditRequest) (string, error) {
	music := renderMusicInput{Key: musicKey, Available: musicPath != "", Volume: effectiveMusicVolume, GameVolume: gameVolume}
	if musicPath != "" {
		// #nosec G304 -- musicPath is the worker-local materialization of a validated durable music artifact.
		f, err := os.Open(musicPath)
		if err != nil {
			return "", fmt.Errorf("open music: %w", err)
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", fmt.Errorf("hash music: %w", copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close music: %w", closeErr)
		}
		music.SHA256 = fmt.Sprintf("%x", h.Sum(nil))
	}
	doc := renderFingerprintInput{
		SchemaVersion: "1.0",
		Variant:       variant,
		Edit:          edit,
		Music:         music,
		Recording:     result,
		KillPlan:      *plan,
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum), nil
}

func renderVariantOutputsReady(store storage.Storage, id uuid.UUID, variant, expectedFingerprint string, states ...*renderplan.RenderVariantState) (bool, []string, []string, error) {
	var state *renderplan.RenderVariantState
	if len(states) > 0 && states[0] != nil {
		state = states[0]
	} else {
		loadout, err := renderplan.LoadoutForVariant(variant)
		if err != nil {
			return false, nil, nil, err
		}
		legacy, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
			JobID:   id,
			Loadout: loadout,
			Status:  renderplan.RenderVariantStatusReady,
		})
		if err != nil {
			return false, nil, nil, err
		}
		state = &legacy
	}
	if state == nil || (state.Status != renderplan.RenderVariantStatusReady && state.Status != renderplan.RenderVariantStatusReview) {
		return false, nil, nil, nil
	}
	resultKey := state.RenderResultKey
	if resultKey == "" {
		return false, nil, nil, nil
	}
	exists, err := store.Exists(resultKey)
	if err != nil || !exists {
		return false, nil, nil, err
	}
	rc, err := store.Open(resultKey)
	if err != nil {
		return false, nil, nil, fmt.Errorf("open render result: %w", err)
	}
	defer rc.Close()
	var result editor.Result
	if err := json.NewDecoder(rc).Decode(&result); err != nil {
		return false, nil, nil, fmt.Errorf("decode render result: %w", err)
	}
	if result.Error != "" {
		return false, nil, nil, nil
	}
	if result.InputFingerprint == "" || result.InputFingerprint != expectedFingerprint {
		return false, nil, nil, nil
	}
	keys := []string{resultKey}
	for _, key := range []string{
		state.EditDocumentKey,
		state.EditManifestKey,
		state.PackManifestKey,
		state.GalleryKey,
		state.PublishSummaryKey,
	} {
		if key == "" {
			return false, nil, nil, nil
		}
		exists, err := store.Exists(key)
		if err != nil || !exists {
			return false, nil, nil, err
		}
		keys = append(keys, key)
	}
	if err := renderplan.ValidateRenderVariantRunResult(result); err != nil {
		return false, nil, nil, nil
	}
	for _, short := range result.Shorts {
		if short.SegmentID == "" {
			return false, nil, nil, nil
		}
		for _, kind := range []renderplan.RenderVariantArtifactKind{
			renderplan.RenderVariantArtifactVideo,
			renderplan.RenderVariantArtifactCaption,
		} {
			ref, refErr := renderplan.NewRenderVariantArtifactRefForState(*state, kind, short.SegmentID)
			if refErr != nil {
				return false, nil, nil, refErr
			}
			exists, existsErr := store.Exists(ref.Key)
			if existsErr != nil || !exists {
				return false, nil, nil, existsErr
			}
			keys = append(keys, ref.Key)
		}
		if result.CoversEnabled {
			ref, refErr := renderplan.NewRenderVariantArtifactRefForState(*state, renderplan.RenderVariantArtifactCover, short.SegmentID)
			if refErr != nil {
				return false, nil, nil, refErr
			}
			exists, existsErr := store.Exists(ref.Key)
			if existsErr != nil || !exists {
				return false, nil, nil, existsErr
			}
			keys = append(keys, ref.Key)
		}
	}
	// A per-segment render is only reusable if it already produced a short for
	// every segment in the (possibly grown) recording result. When a later reel
	// records an additional segment, this forces a re-render so the new clip gets
	// its short instead of being skipped as "already rendered".
	covers, err := renderCoversRecordedSegments(store, id, result)
	if err != nil {
		return false, nil, nil, err
	}
	if !covers {
		return false, nil, nil, nil
	}
	return true, renderplan.CompleteRenderWarnings(result), keys, nil
}

// compilationSegmentID is the synthetic segment id the editor uses for a single
// all-kills compilation short (see internal/editor/manifest.go).
const compilationSegmentID = "demo-compilation"

// renderCoversRecordedSegments reports whether render already has a short for
// every recorded segment that actually has a clip. Coverage is measured against
// clip-bearing artifacts, not all plan segments, because the editor only emits a
// short for segments with a recorded clip - a plan segment with no clip (a
// partial capture) must not make coverage permanently unsatisfiable. A
// compilation render (one "demo-compilation" short) is always treated as
// covered. An unreadable recording result falls back to covered, leaving the
// key-based readiness check authoritative.
func renderCoversRecordedSegments(store storage.Storage, id uuid.UUID, render editor.Result) (bool, error) {
	exists, err := store.Exists(recording.ResultArtifactKey(id))
	if err != nil || !exists {
		return true, err
	}
	rec, err := decodeStoredRecordingResult(store, id)
	if err != nil {
		return true, nil
	}
	rendered := make(map[string]bool, len(render.Shorts))
	for _, s := range render.Shorts {
		if s.SegmentID == compilationSegmentID {
			return true, nil
		}
		rendered[s.SegmentID] = true
	}
	for _, a := range rec.Artifacts {
		if isSegmentClip(a) && !rendered[a.SegmentID] {
			return false, nil
		}
	}
	return true, nil
}

func localizeSegmentClips(store storage.Storage, id uuid.UUID, workDir string, result *recording.RecordingResult) error {
	localizations, err := recording.NewSegmentClipLocalizations(id, workDir, *result)
	if err != nil {
		return err
	}
	if len(localizations) == 0 {
		return fmt.Errorf("recording result has no segment clips")
	}
	// Each localization copies a distinct clip and updates a distinct artifact
	// index, so the copies run concurrently (bounded) without racing on the slice.
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	sem := make(chan struct{}, localizeConcurrency)
	for _, localization := range localizations {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if copyErr := materializeStorageFile(store, localization.Key, localization.LocalPath); copyErr != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("localize segment %s: %w", localization.SegmentID, copyErr)
				}
				mu.Unlock()
				return
			}
			result.Artifacts[localization.ArtifactIndex].Path = localization.LocalPath
		}()
	}
	wg.Wait()
	return firstErr
}

func readJSONFile(path string, dst any) error {
	// #nosec G304 -- worker JSON paths are generated inside the stage work directory.
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func putJSONToStorage(store storage.Storage, key string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return store.Put(key, bytes.NewReader(append(b, '\n')))
}
