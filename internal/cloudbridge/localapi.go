package cloudbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rechedev9/cliphub/internal/killplan"
)

// Local job and render states the worker reacts to, as the loopback API
// spells them.
const (
	localJobParsed    = "parsed"
	localJobRecording = "recording"
	localJobFailed    = "failed"

	localVariantQueued    = "queued"
	localVariantRendering = "rendering"
	localVariantReady     = "ready"
	localVariantReview    = "review_required"
	localVariantFailed    = "failed"
)

// ErrLocalJobBusy means the local job still has work in flight, so the
// pipeline refused to delete it.
var ErrLocalJobBusy = errors.New("cloudbridge: local job still has work in flight")

// LocalProgress is the percent the local pipeline publishes for the stage it
// is running (capture or render).
type LocalProgress struct {
	Stage   string `json:"stage"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Percent int    `json:"percent"`
}

// LocalJob is the status view of one local job.
type LocalJob struct {
	Status        string         `json:"status"`
	FailureReason string         `json:"failure_reason"`
	FailureCode   string         `json:"failure_code"`
	Progress      *LocalProgress `json:"progress"`
}

// LocalVariant is the state of one render variant of a local job.
type LocalVariant struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// GenerateResult is the pipeline's answer to a generate request. Variant is
// the render the request will produce; Code and Message explain a rejection.
type GenerateResult struct {
	Status  int
	Code    string
	Message string
	Variant string
}

// LocalJobRef links a local job to the cloud job it was admitted for.
type LocalJobRef struct {
	ID             string `json:"id"`
	CloudRequestID string `json:"cloud_request_id"`
}

// LocalPipeline is the narrow part of the orchestrator's own API the worker
// drives. Going through the API reuses every validation a local user gets.
type LocalPipeline interface {
	// Generate posts body unchanged to the job's generate endpoint.
	Generate(ctx context.Context, jobID string, body json.RawMessage) (GenerateResult, error)
	// JobView reports found=false once the job no longer exists.
	JobView(ctx context.Context, jobID string) (job LocalJob, found bool, err error)
	KillPlan(ctx context.Context, jobID string) (*killplan.Plan, error)
	// VariantView reports found=false until the render has a state.
	VariantView(ctx context.Context, jobID, variant string) (state LocalVariant, found bool, err error)
	// DeleteJob removes the job and its files. A job that is already gone is
	// not an error; one with work in flight answers ErrLocalJobBusy.
	DeleteJob(ctx context.Context, jobID string) error
	// CloudJobs lists the recent local jobs that were admitted for a cloud job.
	CloudJobs(ctx context.Context) ([]LocalJobRef, error)
}

const localCallTimeout = 30 * time.Second

// LoopbackPipeline talks to the orchestrator this worker runs inside, over
// its loopback listener and with its session token.
type LoopbackPipeline struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewLoopbackPipeline targets the listener address the orchestrator bound.
func NewLoopbackPipeline(listenAddr, token string) *LoopbackPipeline {
	return &LoopbackPipeline{
		baseURL: "http://" + listenAddr,
		token:   token,
		http:    &http.Client{Timeout: localCallTimeout},
	}
}

type localResponse struct {
	status int
	body   []byte
}

type localError struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

func (r localResponse) failure() localError {
	var out localError
	_ = json.Unmarshal(r.body, &out)
	return out
}

func (r localResponse) unexpected(action string) error {
	failure := r.failure()
	return fmt.Errorf("%s: local pipeline answered %d %s: %s", action, r.status, failure.Code, failure.Error)
}

func (p *LoopbackPipeline) call(ctx context.Context, method, path string, body []byte) (localResponse, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, reader)
	if err != nil {
		return localResponse{}, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-ClipHub-Token", p.token)
	resp, err := p.http.Do(req)
	if err != nil {
		return localResponse{}, err
	}
	defer resp.Body.Close()
	// A kill plan of a long match is a few megabytes of JSON.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return localResponse{}, fmt.Errorf("read local response: %w", err)
	}
	return localResponse{status: resp.StatusCode, body: raw}, nil
}

func jobPath(jobID string, suffix string) string {
	return "/api/jobs/" + url.PathEscape(jobID) + suffix
}

// Generate implements LocalPipeline.
func (p *LoopbackPipeline) Generate(ctx context.Context, jobID string, body json.RawMessage) (GenerateResult, error) {
	resp, err := p.call(ctx, http.MethodPost, jobPath(jobID, "/generate"), body)
	if err != nil {
		return GenerateResult{}, err
	}
	result := GenerateResult{Status: resp.status}
	if resp.status == http.StatusAccepted {
		var accepted struct {
			Variant string `json:"variant"`
		}
		if err := json.Unmarshal(resp.body, &accepted); err != nil || accepted.Variant == "" {
			return GenerateResult{}, errors.New("local pipeline accepted generate without naming the variant")
		}
		result.Variant = accepted.Variant
		return result, nil
	}
	failure := resp.failure()
	result.Code, result.Message = failure.Code, failure.Error
	return result, nil
}

// JobView implements LocalPipeline.
func (p *LoopbackPipeline) JobView(ctx context.Context, jobID string) (LocalJob, bool, error) {
	resp, err := p.call(ctx, http.MethodGet, jobPath(jobID, "?view=status"), nil)
	if err != nil {
		return LocalJob{}, false, err
	}
	switch resp.status {
	case http.StatusOK:
		var job LocalJob
		if err := json.Unmarshal(resp.body, &job); err != nil {
			return LocalJob{}, false, fmt.Errorf("decode local job: %w", err)
		}
		return job, true, nil
	case http.StatusNotFound:
		return LocalJob{}, false, nil
	default:
		return LocalJob{}, false, resp.unexpected("read local job")
	}
}

// KillPlan implements LocalPipeline.
func (p *LoopbackPipeline) KillPlan(ctx context.Context, jobID string) (*killplan.Plan, error) {
	resp, err := p.call(ctx, http.MethodGet, jobPath(jobID, "/plan"), nil)
	if err != nil {
		return nil, err
	}
	if resp.status != http.StatusOK {
		return nil, resp.unexpected("read kill plan")
	}
	var plan killplan.Plan
	if err := json.Unmarshal(resp.body, &plan); err != nil {
		return nil, fmt.Errorf("decode kill plan: %w", err)
	}
	return &plan, nil
}

// VariantView implements LocalPipeline.
func (p *LoopbackPipeline) VariantView(ctx context.Context, jobID, variant string) (LocalVariant, bool, error) {
	resp, err := p.call(ctx, http.MethodGet, jobPath(jobID, "/renders/"+url.PathEscape(variant)), nil)
	if err != nil {
		return LocalVariant{}, false, err
	}
	switch resp.status {
	case http.StatusOK:
		var state LocalVariant
		if err := json.Unmarshal(resp.body, &state); err != nil {
			return LocalVariant{}, false, fmt.Errorf("decode render state: %w", err)
		}
		return state, true, nil
	case http.StatusNotFound:
		return LocalVariant{}, false, nil
	default:
		return LocalVariant{}, false, resp.unexpected("read render state")
	}
}

// DeleteJob implements LocalPipeline.
func (p *LoopbackPipeline) DeleteJob(ctx context.Context, jobID string) error {
	resp, err := p.call(ctx, http.MethodDelete, jobPath(jobID, ""), nil)
	if err != nil {
		return err
	}
	switch resp.status {
	case http.StatusNoContent, http.StatusNotFound:
		return nil
	case http.StatusConflict:
		return fmt.Errorf("%w: %s", ErrLocalJobBusy, resp.failure().Error)
	default:
		return resp.unexpected("delete local job")
	}
}

// CloudJobs implements LocalPipeline.
func (p *LoopbackPipeline) CloudJobs(ctx context.Context) ([]LocalJobRef, error) {
	resp, err := p.call(ctx, http.MethodGet, "/api/jobs?limit=100", nil)
	if err != nil {
		return nil, err
	}
	if resp.status != http.StatusOK {
		return nil, resp.unexpected("list local jobs")
	}
	var list struct {
		Jobs []LocalJobRef `json:"jobs"`
	}
	if err := json.Unmarshal(resp.body, &list); err != nil {
		return nil, fmt.Errorf("decode local jobs: %w", err)
	}
	cloud := make([]LocalJobRef, 0, len(list.Jobs))
	for _, job := range list.Jobs {
		if job.CloudRequestID != "" {
			cloud = append(cloud, job)
		}
	}
	return cloud, nil
}
