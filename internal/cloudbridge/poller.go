package cloudbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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
// Job id back. It only ever makes outbound HTTP calls.
type Poller struct {
	baseURL    string
	token      string
	httpClient *http.Client
	admit      Admitter
	state      *State
	interval   time.Duration
}

// NewPoller builds a Poller against a running ClipHub Portal deployment.
func NewPoller(baseURL, token string, admit Admitter, state *State) *Poller {
	return &Poller{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		admit:      admit,
		state:      state,
		interval:   DefaultInterval,
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

	claimed, ok, err := p.claim(ctx)
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
		p.reportFailure(ctx, claimed.ID, err.Error())
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
		if err := p.reportLocalJob(ctx, tr.CloudRequestID, tr.LocalJobID); err != nil {
			return fmt.Errorf("report local job for %s: %w", tr.CloudRequestID, err)
		}
		tr.LocalJobReported = true
		if err := p.state.Upsert(tr); err != nil {
			return fmt.Errorf("persist reconciled state for %s: %w", tr.CloudRequestID, err)
		}
	}
	return nil
}

func (p *Poller) admitClaimed(ctx context.Context, claimed claimedRequest) error {
	demo, err := p.downloadDemo(ctx, claimed.ID)
	if err != nil {
		return fmt.Errorf("download demo: %w", err)
	}
	defer demo.Close()

	fileName := cloudFileName(claimed.ID, claimed.SubmitterLabel, claimed.DemoOriginalName)
	j, err := p.admit.AdmitCloudDemo(ctx, demo, fileName, claimed.ID)
	if err != nil {
		return fmt.Errorf("admit: %w", err)
	}

	tr := &TrackedRequest{CloudRequestID: claimed.ID, LocalJobID: j.ID.String()}
	if err := p.state.Upsert(tr); err != nil {
		return fmt.Errorf("persist tracking state: %w", err)
	}

	if err := p.reportLocalJob(ctx, claimed.ID, j.ID.String()); err != nil {
		// Not fatal: the local Job exists and is durably tracked, so the next
		// tick's reconcile step retries this report before claiming more work.
		log.Printf("cloudbridge: report local-job for %s (will retry): %v", claimed.ID, err)
		return nil
	}
	tr.LocalJobReported = true
	return p.state.Upsert(tr)
}

type claimedRequest struct {
	ID               string `json:"id"`
	Note             string `json:"note"`
	DemoOriginalName string `json:"demoOriginalName"`
	SubmitterLabel   string `json:"submitterLabel"`
}

type claimResponse struct {
	Claimed bool `json:"claimed"`
	claimedRequest
}

func (p *Poller) claim(ctx context.Context) (claimedRequest, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/bridge/claim", nil)
	if err != nil {
		return claimedRequest{}, false, err
	}
	p.authorize(req)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return claimedRequest{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return claimedRequest{}, false, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var out claimResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return claimedRequest{}, false, fmt.Errorf("decode response: %w", err)
	}
	return out.claimedRequest, out.Claimed, nil
}

func (p *Poller) downloadDemo(ctx context.Context, requestID string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/bridge/requests/"+requestID+"/demo", nil)
	if err != nil {
		return nil, err
	}
	p.authorize(req)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

func (p *Poller) reportLocalJob(ctx context.Context, requestID, localJobID string) error {
	return p.postJSON(ctx, "/api/bridge/requests/"+requestID+"/local-job", map[string]string{
		"localJobId": localJobID,
	})
}

func (p *Poller) reportFailure(ctx context.Context, requestID, reason string) {
	if err := p.postJSON(ctx, "/api/bridge/requests/"+requestID+"/status", map[string]string{
		"status": "failed",
		"reason": reason,
	}); err != nil {
		log.Printf("cloudbridge: report failure for %s: %v", requestID, err)
	}
}

func (p *Poller) postJSON(ctx context.Context, path string, payload map[string]string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	p.authorize(req)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (p *Poller) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.token)
}
