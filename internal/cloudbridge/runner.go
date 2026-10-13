package cloudbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

// maxLocalPollErrors is how many status polls in a row may fail before the
// attempt is given up; one failed poll of the loopback API means nothing.
const maxLocalPollErrors = 15

// runner takes one job at a time from claim to finished render. It is a
// single goroutine on purpose: the machine records one capture at a time,
// and nothing else may compete with CS2 while it does.
type runner struct {
	*core
	admitter     Admitter
	files        ArtifactStore
	machine      Machine
	uploader     *uploader
	kinds        []string
	minFreeBytes uint64
	lastClaimErr string
	lastJanitor  time.Time
	backlogged   bool
}

func (r *runner) run(ctx context.Context) {
	r.recover(ctx)
	for ctx.Err() == nil {
		if !sleep(ctx, r.step(ctx)) {
			return
		}
	}
}

// step does one round of the loop and returns how long to wait before the
// next: preflight, claim, and when there is a job, the whole job.
func (r *runner) step(ctx context.Context) time.Duration {
	if stop := r.tracker.claimable(); stop.unauthorized || stop.rejected {
		return r.timing.Claim
	}
	// Work of an earlier job that has not stopped yet still owns the machine.
	if block := r.stillStopping(ctx); block != nil {
		r.tracker.setBlocked(block)
		return r.timing.Claim
	}
	if r.now().Sub(r.lastJanitor) >= r.timing.Janitor {
		r.janitor(ctx)
	}
	health, block := preflight(ctx, r.machine, r.minFreeBytes)
	r.tracker.setBlocked(block)
	if block != nil {
		return r.timing.Claim
	}
	if r.tracker.claimable().paused || r.uploadsBacklogged() {
		return r.timing.Claim
	}
	job, _, err := r.client.claim(ctx, claimRequest{Kinds: r.kinds, DiskFreeBytes: health.DiskFreeBytes})
	switch {
	case errors.Is(err, errUnauthorized):
		r.tracker.revoke()
		return r.timing.Claim
	case err != nil:
		if message := err.Error(); message != r.lastClaimErr && ctx.Err() == nil {
			r.lastClaimErr = message
			log.Printf("cloudbridge: claim: %v", err)
		}
		return r.timing.Claim
	}
	r.lastClaimErr = ""
	if job == nil {
		return r.timing.Claim
	}
	log.Printf("cloudbridge: claimed job %s (%s, attempt %d)", job.ID, job.Kind, job.Attempt)
	r.runJob(ctx, job)
	return 0
}

// uploadsBacklogged reports whether finished jobs are piling up behind a slow
// upload. Capturing more would only add to what the heartbeat must keep alive.
func (r *runner) uploadsBacklogged() bool {
	backlogged := r.uploader.backlog() >= maxUploadBacklog
	if backlogged != r.backlogged {
		r.backlogged = backlogged
		if backlogged {
			log.Printf("cloudbridge: %d jobs are waiting to upload; not claiming until the backlog shrinks", maxUploadBacklog)
		}
	}
	return backlogged
}

// attempt is the working memory of one claimed job.
type attempt struct {
	job       *workerJob
	ref       jobRef
	claimedAt time.Time
	deadline  time.Time
	spec      jobSpec
	rules     rules.Rules
	variant   string
}

// runJob owns one attempt from claim to its single outcome: handed to the
// uploader, failed with one report, or taken away by the portal.
func (r *runner) runJob(ctx context.Context, job *workerJob) {
	ref := jobRef{ID: job.ID, Attempt: job.Attempt}
	r.settlePrevious(ctx, ref)
	if ctx.Err() != nil {
		// Shutting down before anything was recorded: the lease expires.
		return
	}
	now := r.now()
	run := &attempt{job: job, ref: ref, claimedAt: now, deadline: now.Add(r.maxRuntime(job))}
	tracked := TrackedRequest{CloudRequestID: job.ID, Phase: phaseRunning, Attempt: job.Attempt, ClaimedAt: now}
	if err := r.state.Insert(tracked); err != nil {
		// Without a durable record a restart could not report this attempt.
		r.reportFailure(ctx, ref, failureOf(codeInternal, "persist worker state: "+err.Error()).request(0))
		return
	}
	jobCtx := r.tracker.begin(ctx, ref, jobProgress{Phase: phaseRunning, Stage: stageDownloading})
	failure := r.execute(jobCtx, run)
	r.settle(ctx, jobCtx, run, failure)
}

// settlePrevious ends what this worker still holds of an older attempt of a
// job the portal just handed back: the portal requeued it, so that attempt
// can no longer deliver. Its upload stops, its local job and files go, and
// nothing is reported, exactly as for a lost lease.
func (r *runner) settlePrevious(ctx context.Context, job jobRef) {
	r.uploader.abandon(ctx, job.ID)
	previous, held := r.state.Get(job.ID)
	// An entry without a phase is the manual bridge's; its local job stays.
	if !held || previous.Phase == "" || ctx.Err() != nil {
		return
	}
	log.Printf("cloudbridge: job %s came back as attempt %d; discarding what attempt %d left here", job.ID, job.Attempt, previous.Attempt)
	r.discardLocal(ctx, previous)
	r.forget(job.ID)
}

func (r *runner) maxRuntime(job *workerJob) time.Duration {
	if job.MaxRuntimeSeconds > 0 {
		return time.Duration(job.MaxRuntimeSeconds) * time.Second
	}
	return r.timing.DefaultMaxRuntime
}

// settle turns the result of execute into the attempt's one outcome.
func (r *runner) settle(ctx, jobCtx context.Context, run *attempt, failure *jobFailure) {
	id, ref := run.job.ID, run.ref
	cause := context.Cause(jobCtx)
	tracked, _ := r.state.Get(id)
	switch {
	case ctx.Err() != nil:
		// Shutting down: the attempt stays tracked and the next start
		// reports it as interrupted.
		r.tracker.end(ref)
		return
	case errors.Is(cause, errLeaseLost), errors.Is(cause, errUnauthorized):
		log.Printf("cloudbridge: job %s was taken back by the portal; discarding local work", id)
		r.tracker.end(ref)
		r.discardLocal(ctx, tracked)
		r.forget(id)
		return
	case errors.Is(cause, errCancelRequested):
		failure = failureOf(codeCanceled, "canceled on the portal")
	case failure == nil:
		r.tracker.end(ref)
		r.uploader.enqueue(ctx, ref)
		return
	}
	// The local job stops first, so cs2.exe is gone before the portal can
	// hand this worker anything else. The job stays in the heartbeat until
	// the report lands, which keeps its lease alive.
	r.discardLocal(ctx, tracked)
	reported := r.reportFailure(ctx, ref, failure.request(machineSeconds(run.claimedAt, r.now())))
	r.tracker.end(ref)
	if reported {
		r.forget(id)
	}
}

// execute runs the attempt. It returns nil once the videos are rendered,
// hashed, and the portal has moved the job to uploading. A non-nil failure
// is the attempt's error. When the job's context ended instead, the return
// value is meaningless and settle reads the cause.
func (r *runner) execute(ctx context.Context, run *attempt) *jobFailure {
	spec, jobRules, err := parseJobSpec(run.job.Kind, run.job.Spec)
	if err != nil {
		return failureOf(codeInvalidSpec, err.Error())
	}
	run.spec, run.rules = spec, jobRules

	demoPath, failure := r.download(ctx, run)
	if failure != nil || ctx.Err() != nil {
		return failure
	}

	r.setStage(run.ref, stageParsing)
	localID, failure := r.admit(ctx, run, demoPath)
	if failure != nil || ctx.Err() != nil {
		return failure
	}
	if failure := r.waitParsed(ctx, run, localID); failure != nil || ctx.Err() != nil {
		return failure
	}
	plan, err := r.local.KillPlan(ctx, localID)
	if err != nil {
		return failureOf(codeInternal, "read the worker's kill plan: "+err.Error())
	}
	if err := verifyWindows(spec, plan); err != nil {
		return failureOf(codeSpecMismatch, err.Error())
	}

	result, err := r.local.Generate(ctx, localID, spec.Generate)
	switch {
	case err != nil:
		return failureOf(codeInternal, "start generate: "+err.Error())
	case result.Status == http.StatusBadRequest:
		return failureOf(codeInvalidSpec, result.Message)
	case result.Status != http.StatusAccepted:
		return failureOf(codeInternal, fmt.Sprintf("generate answered %d %s: %s", result.Status, result.Code, result.Message))
	}
	run.variant = result.Variant

	r.setStage(run.ref, stageCapturing)
	if failure := r.monitor(ctx, run, localID); failure != nil || ctx.Err() != nil {
		return failure
	}
	return r.handOver(ctx, run, localID)
}

func (r *runner) setStage(job jobRef, stage string) {
	r.tracker.update(job, func(progress *jobProgress) {
		progress.Stage, progress.Percent, progress.Detail = stage, 0, ""
	})
}

// download fetches the demo, resuming across network errors. Corrupt bytes
// get one fresh start; after that the portal's copy is assumed broken.
func (r *runner) download(ctx context.Context, run *attempt) (string, *jobFailure) {
	id := run.job.ID
	demoPath, err := r.demoPath(id)
	if err != nil {
		return "", failureOf(codeInternal, err.Error())
	}
	if err := r.state.Update(id, func(entry *TrackedRequest) { entry.DemoPath = demoPath }); err != nil {
		return "", failureOf(codeInternal, "persist worker state: "+err.Error())
	}
	retry := backoff{lowest: r.timing.RetryMin, highest: r.timing.RetryMax}
	corrupt := 0
	var lastErr error
	for try := 1; try <= r.timing.DownloadAttempts; try++ {
		lastErr = r.client.downloadDemo(ctx, run.ref, demoPath, run.job.Demo)
		switch {
		case lastErr == nil:
			return demoPath, nil
		case ctx.Err() != nil:
			return "", nil
		case r.portalTookJob(run.ref, lastErr):
			return "", nil
		case errors.Is(lastErr, errDemoCorrupt):
			if corrupt++; corrupt > 1 {
				return "", failureOf(codeDemoDownloadFailed, lastErr.Error())
			}
			continue
		case !retryablePortalError(lastErr):
			return "", failureOf(codeDemoDownloadFailed, lastErr.Error())
		}
		if !sleep(ctx, retry.next()) {
			return "", nil
		}
	}
	return "", failureOf(codeDemoDownloadFailed, lastErr.Error())
}

func (r *runner) admit(ctx context.Context, run *attempt, demoPath string) (string, *jobFailure) {
	id := run.job.ID
	demo, err := os.Open(demoPath) //nolint:gosec // demoPath is built from the data dir and a validated job id
	if err != nil {
		return "", failureOf(codeInternal, "open downloaded demo: "+err.Error())
	}
	defer demo.Close()
	localID, err := r.admitter.AdmitCloudDemo(ctx, DemoAdmission{
		Demo:           demo,
		FileName:       cloudFileName(id, run.job.SubmitterLabel, run.job.Demo.FileName),
		CloudRequestID: id,
		TargetSteamID:  run.spec.TargetSteamID,
		Rules:          run.rules,
	})
	switch {
	case errors.Is(err, ErrDemoRejected):
		return "", failureOf(codeDemoIncompatible, err.Error())
	case err != nil:
		return "", failureOf(codeInternal, "admit demo: "+err.Error())
	}
	// From here a restart must be able to find and delete the local job.
	if err := r.state.Update(id, func(entry *TrackedRequest) { entry.LocalJobID = localID }); err != nil {
		r.discardLocal(ctx, TrackedRequest{CloudRequestID: id, LocalJobID: localID})
		return "", failureOf(codeInternal, "persist worker state: "+err.Error())
	}
	r.tracker.update(run.ref, func(progress *jobProgress) { progress.LocalJobID = localID })
	return localID, nil
}

// localPoller reads the local job's status and tolerates a few failed polls.
type localPoller struct {
	runner *runner
	jobID  string
	errors int
}

// poll returns ok=false when this round has nothing to act on yet.
func (p *localPoller) poll(ctx context.Context) (job LocalJob, ok bool, failure *jobFailure) {
	job, found, err := p.runner.local.JobView(ctx, p.jobID)
	switch {
	case err != nil:
		if p.errors++; p.errors > maxLocalPollErrors {
			return LocalJob{}, false, failureOf(codeInternal, "read local job status: "+err.Error())
		}
		return LocalJob{}, false, nil
	case !found:
		return LocalJob{}, false, failureOf(codeInternal, "the local job disappeared while the worker was running it")
	}
	p.errors = 0
	if job.Status == localJobFailed {
		return job, false, localJobFailure(job)
	}
	return job, true, nil
}

func (r *runner) timedOut(run *attempt) *jobFailure {
	if r.now().Before(run.deadline) {
		return nil
	}
	return failureOf(codeTimeout, fmt.Sprintf("the attempt exceeded its limit of %s", run.deadline.Sub(run.claimedAt)))
}

func (r *runner) waitParsed(ctx context.Context, run *attempt, localID string) *jobFailure {
	poller := &localPoller{runner: r, jobID: localID}
	parseDeadline := r.now().Add(r.timing.ParseTimeout)
	for {
		job, ok, failure := poller.poll(ctx)
		switch {
		case failure != nil:
			return failure
		case ok && job.Status == localJobParsed:
			return nil
		case ok && job.Status != "queued" && job.Status != "parsing":
			return failureOf(codeInternal, "the local job went to "+job.Status+" instead of parsed")
		}
		if failure := r.timedOut(run); failure != nil {
			return failure
		}
		if !r.now().Before(parseDeadline) {
			return failureOf(codeTimeout, fmt.Sprintf("the demo was not parsed within %s", r.timing.ParseTimeout))
		}
		if !sleep(ctx, r.timing.Poll) {
			return nil
		}
	}
}

// monitor follows the local job through capture and render until the
// variant is ready, publishing stage and percent for the heartbeat.
func (r *runner) monitor(ctx context.Context, run *attempt, localID string) *jobFailure {
	id := run.job.ID
	poller := &localPoller{runner: r, jobID: localID}
	var capturedAt time.Time
	lastSave := r.now()
	for {
		job, ok, failure := poller.poll(ctx)
		if failure != nil {
			return failure
		}
		if ok {
			done, failure := r.observe(ctx, run, localID, job, &capturedAt)
			if failure != nil || done {
				return failure
			}
		}
		if failure := r.timedOut(run); failure != nil {
			return failure
		}
		if r.now().Sub(lastSave) >= r.timing.ProgressSave {
			lastSave = r.now()
			seconds := machineSeconds(run.claimedAt, lastSave)
			if err := r.state.Update(id, func(entry *TrackedRequest) { entry.MachineSeconds = seconds }); err != nil {
				log.Printf("cloudbridge: save progress of %s: %v", id, err)
			}
		}
		if !sleep(ctx, r.timing.Poll) {
			return nil
		}
	}
}

// observe handles one status reading and reports done once the render is
// ready.
func (r *runner) observe(ctx context.Context, run *attempt, localID string, job LocalJob, capturedAt *time.Time) (bool, *jobFailure) {
	ref := run.ref
	percent := 0
	if job.Progress != nil {
		percent = job.Progress.Percent
	}
	// Generate leaves the job parsed until the capture lane picks it up.
	if job.Status == localJobParsed || job.Status == localJobRecording {
		detail := ""
		if job.Progress != nil && job.Progress.Total > 0 {
			detail = fmt.Sprintf("REC %d/%d", job.Progress.Done, job.Progress.Total)
		}
		r.tracker.update(ref, func(progress *jobProgress) {
			progress.Stage, progress.Percent, progress.Detail = stageCapturing, percent, detail
		})
		return false, nil
	}
	if capturedAt.IsZero() {
		*capturedAt = r.now()
	}
	variant, found, err := r.local.VariantView(ctx, localID, run.variant)
	switch {
	case err != nil:
		// The next poll retries; the attempt's time limit bounds it.
		return false, nil
	case !found:
		if r.now().Sub(*capturedAt) >= r.timing.RenderStartTimeout {
			return false, failureOf(codeInternal, "the capture finished but its render never started")
		}
	case variant.Status == localVariantReady || variant.Status == localVariantReview:
		return true, nil
	case variant.Status == localVariantFailed:
		return false, renderFailure(variant)
	}
	detail := ""
	if job.Progress != nil {
		detail = job.Progress.Stage
	}
	r.tracker.update(ref, func(progress *jobProgress) {
		progress.Stage, progress.Percent, progress.Detail = stageRendering, percent, detail
	})
	return false, nil
}

// handOver hashes the rendered videos, records them durably, and tells the
// portal the capture slot is free. Only then may the uploader have the job.
func (r *runner) handOver(ctx context.Context, run *attempt, localID string) *jobFailure {
	id := run.job.ID
	r.tracker.update(run.ref, func(progress *jobProgress) {
		progress.Stage, progress.Percent, progress.Detail = stageRendering, 100, ""
	})
	if failure := r.checkCapture(localID); failure != nil {
		return failure
	}
	videos, failure := r.collectVideos(localID, run.variant)
	if failure != nil {
		return failure
	}
	now := r.now()
	seconds := machineSeconds(run.claimedAt, now)
	if err := r.state.Update(id, func(entry *TrackedRequest) {
		entry.Phase = phaseUploading
		entry.Artifacts = videos
		entry.MachineSeconds = seconds
		entry.UploadStartedAt = now
	}); err != nil {
		return failureOf(codeInternal, "persist worker state: "+err.Error())
	}
	return r.enterUploading(ctx, run.ref)
}

// enterUploading repeats the phase report until the portal has it. Nothing
// else can be claimed meanwhile, which is right: the portal still counts
// this job as the one capture in progress.
func (r *runner) enterUploading(ctx context.Context, job jobRef) *jobFailure {
	id := job.ID
	tracked, _ := r.state.Get(id)
	retry := backoff{lowest: r.timing.RetryMin, highest: r.timing.RetryMax}
	for {
		err := r.client.enterUploading(ctx, job, tracked.MachineSeconds, tracked.LocalJobID)
		switch {
		case err == nil:
			if err := r.state.Update(id, func(entry *TrackedRequest) { entry.PhaseReported = true }); err != nil {
				log.Printf("cloudbridge: save phase of %s: %v", id, err)
			}
			return nil
		case ctx.Err() != nil:
			return nil
		case r.portalTookJob(job, err):
			return nil
		case !retryablePortalError(err):
			return failureOf(codeInternal, "report uploading phase: "+err.Error())
		}
		if !sleep(ctx, retry.next()) {
			return nil
		}
	}
}

// collectVideos lists the pack's videos with their size and sha256.
func (r *runner) collectVideos(localID, variant string) ([]TrackedArtifact, *jobFailure) {
	jobID, err := uuid.Parse(localID)
	if err != nil {
		return nil, failureOf(codeInternal, "invalid local job id "+localID)
	}
	manifest, err := r.readPackManifest(jobID, variant)
	if err != nil {
		return nil, failureOf(codeInternal, "read pack manifest: "+err.Error())
	}
	jobPrefix := artifacts.JobPrefix(jobID) + "/"
	videos := make([]TrackedArtifact, 0, len(manifest.Items))
	for _, item := range manifest.Items {
		if item.Video == "" {
			continue
		}
		// The manifest names storage keys; anything outside this job's
		// tree is not a video this job rendered.
		if !strings.HasPrefix(item.Video, jobPrefix) || path.Ext(item.Video) != ".mp4" {
			return nil, failureOf(codeInternal, "pack manifest names an unexpected video key "+item.Video)
		}
		videoPath, err := r.files.ResolvePath(item.Video)
		if err != nil {
			return nil, failureOf(codeInternal, "resolve video path: "+err.Error())
		}
		sum, size, err := hashFile(videoPath)
		if err != nil {
			return nil, failureOf(codeRenderFailed, "read rendered video: "+err.Error())
		}
		if size == 0 {
			return nil, failureOf(codeRenderFailed, "rendered video "+path.Base(item.Video)+" is empty")
		}
		videos = append(videos, TrackedArtifact{
			Variant: variant,
			Name:    path.Base(item.Video),
			Path:    videoPath,
			Size:    size,
			SHA256:  sum,
		})
	}
	if len(videos) == 0 {
		return nil, failureOf(codeRenderFailed, "the render finished without any video")
	}
	return videos, nil
}

func (r *runner) readPackManifest(jobID uuid.UUID, variant string) (*editor.PackManifest, error) {
	stateKey, err := artifacts.RenderVariantStatusKey(jobID, variant)
	if err != nil {
		return nil, fmt.Errorf("resolve state key: %w", err)
	}
	var state renderplan.RenderVariantState
	if err := r.readJSON(stateKey, &state); err != nil {
		return nil, err
	}
	// A render revision keeps its manifest under its own prefix, and the
	// state names it; older states use the fixed variant key.
	manifestKey := state.PackManifestKey
	if manifestKey == "" {
		manifestKey, err = artifacts.RenderVariantPackManifestKey(jobID, variant)
		if err != nil {
			return nil, fmt.Errorf("resolve manifest key: %w", err)
		}
	}
	var manifest editor.PackManifest
	if err := r.readJSON(manifestKey, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (r *runner) readJSON(key string, target any) error {
	rc, err := r.files.Open(key)
	if err != nil {
		return fmt.Errorf("open %s: %w", key, err)
	}
	defer rc.Close()
	if err := json.NewDecoder(rc).Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", key, err)
	}
	return nil
}

// recover settles what the previous run of this process left behind, before
// anything new is claimed.
func (r *runner) recover(ctx context.Context) {
	for _, tracked := range r.state.All() {
		if ctx.Err() != nil {
			return
		}
		id := tracked.CloudRequestID
		ref := jobRef{ID: id, Attempt: tracked.Attempt}
		switch {
		case tracked.Phase == phaseRunning:
			// The capture died with the process and cannot be resumed.
			log.Printf("cloudbridge: job %s was interrupted by a restart", id)
			r.discardLocal(ctx, tracked)
			failure := failureOf(codeInterrupted, "the worker restarted while the job was running")
			if r.reportFailure(ctx, ref, failure.request(tracked.MachineSeconds)) {
				r.forget(id)
			}
		case tracked.Phase == phaseUploading && !tracked.PhaseReported:
			r.resumeHandOver(ctx, tracked)
		case tracked.Phase == phaseUploading:
			r.uploader.enqueue(ctx, ref)
		}
	}
	r.janitor(ctx)
}

// resumeHandOver finishes a hand-over the previous run started: the videos
// are on disk and recorded, only the phase report is missing.
func (r *runner) resumeHandOver(ctx context.Context, tracked TrackedRequest) {
	ref := jobRef{ID: tracked.CloudRequestID, Attempt: tracked.Attempt}
	jobCtx := r.tracker.begin(ctx, ref, jobProgress{Phase: phaseRunning, Stage: stageRendering, Percent: 100, LocalJobID: tracked.LocalJobID})
	failure := r.enterUploading(jobCtx, ref)
	// Only the time the attempt had used before the restart is charged.
	used := time.Duration(tracked.MachineSeconds) * time.Second
	run := &attempt{job: &workerJob{ID: ref.ID, Attempt: ref.Attempt}, ref: ref, claimedAt: r.now().Add(-used)}
	r.settle(ctx, jobCtx, run, failure)
}

// janitor deletes local jobs and downloaded demos that no tracked cloud job
// owns any more, so neither a crash at the wrong moment nor a delete that
// failed can leak a capture. It runs at start and then whenever the runner
// is between jobs, and tries each leftover once: the next run tries again.
func (r *runner) janitor(ctx context.Context) {
	r.lastJanitor = r.now()
	refs, err := r.local.CloudJobs(ctx)
	if err != nil {
		log.Printf("cloudbridge: janitor could not list local jobs: %v", err)
	}
	for _, ref := range refs {
		tracked, ok := r.state.Get(ref.CloudRequestID)
		// Entries without a phase belong to the manual bridge; an operator
		// may still be working on their local jobs.
		if ok && (tracked.Phase == "" || tracked.LocalJobID == ref.ID) {
			continue
		}
		localID, err := uuid.Parse(ref.ID)
		if err != nil {
			continue
		}
		if _, err := r.stopAndDelete(ctx, localID); err != nil {
			log.Printf("cloudbridge: janitor could not remove local job %s of finished cloud job %s yet: %v", ref.ID, ref.CloudRequestID, err)
			continue
		}
		log.Printf("cloudbridge: janitor removed local job %s of finished cloud job %s", ref.ID, ref.CloudRequestID)
	}
	entries, err := os.ReadDir(r.incoming)
	if err != nil {
		return
	}
	for _, entry := range entries {
		id, isDemo := strings.CutSuffix(entry.Name(), ".dem")
		if !isDemo || entry.IsDir() {
			continue
		}
		if _, ok := r.state.Get(id); !ok {
			r.removeDemo(filepath.Join(r.incoming, entry.Name()))
		}
	}
}
