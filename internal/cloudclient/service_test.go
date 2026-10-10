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
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/storage"
)

const (
	testDeviceToken = "chd_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testTarget      = "76561198000000001"
)

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

// ---- fake portal (user API) ----

type userPortal struct {
	t      *testing.T
	server *httptest.Server

	mu          sync.Mutex
	me          Me
	jobs        []*PortalJob
	created     []CreateJobRequest
	demoUpload  string
	demos       map[string][]byte
	artifacts   map[string][]byte
	received    []string
	cancels     []string
	linkStatus  string
	deviceNames []string
	revoked     bool
	deleted     bool
	// createError, when set, is the answer to the next job creation.
	createError *APIError
	outages     map[string]int
	// refusals are coded answers a route gives, one per call, before it works.
	refusals map[string][]APIError
	// hidden jobs are gone from the portal: not listed, 404 by id.
	hidden map[string]bool
	calls  map[string]int
	// entering, when set, runs as a request arrives, before the portal's lock.
	entering atomic.Pointer[func(route string)]
}

func newUserPortal(t *testing.T) *userPortal {
	t.Helper()
	wait := 1500
	p := &userPortal{
		t:          t,
		demoUpload: "present",
		demos:      map[string][]byte{},
		artifacts:  map[string][]byte{},
		linkStatus: "pending",
		outages:    map[string]int{},
		refusals:   map[string][]APIError{},
		hidden:     map[string]bool{},
		calls:      map[string]int{},
	}
	p.me.User = PortalUser{ID: "user-1", Name: "Luis", Email: "luis@example.com"}
	p.me.Access = "allowed"
	p.me.Limits.MaxActive, p.me.Limits.DailySeconds, p.me.Limits.MaxDemoBytes = 3, 5400, 734003200
	p.me.Usage.Active, p.me.Usage.SecondsLast24h, p.me.Usage.SecondsCommitted = 1, 1200, 480
	p.me.Kinds = []string{"short"}
	p.me.Queue = PortalQueue{State: "online", Queued: 4, WaitSeconds: map[string]*int{"short": &wait}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/studio/me", p.handle("me", true, func(http.ResponseWriter, *http.Request) (int, any) {
		return http.StatusOK, p.me
	}))
	mux.HandleFunc("POST /api/studio/jobs", p.handle("create", true, p.create))
	mux.HandleFunc("PUT /api/studio/jobs/{id}/demo", p.handle("demo", true, p.putDemo))
	mux.HandleFunc("GET /api/studio/jobs", p.handle("list", true, func(http.ResponseWriter, *http.Request) (int, any) {
		listed := []*PortalJob{}
		for _, existing := range p.jobs {
			if !p.hidden[existing.ID] {
				listed = append(listed, existing)
			}
		}
		return http.StatusOK, map[string]any{"jobs": listed}
	}))
	mux.HandleFunc("GET /api/studio/jobs/{id}", p.handle("get", true, func(_ http.ResponseWriter, r *http.Request) (int, any) {
		if found := p.find(r.PathValue("id")); found != nil && !p.hidden[found.ID] {
			return http.StatusOK, found
		}
		return http.StatusNotFound, map[string]string{"error": "not found"}
	}))
	mux.HandleFunc("POST /api/studio/jobs/{id}/cancel", p.handle("cancel", true, p.cancel))
	mux.HandleFunc("GET /api/studio/jobs/{id}/artifacts/{artifact}", p.handle("artifact", true, p.artifact))
	mux.HandleFunc("POST /api/studio/jobs/{id}/artifacts/{artifact}/received", p.handle("received", true, func(_ http.ResponseWriter, r *http.Request) (int, any) {
		p.received = append(p.received, r.PathValue("artifact"))
		return http.StatusOK, map[string]bool{"ok": true}
	}))
	mux.HandleFunc("POST /api/studio/link/start", p.handle("linkStart", false, p.linkStart))
	mux.HandleFunc("POST /api/studio/link/poll", p.handle("linkPoll", false, p.linkPoll))
	mux.HandleFunc("DELETE /api/studio/device", p.handle("deleteDevice", true, func(http.ResponseWriter, *http.Request) (int, any) {
		p.deleted, p.revoked = true, true
		return http.StatusNoContent, nil
	}))
	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	return p
}

type portalHandler func(w http.ResponseWriter, r *http.Request) (int, any)

func (p *userPortal) handle(route string, needsToken bool, next portalHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if entering := p.entering.Load(); entering != nil {
			(*entering)(route)
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		p.calls[route]++
		var status int
		var body any
		switch {
		case needsToken && (p.revoked || r.Header.Get("Authorization") != "Bearer "+testDeviceToken):
			status, body = http.StatusUnauthorized, map[string]string{"error": "unauthorized"}
		case p.outages[route] > 0:
			p.outages[route]--
			status, body = http.StatusServiceUnavailable, map[string]string{"error": "portal restarting"}
		case len(p.refusals[route]) > 0:
			refusal := p.refusals[route][0]
			p.refusals[route] = p.refusals[route][1:]
			status, body = refusal.Status, map[string]string{"error": refusal.Code, "code": refusal.Code}
		default:
			status, body = next(w, r)
		}
		if status == 0 {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}
}

func (p *userPortal) find(id string) *PortalJob {
	for _, existing := range p.jobs {
		if existing.ID == id {
			return existing
		}
	}
	return nil
}

func (p *userPortal) create(_ http.ResponseWriter, r *http.Request) (int, any) {
	var body CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, map[string]string{"error": err.Error()}
	}
	if p.createError != nil {
		refusal := p.createError
		return refusal.Status, map[string]string{"error": refusal.Message, "code": refusal.Code}
	}
	p.created = append(p.created, body)
	created := &PortalJob{ID: uuid.NewString(), Kind: body.Kind, Title: body.Title, Status: portalQueued, CreatedAt: time.Now().UnixMilli()}
	if p.demoUpload == demoUploadRequired {
		created.Status = portalAwaitingDemo
	}
	p.jobs = append(p.jobs, created)
	return http.StatusCreated, CreateJobResponse{ID: created.ID, Status: created.Status, DemoUpload: p.demoUpload}
}

func (p *userPortal) putDemo(_ http.ResponseWriter, r *http.Request) (int, any) {
	target := p.find(r.PathValue("id"))
	if target == nil {
		return http.StatusNotFound, map[string]string{"error": "not found"}
	}
	if target.Status != portalAwaitingDemo {
		return http.StatusConflict, map[string]string{"error": "invalid state", "code": "invalid_state"}
	}
	data, err := io.ReadAll(r.Body)
	if err != nil || r.ContentLength != int64(len(data)) {
		return http.StatusBadRequest, map[string]string{"error": "bad body"}
	}
	p.demos[target.ID] = data
	target.Status = portalQueued
	return http.StatusOK, map[string]any{"ok": true, "status": portalQueued}
}

func (p *userPortal) cancel(_ http.ResponseWriter, r *http.Request) (int, any) {
	target := p.find(r.PathValue("id"))
	if target == nil {
		return http.StatusNotFound, map[string]string{"error": "not found"}
	}
	if terminal(target.Status) {
		return http.StatusConflict, map[string]string{"error": "invalid state", "code": "invalid_state"}
	}
	p.cancels = append(p.cancels, target.ID)
	if target.Status == "running" || target.Status == "uploading" {
		target.CancelRequested = true
	} else {
		target.Status = portalCanceled
	}
	return http.StatusOK, CancelResult{Status: target.Status, CancelRequested: target.CancelRequested}
}

func (p *userPortal) artifact(w http.ResponseWriter, r *http.Request) (int, any) {
	data, ok := p.artifacts[r.PathValue("artifact")]
	if !ok {
		return http.StatusGone, map[string]string{"error": "expired", "code": "expired"}
	}
	offset := 0
	if header := r.Header.Get("Range"); header != "" {
		parsed, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(header, "bytes="), "-"))
		if err != nil || parsed >= len(data) {
			return http.StatusRequestedRangeNotSatisfiable, map[string]string{"error": "range"}
		}
		offset = parsed
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, len(data)-1, len(data)))
	}
	w.Header().Set("Content-Type", "video/mp4")
	if offset > 0 {
		w.WriteHeader(http.StatusPartialContent)
	}
	_, _ = w.Write(data[offset:])
	return 0, nil
}

func (p *userPortal) linkStart(_ http.ResponseWriter, r *http.Request) (int, any) {
	var body struct {
		DeviceName string `json:"deviceName"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	p.deviceNames = append(p.deviceNames, body.DeviceName)
	return http.StatusOK, LinkStart{
		LinkID:          "link-1",
		PollToken:       strings.Repeat("ab", 32),
		UserCode:        "K7QM-2XHD",
		VerifyURL:       p.server.URL + "/link?code=K7QM-2XHD",
		ExpiresAt:       time.Now().Add(10 * time.Minute).UnixMilli(),
		IntervalSeconds: 3,
	}
}

func (p *userPortal) linkPoll(_ http.ResponseWriter, r *http.Request) (int, any) {
	var body struct {
		LinkID    string `json:"linkId"`
		PollToken string `json:"pollToken"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.LinkID != "link-1" || body.PollToken != strings.Repeat("ab", 32) {
		return http.StatusNotFound, map[string]string{"error": "unknown link"}
	}
	switch p.linkStatus {
	case "approved":
		p.linkStatus = "consumed"
		return http.StatusOK, LinkPoll{Status: "approved", DeviceToken: testDeviceToken, User: &PortalUser{Name: "Luis", Email: "luis@example.com"}}
	case "consumed":
		return http.StatusOK, LinkPoll{Status: "expired"}
	default:
		return http.StatusOK, LinkPoll{Status: p.linkStatus}
	}
}

func (p *userPortal) with(inspect func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	inspect()
}

// ---- local side ----

type jobMap struct {
	mu   sync.Mutex
	jobs map[uuid.UUID]job.Job
}

func (m *jobMap) Get(_ context.Context, id uuid.UUID) (job.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	found, ok := m.jobs[id]
	if !ok {
		return job.Job{}, job.ErrNotFound
	}
	return found, nil
}

type fixture struct {
	t       *testing.T
	portal  *userPortal
	dataDir string
	files   *storage.Local
	jobs    *jobMap
	service *Service
	timing  Timing
	stop    func()
	demo    []byte
	parsed  job.Job
}

func testTiming() Timing {
	return Timing{
		AccountTTL:       5 * time.Millisecond,
		ActiveSync:       3 * time.Millisecond,
		IdleSync:         5 * time.Millisecond,
		LinkPoll:         2 * time.Millisecond,
		RetryWatch:       5 * time.Millisecond,
		UploadRetry:      2 * time.Millisecond,
		UploadRetryMax:   4 * time.Millisecond,
		DownloadRetryMax: 6 * time.Millisecond,
	}
}

// newFixture builds a Studio with one parsed local job. linked decides
// whether client.json already holds a device token.
func newFixture(t *testing.T, linked bool) *fixture {
	t.Helper()
	dataDir := t.TempDir()
	files, err := storage.NewLocal(dataDir)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	f := &fixture{
		t:       t,
		portal:  newUserPortal(t),
		dataDir: dataDir,
		files:   files,
		jobs:    &jobMap{jobs: map[uuid.UUID]job.Job{}},
		timing:  testTiming(),
		stop:    func() {},
		demo:    append([]byte("PBDEMS2"), bytes.Repeat([]byte("demo"), 300)...),
	}
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Segments = []killplan.Segment{
		{ID: "seg-001", TickStart: 1000, TickEnd: 1640},
		{ID: "seg-002", TickStart: 9000, TickEnd: 9800},
		{ID: "seg-003", TickStart: 51234, TickEnd: 52010},
	}
	awpOnly := rules.Default()
	awpOnly.Weapons = []string{"awp"}
	id := uuid.New()
	f.parsed = job.Job{
		ID:            id,
		Status:        job.StatusParsed,
		DemoFileName:  "match.dem",
		DemoPath:      "demos/" + id.String() + ".dem",
		DemoSHA256:    sha256Hex(f.demo),
		TargetSteamID: testTarget,
		Rules:         awpOnly,
		KillPlan:      &plan,
	}
	if err := files.Put(f.parsed.DemoPath, bytes.NewReader(f.demo)); err != nil {
		t.Fatalf("store demo: %v", err)
	}
	f.jobs.jobs[id] = f.parsed
	if linked {
		store, err := OpenStore(filepath.Join(dataDir, "cloud", "client.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Link(testDeviceToken, StoredUser{Name: "Luis", Email: "luis@example.com"}); err != nil {
			t.Fatal(err)
		}
	}
	f.start()
	return f
}

// start builds the service from what is on disk, like a Studio start. A
// service that is already running is stopped first, like a Studio restart.
func (f *fixture) start() {
	f.t.Helper()
	f.stop()
	service, err := NewService(Config{
		PortalURL:     f.portal.server.URL,
		DataDir:       f.dataDir,
		Jobs:          f.jobs,
		Files:         f.files,
		StudioVersion: "5.4.4",
		DeviceName:    "DESKTOP-TEST",
		Timing:        f.timing,
	})
	if err != nil {
		f.t.Fatalf("NewService error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	// A stopped service is waited for: its goroutines write under the data
	// directory, which the test removes (and a restart reads) afterwards.
	stop := func() {
		cancel()
		service.wait()
	}
	f.t.Cleanup(stop)
	service.Start(ctx)
	f.service, f.stop = service, stop
}

func (f *fixture) submit(mutate func(*SubmitInput)) (LocalJob, error) {
	in := SubmitInput{
		JobID:      f.parsed.ID.String(),
		Kind:       "short",
		Title:      "R3 4k",
		Preset:     "viral-60-clean",
		Music:      json.RawMessage(`{"key":"track-1","volume":0.4}`),
		Edit:       json.RawMessage(`{"format":"short_9x16","intro":true}`),
		SegmentIDs: []string{"seg-003", "seg-001"},
	}
	if mutate != nil {
		mutate(&in)
	}
	return f.service.Submit(context.Background(), in)
}

func (f *fixture) job(id string) LocalJob {
	f.t.Helper()
	for _, current := range f.service.Jobs() {
		if current.ID == id {
			return current
		}
	}
	return LocalJob{}
}

func (f *fixture) calls(route string) int {
	count := 0
	f.portal.with(func() { count = f.portal.calls[route] })
	return count
}

func (f *fixture) portalStatus(id string) string {
	status := ""
	f.portal.with(func() {
		if found := f.portal.find(id); found != nil {
			status = found.Status
		}
	})
	return status
}

func stalledReason(current LocalJob) string {
	if current.Stalled == nil {
		return ""
	}
	return *current.Stalled
}

func cloudError(err error) *Error {
	var cloudErr *Error
	if errors.As(err, &cloudErr) {
		return cloudErr
	}
	return &Error{}
}

// ---- tests ----

func TestLinkFlowStoresTheDeviceTokenAndExposesTheAccount(t *testing.T) {
	f := newFixture(t, false)
	ctx := context.Background()

	before := f.service.Account(ctx)
	if before.Linked || before.User != nil || before.Access != nil || before.Limits != nil || before.Usage != nil || before.Queue != nil || len(before.Kinds) != 0 || before.Link != nil {
		t.Fatalf("account before linking = %+v, want everything empty", before)
	}

	pending, err := f.service.StartLink(ctx)
	if err != nil {
		t.Fatalf("StartLink error = %v", err)
	}
	if pending.Linked || pending.Link == nil || pending.Link.Status != "pending" || pending.Link.UserCode != "K7QM-2XHD" || !strings.HasSuffix(pending.Link.VerifyURL, "/link?code=K7QM-2XHD") {
		t.Fatalf("account while linking = %+v (link %+v), want the pending code to show", pending, pending.Link)
	}
	if _, err := time.Parse(time.RFC3339, pending.Link.ExpiresAt); err != nil {
		t.Fatalf("link expires_at = %q, want RFC 3339", pending.Link.ExpiresAt)
	}
	// Asking again while the code is valid must not burn another code.
	if again, err := f.service.StartLink(ctx); err != nil || again.Link == nil || again.Link.UserCode != "K7QM-2XHD" {
		t.Fatalf("second StartLink = %+v, err=%v, want the same pending link", again, err)
	}
	f.portal.with(func() {
		if len(f.portal.deviceNames) != 1 || f.portal.deviceNames[0] != "DESKTOP-TEST" {
			t.Fatalf("link starts = %q, want one for this device", f.portal.deviceNames)
		}
		f.portal.linkStatus = "approved"
	})

	eventually(t, "the link to complete", func() bool { return f.service.Account(ctx).Linked })
	var account Account
	eventually(t, "the account details", func() bool {
		account = f.service.Account(ctx)
		return account.Access != nil
	})
	if account.Link != nil || account.Error != nil || account.PortalURL != f.portal.server.URL {
		t.Fatalf("linked account = %+v, want no link and no error", account)
	}
	if account.User == nil || account.User.Name != "Luis" || *account.Access != "allowed" {
		t.Fatalf("linked account user=%+v access=%v", account.User, account.Access)
	}
	if *account.Limits != (AccountLimits{MaxActive: 3, DailySeconds: 5400}) || *account.Usage != (AccountUsage{Active: 1, SecondsLast24h: 1200, SecondsCommitted: 480}) {
		t.Fatalf("limits=%+v usage=%+v", account.Limits, account.Usage)
	}
	if !slices.Equal(account.Kinds, []string{"short"}) || account.Queue.State != "online" || account.Queue.Queued != 4 || *account.Queue.WaitSeconds["short"] != 1500 {
		t.Fatalf("kinds=%v queue=%+v", account.Kinds, account.Queue)
	}

	// The token is on disk for the next start and nowhere in the document.
	encoded, _ := json.Marshal(account)
	if strings.Contains(string(encoded), "chd_") {
		t.Fatalf("the account document leaks the device token: %s", encoded)
	}
	stored, err := os.ReadFile(filepath.Join(f.dataDir, "cloud", "client.json"))
	if err != nil || !strings.Contains(string(stored), testDeviceToken) {
		t.Fatalf("client.json does not hold the device token (err %v)", err)
	}
	f.start()
	if restarted := f.service.Account(ctx); !restarted.Linked {
		t.Fatal("the link did not survive a restart")
	}
}

func TestLinkDeniedIsReportedAndLeavesThePCUnlinked(t *testing.T) {
	f := newFixture(t, false)
	ctx := context.Background()
	if _, err := f.service.StartLink(ctx); err != nil {
		t.Fatalf("StartLink error = %v", err)
	}
	f.portal.with(func() { f.portal.linkStatus = "denied" })

	eventually(t, "the denial", func() bool {
		account := f.service.Account(ctx)
		return account.Link != nil && account.Link.Status == "denied"
	})
	if f.service.Account(ctx).Linked {
		t.Fatal("a denied link left the PC linked")
	}
}

func TestSubmitSendsTheSpecOfTheContract(t *testing.T) {
	f := newFixture(t, true)

	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	if created.Status != portalQueued || created.LocalJobID != f.parsed.ID.String() || created.Kind != "short" || created.Title != "R3 4k" {
		t.Fatalf("created job = %+v, want it queued for the local job", created)
	}
	var request CreateJobRequest
	f.portal.with(func() {
		if len(f.portal.created) != 1 {
			t.Fatalf("jobs created on the portal = %d, want 1", len(f.portal.created))
		}
		request = f.portal.created[0]
		if f.portal.calls["demo"] != 0 {
			t.Fatal("the demo was uploaded although the portal already had it")
		}
	})
	if request.Kind != "short" || request.Title != "R3 4k" {
		t.Fatalf("request = %+v", request)
	}
	if request.Demo != (PortalDemo{SHA256: sha256Hex(f.demo), SizeBytes: int64(len(f.demo)), FileName: "match.dem"}) {
		t.Fatalf("demo = %+v, want the stored hash and the real size", request.Demo)
	}
	var spec struct {
		Version       int             `json:"version"`
		TargetSteamID string          `json:"targetSteamId"`
		Rules         rules.Rules     `json:"rules"`
		Capture       json.RawMessage `json:"capture"`
		Generate      json.RawMessage `json:"generate"`
		Client        struct {
			StudioVersion string `json:"studioVersion"`
			PlanSchema    string `json:"planSchema"`
		} `json:"client"`
	}
	if err := json.Unmarshal(request.Spec, &spec); err != nil {
		t.Fatalf("spec is not JSON: %v", err)
	}
	if spec.Version != 1 || spec.TargetSteamID != testTarget || !slices.Equal(spec.Rules.Weapons, []string{"awp"}) {
		t.Fatalf("spec envelope = %+v", spec)
	}
	// Windows follow the selection order and carry the plan's own ticks.
	const wantCapture = `{"tickrate":64,"windows":[{"id":"seg-003","tickStart":51234,"tickEnd":52010},{"id":"seg-001","tickStart":1000,"tickEnd":1640}]}`
	if string(spec.Capture) != wantCapture {
		t.Fatalf("capture = %s, want %s", spec.Capture, wantCapture)
	}
	const wantGenerate = `{"preset":"viral-60-clean","music":{"key":"track-1","volume":0.4},"segment_ids":["seg-003","seg-001"],"edit":{"format":"short_9x16","intro":true}}`
	if string(spec.Generate) != wantGenerate {
		t.Fatalf("generate = %s, want %s", spec.Generate, wantGenerate)
	}
	if spec.Client.StudioVersion != "5.4.4" || spec.Client.PlanSchema != killplan.SchemaVersion {
		t.Fatalf("client = %+v", spec.Client)
	}
}

func TestSubmitSpellsOutAnEmptySelectionAndABareMusicKey(t *testing.T) {
	f := newFixture(t, true)

	if _, err := f.submit(func(in *SubmitInput) {
		in.SegmentIDs = nil
		in.Music = json.RawMessage(`"track-1"`)
		in.Edit = nil
	}); err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	f.portal.with(func() {
		spec := string(f.portal.created[0].Spec)
		// Locally an empty selection records everything; the portal has to
		// see, and charge for, every window.
		for _, want := range []string{
			`"segment_ids":["seg-001","seg-002","seg-003"]`,
			`"windows":[{"id":"seg-001","tickStart":1000,"tickEnd":1640},{"id":"seg-002","tickStart":9000,"tickEnd":9800},{"id":"seg-003","tickStart":51234,"tickEnd":52010}]`,
			`"music":{"key":"track-1"}`,
			`"edit":null`,
		} {
			if !strings.Contains(spec, want) {
				t.Fatalf("spec = %s\nwant it to contain %s", spec, want)
			}
		}
	})
}

func TestSubmitUploadsTheDemoWhenThePortalNeedsIt(t *testing.T) {
	f := newFixture(t, true)
	f.portal.demoUpload = demoUploadRequired

	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	if created.Status != StatusUploadingDemo || created.Percent == nil {
		t.Fatalf("created job = %+v, want uploading_demo with a percent", created)
	}

	eventually(t, "the job to be queued", func() bool { return f.job(created.ID).Status == portalQueued })
	f.portal.with(func() {
		if !bytes.Equal(f.portal.demos[created.ID], f.demo) {
			t.Fatalf("the portal received %d demo bytes, want the %d stored ones", len(f.portal.demos[created.ID]), len(f.demo))
		}
	})
}

func TestSubmitResumesTheDemoUploadAfterARestart(t *testing.T) {
	f := newFixture(t, true)
	f.portal.demoUpload = demoUploadRequired
	f.portal.outages["demo"] = 1 << 30
	// A long pause between attempts, so Studio closes during the first one.
	f.timing.UploadRetry, f.timing.UploadRetryMax = time.Minute, time.Minute
	f.start()
	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	eventually(t, "the first upload attempt", func() bool {
		attempts := 0
		f.portal.with(func() { attempts = f.portal.calls["demo"] })
		return attempts == 1
	})
	f.portal.with(func() { f.portal.outages["demo"] = 0 })
	f.timing = testTiming()
	f.start()
	if resumed := f.job(created.ID); resumed.Status != StatusUploadingDemo && resumed.Status != portalQueued {
		t.Fatalf("job after the restart = %+v, want its demo upload pending or done", resumed)
	}

	eventually(t, "the resumed upload", func() bool { return f.job(created.ID).Status == portalQueued })
}

func TestADemoUploadOutlivesAnOutage(t *testing.T) {
	f := newFixture(t, true)
	f.portal.demoUpload = demoUploadRequired
	f.portal.outages["demo"] = 1 << 30

	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	// Far more attempts than a laptop waking from sleep needs.
	eventually(t, "the upload to be retried", func() bool { return f.calls("demo") >= 8 })
	waiting := f.job(created.ID)
	if waiting.Status != StatusUploadingDemo || waiting.Failure != nil {
		t.Fatalf("job during the outage = %+v, want it still uploading its demo", waiting)
	}
	eventually(t, "the row to say the upload is waiting", func() bool {
		return stalledReason(f.job(created.ID)) == StalledDemoUpload
	})
	if canceled := f.calls("cancel"); canceled != 0 {
		t.Fatalf("the portal job was canceled %d times during an outage, want it kept", canceled)
	}

	f.portal.with(func() { f.portal.outages["demo"] = 0 })
	eventually(t, "the job to be queued", func() bool { return f.job(created.ID).Status == portalQueued })
	if stalled := stalledReason(f.job(created.ID)); stalled != "" {
		t.Fatalf("stalled after the upload = %q, want none", stalled)
	}
	f.portal.with(func() {
		if !bytes.Equal(f.portal.demos[created.ID], f.demo) {
			t.Fatal("the portal did not receive the demo once it was back")
		}
	})
}

func TestADemoUploadWaitsWhileThePortalStillReadsAnEarlierOne(t *testing.T) {
	f := newFixture(t, true)
	f.portal.demoUpload = demoUploadRequired
	busy := APIError{Status: http.StatusConflict, Code: "upload_in_progress"}
	f.portal.refusals["demo"] = []APIError{busy, busy}

	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	eventually(t, "the job to be queued", func() bool { return f.job(created.ID).Status == portalQueued })
	if attempts := f.calls("demo"); attempts != 3 {
		t.Fatalf("upload attempts = %d, want the two refused ones and the one that got through", attempts)
	}
}

// The first upload got through but its answer was lost, and the job even
// finished before Studio tried again.
func TestAnUploadWhoseAnswerWasLostFollowsTheJobFromWhereItIs(t *testing.T) {
	f := newFixture(t, true)
	f.portal.demoUpload = demoUploadRequired
	video := []byte("\x00\x00\x00\x18ftypisom-finished-meanwhile")
	var once sync.Once
	entering := func(route string) {
		if route != "demo" {
			return
		}
		once.Do(func() {
			f.portal.with(func() {
				finished := f.portal.jobs[0]
				finished.Status = portalDone
				f.portal.artifacts["artifact-1"] = video
				finished.Artifacts = []PortalArtifact{{ID: "artifact-1", Kind: "video", Variant: "viral-60-clean", Name: "seg-003.mp4", SizeBytes: int64(len(video)), SHA256: sha256Hex(video)}}
			})
		})
	}
	f.portal.entering.Store(&entering)

	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	eventually(t, "the finished job to be downloaded", func() bool { return f.job(created.ID).Status == portalDone })
	if done := f.job(created.ID); len(done.Videos) != 1 || !done.Videos[0].Ready {
		t.Fatalf("job = %+v, want its video on this PC", done)
	}
}

func TestARefusedDemoUploadFailsWithItsCodeAndFreesTheSlot(t *testing.T) {
	cases := []struct {
		name     string
		refusal  APIError
		wantCode string
	}{
		{name: "two uploads already in flight", refusal: APIError{Status: http.StatusTooManyRequests, Code: "limit_uploads"}, wantCode: FailureLimitUploads},
		{name: "the user's demo storage is full", refusal: APIError{Status: http.StatusTooManyRequests, Code: "limit_storage"}, wantCode: FailureLimitStorage},
		{name: "the portal is out of space", refusal: APIError{Status: http.StatusServiceUnavailable, Code: "cloud_storage_full"}, wantCode: FailureStorageFull},
		{name: "the demo is not what the job announced", refusal: APIError{Status: http.StatusUnprocessableEntity, Code: "sha256_mismatch"}, wantCode: FailureDemoUpload},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, true)
			f.portal.demoUpload = demoUploadRequired
			f.portal.refusals["demo"] = []APIError{tc.refusal}
			// The portal cannot be told at first, as in the outage that caused this.
			f.portal.outages["cancel"] = 1 << 30

			created, err := f.submit(nil)
			if err != nil {
				t.Fatalf("Submit error = %v", err)
			}
			eventually(t, "the refusal to show", func() bool { return f.job(created.ID).Status == portalFailed })
			failed := f.job(created.ID)
			if failed.Failure == nil || failed.Failure.Code != tc.wantCode || failed.Failure.Message == "" {
				t.Fatalf("failure = %+v, want %s with a message", failed.Failure, tc.wantCode)
			}
			eventually(t, "a cancel that the portal does not answer", func() bool { return f.calls("cancel") >= 2 })
			if attempts := f.calls("demo"); attempts != 1 {
				t.Fatalf("upload attempts = %d, want 1: a refusal is not retried", attempts)
			}
			// The row can go; the portal job still has to stop counting as active.
			if err := f.service.Remove(context.Background(), created.ID); err != nil {
				t.Fatalf("Remove of a failed job error = %v", err)
			}
			if status := f.portalStatus(created.ID); status != portalAwaitingDemo {
				t.Fatalf("portal job = %q while the cancel cannot get through, want awaiting_demo", status)
			}

			f.portal.with(func() { f.portal.outages["cancel"] = 0 })
			eventually(t, "the portal job to be canceled", func() bool { return f.portalStatus(created.ID) == portalCanceled })
			// Settled: the sync loop must not keep asking.
			settled := f.calls("cancel")
			time.Sleep(60 * time.Millisecond)
			if again := f.calls("cancel"); again != settled {
				t.Fatalf("cancel calls went from %d to %d after the job was canceled", settled, again)
			}
		})
	}
}

func TestADemoUploadIsGivenUpOnceItsWindowHasPassed(t *testing.T) {
	f := newFixture(t, true)
	f.portal.demoUpload = demoUploadRequired
	f.portal.outages["demo"] = 1 << 30
	// Wide enough for a loaded machine, where a 2 ms pause can take 15 ms.
	f.timing.UploadWindow = 300 * time.Millisecond
	f.start()

	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	eventually(t, "the upload to be given up", func() bool { return f.job(created.ID).Status == portalFailed })
	if failure := f.job(created.ID).Failure; failure == nil || failure.Code != FailureDemoUpload || failure.Message == "" {
		t.Fatalf("failure = %+v, want demo_upload_failed with a message", failure)
	}
	if attempts := f.calls("demo"); attempts < 4 {
		t.Fatalf("upload attempts = %d, want it to have kept trying for the whole window", attempts)
	}
	// The orphan must not keep one of the user's three active slots.
	eventually(t, "the portal job to be canceled", func() bool { return f.portalStatus(created.ID) == portalCanceled })
}

func TestCancelReachesThePortalForAnUploadThatFailedOnThisPC(t *testing.T) {
	f := newFixture(t, true)
	f.portal.demoUpload = demoUploadRequired
	f.portal.refusals["demo"] = []APIError{{Status: http.StatusTooManyRequests, Code: "limit_storage"}}
	f.portal.outages["cancel"] = 1 << 30
	// The sync loop tries the cancel as the upload fails, then sleeps for the test.
	f.timing.IdleSync = time.Hour
	f.start()

	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	eventually(t, "the refusal to show", func() bool { return f.job(created.ID).Status == portalFailed })
	eventually(t, "the cancel that failed", func() bool { return f.calls("cancel") >= 1 })
	// The answer is the portal's, not "this job already ended" from this PC.
	if _, err := f.service.Cancel(context.Background(), created.ID); cloudError(err).Code != "portal_unreachable" {
		t.Fatalf("Cancel during the outage error = %v, want portal_unreachable", err)
	}
	if status := f.portalStatus(created.ID); status != portalAwaitingDemo {
		t.Fatalf("portal job = %q, want it still awaiting its demo", status)
	}

	f.portal.with(func() { f.portal.outages["cancel"] = 0 })
	canceled, err := f.service.Cancel(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Cancel of a job the portal still holds error = %v", err)
	}
	if canceled.Status != portalFailed || canceled.Failure == nil || canceled.Failure.Code != FailureLimitStorage {
		t.Fatalf("job after the cancel = %+v, want it to keep the failure the user read", canceled)
	}
	if status := f.portalStatus(created.ID); status != portalCanceled {
		t.Fatalf("portal job = %q after Cancel, want canceled", status)
	}
	if _, err := f.service.Cancel(context.Background(), created.ID); cloudError(err).Code != "invalid_state" {
		t.Fatalf("second Cancel error = %v, want invalid_state: nothing is left to cancel", err)
	}
}

func TestOnlyTwoDemosTravelAtOnce(t *testing.T) {
	f := newFixture(t, true)
	f.portal.demoUpload = demoUploadRequired
	var inFlight, peak atomic.Int32
	release := make(chan struct{})
	entering := func(route string) {
		if route != "demo" {
			return
		}
		now := inFlight.Add(1)
		for {
			seen := peak.Load()
			if now <= seen || peak.CompareAndSwap(seen, now) {
				break
			}
		}
		<-release
		inFlight.Add(-1)
	}
	f.portal.entering.Store(&entering)

	ids := make([]string, 0, 3)
	for range 3 {
		created, err := f.submit(nil)
		if err != nil {
			t.Fatalf("Submit error = %v", err)
		}
		ids = append(ids, created.ID)
	}
	eventually(t, "two uploads in flight", func() bool { return inFlight.Load() == 2 })
	time.Sleep(60 * time.Millisecond)
	if got := peak.Load(); got != 2 {
		t.Fatalf("uploads in flight at once = %d, want 2: the portal refuses a third", got)
	}
	close(release)
	for _, id := range ids {
		eventually(t, "every job to be queued", func() bool { return f.job(id).Status == portalQueued })
	}
}

func TestSubmitRejections(t *testing.T) {
	cases := []struct {
		name       string
		linked     bool
		prepare    func(*fixture)
		input      func(*SubmitInput)
		wantStatus int
		wantCode   string
	}{
		{name: "this PC is not linked", wantStatus: http.StatusConflict, wantCode: "not_linked"},
		{
			name: "the local job has no kill plan yet", linked: true,
			prepare: func(f *fixture) {
				scanned := f.parsed
				scanned.KillPlan, scanned.Status = nil, job.StatusScanned
				f.jobs.jobs[scanned.ID] = scanned
			},
			wantStatus: http.StatusConflict, wantCode: "job_not_parsed",
		},
		{
			name: "a segment that is not in the plan", linked: true,
			input:      func(in *SubmitInput) { in.SegmentIDs = []string{"seg-003", "seg-404"} },
			wantStatus: http.StatusBadRequest, wantCode: "unknown_segment",
		},
		{
			name: "a local job that does not exist", linked: true,
			input:      func(in *SubmitInput) { in.JobID = uuid.NewString() },
			wantStatus: http.StatusNotFound, wantCode: "not_found",
		},
		{
			name: "the portal refuses for a limit", linked: true,
			prepare: func(f *fixture) {
				f.portal.createError = &APIError{Status: http.StatusTooManyRequests, Code: "limit_active", Message: "Ya tienes 3 trabajos activos."}
			},
			wantStatus: http.StatusTooManyRequests, wantCode: "limit_active",
		},
		{
			name: "this Studio and the cloud machine disagree on the plan version", linked: true,
			prepare: func(f *fixture) {
				f.portal.createError = &APIError{Status: http.StatusConflict, Code: "studio_version_mismatch", Message: "studio_version_mismatch"}
			},
			wantStatus: http.StatusConflict, wantCode: "studio_version_mismatch",
		},
		{
			name: "the user's demo storage is full", linked: true,
			prepare: func(f *fixture) {
				f.portal.createError = &APIError{Status: http.StatusTooManyRequests, Code: "limit_storage", Message: "limit_storage"}
			},
			wantStatus: http.StatusTooManyRequests, wantCode: "limit_storage",
		},
		{
			name: "the cloud is full", linked: true,
			prepare: func(f *fixture) {
				f.portal.createError = &APIError{Status: http.StatusServiceUnavailable, Code: "cloud_queue_full", Message: "La nube está llena."}
			},
			wantStatus: http.StatusServiceUnavailable, wantCode: "cloud_queue_full",
		},
		{
			name: "the portal cannot be reached", linked: true,
			prepare:    func(f *fixture) { f.portal.server.Close() },
			wantStatus: http.StatusServiceUnavailable, wantCode: "portal_unreachable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, tc.linked)
			if tc.prepare != nil {
				tc.prepare(f)
			}
			_, err := f.submit(tc.input)
			got := cloudError(err)
			if got.Status != tc.wantStatus || got.Code != tc.wantCode {
				t.Fatalf("Submit error = %v, want status %d with code %q", err, tc.wantStatus, tc.wantCode)
			}
			if len(f.service.Jobs()) != 0 {
				t.Fatalf("a rejected submit left %d local records", len(f.service.Jobs()))
			}
		})
	}
}

func TestARejectedSubmitRefreshesTheCachedAccount(t *testing.T) {
	f := newFixture(t, true)
	// Long enough that only the refusal can make the next read ask the portal.
	f.timing.AccountTTL = time.Hour
	f.start()
	access := func() string {
		if current := f.service.Account(context.Background()).Access; current != nil {
			return *current
		}
		return ""
	}
	if got := access(); got != "allowed" {
		t.Fatalf("access before the refusal = %q, want allowed", got)
	}
	f.portal.with(func() {
		f.portal.me.Access = "blocked"
		f.portal.createError = &APIError{Status: http.StatusForbidden, Code: "cloud_access_blocked", Message: "cloud_access_blocked"}
	})
	if _, err := f.submit(nil); cloudError(err).Code != "cloud_access_blocked" {
		t.Fatalf("Submit error = %v, want cloud_access_blocked", err)
	}
	if got := access(); got != "blocked" {
		t.Fatalf("access after the refusal = %q, want blocked from a fresh portal read", got)
	}
}

func TestJobsFollowThePortalAndDownloadTheResults(t *testing.T) {
	f := newFixture(t, true)
	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	startAt := time.Date(2026, 10, 9, 10, 25, 0, 0, time.UTC).UnixMilli()
	f.portal.with(func() {
		f.portal.jobs[0].Queue = &PortalJobQueue{Position: 3, EstimatedStartAt: &startAt, State: "online"}
	})
	eventually(t, "the queue position", func() bool { return f.job(created.ID).Queue != nil })
	queued := f.job(created.ID)
	if queued.Queue.Position != 3 || queued.Queue.EstimatedStartAt == nil || *queued.Queue.EstimatedStartAt != "2026-10-09T10:25:00Z" || queued.Queue.EstimatedDoneAt != nil {
		t.Fatalf("queue = %+v, want position 3 starting at 10:25 UTC", queued.Queue)
	}

	stage, percent := "capturing", 62
	f.portal.with(func() {
		running := f.portal.jobs[0]
		running.Status, running.Stage, running.ProgressPercent, running.Queue = "running", &stage, &percent, nil
	})
	eventually(t, "the capture progress", func() bool {
		current := f.job(created.ID)
		return current.Status == "running" && current.Stage != nil && *current.Stage == "capturing" && current.Percent != nil && *current.Percent == 62
	})

	first, second := []byte("\x00\x00\x00\x18ftypisom-first-video"), []byte("\x00\x00\x00\x18ftypisom-second")
	finishedAt := time.Date(2026, 10, 9, 11, 0, 0, 0, time.UTC).UnixMilli()
	f.portal.with(func() {
		f.portal.artifacts["artifact-1"], f.portal.artifacts["artifact-2"] = first, second
		done := f.portal.jobs[0]
		done.Status, done.Stage, done.ProgressPercent, done.FinishedAt = portalDone, nil, nil, &finishedAt
		done.Artifacts = []PortalArtifact{
			{ID: "artifact-1", Kind: "video", Variant: "viral-60-clean", Name: "seg-003.mp4", SizeBytes: int64(len(first)), SHA256: sha256Hex(first)},
			{ID: "artifact-2", Kind: "video", Variant: "viral-60-clean", Name: "seg-001.mp4", SizeBytes: int64(len(second)), SHA256: sha256Hex(second)},
			// A name that tries to leave the results folder is not a video of ours.
			{ID: "artifact-3", Kind: "video", Variant: "viral-60-clean", Name: "../../evil.mp4", SizeBytes: 4, SHA256: sha256Hex([]byte("evil"))},
		}
	})

	eventually(t, "the videos on disk", func() bool { return f.job(created.ID).Status == portalDone })
	done := f.job(created.ID)
	want := []LocalVideo{
		{Name: "seg-003.mp4", Variant: "viral-60-clean", SizeBytes: int64(len(first)), Ready: true},
		{Name: "seg-001.mp4", Variant: "viral-60-clean", SizeBytes: int64(len(second)), Ready: true},
	}
	if !slices.Equal(done.Videos, want) || done.FinishedAt == nil || *done.FinishedAt != "2026-10-09T11:00:00Z" {
		t.Fatalf("done job = %+v, want the two videos ready", done)
	}
	path, err := f.service.VideoPath(created.ID, "seg-003.mp4")
	if err != nil {
		t.Fatalf("VideoPath error = %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, first) {
		t.Fatalf("downloaded file differs from the portal's (err %v)", err)
	}
	if want := filepath.Join(f.dataDir, "cloud", "results", created.ID, "seg-003.mp4"); path != want {
		t.Fatalf("video path = %s, want %s", path, want)
	}
	if _, err := f.service.VideoPath(created.ID, "../client.json"); cloudError(err).Status != http.StatusNotFound {
		t.Fatalf("VideoPath of a name that is not a video of the job error = %v, want 404", err)
	}
	if _, err := os.Stat(filepath.Join(f.dataDir, "evil.mp4")); !os.IsNotExist(err) {
		t.Fatal("an artifact name escaped the results folder")
	}
	// The portal learns it can delete its copies.
	eventually(t, "the receipts", func() bool {
		confirmed := false
		f.portal.with(func() {
			confirmed = slices.Contains(f.portal.received, "artifact-1") && slices.Contains(f.portal.received, "artifact-2")
		})
		return confirmed
	})

	// Removing a finished job deletes its videos from this PC.
	if err := f.service.Remove(context.Background(), created.ID); err != nil {
		t.Fatalf("Remove error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("Remove left the downloaded video on disk")
	}
	if len(f.service.Jobs()) != 0 {
		t.Fatal("Remove left the job in the list")
	}
}

// finishWithOneVideo makes the portal's first job done with one video whose
// announced hash is that of good; the bytes it serves are served.
func (f *fixture) finishWithOneVideo(good, served []byte) {
	f.portal.with(func() {
		f.portal.artifacts["artifact-1"] = served
		done := f.portal.jobs[0]
		done.Status = portalDone
		done.Artifacts = []PortalArtifact{{ID: "artifact-1", Kind: "video", Variant: "viral-60-clean", Name: "seg-003.mp4", SizeBytes: int64(len(good)), SHA256: sha256Hex(good)}}
	})
}

func TestAResultThatKeepsArrivingDamagedFailsTheJobOnThisPC(t *testing.T) {
	f := newFixture(t, true)
	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	good := []byte("\x00\x00\x00\x18ftypisom-good-video")
	f.finishWithOneVideo(good, []byte("\x00\x00\x00\x18ftypisom-corrupted!!"))

	eventually(t, "the job to fail on this PC", func() bool { return f.job(created.ID).Status == portalFailed })
	failed := f.job(created.ID)
	if failed.Failure == nil || failed.Failure.Code != FailureDownload || failed.Failure.Message == "" {
		t.Fatalf("failure = %+v, want download_failed with a message", failed.Failure)
	}
	if len(failed.Videos) != 1 || failed.Videos[0].Ready {
		t.Fatalf("videos = %+v, want the one video not ready", failed.Videos)
	}
	if _, err := f.service.VideoPath(created.ID, "seg-003.mp4"); err == nil {
		t.Fatal("a video with the wrong hash can be played")
	}
	// The whole file is not pulled from the portal again, now or after a restart.
	time.Sleep(60 * time.Millisecond)
	f.start()
	time.Sleep(60 * time.Millisecond)
	if fetched := f.calls("artifact"); fetched != maxDownloadMismatches {
		t.Fatalf("downloads of the damaged file = %d, want %d", fetched, maxDownloadMismatches)
	}
	if err := f.service.Remove(context.Background(), created.ID); err != nil {
		t.Fatalf("Remove of the failed job error = %v", err)
	}
}

func TestADownloadThatKeepsFailingBacksOffAndCanBeRemoved(t *testing.T) {
	f := newFixture(t, true)
	f.timing.DownloadRetryMax = 50 * time.Millisecond
	f.start()
	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	good := []byte("\x00\x00\x00\x18ftypisom-good-video")
	f.portal.with(func() { f.portal.outages["artifact"] = 1 << 30 })
	f.finishWithOneVideo(good, good)

	// The pauses double from the sync interval up to the cap.
	eventually(t, "the pause to reach its cap", func() bool { return f.calls("artifact") >= 5 })
	stuck := f.job(created.ID)
	if stuck.Status != StatusDownloading || stalledReason(stuck) != StalledDownload {
		t.Fatalf("job = %+v (stalled %q), want it downloading and saying the download is failing", stuck, stalledReason(stuck))
	}
	before := f.calls("artifact")
	time.Sleep(300 * time.Millisecond)
	// One attempt per sync round would be about a hundred in this time.
	if attempts := f.calls("artifact") - before; attempts > 10 {
		t.Fatalf("%d download attempts in 300 ms with a 50 ms cap, want at most a handful", attempts)
	}

	// The portal already finished the job: there is nothing to cancel there.
	if _, err := f.service.Cancel(context.Background(), created.ID); cloudError(err).Code != "invalid_state" {
		t.Fatalf("Cancel of a job that is only downloading error = %v, want invalid_state", err)
	}
	if canceled := f.calls("cancel"); canceled != 0 {
		t.Fatalf("the portal was asked to cancel a finished job %d times", canceled)
	}
	if err := f.service.Remove(context.Background(), created.ID); err != nil {
		t.Fatalf("Remove of a job stuck in downloading error = %v", err)
	}
	if len(f.service.Jobs()) != 0 {
		t.Fatal("Remove left the job in the list")
	}
	if _, err := os.Stat(filepath.Join(f.dataDir, "cloud", "results", created.ID)); !os.IsNotExist(err) {
		t.Fatalf("Remove left the results folder behind (stat error %v)", err)
	}
	removedAt := f.calls("artifact")
	time.Sleep(120 * time.Millisecond)
	if again := f.calls("artifact"); again != removedAt {
		t.Fatalf("the removed job was downloaded again (%d to %d attempts)", removedAt, again)
	}
}

func TestADownloadThatFailedIsFinishedOnceThePortalServesTheFile(t *testing.T) {
	f := newFixture(t, true)
	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	good := []byte("\x00\x00\x00\x18ftypisom-good-video")
	f.portal.with(func() { f.portal.outages["artifact"] = 4 })
	f.finishWithOneVideo(good, good)

	eventually(t, "the video on disk", func() bool { return f.job(created.ID).Status == portalDone })
	done := f.job(created.ID)
	if !done.Videos[0].Ready || done.Stalled != nil {
		t.Fatalf("job = %+v, want its video ready and nothing stalled", done)
	}
	path, err := f.service.VideoPath(created.ID, "seg-003.mp4")
	if err != nil {
		t.Fatalf("VideoPath error = %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, good) {
		t.Fatalf("downloaded file differs from the portal's (err %v)", err)
	}
}

func TestResultsThatExpiredBeforeTheDownloadFailTheJobLocally(t *testing.T) {
	f := newFixture(t, true)
	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	// Studio stayed closed for a week: the job is done, the files are gone.
	f.portal.with(func() { f.portal.jobs[0].Status = portalDone })

	eventually(t, "the expiry to show", func() bool { return f.job(created.ID).Status == portalFailed })
	if failure := f.job(created.ID).Failure; failure == nil || failure.Code != FailureResultsExpired || failure.Message == "" {
		t.Fatalf("failure = %+v, want results_expired with a message", failure)
	}
}

func TestCancelAsksThePortalAndOnlyWorksOnActiveJobs(t *testing.T) {
	f := newFixture(t, true)
	ctx := context.Background()
	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	// A running job only gets a cancel request; the worker stops it later.
	f.portal.with(func() { f.portal.jobs[0].Status = "running" })
	eventually(t, "the job to run", func() bool { return f.job(created.ID).Status == "running" })

	requested, err := f.service.Cancel(ctx, created.ID)
	if err != nil {
		t.Fatalf("Cancel error = %v", err)
	}
	if requested.Status != "running" || !requested.CancelRequested {
		t.Fatalf("job after cancel = %+v, want running with the cancel pending", requested)
	}

	f.portal.with(func() { f.portal.jobs[0].Status = portalCanceled })
	eventually(t, "the cancel to land", func() bool { return f.job(created.ID).Status == portalCanceled })
	if _, err := f.service.Cancel(ctx, created.ID); cloudError(err).Code != "invalid_state" {
		t.Fatalf("Cancel of a finished job error = %v, want invalid_state", err)
	}
	if _, err := f.service.Cancel(ctx, uuid.NewString()); cloudError(err).Status != http.StatusNotFound {
		t.Fatalf("Cancel of an unknown job error = %v, want 404", err)
	}
}

func TestARevokedDeviceIsUnlinkedAndReported(t *testing.T) {
	f := newFixture(t, true)
	ctx := context.Background()
	eventually(t, "the account", func() bool { return f.service.Account(ctx).Access != nil })

	f.portal.with(func() { f.portal.revoked = true })

	eventually(t, "the revocation to show", func() bool { return !f.service.Account(ctx).Linked })
	account := f.service.Account(ctx)
	if account.Error == nil || *account.Error != "unauthorized" || account.User != nil || account.Access != nil {
		t.Fatalf("account after revocation = %+v (error %v), want unlinked with error unauthorized", account, account.Error)
	}
	stored, err := os.ReadFile(filepath.Join(f.dataDir, "cloud", "client.json"))
	if err != nil || strings.Contains(string(stored), testDeviceToken) {
		t.Fatalf("the rejected token is still on disk (err %v)", err)
	}
	if _, err := f.submit(nil); cloudError(err).Code != "not_linked" {
		t.Fatalf("Submit after revocation error = %v, want not_linked", err)
	}
}

func TestUnlinkRevokesTheDeviceAndForgetsTheToken(t *testing.T) {
	f := newFixture(t, true)
	ctx := context.Background()

	if err := f.service.Unlink(ctx); err != nil {
		t.Fatalf("Unlink error = %v", err)
	}
	f.portal.with(func() {
		if !f.portal.deleted {
			t.Fatal("the device was not revoked on the portal")
		}
	})
	account := f.service.Account(ctx)
	if account.Linked || account.Error != nil {
		t.Fatalf("account after unlink = %+v, want unlinked without an error", account)
	}

	// With the portal unreachable the local token still goes away.
	offline := newFixture(t, true)
	offline.portal.server.Close()
	if err := offline.service.Unlink(ctx); err != nil {
		t.Fatalf("Unlink while offline error = %v", err)
	}
	if offline.service.Account(ctx).Linked {
		t.Fatal("Unlink while offline left the PC linked")
	}
}

func TestAccountReportsAnUnreachablePortalWithoutUnlinking(t *testing.T) {
	f := newFixture(t, true)
	ctx := context.Background()
	eventually(t, "the account", func() bool { return f.service.Account(ctx).Access != nil })

	f.portal.server.Close()

	eventually(t, "the outage to show", func() bool {
		account := f.service.Account(ctx)
		return account.Error != nil && *account.Error == "portal_unreachable"
	})
	account := f.service.Account(ctx)
	if !account.Linked || account.User == nil || account.User.Name != "Luis" {
		t.Fatalf("account while offline = %+v, want it still linked to Luis", account)
	}
}

// An operator retries a failed job on the portal. Studio holds it as failed
// and has nothing else in flight.
func TestAnOperatorRetryOfAFailedJobIsFollowedAgain(t *testing.T) {
	for _, restart := range []bool{false, true} {
		name := "while Studio is open"
		if restart {
			name = "while Studio was closed"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, true)
			created, err := f.submit(nil)
			if err != nil {
				t.Fatalf("Submit error = %v", err)
			}
			f.portal.with(func() {
				failed := f.portal.jobs[0]
				failed.Status = portalFailed
				failed.Failure = &PortalFailure{Code: "worker_lost", Message: "Perdimos la conexión"}
			})
			eventually(t, "the failure", func() bool { return f.job(created.ID).Status == portalFailed })
			if restart {
				f.stop()
			}

			video := []byte("\x00\x00\x00\x18ftypisom-retried-video")
			f.portal.with(func() {
				retried := f.portal.jobs[0]
				retried.Status, retried.Failure = portalDone, nil
				f.portal.artifacts["artifact-1"] = video
				retried.Artifacts = []PortalArtifact{{ID: "artifact-1", Kind: "video", Variant: "viral-60-clean", Name: "seg-003.mp4", SizeBytes: int64(len(video)), SHA256: sha256Hex(video)}}
			})
			if restart {
				f.start()
			}

			eventually(t, "the retried job to be downloaded", func() bool { return f.job(created.ID).Status == portalDone })
			done := f.job(created.ID)
			if done.Failure != nil || len(done.Videos) != 1 || !done.Videos[0].Ready {
				t.Fatalf("job after the retry = %+v, want it done with its video", done)
			}
		})
	}
}

// A store that holds a finished job without its videos must not leave it
// "downloading" for ever with nothing to download.
func TestAFinishedJobWhoseVideosWereNeverReadIsPickedUp(t *testing.T) {
	f := newFixture(t, true)
	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	f.stop()
	video := []byte("\x00\x00\x00\x18ftypisom-never-read")
	var finished PortalJob
	f.portal.with(func() {
		done := f.portal.jobs[0]
		done.Status = portalDone
		f.portal.artifacts["artifact-1"] = video
		done.Artifacts = []PortalArtifact{{ID: "artifact-1", Kind: "video", Variant: "viral-60-clean", Name: "seg-003.mp4", SizeBytes: int64(len(video)), SHA256: sha256Hex(video)}}
		finished = *done
	})
	store, err := OpenStore(filepath.Join(f.dataDir, "cloud", "client.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(created.ID, func(entry *Submission) { entry.LastPortalView = &finished }); err != nil {
		t.Fatal(err)
	}

	f.start()
	eventually(t, "the video on disk", func() bool { return f.job(created.ID).Status == portalDone })
	if done := f.job(created.ID); len(done.Videos) != 1 || !done.Videos[0].Ready {
		t.Fatalf("job = %+v, want its video on this PC", done)
	}
}

func TestAnOldFailedJobIsNotWatchedForARetry(t *testing.T) {
	f := newFixture(t, true)
	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	longAgo := time.Now().Add(-retryWatchWindow - time.Hour).UnixMilli()
	f.portal.with(func() {
		failed := f.portal.jobs[0]
		failed.Status, failed.FinishedAt = portalFailed, &longAgo
		failed.Failure = &PortalFailure{Code: "timeout", Message: "Tardó demasiado"}
	})
	eventually(t, "the failure", func() bool { return f.job(created.ID).Status == portalFailed })

	before := f.calls("list")
	time.Sleep(80 * time.Millisecond)
	if asked := f.calls("list") - before; asked != 0 {
		t.Fatalf("the portal was asked %d times about a job no operator can retry any more", asked)
	}
}

// One job in flight must not make every old job cost a request and a write.
func TestSettledJobsCostNoRequestsAndNothingIsWrittenWithoutAChange(t *testing.T) {
	f := newFixture(t, true)
	ctx := context.Background()
	submitOne := func() string {
		created, err := f.submit(nil)
		if err != nil {
			t.Fatalf("Submit error = %v", err)
		}
		return created.ID
	}
	// One job waits in the queue for the whole test, so the loop keeps polling.
	movingID := submitOne()
	failedID, canceledID := submitOne(), submitOne()
	f.portal.with(func() {
		f.portal.find(failedID).Status = portalFailed
		f.portal.find(failedID).Failure = &PortalFailure{Code: "render_failed", Message: "Falló el montaje"}
	})
	if _, err := f.service.Cancel(ctx, canceledID); err != nil {
		t.Fatalf("Cancel error = %v", err)
	}
	eventually(t, "the failure", func() bool { return f.job(failedID).Status == portalFailed })
	// Let the round that began before the cancel end: it still saw a queued job.
	settled := f.calls("list")
	eventually(t, "both jobs to be settled for the sync loop", func() bool { return f.calls("list") >= settled+2 })
	// Months later the portal has purged both.
	f.portal.with(func() { f.portal.hidden[failedID], f.portal.hidden[canceledID] = true, true })
	purged := f.calls("list")
	eventually(t, "a few rounds without them", func() bool { return f.calls("list") >= purged+3 })

	storePath := filepath.Join(f.dataDir, "cloud", "client.json")
	stat := func() time.Time {
		info, err := os.Stat(storePath)
		if err != nil {
			t.Fatalf("stat client.json: %v", err)
		}
		return info.ModTime()
	}
	written := stat()
	rounds := f.calls("list")
	eventually(t, "twenty more sync rounds", func() bool { return f.calls("list") >= rounds+20 })
	if asked := f.calls("get"); asked != 0 {
		t.Fatalf("%d requests for single jobs, want 0: the list already covers what is in flight", asked)
	}
	if got := stat(); !got.Equal(written) {
		t.Fatal("client.json was rewritten although no job changed")
	}
	if kept := len(f.service.Jobs()); kept != 3 {
		t.Fatalf("jobs in the hub = %d, want the old failed and canceled ones kept", kept)
	}

	// Progress reaches the UI without touching the disk.
	stage, percent := "capturing", 40
	f.portal.with(func() {
		running := f.portal.find(movingID)
		running.Status, running.Stage, running.ProgressPercent = "running", &stage, &percent
	})
	eventually(t, "the job to run", func() bool { return f.job(movingID).Status == "running" })
	written = stat()
	f.portal.with(func() { percent = 62 })
	eventually(t, "the progress", func() bool {
		current := f.job(movingID)
		return current.Percent != nil && *current.Percent == 62
	})
	if got := stat(); !got.Equal(written) {
		t.Fatal("client.json was rewritten for a progress update")
	}

	// A change of status is saved: it has to survive a restart.
	f.portal.with(func() { f.portal.find(movingID).Status = portalCanceled })
	eventually(t, "the cancel", func() bool { return f.job(movingID).Status == portalCanceled })
	f.start()
	if restored := f.job(movingID).Status; restored != portalCanceled {
		t.Fatalf("status after a restart = %q, want canceled", restored)
	}
}

func TestAMovingJobSaysWhenThePortalCannotBeReached(t *testing.T) {
	f := newFixture(t, true)
	created, err := f.submit(nil)
	if err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	stage, percent := "capturing", 40
	f.portal.with(func() {
		running := f.portal.jobs[0]
		running.Status, running.Stage, running.ProgressPercent = "running", &stage, &percent
	})
	eventually(t, "the capture progress", func() bool { return f.job(created.ID).Status == "running" })
	if stalled := stalledReason(f.job(created.ID)); stalled != "" {
		t.Fatalf("stalled while the portal answers = %q, want none", stalled)
	}

	f.portal.with(func() { f.portal.outages["list"] = 1 << 30 })
	eventually(t, "the outage to show on the job", func() bool { return stalledReason(f.job(created.ID)) == StalledPortal })
	stale := f.job(created.ID)
	if stale.Status != "running" || stale.Percent == nil || *stale.Percent != 40 {
		t.Fatalf("job during the outage = %+v, want its last known status kept", stale)
	}

	f.portal.with(func() { f.portal.outages["list"] = 0 })
	eventually(t, "the job to be fresh again", func() bool { return stalledReason(f.job(created.ID)) == "" })
}

// A store file that lost its content, as after a power cut.
func TestADamagedStoreDoesNotSwitchTheCloudOff(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloud", "client.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(Config{PortalURL: "https://portal.invalid", DataDir: dir})
	if err != nil {
		t.Fatalf("NewService with an empty client.json error = %v, want a working client that asks to link again", err)
	}
	if account := service.Account(context.Background()); account.Linked || len(service.Jobs()) != 0 {
		t.Fatalf("account = %+v with %d jobs, want an unlinked, empty client", account, len(service.Jobs()))
	}
}
