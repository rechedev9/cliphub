package cloudbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/rules"
)

// ErrDemoRejected is what an Admitter returns for a file the local pipeline
// refuses as a demo (not a CS2 demo, or a CS:GO one). No retry can fix it.
var ErrDemoRejected = errors.New("cloudbridge: the local pipeline rejected the demo")

// DemoAdmission is one downloaded cloud demo handed to the local pipeline.
type DemoAdmission struct {
	Demo           io.Reader
	FileName       string
	CloudRequestID string
	TargetSteamID  string
	Rules          rules.Rules
}

// Admitter is the local side of admission. The orchestrator adapts
// *httpapi.Handlers to it, so the bridge never imports the HTTP layer.
type Admitter interface {
	// AdmitCloudDemo stores the demo, queues its parse and returns the id of
	// the local job.
	AdmitCloudDemo(ctx context.Context, in DemoAdmission) (localJobID string, err error)
}

// JobCanceler stops the local work of one job. The orchestrator's inline
// queue satisfies it: canceling a task ends the recorder or the editor with
// every process they started, and the task only returns once they are gone.
type JobCanceler interface {
	// CancelJob reports whether the job still had work pending or in flight,
	// so a caller that must be sure asks again until it answers false.
	CancelJob(jobID uuid.UUID) bool
}

// ArtifactStore reads render artifacts off local disk. *storage.Local
// satisfies it.
type ArtifactStore interface {
	Open(key string) (io.ReadCloser, error)
	ResolvePath(key string) (string, error)
}

// Timing holds every interval and budget of the worker. The zero value of a
// field means its default; tests shrink them.
type Timing struct {
	Heartbeat            time.Duration
	UnauthorizedInterval time.Duration
	Claim                time.Duration
	Poll                 time.Duration
	ParseTimeout         time.Duration
	// RenderStartTimeout bounds how long a finished capture may sit without
	// its render appearing before the attempt is given up as internal.
	RenderStartTimeout time.Duration
	// SettleTimeout bounds the wait for a canceled local job to stop.
	SettleTimeout time.Duration
	RetryMin      time.Duration
	RetryMax      time.Duration
	// DownloadAttempts is how many times a demo download is resumed.
	DownloadAttempts  int
	UploadRetryMin    time.Duration
	UploadRetryMax    time.Duration
	UploadDeadline    time.Duration
	ProgressSave      time.Duration
	DefaultMaxRuntime time.Duration
	// Janitor is how often an idle worker looks for leftovers of jobs it no
	// longer holds.
	Janitor time.Duration
}

func (t Timing) withDefaults() Timing {
	setDuration := func(target *time.Duration, fallback time.Duration) {
		if *target <= 0 {
			*target = fallback
		}
	}
	setDuration(&t.Heartbeat, 15*time.Second)
	setDuration(&t.UnauthorizedInterval, 60*time.Second)
	setDuration(&t.Claim, 10*time.Second)
	setDuration(&t.Poll, 2*time.Second)
	setDuration(&t.ParseTimeout, 15*time.Minute)
	setDuration(&t.RenderStartTimeout, 2*time.Minute)
	setDuration(&t.SettleTimeout, 90*time.Second)
	setDuration(&t.RetryMin, 2*time.Second)
	setDuration(&t.RetryMax, 60*time.Second)
	setDuration(&t.UploadRetryMin, 5*time.Second)
	setDuration(&t.UploadRetryMax, 5*time.Minute)
	setDuration(&t.UploadDeadline, 6*time.Hour)
	setDuration(&t.ProgressSave, 30*time.Second)
	setDuration(&t.DefaultMaxRuntime, 4*time.Hour)
	setDuration(&t.Janitor, 10*time.Minute)
	if t.DownloadAttempts <= 0 {
		t.DownloadAttempts = 5
	}
	return t
}

// Config wires a Worker to the portal and to the orchestrator it runs in.
type Config struct {
	BaseURL string
	Token   string
	// DataDir is the orchestrator data directory; demos are downloaded to
	// <DataDir>/cloudbridge/incoming.
	DataDir  string
	State    *State
	Admitter Admitter
	Local    LocalPipeline
	Canceler JobCanceler
	Files    ArtifactStore
	Machine  Machine
	// Kinds are the job kinds this worker claims.
	Kinds        []string
	MinFreeBytes uint64
	Timing       Timing
}

// Worker is the unattended cloud worker: a runner that takes one capture at
// a time, an uploader that sends finished videos while the next capture
// runs, and a heartbeat that keeps the portal informed throughout.
type Worker struct {
	runner    *runner
	uploader  *uploader
	heartbeat *heartbeat
}

// core is what the runner and the uploader share.
type core struct {
	client   *client
	state    *State
	tracker  *tracker
	local    LocalPipeline
	canceler JobCanceler
	timing   Timing
	incoming string
	now      func() time.Time

	// stopping holds the local jobs whose work had not stopped when their
	// discard gave up. The runner claims nothing until they have.
	stoppingMu sync.Mutex
	stopping   []TrackedRequest
}

// NewWorker builds a Worker. Nothing runs until Run.
func NewWorker(cfg Config) *Worker {
	timing := cfg.Timing.withDefaults()
	shared := &core{
		client:   newClient(cfg.BaseURL, cfg.Token),
		state:    cfg.State,
		tracker:  newTracker(),
		local:    cfg.Local,
		canceler: cfg.Canceler,
		timing:   timing,
		incoming: filepath.Join(cfg.DataDir, "cloudbridge", "incoming"),
		now:      time.Now,
	}
	kinds := cfg.Kinds
	if len(kinds) == 0 {
		kinds = []string{kindShort}
	}
	up := newUploader(shared)
	return &Worker{
		runner: &runner{
			core:         shared,
			admitter:     cfg.Admitter,
			files:        cfg.Files,
			machine:      cfg.Machine,
			uploader:     up,
			kinds:        kinds,
			minFreeBytes: cfg.MinFreeBytes,
		},
		uploader: up,
		heartbeat: &heartbeat{
			client:               shared.client,
			tracker:              shared.tracker,
			machine:              cfg.Machine,
			interval:             timing.Heartbeat,
			unauthorizedInterval: timing.UnauthorizedInterval,
		},
	}
}

// Run works until ctx is canceled. The orchestrator must already serve its
// loopback API, because the runner drives the local pipeline through it.
func (w *Worker) Run(ctx context.Context) {
	done := make(chan struct{}, 2)
	go func() {
		w.heartbeat.run(ctx)
		done <- struct{}{}
	}()
	go func() {
		w.uploader.run(ctx)
		done <- struct{}{}
	}()
	w.runner.run(ctx)
	<-done
	<-done
}

// sleep waits for d and reports false when ctx ended first.
func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// backoff doubles from lowest up to highest.
type backoff struct {
	lowest  time.Duration
	highest time.Duration
	current time.Duration
}

func (b *backoff) next() time.Duration {
	if b.current < b.lowest {
		b.current = b.lowest
	} else {
		b.current = min(b.current*2, b.highest)
	}
	return b.current
}

// reportFailure ends an attempt on the portal. It repeats the one logical
// fail call until the portal has answered it, so a job is neither left
// hanging on a network blip nor failed twice. A lost lease or a rejected
// token means the portal already took the job away, which also settles it.
// It returns false only when ctx ended first; the attempt then stays in the
// state file and the next start reports it.
func (c *core) reportFailure(ctx context.Context, job jobRef, body failRequest) bool {
	jobID := job.ID
	retry := backoff{lowest: c.timing.RetryMin, highest: c.timing.RetryMax}
	for {
		outcome, err := c.client.fail(ctx, job, body)
		switch {
		case err == nil:
			log.Printf("cloudbridge: job %s ended as %s (%s)%s", jobID, body.Code, outcome.Outcome, pausedSuffix(outcome.WorkerPaused))
			if outcome.WorkerPaused {
				// Do not wait for the next heartbeat to learn about the pause.
				c.tracker.pause()
			}
			return true
		case errors.Is(err, errLeaseLost):
			log.Printf("cloudbridge: job %s was already taken back by the portal; %s not reported", jobID, body.Code)
			return true
		case errors.Is(err, errUnauthorized):
			c.tracker.revoke()
			return true
		case !retryablePortalError(err):
			log.Printf("cloudbridge: the portal refused the %s report for job %s: %v", body.Code, jobID, err)
			return true
		}
		if !sleep(ctx, retry.next()) {
			return false
		}
	}
}

func pausedSuffix(paused bool) string {
	if paused {
		return "; the portal paused this worker"
	}
	return ""
}

// portalTookJob ends the job's context when a portal call says the job (or
// the whole worker) is no longer ours, and reports whether it did.
func (c *core) portalTookJob(job jobRef, err error) bool {
	switch {
	case errors.Is(err, errLeaseLost):
		c.tracker.signal(job, errLeaseLost)
		return true
	case errors.Is(err, errUnauthorized):
		c.tracker.revoke()
		return true
	default:
		return false
	}
}

// discardLocal removes everything an attempt left on this machine: the local
// job with its capture and renders, and the downloaded demo. It stops the job
// and deletes it only once none of its work is in flight, so the recorder,
// the editor and every process they started are gone before anything else
// runs here. A job that is still working at the deadline holds every claim
// until it stops; one that only could not be deleted is left to the janitor.
func (c *core) discardLocal(ctx context.Context, tr TrackedRequest) {
	c.removeDemo(tr.DemoPath)
	if tr.LocalJobID == "" {
		return
	}
	localID, err := uuid.Parse(tr.LocalJobID)
	if err != nil {
		log.Printf("cloudbridge: job %s has an invalid local job id %q", tr.CloudRequestID, tr.LocalJobID)
		return
	}
	deadline := c.now().Add(c.timing.SettleTimeout)
	for {
		working, err := c.stopAndDelete(ctx, localID)
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			return
		}
		if !c.now().Before(deadline) {
			log.Printf("cloudbridge: could not delete local job %s of %s: %v", tr.LocalJobID, tr.CloudRequestID, err)
			if working {
				c.stoppingMu.Lock()
				c.stopping = append(c.stopping, TrackedRequest{CloudRequestID: tr.CloudRequestID, LocalJobID: tr.LocalJobID})
				c.stoppingMu.Unlock()
			}
			return
		}
		if !sleep(ctx, c.timing.Poll) {
			return
		}
	}
}

// stopAndDelete cancels the local job's work and deletes the job once none
// of it is in flight. It reports whether work was still pending or running.
func (c *core) stopAndDelete(ctx context.Context, localID uuid.UUID) (working bool, err error) {
	if c.canceler.CancelJob(localID) {
		return true, ErrLocalJobBusy
	}
	return false, c.local.DeleteJob(ctx, localID.String())
}

// stillStopping looks again at the local jobs that outlived their discard.
// It returns why the machine is not free while one of them still works.
func (c *core) stillStopping(ctx context.Context) *Block {
	c.stoppingMu.Lock()
	pending := slices.Clone(c.stopping)
	c.stoppingMu.Unlock()
	var block *Block
	settled := map[string]bool{}
	for _, tr := range pending {
		localID, err := uuid.Parse(tr.LocalJobID)
		if err != nil {
			settled[tr.LocalJobID] = true
			continue
		}
		working, err := c.stopAndDelete(ctx, localID)
		if working {
			block = &Block{
				Code:   blockCS2Running,
				Detail: fmt.Sprintf("local job %s of cloud job %s has not stopped its capture or render yet", tr.LocalJobID, tr.CloudRequestID),
			}
			continue
		}
		if err != nil {
			log.Printf("cloudbridge: local job %s of %s stopped but could not be deleted; the janitor tries again: %v", tr.LocalJobID, tr.CloudRequestID, err)
		}
		settled[tr.LocalJobID] = true
	}
	c.stoppingMu.Lock()
	c.stopping = slices.DeleteFunc(c.stopping, func(tr TrackedRequest) bool { return settled[tr.LocalJobID] })
	c.stoppingMu.Unlock()
	return block
}

// removeDemo deletes a downloaded demo, and only ever a file inside the
// incoming directory: the path comes from the state file.
func (c *core) removeDemo(path string) {
	if path == "" {
		return
	}
	rel, err := filepath.Rel(c.incoming, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("cloudbridge: remove downloaded demo %s: %v", path, err)
	}
}

// forget drops a job from the state file. A failed write is logged: the
// worst outcome is one extra, harmless report after the next start.
func (c *core) forget(jobID string) {
	if err := c.state.Remove(jobID); err != nil {
		log.Printf("cloudbridge: forget job %s: %v", jobID, err)
	}
}

func (c *core) demoPath(jobID string) (string, error) {
	parsed, err := uuid.Parse(jobID)
	if err != nil {
		return "", fmt.Errorf("job id %q is not a UUID", jobID)
	}
	return filepath.Join(c.incoming, parsed.String()+".dem"), nil
}

func machineSeconds(since, now time.Time) int {
	if since.IsZero() || now.Before(since) {
		return 0
	}
	return int(now.Sub(since).Seconds())
}
