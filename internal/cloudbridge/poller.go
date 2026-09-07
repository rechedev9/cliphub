package cloudbridge

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/rechedev9/cliphub/internal/job"
)

// DefaultInterval is how often the poller checks the portal for reconcile
// work and new approved requests.
const DefaultInterval = 20 * time.Second

// Admitter is the local side of admission: *httpapi.Handlers satisfies this
// without cloudbridge importing httpapi, so the bridge only ever depends on
// the one method it actually calls.
type Admitter interface {
	AdmitCloudDemo(ctx context.Context, demo io.Reader, fileName, cloudRequestID string) (*job.Job, error)
}

// Poller periodically claims one approved ClipHub Portal request at a time,
// admits its demo into the local pipeline, and reports the resulting local
// Job id back. One at a time is deliberate: the capture lane can only drive
// one cs2.exe anyway.
type Poller struct {
	client   *client
	admit    Admitter
	state    *State
	interval time.Duration
}

// NewPoller builds a Poller against a running ClipHub Portal deployment.
func NewPoller(baseURL, token string, admit Admitter, state *State) *Poller {
	return &Poller{
		client:   newClient(baseURL, token),
		admit:    admit,
		state:    state,
		interval: DefaultInterval,
	}
}

// Run polls until ctx is canceled. It ticks once immediately so a freshly
// started orchestrator does not wait a full interval before its first check.
func (p *Poller) Run(ctx context.Context) {
	p.tick(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

func (p *Poller) tick(ctx context.Context) {
	if err := p.reconcile(ctx); err != nil {
		log.Printf("cloudbridge: reconcile: %v", err)
		// A prior admission is still unconfirmed; do not claim more work
		// until the portal has acknowledged it, or the mapping could be lost.
		return
	}

	claimed, ok, err := p.client.claim(ctx)
	if err != nil {
		log.Printf("cloudbridge: claim: %v", err)
		return
	}
	if !ok {
		return
	}

	log.Printf("cloudbridge: claimed request %s (%s)", claimed.ID, claimed.DemoOriginalName)
	if err := p.admitClaimed(ctx, claimed); err != nil {
		log.Printf("cloudbridge: admit %s: %v", claimed.ID, err)
		if reportErr := p.client.reportFailure(ctx, claimed.ID, err.Error()); reportErr != nil {
			log.Printf("cloudbridge: report failure for %s: %v", claimed.ID, reportErr)
		}
	}
}

// reconcile re-sends the local-job report for any tracked request the bridge
// created a local Job for but never confirmed with the portal (a crash
// between the two). It stops at the first failure so a still-unreachable
// portal does not spin through every tracked entry every tick.
func (p *Poller) reconcile(ctx context.Context) error {
	for _, tr := range p.state.All() {
		if tr.LocalJobReported {
			continue
		}
		if err := p.client.reportLocalJob(ctx, tr.CloudRequestID, tr.LocalJobID); err != nil {
			return fmt.Errorf("report local job for %s: %w", tr.CloudRequestID, err)
		}
		if err := p.state.Update(tr.CloudRequestID, func(entry *TrackedRequest) {
			entry.LocalJobReported = true
		}); err != nil {
			return fmt.Errorf("persist reconciled state for %s: %w", tr.CloudRequestID, err)
		}
	}
	return nil
}

func (p *Poller) admitClaimed(ctx context.Context, claimed claimedRequest) error {
	demo, err := p.client.downloadDemo(ctx, claimed.ID)
	if err != nil {
		return fmt.Errorf("download demo: %w", err)
	}
	defer demo.Close()

	fileName := cloudFileName(claimed.ID, claimed.SubmitterLabel, claimed.DemoOriginalName)
	j, err := p.admit.AdmitCloudDemo(ctx, demo, fileName, claimed.ID)
	if err != nil {
		return fmt.Errorf("admit: %w", err)
	}

	if err := p.state.Insert(TrackedRequest{
		CloudRequestID: claimed.ID,
		LocalJobID:     j.ID.String(),
	}); err != nil {
		return fmt.Errorf("persist tracking state: %w", err)
	}

	if err := p.client.reportLocalJob(ctx, claimed.ID, j.ID.String()); err != nil {
		// Not fatal: the local Job exists and is durably tracked, so the next
		// tick's reconcile step retries this report before claiming more work.
		log.Printf("cloudbridge: report local-job for %s (will retry): %v", claimed.ID, err)
		return nil
	}
	return p.state.Update(claimed.ID, func(entry *TrackedRequest) {
		entry.LocalJobReported = true
	})
}
