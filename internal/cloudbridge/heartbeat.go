package cloudbridge

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"sync"
	"time"
)

// Worker states and job stages, as the portal's contract spells them.
const (
	stateIdle    = "idle"
	stateBusy    = "busy"
	stateBlocked = "blocked"

	stageDownloading = "downloading"
	stageParsing     = "parsing"
	stageCapturing   = "capturing"
	stageRendering   = "rendering"

	leaseLost = "lost"
)

// errCancelRequested is the cause a job's context carries once the user or
// an operator canceled the job on the portal.
var errCancelRequested = errors.New("cloudbridge: the job was canceled on the portal")

// jobProgress is what the heartbeat says about one job this worker holds.
type jobProgress struct {
	Phase      string
	Stage      string
	Percent    int
	Detail     string
	LocalJobID string
}

type trackedJob struct {
	attempt  int
	progress jobProgress
	cancel   context.CancelCauseFunc
}

// tracker is the snapshot the runner and the uploader publish and the
// heartbeat sends. It also carries the portal's answers back: a pause, a
// lost lease or a cancel request stops the job through its context. An entry
// belongs to one attempt, and only that attempt can change or end it.
type tracker struct {
	mu           sync.Mutex
	jobs         map[string]*trackedJob
	blocked      *Block
	paused       bool
	unauthorized bool
	// rejected is set while the portal answers heartbeats with a 4xx: it no
	// longer renews any lease, so nothing new may be claimed.
	rejected bool
}

func newTracker() *tracker {
	return &tracker{jobs: map[string]*trackedJob{}}
}

// begin starts publishing one attempt of a job and returns the context its
// work must run under. The context ends when the portal takes the job away;
// its cause is errCancelRequested, errLeaseLost or errUnauthorized.
func (t *tracker) begin(ctx context.Context, job jobRef, progress jobProgress) context.Context {
	jobCtx, cancel := context.WithCancelCause(ctx)
	t.mu.Lock()
	defer t.mu.Unlock()
	if previous, ok := t.jobs[job.ID]; ok {
		// An older attempt that is still published lost the job to this one.
		var cause error
		if previous.attempt != job.Attempt {
			cause = errLeaseLost
		}
		previous.cancel(cause)
	}
	t.jobs[job.ID] = &trackedJob{attempt: job.Attempt, progress: progress, cancel: cancel}
	return jobCtx
}

// own returns the entry of exactly this attempt. Callers hold the lock.
func (t *tracker) own(job jobRef) (*trackedJob, bool) {
	entry, ok := t.jobs[job.ID]
	if !ok || entry.attempt != job.Attempt {
		return nil, false
	}
	return entry, true
}

func (t *tracker) update(job jobRef, mutate func(*jobProgress)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if entry, ok := t.own(job); ok {
		mutate(&entry.progress)
	}
}

func (t *tracker) end(job jobRef) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if entry, ok := t.own(job); ok {
		entry.cancel(nil)
		delete(t.jobs, job.ID)
	}
}

func (t *tracker) signal(job jobRef, cause error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if entry, ok := t.own(job); ok {
		entry.cancel(cause)
	}
}

// revoke records that the portal rejected the token and takes every job
// away: a revoked worker can neither finish nor report them.
func (t *tracker) revoke() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.unauthorized = true
	for _, job := range t.jobs {
		job.cancel(errUnauthorized)
	}
}

// pause stops claiming until a heartbeat says the worker was resumed.
func (t *tracker) pause() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = true
}

func (t *tracker) setBlocked(block *Block) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.blocked = block
}

func (t *tracker) setRejected(rejected bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rejected = rejected
}

// claimStop is why the portal would not hand this worker a job, or could not
// keep the lease of one alive.
type claimStop struct {
	paused       bool
	unauthorized bool
	rejected     bool
}

func (t *tracker) claimable() claimStop {
	t.mu.Lock()
	defer t.mu.Unlock()
	return claimStop{paused: t.paused, unauthorized: t.unauthorized, rejected: t.rejected}
}

func (t *tracker) request() (body heartbeatRequest, capturing bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	body.Jobs = make([]heartbeatJob, 0, len(t.jobs))
	for id, job := range t.jobs {
		entry := heartbeatJob{
			ID:         id,
			Attempt:    job.attempt,
			Phase:      job.progress.Phase,
			Percent:    min(max(job.progress.Percent, 0), 100),
			Detail:     job.progress.Detail,
			LocalJobID: job.progress.LocalJobID,
		}
		if job.progress.Phase == phaseRunning {
			stage := job.progress.Stage
			entry.Stage = &stage
			capturing = true
		}
		body.Jobs = append(body.Jobs, entry)
	}
	slices.SortFunc(body.Jobs, func(a, b heartbeatJob) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	switch {
	case capturing:
		body.State = stateBusy
	case t.blocked != nil:
		body.State = stateBlocked
		block := *t.blocked
		body.Blocked = &block
	default:
		body.State = stateIdle
	}
	return body, capturing
}

// apply feeds the answer to one heartbeat back. sent is that heartbeat: an
// answer only reaches the attempt it was about, never a newer one that took
// the same job id while the request was on its way.
func (t *tracker) apply(sent heartbeatRequest, resp heartbeatResponse) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = resp.Paused
	t.unauthorized = false
	t.rejected = false
	reported := make(map[string]int, len(sent.Jobs))
	for _, job := range sent.Jobs {
		reported[job.ID] = job.Attempt
	}
	for _, status := range resp.Jobs {
		attempt, asked := reported[status.ID]
		if !asked {
			continue
		}
		job, ok := t.own(jobRef{ID: status.ID, Attempt: attempt})
		if !ok {
			continue
		}
		// A lost lease wins over a cancel: the job is no longer ours to
		// acknowledge.
		switch {
		case status.Lease == leaseLost:
			job.cancel(errLeaseLost)
		case status.CancelRequested:
			job.cancel(errCancelRequested)
		}
	}
}

// heartbeat reports the worker's state every interval and feeds the
// portal's answers into the tracker. It runs for the whole life of the
// worker, also while a capture blocks the runner.
type heartbeat struct {
	client               *client
	tracker              *tracker
	machine              Machine
	interval             time.Duration
	unauthorizedInterval time.Duration
	lastErr              string
}

func (h *heartbeat) run(ctx context.Context) {
	for {
		wait := h.interval
		if !h.beat(ctx) {
			wait = h.unauthorizedInterval
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// beat sends one heartbeat. It returns false when the token was rejected, so
// the loop slows down until a new credential shows up after a restart.
func (h *heartbeat) beat(ctx context.Context) bool {
	body, capturing := h.tracker.request()
	body.Health = h.machine.Health(ctx, capturing)
	resp, err := h.client.heartbeat(ctx, body)
	switch {
	case errors.Is(err, errUnauthorized):
		h.tracker.revoke()
		h.logOnce("the portal rejected the worker token; not claiming until a valid ZV_BRIDGE_TOKEN is configured")
		return false
	case refusedByPortal(err):
		// Unlike an outage, a refused heartbeat does not fix itself: every
		// lease would lapse while the worker kept burning queued jobs.
		h.tracker.setRejected(true)
		h.logOnce(fmt.Sprintf("the portal refused the heartbeat (%v); not claiming until one is accepted", err))
		return true
	case err != nil:
		if ctx.Err() == nil {
			h.logOnce("heartbeat: " + err.Error())
		}
		return true
	}
	h.lastErr = ""
	h.tracker.apply(body, resp)
	return true
}

// refusedByPortal reports a 4xx answer. A rejected token (401) is not one of
// them: responseError turns it into errUnauthorized.
func refusedByPortal(err error) bool {
	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Status >= http.StatusBadRequest && apiErr.Status < http.StatusInternalServerError
}

// logOnce keeps an unreachable portal from writing the same line every 15 s.
func (h *heartbeat) logOnce(message string) {
	if message == h.lastErr {
		return
	}
	h.lastErr = message
	log.Printf("cloudbridge: %s", message)
}
