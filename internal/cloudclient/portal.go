// Package cloudclient is the Studio side of the ClipHub cloud: it links this
// PC to a portal account, submits jobs with their demo, follows them and
// downloads the finished videos. The device token never leaves the
// orchestrator; the Studio UI only talks to the loopback /api/cloud routes.
package cloudclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	portalCallTimeout = 30 * time.Second
	// transferIdleTimeout aborts an upload or download that stops moving;
	// transfers have no total deadline because sizes vary a hundredfold.
	transferIdleTimeout = 90 * time.Second
)

// ErrUnauthorized means the portal no longer accepts this device's token:
// the user revoked the device, or the token was never valid.
var ErrUnauthorized = errors.New("cloudclient: the portal rejected the device token")

// ErrChecksum means a downloaded file does not match the sha256 the portal
// announced. The partial file is already removed.
var ErrChecksum = errors.New("cloudclient: downloaded file does not match its sha256")

// APIError is a portal answer outside 2xx other than a rejected token.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("portal answered %d %s: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("portal answered %d: %s", e.Status, e.Message)
}

// PortalUser is the account a device is linked to.
type PortalUser struct {
	ID    string  `json:"id,omitempty"`
	Name  string  `json:"name"`
	Email string  `json:"email"`
	Image *string `json:"image,omitempty"`
}

// PortalQueue is the public state of the cloud queue.
type PortalQueue struct {
	State       string          `json:"state"`
	Queued      int             `json:"queued"`
	WaitSeconds map[string]*int `json:"waitSeconds"`
}

// Me is the portal's view of the linked account: access, limits and usage.
type Me struct {
	User   PortalUser `json:"user"`
	Access string     `json:"access"`
	Limits struct {
		MaxActive    int   `json:"maxActive"`
		DailySeconds int   `json:"dailySeconds"`
		MaxDemoBytes int64 `json:"maxDemoBytes"`
	} `json:"limits"`
	Usage struct {
		Active           int `json:"active"`
		SecondsLast24h   int `json:"secondsLast24h"`
		SecondsCommitted int `json:"secondsCommitted"`
	} `json:"usage"`
	Kinds []string    `json:"kinds"`
	Queue PortalQueue `json:"queue"`
}

// PortalDemo describes the demo of a new job.
type PortalDemo struct {
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
	FileName  string `json:"fileName"`
}

// CreateJobRequest is the body of POST /api/studio/jobs.
type CreateJobRequest struct {
	Kind  string          `json:"kind"`
	Title string          `json:"title"`
	Demo  PortalDemo      `json:"demo"`
	Spec  json.RawMessage `json:"spec"`
}

// CreateJobResponse says whether the portal already holds the demo.
type CreateJobResponse struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	DemoUpload string `json:"demoUpload"`
}

// PortalJobQueue is where a queued job stands.
type PortalJobQueue struct {
	Position         int    `json:"position"`
	EstimatedStartAt *int64 `json:"estimatedStartAt"`
	EstimatedDoneAt  *int64 `json:"estimatedDoneAt"`
	State            string `json:"state"`
}

// PortalFailure is why a job failed, in words the user can read.
type PortalFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// PortalArtifact is one result file ready to download.
type PortalArtifact struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Variant   string `json:"variant"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	SHA256    string `json:"sha256"`
	ExpiresAt int64  `json:"expiresAt"`
}

// PortalJob is the portal's view of one cloud job (StudioJob). Times are
// epoch milliseconds.
type PortalJob struct {
	ID               string           `json:"id"`
	Kind             string           `json:"kind"`
	Title            string           `json:"title"`
	Status           string           `json:"status"`
	Stage            *string          `json:"stage"`
	ProgressPercent  *int             `json:"progressPercent"`
	CreatedAt        int64            `json:"createdAt"`
	EnqueuedAt       *int64           `json:"enqueuedAt"`
	StartedAt        *int64           `json:"startedAt"`
	FinishedAt       *int64           `json:"finishedAt"`
	EstimatedSeconds *int             `json:"estimatedSeconds"`
	Attempt          int              `json:"attempt"`
	CancelRequested  bool             `json:"cancelRequested"`
	Queue            *PortalJobQueue  `json:"queue"`
	Failure          *PortalFailure   `json:"failure"`
	Artifacts        []PortalArtifact `json:"artifacts"`
}

// LinkStart is a device link waiting for the user to approve it in a browser.
type LinkStart struct {
	LinkID          string `json:"linkId"`
	PollToken       string `json:"pollToken"`
	UserCode        string `json:"userCode"`
	VerifyURL       string `json:"verifyUrl"`
	ExpiresAt       int64  `json:"expiresAt"`
	IntervalSeconds int    `json:"intervalSeconds"`
}

// LinkPoll is the state of a pending link. DeviceToken is set exactly once,
// in the answer that reports "approved".
type LinkPoll struct {
	Status      string      `json:"status"`
	DeviceToken string      `json:"deviceToken"`
	User        *PortalUser `json:"user"`
}

// Portal is the HTTP client for the portal's user API.
type Portal struct {
	baseURL string
	http    *http.Client
}

// NewPortal targets one portal deployment, for example
// https://cliphub.gravityroom.app.
func NewPortal(baseURL string) *Portal {
	// No client-wide timeout: a 700 MiB demo upload and a status poll need
	// very different budgets, so each call carries its own.
	return &Portal{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{}}
}

// BaseURL is the portal root, for display.
func (p *Portal) BaseURL() string { return p.baseURL }

// Me reads the linked account.
func (p *Portal) Me(ctx context.Context, token string) (Me, error) {
	var out Me
	err := p.doJSON(ctx, call{method: http.MethodGet, path: "/api/studio/me", token: token}, &out)
	return out, err
}

// CreateJob submits a job. The portal applies every limit before any demo
// byte is uploaded.
func (p *Portal) CreateJob(ctx context.Context, token string, body CreateJobRequest) (CreateJobResponse, error) {
	var out CreateJobResponse
	err := p.doJSON(ctx, call{method: http.MethodPost, path: "/api/studio/jobs", token: token, body: body}, &out)
	return out, err
}

// ListJobs returns the user's active and recent cloud jobs.
func (p *Portal) ListJobs(ctx context.Context, token string) ([]PortalJob, error) {
	var out struct {
		Jobs []PortalJob `json:"jobs"`
	}
	err := p.doJSON(ctx, call{method: http.MethodGet, path: "/api/studio/jobs", token: token}, &out)
	return out.Jobs, err
}

// GetJob reads one job. A job of another user answers 404.
func (p *Portal) GetJob(ctx context.Context, token, jobID string) (PortalJob, error) {
	var out PortalJob
	err := p.doJSON(ctx, call{method: http.MethodGet, path: "/api/studio/jobs/" + url.PathEscape(jobID), token: token}, &out)
	return out, err
}

// CancelResult is the portal's answer to a cancel.
type CancelResult struct {
	Status          string `json:"status"`
	CancelRequested bool   `json:"cancelRequested"`
}

// CancelJob cancels a job, or asks the worker to stop it when it is running.
func (p *Portal) CancelJob(ctx context.Context, token, jobID string) (CancelResult, error) {
	var out CancelResult
	err := p.doJSON(ctx, call{method: http.MethodPost, path: "/api/studio/jobs/" + url.PathEscape(jobID) + "/cancel", token: token, body: struct{}{}}, &out)
	return out, err
}

// DemoUpload is one demo to send for a job that waits for it.
type DemoUpload struct {
	JobID string
	Body  io.Reader
	Size  int64
	// Progress receives the bytes sent so far. It may be nil.
	Progress func(sent int64)
}

// PutDemo streams the demo with its exact Content-Length and returns the
// job's new status.
func (p *Portal) PutDemo(ctx context.Context, token string, upload DemoUpload) (string, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	idle := time.AfterFunc(transferIdleTimeout, func() {
		cancel(fmt.Errorf("no demo bytes sent for %s", transferIdleTimeout))
	})
	defer idle.Stop()
	var sent int64
	body := &activityReader{reader: upload.Body, onRead: func(n int) {
		idle.Reset(transferIdleTimeout)
		sent += int64(n)
		if upload.Progress != nil {
			upload.Progress(sent)
		}
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, p.baseURL+"/api/studio/jobs/"+url.PathEscape(upload.JobID)+"/demo", body)
	if err != nil {
		return "", err
	}
	req.ContentLength = upload.Size
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := p.http.Do(req)
	if err != nil {
		return "", transferError(ctx, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", responseError(resp)
	}
	var out struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return out.Status, nil
}

// ArtifactDownload is one result file to fetch into Dest.
type ArtifactDownload struct {
	JobID      string
	ArtifactID string
	Dest       string
	SizeBytes  int64
	SHA256     string
	// Progress receives the bytes on disk so far. It may be nil.
	Progress func(received int64)
}

// DownloadArtifact fetches a result file, resuming whatever an earlier
// attempt left at Dest, and returns only once the whole file matches the
// announced sha256. A mismatch removes the file and reports ErrChecksum.
func (p *Portal) DownloadArtifact(ctx context.Context, token string, download ArtifactDownload) error {
	if err := os.MkdirAll(filepath.Dir(download.Dest), 0o750); err != nil {
		return fmt.Errorf("create results directory: %w", err)
	}
	offset := int64(0)
	if info, err := os.Stat(download.Dest); err == nil {
		offset = info.Size()
	}
	if download.SizeBytes > 0 && offset > download.SizeBytes {
		offset = 0
	}
	if download.SizeBytes <= 0 || offset < download.SizeBytes {
		if err := p.fetchArtifact(ctx, token, download, offset); err != nil {
			return err
		}
	}
	sum, size, err := hashFile(download.Dest)
	if err != nil {
		return fmt.Errorf("hash downloaded file: %w", err)
	}
	if (download.SizeBytes > 0 && size != download.SizeBytes) || !strings.EqualFold(sum, download.SHA256) {
		if removeErr := os.Remove(download.Dest); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("%w; remove it: %v", ErrChecksum, removeErr)
		}
		return fmt.Errorf("%w: got %d bytes with sha256 %s", ErrChecksum, size, sum)
	}
	return nil
}

func (p *Portal) fetchArtifact(ctx context.Context, token string, download ArtifactDownload, offset int64) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	target := p.baseURL + "/api/studio/jobs/" + url.PathEscape(download.JobID) + "/artifacts/" + url.PathEscape(download.ArtifactID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if offset > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	idle := time.AfterFunc(transferIdleTimeout, func() {
		cancel(fmt.Errorf("no result bytes for %s", transferIdleTimeout))
	})
	defer idle.Stop()
	resp, err := p.http.Do(req)
	if err != nil {
		return transferError(ctx, err)
	}
	defer resp.Body.Close()

	flags := os.O_WRONLY | os.O_CREATE
	switch {
	case resp.StatusCode == http.StatusOK:
		flags |= os.O_TRUNC
		offset = 0
	case resp.StatusCode == http.StatusPartialContent && offset > 0 && strings.HasPrefix(resp.Header.Get("Content-Range"), "bytes "+strconv.FormatInt(offset, 10)+"-"):
		flags |= os.O_APPEND
	case resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		// The partial file cannot be continued; the next attempt starts over.
		if removeErr := os.Remove(download.Dest); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("discard unusable partial download: %w", removeErr)
		}
		return fmt.Errorf("portal answered %d to a resume from byte %d", resp.StatusCode, offset)
	default:
		return responseError(resp)
	}
	file, err := os.OpenFile(download.Dest, flags, 0o600) //nolint:gosec // Dest is built by the service from validated names
	if err != nil {
		return fmt.Errorf("open result file: %w", err)
	}
	received := offset
	body := &activityReader{reader: resp.Body, onRead: func(n int) {
		idle.Reset(transferIdleTimeout)
		received += int64(n)
		if download.Progress != nil {
			download.Progress(received)
		}
	}}
	_, copyErr := io.Copy(file, body)
	if closeErr := file.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return transferError(ctx, copyErr)
	}
	return nil
}

// ArtifactReceived tells the portal this Studio has the file, which lets it
// delete its copy sooner.
func (p *Portal) ArtifactReceived(ctx context.Context, token, jobID, artifactID string) error {
	path := "/api/studio/jobs/" + url.PathEscape(jobID) + "/artifacts/" + url.PathEscape(artifactID) + "/received"
	return p.doJSON(ctx, call{method: http.MethodPost, path: path, token: token, body: struct{}{}}, nil)
}

// LinkStart opens a device link. It needs no token.
func (p *Portal) LinkStart(ctx context.Context, deviceName string) (LinkStart, error) {
	var out LinkStart
	body := map[string]string{"deviceName": deviceName}
	err := p.doJSON(ctx, call{method: http.MethodPost, path: "/api/studio/link/start", body: body}, &out)
	return out, err
}

// LinkPoll asks whether the user approved a pending link.
func (p *Portal) LinkPoll(ctx context.Context, linkID, pollToken string) (LinkPoll, error) {
	var out LinkPoll
	body := map[string]string{"linkId": linkID, "pollToken": pollToken}
	err := p.doJSON(ctx, call{method: http.MethodPost, path: "/api/studio/link/poll", body: body}, &out)
	return out, err
}

// DeleteDevice revokes the calling device on the portal.
func (p *Portal) DeleteDevice(ctx context.Context, token string) error {
	return p.doJSON(ctx, call{method: http.MethodDelete, path: "/api/studio/device", token: token}, nil)
}

type call struct {
	method string
	path   string
	token  string
	body   any
}

func (p *Portal) doJSON(ctx context.Context, c call, out any) error {
	ctx, cancel := context.WithTimeout(ctx, portalCallTimeout)
	defer cancel()
	var reader io.Reader
	if c.body != nil {
		encoded, err := json.Marshal(c.body)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, c.method, p.baseURL+c.path, reader)
	if err != nil {
		return err
	}
	if c.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return responseError(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func responseError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	_ = json.Unmarshal(raw, &body)
	message := body.Error
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return &APIError{Status: resp.StatusCode, Code: body.Code, Message: message}
}

// transferError prefers the idle timeout's explanation over the bare
// "context canceled" it causes.
func transferError(ctx context.Context, err error) error {
	if cause := context.Cause(ctx); cause != nil && !errors.Is(cause, context.Canceled) && !errors.Is(cause, context.DeadlineExceeded) {
		return cause
	}
	return err
}

type activityReader struct {
	reader io.Reader
	onRead func(n int)
}

func (r *activityReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.onRead(n)
	}
	return n, err
}

func hashFile(path string) (string, int64, error) {
	file, err := os.Open(path) //nolint:gosec // callers pass paths inside the data dir
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

// retryable reports whether repeating a portal call can succeed: the portal
// was unreachable or answered 5xx, 408 or 429.
func retryable(err error) bool {
	if err == nil || errors.Is(err, ErrUnauthorized) || errors.Is(err, context.Canceled) || errors.Is(err, errDemoGone) {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status >= 500 || apiErr.Status == http.StatusTooManyRequests || apiErr.Status == http.StatusRequestTimeout
	}
	return true
}
