package cloudbridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

const (
	testToken   = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testVariant = "viral-60-clean"
	testTarget  = "76561198000000001"
)

// testTiming makes every wait a few milliseconds, so a whole job fits in a
// test without changing what the worker does.
func testTiming() Timing {
	return Timing{
		Heartbeat:            5 * time.Millisecond,
		UnauthorizedInterval: 5 * time.Millisecond,
		Claim:                3 * time.Millisecond,
		Poll:                 2 * time.Millisecond,
		ParseTimeout:         2 * time.Second,
		RenderStartTimeout:   150 * time.Millisecond,
		SettleTimeout:        2 * time.Second,
		RetryMin:             2 * time.Millisecond,
		RetryMax:             8 * time.Millisecond,
		UploadRetryMin:       2 * time.Millisecond,
		UploadRetryMax:       8 * time.Millisecond,
		UploadDeadline:       time.Minute,
		ProgressSave:         time.Millisecond,
	}
}

func eventually(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// fakeVideo is a body the portal's mp4 check accepts.
func fakeVideo(seed string, size int) []byte {
	video := append([]byte{0, 0, 0, 24}, []byte("ftypisom")...)
	for len(video) < size {
		video = append(video, seed...)
	}
	return video[:max(size, 12)]
}

// ---- portal ----

type portalArtifact struct {
	id       string
	init     artifactInit
	parts    map[int][]byte
	puts     map[int]int
	complete bool
}

type portalJob struct {
	job             *workerJob
	status          string
	cancelRequested bool
	leaseLost       bool
	phases          []phaseRequest
	fails           []failRequest
	lateFails       int
	artifacts       []*portalArtifact
}

// fakePortal implements the worker API of the portal closely enough to hold
// the worker to its side of the contract.
type fakePortal struct {
	t        *testing.T
	server   *httptest.Server
	partSize int64

	mu           sync.Mutex
	demo         []byte
	queue        []*workerJob
	jobs         map[string]*portalJob
	heartbeats   []heartbeatRequest
	claims       int
	paused       bool
	unauthorized bool
	// outages makes the next n calls of a route answer 503. The key is the
	// route name used in handle, such as "fail" or "part".
	outages map[string]int
	// partOutages does the same for single part numbers.
	partOutages map[int]int
	calls       map[string]int
	// demoRanges is the Range header of every demo download, "" for none.
	demoRanges []string
	// claimRefusal, when set, is the reason every claim is refused with.
	claimRefusal string
	// heartbeatStatus, when set, is the status every heartbeat is answered with.
	heartbeatStatus int
	// unfenced counts job calls without a valid attempt header, and stale
	// counts those that carried the attempt of an older claim.
	unfenced int
	stale    int
}

func newFakePortal(t *testing.T) *fakePortal {
	t.Helper()
	p := &fakePortal{
		t:           t,
		partSize:    16,
		demo:        append([]byte("PBDEMS2\x00"), bytes.Repeat([]byte("demo-bytes"), 40)...),
		jobs:        map[string]*portalJob{},
		outages:     map[string]int{},
		partOutages: map[int]int{},
		calls:       map[string]int{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/worker/heartbeat", p.handle("heartbeat", p.heartbeat))
	mux.HandleFunc("POST /api/worker/claim", p.handle("claim", p.claim))
	mux.HandleFunc("GET /api/worker/jobs/{id}/demo", p.handle("demo", p.serveDemo))
	mux.HandleFunc("POST /api/worker/jobs/{id}/phase", p.handle("phase", p.phase))
	mux.HandleFunc("POST /api/worker/jobs/{id}/artifacts", p.handle("init", p.initArtifact))
	mux.HandleFunc("PUT /api/worker/jobs/{id}/artifacts/{artifact}/parts/{n}", p.handle("part", p.putPart))
	mux.HandleFunc("POST /api/worker/jobs/{id}/artifacts/{artifact}/complete", p.handle("completeArtifact", p.completeArtifact))
	mux.HandleFunc("POST /api/worker/jobs/{id}/complete", p.handle("complete", p.completeJob))
	mux.HandleFunc("POST /api/worker/jobs/{id}/fail", p.handle("fail", p.fail))
	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	return p
}

type portalHandler func(w http.ResponseWriter, r *http.Request) (status int, body any)

// handle applies the checks every worker route shares, with the lock held.
func (p *fakePortal) handle(route string, next portalHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.calls[route]++
		status, body := http.StatusUnauthorized, any(map[string]string{"error": "unauthorized"})
		switch {
		case p.unauthorized || r.Header.Get("Authorization") != "Bearer "+testToken:
		case p.outages[route] > 0:
			p.outages[route]--
			status, body = http.StatusServiceUnavailable, map[string]string{"error": "portal restarting"}
		case r.PathValue("id") != "" && !p.fenced(r):
			p.unfenced++
			status, body = http.StatusBadRequest, map[string]string{"error": "missing attempt", "code": "invalid_request"}
		case r.PathValue("id") != "" && p.staleAttempt(r):
			p.stale++
			status, body = leaseLostBody()
		default:
			status, body = next(w, r)
		}
		if status == 0 {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}

// fenced reports whether a job call names the attempt it works for.
func (p *fakePortal) fenced(r *http.Request) bool {
	attempt, err := strconv.Atoi(r.Header.Get(attemptHeader))
	return err == nil && attempt > 0
}

// staleAttempt reports a call from an attempt the portal no longer leases.
func (p *fakePortal) staleAttempt(r *http.Request) bool {
	job, ok := p.jobs[r.PathValue("id")]
	if !ok {
		return false
	}
	attempt, _ := strconv.Atoi(r.Header.Get(attemptHeader))
	return attempt != job.job.Attempt
}

func leaseLostBody() (int, any) {
	return http.StatusConflict, map[string]string{"error": "lease_lost", "code": "lease_lost"}
}

// leased returns the job when it is still this worker's in one of statuses.
func (p *fakePortal) leased(r *http.Request, statuses ...string) *portalJob {
	job, ok := p.jobs[r.PathValue("id")]
	if !ok || job.leaseLost || !slices.Contains(statuses, job.status) {
		return nil
	}
	return job
}

func (p *fakePortal) heartbeat(_ http.ResponseWriter, r *http.Request) (int, any) {
	var body heartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, map[string]string{"error": err.Error()}
	}
	p.heartbeats = append(p.heartbeats, body)
	if p.heartbeatStatus != 0 {
		return p.heartbeatStatus, map[string]string{"error": "invalid heartbeat", "code": "invalid_request"}
	}
	statuses := []heartbeatJobStatus{}
	for _, reported := range body.Jobs {
		status := heartbeatJobStatus{ID: reported.ID, Lease: "ok"}
		job, ok := p.jobs[reported.ID]
		switch {
		case !ok || job.leaseLost || (job.status != phaseRunning && job.status != phaseUploading):
			status.Lease = leaseLost
		case reported.Attempt != job.job.Attempt:
			status.Lease = leaseLost
		default:
			status.CancelRequested = job.cancelRequested
		}
		statuses = append(statuses, status)
	}
	return http.StatusOK, heartbeatResponse{Now: time.Now().UnixMilli(), Paused: p.paused, Jobs: statuses}
}

func (p *fakePortal) claim(_ http.ResponseWriter, _ *http.Request) (int, any) {
	p.claims++
	if p.paused {
		return http.StatusOK, claimResponse{Reason: "paused"}
	}
	if p.claimRefusal != "" {
		return http.StatusOK, claimResponse{Reason: p.claimRefusal}
	}
	for _, job := range p.jobs {
		if job.status == phaseRunning && !job.leaseLost {
			return http.StatusOK, claimResponse{Reason: "busy"}
		}
	}
	if len(p.queue) == 0 {
		return http.StatusOK, claimResponse{Reason: "empty"}
	}
	next := p.queue[0]
	p.queue = p.queue[1:]
	next.Attempt++
	p.jobs[next.ID] = &portalJob{job: next, status: phaseRunning}
	return http.StatusOK, claimResponse{Claimed: true, Job: next}
}

func (p *fakePortal) serveDemo(w http.ResponseWriter, r *http.Request) (int, any) {
	if p.leased(r, phaseRunning) == nil {
		return leaseLostBody()
	}
	offset := 0
	p.demoRanges = append(p.demoRanges, r.Header.Get("Range"))
	if header := r.Header.Get("Range"); header != "" {
		parsed, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(header, "bytes="), "-"))
		if err != nil || parsed >= len(p.demo) {
			return http.StatusRequestedRangeNotSatisfiable, map[string]string{"error": "range"}
		}
		offset = parsed
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, len(p.demo)-1, len(p.demo)))
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Accept-Ranges", "bytes")
	if offset > 0 {
		w.WriteHeader(http.StatusPartialContent)
	}
	_, _ = w.Write(p.demo[offset:])
	return 0, nil
}

func (p *fakePortal) phase(_ http.ResponseWriter, r *http.Request) (int, any) {
	job := p.leased(r, phaseRunning)
	if job == nil {
		return leaseLostBody()
	}
	var body phaseRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Phase != phaseUploading {
		return http.StatusBadRequest, map[string]string{"error": "bad phase"}
	}
	job.phases = append(job.phases, body)
	job.status = phaseUploading
	return http.StatusOK, map[string]bool{"ok": true}
}

func (p *fakePortal) initArtifact(_ http.ResponseWriter, r *http.Request) (int, any) {
	job := p.leased(r, phaseUploading)
	if job == nil {
		return leaseLostBody()
	}
	var body artifactInit
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, map[string]string{"error": err.Error()}
	}
	var artifact *portalArtifact
	for _, existing := range job.artifacts {
		if existing.init.Variant == body.Variant && existing.init.Name == body.Name {
			artifact = existing
		}
	}
	switch {
	case artifact == nil:
		artifact = &portalArtifact{id: uuid.NewString(), init: body, parts: map[int][]byte{}, puts: map[int]int{}}
		job.artifacts = append(job.artifacts, artifact)
	case artifact.init != body:
		artifact.init, artifact.parts, artifact.complete = body, map[int][]byte{}, false
	}
	return http.StatusOK, artifactUpload{
		ArtifactID:    artifact.id,
		PartSize:      p.partSize,
		PartCount:     int((body.SizeBytes + p.partSize - 1) / p.partSize),
		ReceivedParts: artifact.received(),
	}
}

func (a *portalArtifact) received() []int {
	numbers := make([]int, 0, len(a.parts))
	for number := range a.parts {
		numbers = append(numbers, number)
	}
	slices.Sort(numbers)
	return numbers
}

func (p *fakePortal) artifact(r *http.Request) *portalArtifact {
	job := p.leased(r, phaseUploading)
	if job == nil {
		return nil
	}
	for _, artifact := range job.artifacts {
		if artifact.id == r.PathValue("artifact") {
			return artifact
		}
	}
	return nil
}

func (p *fakePortal) putPart(_ http.ResponseWriter, r *http.Request) (int, any) {
	artifact := p.artifact(r)
	if artifact == nil {
		return leaseLostBody()
	}
	number, err := strconv.Atoi(r.PathValue("n"))
	data, readErr := io.ReadAll(r.Body)
	if err != nil || readErr != nil {
		return http.StatusBadRequest, map[string]string{"error": "bad part"}
	}
	want := min(p.partSize, artifact.init.SizeBytes-int64(number-1)*p.partSize)
	if number < 1 || want <= 0 || r.ContentLength != want || int64(len(data)) != want {
		return http.StatusBadRequest, map[string]string{"error": "bad part length", "code": "bad_part_length"}
	}
	if p.partOutages[number] > 0 {
		p.partOutages[number]--
		return http.StatusServiceUnavailable, map[string]string{"error": "portal restarting"}
	}
	artifact.parts[number] = data
	artifact.puts[number]++
	return http.StatusOK, map[string]any{"ok": true, "receivedParts": artifact.received()}
}

func (p *fakePortal) completeArtifact(_ http.ResponseWriter, r *http.Request) (int, any) {
	artifact := p.artifact(r)
	if artifact == nil {
		return leaseLostBody()
	}
	count := int((artifact.init.SizeBytes + p.partSize - 1) / p.partSize)
	var whole []byte
	missing := []int{}
	for number := 1; number <= count; number++ {
		part, ok := artifact.parts[number]
		if !ok {
			missing = append(missing, number)
		}
		whole = append(whole, part...)
	}
	switch {
	case len(missing) > 0:
		return http.StatusConflict, map[string]any{"error": "parts missing", "code": "parts_missing", "missing": missing}
	case sha256Hex(whole) != artifact.init.SHA256:
		artifact.parts = map[int][]byte{}
		return http.StatusUnprocessableEntity, map[string]string{"error": "sha256 mismatch", "code": "sha256_mismatch"}
	case len(whole) < 8 || string(whole[4:8]) != "ftyp":
		artifact.parts = map[int][]byte{}
		return http.StatusUnprocessableEntity, map[string]string{"error": "not a video", "code": "not_a_video"}
	}
	artifact.complete = true
	return http.StatusOK, map[string]bool{"ok": true}
}

func (p *fakePortal) completeJob(_ http.ResponseWriter, r *http.Request) (int, any) {
	job := p.leased(r, phaseUploading)
	if job == nil {
		return leaseLostBody()
	}
	ready := slices.ContainsFunc(job.artifacts, func(artifact *portalArtifact) bool { return artifact.complete })
	if !ready {
		return http.StatusConflict, map[string]string{"error": "no artifacts", "code": "no_artifacts"}
	}
	job.status = "done"
	return http.StatusOK, map[string]bool{"ok": true}
}

func (p *fakePortal) fail(_ http.ResponseWriter, r *http.Request) (int, any) {
	job := p.leased(r, phaseRunning, phaseUploading)
	if job == nil {
		if known, ok := p.jobs[r.PathValue("id")]; ok {
			known.lateFails++
		}
		return leaseLostBody()
	}
	var body failRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, map[string]string{"error": err.Error()}
	}
	job.fails = append(job.fails, body)
	job.status = "failed"
	return http.StatusOK, failResponse{Outcome: "failed"}
}

// with runs inspect with the portal's lock held.
func (p *fakePortal) with(inspect func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	inspect()
}

func (p *fakePortal) jobStatus(id string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if job, ok := p.jobs[id]; ok {
		return job.status
	}
	return ""
}

// ---- local pipeline ----

type fakeLocalJob struct {
	id            string
	cloudID       string
	view          LocalJob
	plan          *killplan.Plan
	variant       *LocalVariant
	generateBody  json.RawMessage
	generateCalls int
	cancelCalls   int
	deleted       bool
	// working is how many more CancelJob calls find work still in flight
	// after the job was told to stop; below zero means it never stops.
	working int
	// deletedWhileWorking records a delete that did not wait for that work.
	deletedWhileWorking bool
	// afterGenerate scripts capture and render: once generate was accepted,
	// every status poll runs the first step, and drops it when it reports
	// that it is done.
	afterGenerate []func(*fakeLocalJob) bool
}

// then is a script step that applies once.
func then(apply func(*fakeLocalJob)) func(*fakeLocalJob) bool {
	return func(job *fakeLocalJob) bool {
		apply(job)
		return true
	}
}

// fakeLocal stands in for the orchestrator's pipeline: it is the Admitter,
// the LocalPipeline and the JobCanceler of the worker under test.
type fakeLocal struct {
	t  *testing.T
	mu sync.Mutex

	jobs       map[string]*fakeLocalJob
	admissions []DemoAdmission
	demos      [][]byte
	admitErr   error
	// onAdmit prepares a freshly admitted job; the default parses at once.
	onAdmit func(*fakeLocalJob)
	// generate answers a generate call; the default accepts it.
	generate func(*fakeLocalJob) GenerateResult
}

func newFakeLocal(t *testing.T) *fakeLocal {
	return &fakeLocal{t: t, jobs: map[string]*fakeLocalJob{}}
}

func (f *fakeLocal) AdmitCloudDemo(_ context.Context, in DemoAdmission) (string, error) {
	demo, err := io.ReadAll(in.Demo)
	if err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	in.Demo = nil
	f.admissions = append(f.admissions, in)
	f.demos = append(f.demos, demo)
	if f.admitErr != nil {
		return "", f.admitErr
	}
	job := &fakeLocalJob{id: uuid.NewString(), cloudID: in.CloudRequestID, view: LocalJob{Status: localJobParsed}, plan: testPlan()}
	if f.onAdmit != nil {
		f.onAdmit(job)
	}
	f.jobs[job.id] = job
	return job.id, nil
}

func (f *fakeLocal) Generate(_ context.Context, jobID string, body json.RawMessage) (GenerateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job := f.jobs[jobID]
	job.generateCalls++
	job.generateBody = slices.Clone(body)
	if f.generate != nil {
		return f.generate(job), nil
	}
	job.view = LocalJob{Status: localJobRecording}
	return GenerateResult{Status: http.StatusAccepted, Variant: testVariant}, nil
}

func (f *fakeLocal) JobView(_ context.Context, jobID string) (LocalJob, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job, ok := f.jobs[jobID]
	if !ok || job.deleted {
		return LocalJob{}, false, nil
	}
	if job.generateCalls > 0 && len(job.afterGenerate) > 0 {
		if job.afterGenerate[0](job) {
			job.afterGenerate = job.afterGenerate[1:]
		}
	}
	return job.view, true, nil
}

func (f *fakeLocal) KillPlan(_ context.Context, jobID string) (*killplan.Plan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.jobs[jobID].plan, nil
}

func (f *fakeLocal) VariantView(_ context.Context, jobID, variant string) (LocalVariant, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job := f.jobs[jobID]
	if job == nil || job.variant == nil || variant != testVariant {
		return LocalVariant{}, false, nil
	}
	return *job.variant, true, nil
}

func (f *fakeLocal) DeleteJob(_ context.Context, jobID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	job, ok := f.jobs[jobID]
	if !ok {
		return nil
	}
	// Like the real pipeline, a job that is recording cannot be deleted.
	if job.view.Status == localJobRecording {
		return ErrLocalJobBusy
	}
	if job.working != 0 {
		job.deletedWhileWorking = true
	}
	job.deleted = true
	return nil
}

func (f *fakeLocal) CloudJobs(context.Context) ([]LocalJobRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	refs := []LocalJobRef{}
	for _, job := range f.jobs {
		if !job.deleted && job.cloudID != "" {
			refs = append(refs, LocalJobRef{ID: job.id, CloudRequestID: job.cloudID})
		}
	}
	return refs, nil
}

// CancelJob behaves like the inline queue: the capture stops and its worker
// marks the job failed.
func (f *fakeLocal) CancelJob(jobID uuid.UUID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	job, ok := f.jobs[jobID.String()]
	if !ok {
		return false
	}
	job.cancelCalls++
	if job.view.Status == localJobRecording {
		job.afterGenerate = nil
		job.view = LocalJob{Status: localJobFailed, FailureReason: "zv-recorder failed: context canceled"}
		return true
	}
	// A canceled task keeps counting as in flight until its process tree is
	// gone, which is what a caller must wait for before deleting.
	if job.working != 0 {
		if job.working > 0 {
			job.working--
		}
		return true
	}
	return false
}

func (f *fakeLocal) only() *fakeLocalJob {
	f.t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.jobs) != 1 {
		f.t.Fatalf("local jobs = %d, want exactly 1", len(f.jobs))
	}
	for _, job := range f.jobs {
		copied := *job
		return &copied
	}
	return nil
}

func (f *fakeLocal) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.jobs)
}

// ---- machine ----

type fakeMachine struct {
	mu     sync.Mutex
	health Health
	cs2    bool
}

func readyMachine() *fakeMachine {
	return &fakeMachine{health: Health{
		Hostname:        "test-worker",
		StudioVersion:   "5.4.4",
		CS2PatchVersion: "1.41.8.5",
		HLAEVersion:     "2.192.6",
		SteamRunning:    true,
		RecordEnabled:   true,
		DiskFreeBytes:   100 << 30,
		DiskTotalBytes:  500 << 30,
		Kinds:           []string{kindShort},
	}}
}

func (m *fakeMachine) Health(context.Context, bool) Health {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.health
}

func (m *fakeMachine) CS2Running(context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cs2, nil
}

// ---- harness ----

type harness struct {
	t       *testing.T
	portal  *fakePortal
	local   *fakeLocal
	machine *fakeMachine
	dataDir string
	files   *storage.Local
	state   *State
	timing  Timing
	stop    func()
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dataDir := t.TempDir()
	files, err := storage.NewLocal(dataDir)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	h := &harness{
		t:       t,
		portal:  newFakePortal(t),
		local:   newFakeLocal(t),
		machine: readyMachine(),
		dataDir: dataDir,
		files:   files,
		timing:  testTiming(),
		stop:    func() {},
	}
	h.loadState()
	return h
}

func (h *harness) statePath() string {
	return filepath.Join(h.dataDir, "cloudbridge", "state.json")
}

func (h *harness) loadState() {
	h.t.Helper()
	state, err := LoadState(h.statePath())
	if err != nil {
		h.t.Fatalf("LoadState: %v", err)
	}
	h.state = state
}

// start runs a worker until the test ends or stop is called. Calling it
// again after stop is a restart of the process: the state file is reread.
func (h *harness) start() {
	h.t.Helper()
	h.loadState()
	worker := NewWorker(Config{
		BaseURL:      h.portal.server.URL,
		Token:        testToken,
		DataDir:      h.dataDir,
		State:        h.state,
		Admitter:     h.local,
		Local:        h.local,
		Canceler:     h.local,
		Files:        h.files,
		Machine:      h.machine,
		Kinds:        []string{kindShort},
		MinFreeBytes: DefaultMinFreeBytes,
		Timing:       h.timing,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()
	var once sync.Once
	h.stop = func() {
		once.Do(func() {
			cancel()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				h.t.Error("worker did not stop")
			}
		})
	}
	h.t.Cleanup(h.stop)
}

func testPlan() *killplan.Plan {
	return &killplan.Plan{
		Demo: killplan.Demo{Tickrate: 64},
		Segments: []killplan.Segment{
			{ID: "seg-001", TickStart: 1000, TickEnd: 1640},
			{ID: "seg-003", TickStart: 51234, TickEnd: 52010},
		},
	}
}

const testGenerate = `{"preset":"viral-60-clean","music":{"key":"track-1","volume":0.4},"segment_ids":["seg-003"],"edit":{"format":"short_9x16","intro":true}}`

func testSpec() json.RawMessage {
	return json.RawMessage(`{"version":1,"targetSteamId":"` + testTarget + `","rules":{"weapons":["awp"],"min_kills_in_window":2,"window_seconds":10,"pre_roll_seconds":3,"post_roll_seconds":4,"min_round":1},` +
		`"capture":{"tickrate":64,"windows":[{"id":"seg-003","tickStart":51234,"tickEnd":52010}]},"generate":` + testGenerate + `,"client":{"studioVersion":"5.4.4"}}`)
}

// queueJob makes one short claimable and returns its id.
func (h *harness) queueJob(mutate func(*workerJob)) string {
	h.t.Helper()
	job := &workerJob{
		ID:                uuid.NewString(),
		Kind:              kindShort,
		MaxRuntimeSeconds: 1800,
		SubmitterLabel:    "Luis",
		Title:             "R3 4k",
		Spec:              testSpec(),
	}
	h.portal.with(func() {
		job.Demo = demoRef{SHA256: sha256Hex(h.portal.demo), SizeBytes: int64(len(h.portal.demo)), FileName: "match.dem"}
		if mutate != nil {
			mutate(job)
		}
		h.portal.queue = append(h.portal.queue, job)
	})
	return job.ID
}

// writeRender puts a finished render of the variant into local storage, the
// way the render worker leaves it: state, pack manifest and videos.
func (h *harness) writeRender(localJobID string, videos map[string][]byte) {
	h.t.Helper()
	jobID := uuid.MustParse(localJobID)
	put := func(key string, data []byte) {
		if err := h.files.Put(key, bytes.NewReader(data)); err != nil {
			h.t.Fatalf("put %s: %v", key, err)
		}
	}
	manifestKey, err := artifacts.RenderVariantPackManifestKey(jobID, testVariant)
	if err != nil {
		h.t.Fatalf("manifest key: %v", err)
	}
	names := make([]string, 0, len(videos))
	for name := range videos {
		names = append(names, name)
	}
	slices.Sort(names)
	manifest := editor.PackManifest{Preset: testVariant}
	for _, name := range names {
		key, err := artifacts.RenderVariantVideoKey(jobID, testVariant, name)
		if err != nil {
			h.t.Fatalf("video key: %v", err)
		}
		put(key, videos[name])
		manifest.Items = append(manifest.Items, editor.PublishItem{SegmentID: name, Video: key})
	}
	manifestJSON, _ := json.Marshal(manifest)
	put(manifestKey, manifestJSON)
	stateKey, err := artifacts.RenderVariantStatusKey(jobID, testVariant)
	if err != nil {
		h.t.Fatalf("state key: %v", err)
	}
	stateJSON, _ := json.Marshal(renderplan.RenderVariantState{
		JobID: jobID, Variant: testVariant, Status: renderplan.RenderVariantStatusReady, PackManifestKey: manifestKey,
	})
	put(stateKey, stateJSON)
}

// renderOnCapture scripts a capture and a render that both succeed, leaving
// the given videos behind.
func (h *harness) renderOnCapture(videos map[string][]byte) {
	h.local.onAdmit = func(job *fakeLocalJob) {
		h.writeRender(job.id, videos)
		job.afterGenerate = []func(*fakeLocalJob) bool{
			then(func(job *fakeLocalJob) {
				job.view = LocalJob{Status: localJobRecording, Progress: &LocalProgress{Done: 1, Total: 2, Percent: 50}}
			}),
			// Each stage lasts until a heartbeat has shown it to the portal.
			func(job *fakeLocalJob) bool {
				if !h.sawStage(job.cloudID, stageCapturing) {
					return false
				}
				job.view = LocalJob{Status: "recorded", Progress: &LocalProgress{Stage: "encode", Percent: 40}}
				job.variant = &LocalVariant{Status: localVariantRendering}
				return true
			},
			func(job *fakeLocalJob) bool {
				if !h.sawStage(job.cloudID, stageRendering) {
					return false
				}
				job.variant = &LocalVariant{Status: localVariantReady}
				return true
			},
		}
	}
}

// sawStage reports whether a heartbeat told the portal the job is in stage.
func (h *harness) sawStage(cloudID, stage string) bool {
	h.portal.mu.Lock()
	defer h.portal.mu.Unlock()
	for _, beat := range h.portal.heartbeats {
		for _, reported := range beat.Jobs {
			if reported.ID == cloudID && reported.Stage != nil && *reported.Stage == stage {
				return true
			}
		}
	}
	return false
}

// tracked is what the running worker holds right now.
func (h *harness) tracked() []TrackedRequest {
	return h.state.All()
}

// persisted is what a restart would find in the state file. Only read it
// while the worker is stopped: on Windows a reader blocks the replace.
func (h *harness) persisted() []TrackedRequest {
	h.t.Helper()
	state, err := LoadState(h.statePath())
	if err != nil {
		h.t.Fatalf("LoadState: %v", err)
	}
	return state.All()
}

func (h *harness) incomingDemos() []string {
	entries, err := os.ReadDir(filepath.Join(h.dataDir, "cloudbridge", "incoming"))
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// settled waits until the worker holds nothing of the job any more.
func (h *harness) settled() {
	h.t.Helper()
	eventually(h.t, "the worker to forget the job", func() bool {
		return len(h.tracked()) == 0
	})
}
