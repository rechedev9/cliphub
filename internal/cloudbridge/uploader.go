package cloudbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"sync"
	"time"
)

const (
	artifactKindVideo = "video"
	// maxChecksumRetries is how often a whole file is sent again after the
	// portal computed a different sha256 for what it received.
	maxChecksumRetries = 2
	// maxUploadBacklog is how many finished jobs may wait for their upload
	// before the worker stops claiming; the portal applies the same bound.
	maxUploadBacklog = 3
)

// errLocalFile marks an upload error caused by the file on this machine, so
// it is not mistaken for a network error worth retrying.
var errLocalFile = errors.New("cloudbridge: result file is not usable")

type queuedUpload struct {
	job jobRef
	ctx context.Context
	// reporting ends when the upload is abandoned, so a failure report that
	// the portal is not answering cannot hold the job id any longer.
	reporting     context.Context
	stopReporting context.CancelFunc
	// done closes once the uploader holds nothing of this upload any more.
	done chan struct{}
}

// uploader sends finished videos to the portal, one job after another, while
// the runner is free to capture the next job. Uploading is bound by the
// network, not by the GPU, so it is the one thing allowed to overlap.
type uploader struct {
	*core
	mu      sync.Mutex
	queue   []*queuedUpload
	current *queuedUpload
	wake    chan struct{}
}

func newUploader(shared *core) *uploader {
	return &uploader{core: shared, wake: make(chan struct{}, 1)}
}

// enqueue takes over a job in phase uploading. It is published to the
// heartbeat at once, also while it waits behind another upload, so its
// lease stays alive.
func (u *uploader) enqueue(ctx context.Context, job jobRef) {
	tracked, _ := u.state.Get(job.ID)
	jobCtx := u.tracker.begin(ctx, job, jobProgress{Phase: phaseUploading, LocalJobID: tracked.LocalJobID})
	reporting, stopReporting := context.WithCancel(ctx)
	item := &queuedUpload{job: job, ctx: jobCtx, reporting: reporting, stopReporting: stopReporting, done: make(chan struct{})}
	u.mu.Lock()
	u.queue = append(u.queue, item)
	u.mu.Unlock()
	select {
	case u.wake <- struct{}{}:
	default:
	}
}

// pop takes the next upload and marks it as the one in progress.
func (u *uploader) pop() (*queuedUpload, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.queue) == 0 {
		return nil, false
	}
	next := u.queue[0]
	u.queue = slices.Delete(u.queue, 0, 1)
	u.current = next
	return next, true
}

func (u *uploader) release(item *queuedUpload) {
	u.mu.Lock()
	u.current = nil
	u.mu.Unlock()
	item.stopReporting()
	close(item.done)
}

// backlog is how many jobs the uploader holds, the one in progress included.
func (u *uploader) backlog() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	held := len(u.queue)
	if u.current != nil {
		held++
	}
	return held
}

// abandon makes the uploader let go of every upload of one job id, and
// returns once it has, so a newer attempt of that job can start clean. An
// upload in progress ends like a lost lease, and a failure report it was
// still repeating is given up. Whatever is left in the state file afterwards
// is for the caller to discard.
func (u *uploader) abandon(ctx context.Context, id string) {
	u.mu.Lock()
	var waiting []*queuedUpload
	u.queue = slices.DeleteFunc(u.queue, func(item *queuedUpload) bool {
		if item.job.ID != id {
			return false
		}
		waiting = append(waiting, item)
		return true
	})
	current := u.current
	u.mu.Unlock()
	for _, item := range waiting {
		u.tracker.end(item.job)
		item.stopReporting()
		close(item.done)
	}
	if current == nil || current.job.ID != id {
		return
	}
	current.stopReporting()
	u.tracker.signal(current.job, errLeaseLost)
	select {
	case <-current.done:
	case <-ctx.Done():
	}
}

func (u *uploader) run(ctx context.Context) {
	for {
		next, ok := u.pop()
		if ok {
			u.process(ctx, next)
			u.release(next)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-u.wake:
		}
	}
}

// process owns one job from the first part to its single outcome: completed,
// failed with one report, or taken away by the portal.
func (u *uploader) process(ctx context.Context, item *queuedUpload) {
	job, id := item.job, item.job.ID
	delivered, failure := u.upload(item.ctx, job)
	cause := context.Cause(item.ctx)
	tracked, held := u.state.Get(id)
	switch {
	case held && tracked.Attempt != job.Attempt:
		// A newer attempt owns the job now; nothing here is this upload's.
		u.tracker.end(job)
		return
	case delivered:
		// Checked first: once the job is done the portal reports its lease
		// as gone, which must not be read as a job taken away.
		log.Printf("cloudbridge: job %s delivered (%d videos)", id, len(tracked.Artifacts))
		u.tracker.end(job)
		u.discardLocal(ctx, tracked)
		u.forget(id)
		return
	case ctx.Err() != nil:
		// Shutting down: the next start resumes from the parts received.
		u.tracker.end(job)
		return
	case errors.Is(cause, errUnauthorized):
		// The videos stay on disk; a start with a valid token settles them.
		u.tracker.end(job)
		return
	case errors.Is(cause, errLeaseLost):
		log.Printf("cloudbridge: job %s was taken back by the portal during upload; discarding its videos", id)
		u.tracker.end(job)
		u.discardLocal(ctx, tracked)
		u.forget(id)
		return
	case errors.Is(cause, errCancelRequested):
		failure = failureOf(codeCanceled, "canceled on the portal during upload")
	case failure == nil:
		// The job's context ended for a reason this worker does not know.
		u.tracker.end(job)
		return
	}
	u.discardLocal(ctx, tracked)
	// The capture's machine time was reported with the phase change.
	reported := u.reportFailure(item.reporting, job, failure.request(0))
	u.tracker.end(job)
	if reported {
		u.forget(id)
	}
}

// upload sends every video that is not done yet and completes the job. It
// reports delivered once the portal has marked the job done. With neither
// delivered nor a failure, ctx ended and the caller reads the cause.
func (u *uploader) upload(ctx context.Context, job jobRef) (delivered bool, failure *jobFailure) {
	id := job.ID
	tracked, ok := u.state.Get(id)
	if !ok || tracked.Attempt != job.Attempt {
		return false, failureOf(codeUploadFailed, "the worker lost track of the job's videos")
	}
	started := tracked.UploadStartedAt
	if started.IsZero() {
		started = u.now()
	}
	budget := &uploadBudget{
		uploader: u,
		deadline: started.Add(u.timing.UploadDeadline),
		retry:    backoff{lowest: u.timing.UploadRetryMin, highest: u.timing.UploadRetryMax},
	}
	var total, done int64
	for _, artifact := range tracked.Artifacts {
		total += artifact.Size
		if artifact.Done {
			done += artifact.Size
		}
	}
	publish := func(current int64) {
		percent := 0
		if total > 0 {
			percent = int((done + current) * 100 / total)
		}
		u.tracker.update(job, func(progress *jobProgress) { progress.Percent = percent })
	}
	publish(0)

	for index, artifact := range tracked.Artifacts {
		if artifact.Done {
			continue
		}
		artifactID, failure := u.sendArtifact(ctx, sendArtifactInput{job: job, artifact: artifact, budget: budget, publish: publish})
		if failure != nil || ctx.Err() != nil {
			return false, failure
		}
		done += artifact.Size
		publish(0)
		if err := u.state.Update(id, func(entry *TrackedRequest) {
			if entry.Attempt != job.Attempt || index >= len(entry.Artifacts) {
				return
			}
			entry.Artifacts[index].Done = true
			entry.Artifacts[index].ArtifactID = artifactID
		}); err != nil {
			// Not fatal: a restart sends the file again and the portal
			// answers that it already holds every part.
			log.Printf("cloudbridge: save upload progress of %s: %v", id, err)
		}
	}

	for {
		err := u.client.completeJob(ctx, job)
		if err == nil {
			return true, nil
		}
		if ctx.Err() != nil || u.portalTookJob(job, err) {
			return false, nil
		}
		if failure := budget.afterError(ctx, err); failure != nil || ctx.Err() != nil {
			return false, failure
		}
	}
}

// uploadBudget decides, after a failed call, between waiting to try again
// and giving up for good.
type uploadBudget struct {
	uploader *uploader
	deadline time.Time
	retry    backoff
}

// afterError returns nil after waiting when the call is worth repeating, and
// the upload_failed failure when it is not, or when the deadline passed.
func (b *uploadBudget) afterError(ctx context.Context, err error) *jobFailure {
	if !retryablePortalError(err) {
		return failureOf(codeUploadFailed, err.Error())
	}
	if !b.uploader.now().Before(b.deadline) {
		return failureOf(codeUploadFailed, fmt.Sprintf("gave up after %s: %v", b.uploader.timing.UploadDeadline, err))
	}
	sleep(ctx, b.retry.next())
	return nil
}

type sendArtifactInput struct {
	job      jobRef
	artifact TrackedArtifact
	budget   *uploadBudget
	publish  func(currentBytes int64)
}

// sendArtifact repeats upload passes for one file until the portal has
// verified it. Every pass starts by asking which parts the portal holds, so
// a pass after a network error or a restart only sends what is missing.
func (u *uploader) sendArtifact(ctx context.Context, in sendArtifactInput) (string, *jobFailure) {
	checksumRetries := 0
	for {
		artifactID, err := u.uploadPass(ctx, in)
		switch {
		case err == nil:
			return artifactID, nil
		case ctx.Err() != nil:
			return "", nil
		case u.portalTookJob(in.job, err):
			return "", nil
		case apiErrorCode(err) == "sha256_mismatch":
			// The portal dropped the parts, so the next pass sends them all.
			if checksumRetries++; checksumRetries > maxChecksumRetries {
				return "", failureOf(codeUploadFailed, in.artifact.Name+": "+err.Error())
			}
			continue
		case apiErrorCode(err) == "parts_missing":
			// The next pass asks again which parts are missing.
			if !u.now().Before(in.budget.deadline) {
				return "", failureOf(codeUploadFailed, in.artifact.Name+": "+err.Error())
			}
			if !sleep(ctx, in.budget.retry.next()) {
				return "", nil
			}
			continue
		}
		if failure := in.budget.afterError(ctx, err); failure != nil {
			failure.Detail = in.artifact.Name + ": " + failure.Detail
			return "", failure
		}
		if ctx.Err() != nil {
			return "", nil
		}
	}
}

// uploadPass registers the file, sends the parts the portal lacks, in
// order, and asks the portal to verify the whole.
func (u *uploader) uploadPass(ctx context.Context, in sendArtifactInput) (string, error) {
	artifact := in.artifact
	file, err := os.Open(artifact.Path) //nolint:gosec // the path was resolved inside the local storage root
	if err != nil {
		return "", fmt.Errorf("%w: %v", errLocalFile, err)
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || info.Size() != artifact.Size {
		return "", fmt.Errorf("%w: %s changed after it was hashed", errLocalFile, artifact.Path)
	}
	upload, err := u.client.initArtifact(ctx, in.job, artifactInit{
		Name:      artifact.Name,
		Kind:      artifactKindVideo,
		Variant:   artifact.Variant,
		SizeBytes: artifact.Size,
		SHA256:    artifact.SHA256,
	})
	if err != nil {
		return "", err
	}
	partCount := int((artifact.Size + upload.PartSize - 1) / upload.PartSize)
	partLength := func(number int) int64 {
		return min(upload.PartSize, artifact.Size-int64(number-1)*upload.PartSize)
	}
	var sent int64
	for _, number := range upload.ReceivedParts {
		if number >= 1 && number <= partCount {
			sent += partLength(number)
		}
	}
	in.publish(sent)
	for number := 1; number <= partCount; number++ {
		if slices.Contains(upload.ReceivedParts, number) {
			continue
		}
		length := partLength(number)
		body := io.NewSectionReader(file, int64(number-1)*upload.PartSize, length)
		part := partRef{Job: in.job, ArtifactID: upload.ArtifactID, Number: number}
		if _, err := u.client.putPart(ctx, part, body, length); err != nil {
			return "", err
		}
		sent += length
		in.publish(sent)
	}
	if err := u.client.completeArtifact(ctx, in.job, upload.ArtifactID); err != nil {
		return "", err
	}
	return upload.ArtifactID, nil
}
