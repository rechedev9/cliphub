package cloudbridge

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
	"unicode/utf8"
)

const (
	jsonCallTimeout = 30 * time.Second
	partCallTimeout = 10 * time.Minute
	// demoIdleTimeout aborts a demo download that stops delivering bytes; a
	// download has no total deadline because its size varies a hundredfold.
	demoIdleTimeout = 60 * time.Second

	maxFailureMessageRunes = 500
	maxFailureDetailRunes  = 4000

	// attemptHeader carries the fencing token of every call about a job: the
	// attempt number of the claim that started the work.
	attemptHeader = "X-ClipHub-Attempt"
)

// jobRef names one attempt of a job. The portal answers lease_lost to any
// call whose attempt is not the one it currently leases.
type jobRef struct {
	ID      string
	Attempt int
}

func (j jobRef) path(suffix string) string {
	return "/api/worker/jobs/" + url.PathEscape(j.ID) + suffix
}

var (
	// errUnauthorized means the portal does not know this worker token, or
	// revoked it. Nothing recovers it short of a new token and a restart.
	errUnauthorized = errors.New("cloudbridge: the portal rejected the worker token")
	// errLeaseLost means the job is no longer this worker's: abandon it and
	// clean up locally without reporting anything.
	errLeaseLost = errors.New("cloudbridge: the job lease is lost")
	// errDemoCorrupt means the downloaded bytes do not match the claim; the
	// partial file is already removed, so a retry starts from zero.
	errDemoCorrupt = errors.New("cloudbridge: downloaded demo does not match the claimed sha256 or size")
)

// apiError is a portal answer outside 2xx that is neither a rejected token
// nor a lost lease.
type apiError struct {
	Status  int
	Code    string
	Message string
	Missing []int
}

func (e *apiError) Error() string {
	text := e.Message
	if text == "" {
		text = http.StatusText(e.Status)
	}
	if e.Code != "" {
		return fmt.Sprintf("portal answered %d %s: %s", e.Status, e.Code, text)
	}
	return fmt.Sprintf("portal answered %d: %s", e.Status, text)
}

// apiErrorCode returns the portal's error code, or "" for any other error.
func apiErrorCode(err error) string {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return ""
}

// retryablePortalError reports whether repeating the same call can succeed:
// the portal was unreachable, timed out, or answered 5xx or 429.
func retryablePortalError(err error) bool {
	if err == nil || errors.Is(err, errUnauthorized) || errors.Is(err, errLeaseLost) || errors.Is(err, errLocalFile) {
		return false
	}
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		return apiErr.Status >= 500 || apiErr.Status == http.StatusTooManyRequests || apiErr.Status == http.StatusRequestTimeout
	}
	return true
}

// client is the outbound half of the bridge: every call the runner, the
// heartbeat and the uploader make to the portal's worker API goes through it.
type client struct {
	baseURL string
	token   string
	http    *http.Client
}

func newClient(baseURL, token string) *client {
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		// No client-wide timeout: a demo download and a 32 MiB part need very
		// different budgets, so each call carries its own deadline.
		http: &http.Client{},
	}
}

// Block is why the worker is not claiming work.
type Block struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

type heartbeatJob struct {
	ID         string  `json:"id"`
	Attempt    int     `json:"attempt"`
	Phase      string  `json:"phase"`
	Stage      *string `json:"stage"`
	Percent    int     `json:"percent"`
	Detail     string  `json:"detail"`
	LocalJobID string  `json:"localJobId"`
}

type heartbeatRequest struct {
	State   string         `json:"state"`
	Blocked *Block         `json:"blocked"`
	Health  Health         `json:"health"`
	Jobs    []heartbeatJob `json:"jobs"`
}

type heartbeatJobStatus struct {
	ID              string `json:"id"`
	Lease           string `json:"lease"`
	LeaseExpiresAt  int64  `json:"leaseExpiresAt"`
	CancelRequested bool   `json:"cancelRequested"`
}

type heartbeatResponse struct {
	Now         int64                `json:"now"`
	Paused      bool                 `json:"paused"`
	PauseReason *string              `json:"pauseReason"`
	Jobs        []heartbeatJobStatus `json:"jobs"`
}

func (c *client) heartbeat(ctx context.Context, body heartbeatRequest) (heartbeatResponse, error) {
	if body.Jobs == nil {
		body.Jobs = []heartbeatJob{}
	}
	var out heartbeatResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/worker/heartbeat", body, &out)
	return out, err
}

type demoRef struct {
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
	FileName  string `json:"fileName"`
}

// workerJob is one claimed job, as the claim returns it.
type workerJob struct {
	ID                string          `json:"id"`
	Kind              string          `json:"kind"`
	Attempt           int             `json:"attempt"`
	MaxRuntimeSeconds int             `json:"maxRuntimeSeconds"`
	LeaseExpiresAt    int64           `json:"leaseExpiresAt"`
	Demo              demoRef         `json:"demo"`
	SubmitterLabel    string          `json:"submitterLabel"`
	Title             string          `json:"title"`
	Spec              json.RawMessage `json:"spec"`
}

type claimRequest struct {
	Kinds         []string `json:"kinds"`
	DiskFreeBytes uint64   `json:"diskFreeBytes"`
}

type claimResponse struct {
	Claimed bool       `json:"claimed"`
	Reason  string     `json:"reason"`
	Job     *workerJob `json:"job"`
}

// claim asks for the next job. A nil job with a reason means there is
// nothing to do right now ("empty", "paused", "busy", "upload_backlog", ...);
// whatever the reason says, the worker waits and asks again.
func (c *client) claim(ctx context.Context, body claimRequest) (*workerJob, string, error) {
	var out claimResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/worker/claim", body, &out); err != nil {
		return nil, "", err
	}
	if !out.Claimed {
		return nil, out.Reason, nil
	}
	if out.Job == nil || out.Job.ID == "" {
		return nil, "", errors.New("portal reported a claim without a job")
	}
	return out.Job, "", nil
}

// downloadDemo fetches the job's demo into dest, resuming whatever an earlier
// attempt left there, and returns only once the whole file matches the
// claimed size and sha256. A mismatch removes the file and reports
// errDemoCorrupt.
func (c *client) downloadDemo(ctx context.Context, job jobRef, dest string, want demoRef) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return fmt.Errorf("create incoming directory: %w", err)
	}
	offset := int64(0)
	if info, err := os.Stat(dest); err == nil {
		offset = info.Size()
	}
	if want.SizeBytes > 0 && offset > want.SizeBytes {
		offset = 0
	}
	if want.SizeBytes <= 0 || offset < want.SizeBytes {
		if err := c.fetchDemo(ctx, job, dest, offset); err != nil {
			return err
		}
	}
	return verifyDemoFile(dest, want)
}

func (c *client) fetchDemo(ctx context.Context, job jobRef, dest string, offset int64) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+job.path("/demo"), nil)
	if err != nil {
		return err
	}
	c.authorize(req)
	fence(req, job.Attempt)
	if offset > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	idle := time.AfterFunc(demoIdleTimeout, func() {
		cancel(fmt.Errorf("no demo bytes for %s", demoIdleTimeout))
	})
	defer idle.Stop()
	resp, err := c.http.Do(req)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil && !errors.Is(cause, context.Canceled) {
			return fmt.Errorf("download demo: %w", cause)
		}
		return err
	}
	defer resp.Body.Close()

	flags := os.O_WRONLY | os.O_CREATE
	switch {
	case resp.StatusCode == http.StatusOK:
		flags |= os.O_TRUNC
	case resp.StatusCode == http.StatusPartialContent && offset > 0 && strings.HasPrefix(resp.Header.Get("Content-Range"), "bytes "+strconv.FormatInt(offset, 10)+"-"):
		flags |= os.O_APPEND
	case resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		// The partial file cannot be continued; the next attempt starts over.
		if removeErr := os.Remove(dest); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("discard unusable partial demo: %w", removeErr)
		}
		return fmt.Errorf("portal answered %d to a resume from byte %d", resp.StatusCode, offset)
	default:
		return responseError(resp)
	}
	file, err := os.OpenFile(dest, flags, 0o600) //nolint:gosec // dest is built from the data dir and a validated job id
	if err != nil {
		return fmt.Errorf("open incoming demo: %w", err)
	}
	_, copyErr := io.Copy(file, &activityReader{reader: resp.Body, onRead: func() { idle.Reset(demoIdleTimeout) }})
	if closeErr := file.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		if cause := context.Cause(ctx); cause != nil && !errors.Is(cause, context.Canceled) {
			return fmt.Errorf("download demo: %w", cause)
		}
		return fmt.Errorf("download demo: %w", copyErr)
	}
	return nil
}

type activityReader struct {
	reader io.Reader
	onRead func()
}

func (r *activityReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.onRead()
	}
	return n, err
}

func verifyDemoFile(path string, want demoRef) error {
	sum, size, err := hashFile(path)
	if err != nil {
		return fmt.Errorf("hash downloaded demo: %w", err)
	}
	if (want.SizeBytes > 0 && size != want.SizeBytes) || !strings.EqualFold(sum, want.SHA256) {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("%w; remove it: %v", errDemoCorrupt, removeErr)
		}
		return fmt.Errorf("%w: got %d bytes with sha256 %s", errDemoCorrupt, size, sum)
	}
	return nil
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

type phaseRequest struct {
	Phase          string `json:"phase"`
	MachineSeconds int    `json:"machineSeconds"`
	LocalJobID     string `json:"localJobId"`
}

// enterUploading moves the job from running to uploading, which frees the
// portal's capture slot for the next claim.
func (c *client) enterUploading(ctx context.Context, job jobRef, machineSeconds int, localJobID string) error {
	body := phaseRequest{Phase: phaseUploading, MachineSeconds: machineSeconds, LocalJobID: localJobID}
	return c.jobJSON(ctx, job, "/phase", body, nil)
}

type artifactInit struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Variant   string `json:"variant"`
	SizeBytes int64  `json:"sizeBytes"`
	SHA256    string `json:"sha256"`
}

type artifactUpload struct {
	ArtifactID    string `json:"artifactId"`
	PartSize      int64  `json:"partSize"`
	PartCount     int    `json:"partCount"`
	ReceivedParts []int  `json:"receivedParts"`
}

// initArtifact registers one result file. It is idempotent: a repeat with
// the same size and sha256 returns the parts the portal already holds.
func (c *client) initArtifact(ctx context.Context, job jobRef, body artifactInit) (artifactUpload, error) {
	var out artifactUpload
	if err := c.jobJSON(ctx, job, "/artifacts", body, &out); err != nil {
		return artifactUpload{}, err
	}
	if out.ArtifactID == "" || out.PartSize <= 0 {
		return artifactUpload{}, errors.New("portal returned an artifact without an id or a part size")
	}
	return out, nil
}

type partRef struct {
	Job        jobRef
	ArtifactID string
	Number     int
}

// putPart sends one part (1-based) and returns the parts the portal holds.
func (c *client) putPart(ctx context.Context, part partRef, body io.Reader, size int64) ([]int, error) {
	ctx, cancel := context.WithTimeout(ctx, partCallTimeout)
	defer cancel()
	target := c.baseURL + part.Job.path(fmt.Sprintf("/artifacts/%s/parts/%d", url.PathEscape(part.ArtifactID), part.Number))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target, body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	c.authorize(req)
	fence(req, part.Job.Attempt)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, responseError(resp)
	}
	var out struct {
		ReceivedParts []int `json:"receivedParts"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return out.ReceivedParts, nil
}

func (c *client) completeArtifact(ctx context.Context, job jobRef, artifactID string) error {
	return c.jobJSON(ctx, job, "/artifacts/"+url.PathEscape(artifactID)+"/complete", struct{}{}, nil)
}

func (c *client) completeJob(ctx context.Context, job jobRef) error {
	return c.jobJSON(ctx, job, "/complete", struct{}{}, nil)
}

type failRequest struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	Detail         string `json:"detail"`
	MachineSeconds int    `json:"machineSeconds"`
}

type failResponse struct {
	Outcome      string `json:"outcome"`
	WorkerPaused bool   `json:"workerPaused"`
}

// fail reports the end of an attempt. The portal decides whether the job is
// requeued, failed or canceled; the worker only names what happened.
func (c *client) fail(ctx context.Context, job jobRef, body failRequest) (failResponse, error) {
	body.Message = truncateRunes(body.Message, maxFailureMessageRunes)
	body.Detail = truncateRunes(body.Detail, maxFailureDetailRunes)
	var out failResponse
	err := c.jobJSON(ctx, job, "/fail", body, &out)
	return out, err
}

func truncateRunes(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit])
}

func (c *client) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
}

// fence marks a request as the work of one attempt of its job.
func fence(req *http.Request, attempt int) {
	req.Header.Set(attemptHeader, strconv.Itoa(attempt))
}

// jsonCall is one JSON request; an attempt above zero adds the fencing header.
type jsonCall struct {
	method  string
	path    string
	attempt int
	payload any
	out     any
}

// jobJSON posts to a route under /api/worker/jobs/:id/ as one attempt.
func (c *client) jobJSON(ctx context.Context, job jobRef, suffix string, payload, out any) error {
	return c.sendJSON(ctx, jsonCall{method: http.MethodPost, path: job.path(suffix), attempt: job.Attempt, payload: payload, out: out})
}

func (c *client) doJSON(ctx context.Context, method, path string, payload, out any) error {
	return c.sendJSON(ctx, jsonCall{method: method, path: path, payload: payload, out: out})
}

func (c *client) sendJSON(ctx context.Context, call jsonCall) error {
	method, path, payload, out := call.method, call.path, call.payload, call.out
	ctx, cancel := context.WithTimeout(ctx, jsonCallTimeout)
	defer cancel()
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.authorize(req)
	if call.attempt > 0 {
		fence(req, call.attempt)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return responseError(resp)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// responseError maps a non-2xx answer onto the two sentinels every caller
// must tell apart, or an apiError carrying the portal's code.
func responseError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var body struct {
		Error   string `json:"error"`
		Code    string `json:"code"`
		Missing []int  `json:"missing"`
	}
	_ = json.Unmarshal(raw, &body)
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return errUnauthorized
	case resp.StatusCode == http.StatusConflict && (body.Code == "lease_lost" || body.Error == "lease_lost"):
		return errLeaseLost
	}
	message := body.Error
	if message == "" {
		message = strings.TrimSpace(truncateRunes(string(raw), 300))
	}
	return &apiError{Status: resp.StatusCode, Code: body.Code, Message: message, Missing: body.Missing}
}
