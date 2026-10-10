package cloudclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/rules"
)

// Portal job statuses the client reacts to.
const (
	portalAwaitingDemo = "awaiting_demo"
	portalQueued       = "queued"
	portalDone         = "done"
	portalFailed       = "failed"
	portalCanceled     = "canceled"

	demoUploadRequired = "required"
	artifactVideo      = "video"

	accountRefreshTimeout = 5 * time.Second

	// codeUploadInProgress is the portal still reading an earlier upload of the
	// same demo, usually one whose connection this PC already lost.
	codeUploadInProgress = "upload_in_progress"

	// maxDemoUploads is how many demo uploads the portal takes from one user
	// at once.
	maxDemoUploads = 2
	// maxDownloadMismatches is how often a result may arrive with the wrong
	// sha256 before the job fails on this PC.
	maxDownloadMismatches = 3
	// retryWatchWindow is how long a failed or canceled job is checked for an
	// operator retry. The portal lists finished jobs for 14 days.
	retryWatchWindow = 14 * 24 * time.Hour
	// abandonedJobLifetime bounds the cancel of an abandoned job: the portal
	// deletes a job that never got its demo after 24 h.
	abandonedJobLifetime = 25 * time.Hour
)

// Local statuses and failure codes: phases only this PC knows about.
const (
	StatusUploadingDemo = "uploading_demo"
	StatusDownloading   = "downloading"

	// FailureDemoUpload means the demo never reached the portal.
	FailureDemoUpload = "demo_upload_failed"
	// FailureResultsExpired means the job finished but the portal deleted
	// its videos before this Studio downloaded them.
	FailureResultsExpired = "results_expired"
	// FailureDownload means the result arrived damaged every time.
	FailureDownload = "download_failed"
	// Refusals of the demo upload that the UI words on their own.
	FailureLimitUploads = "limit_uploads"
	FailureLimitStorage = "limit_storage"
	FailureStorageFull  = "cloud_storage_full"

	// Why a job that has not finished is not advancing (LocalJob.Stalled).
	StalledPortal     = "portal_unreachable"
	StalledDemoUpload = "demo_upload"
	StalledDownload   = "download"

	errorPortalUnreachable = "portal_unreachable"
	errorUnauthorized      = "unauthorized"
)

// localFailureMessages is what the user reads for a failure the portal does
// not know about.
var localFailureMessages = map[string]string{
	FailureDemoUpload:     "No se pudo subir la demo a la nube.",
	FailureResultsExpired: "Los vídeos de este trabajo ya no están disponibles en la nube.",
	FailureDownload:       "El vídeo llegó dañado a este PC varias veces y no se pudo guardar.",
	FailureLimitUploads:   "Ya hay dos demos tuyas subiendo a la nube.",
	FailureLimitStorage:   "Tus demos ocupan todo tu espacio en la nube.",
	FailureStorageFull:    "La nube se ha quedado sin espacio por ahora.",
}

// uploadRefusals are the portal's answers to a demo upload that waiting does
// not fix and that the user should read as such.
var uploadRefusals = map[string]bool{
	FailureLimitUploads: true,
	FailureLimitStorage: true,
	FailureStorageFull:  true,
}

// Error is a failure of the loopback cloud API, with the HTTP status and the
// code the Studio UI branches on.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func errNotLinked() *Error {
	return &Error{Status: http.StatusConflict, Code: "not_linked", Message: "Conecta tu cuenta de ClipHub para grabar en la nube."}
}

func errPortalUnreachable() *Error {
	return &Error{Status: http.StatusServiceUnavailable, Code: errorPortalUnreachable, Message: "No se pudo conectar con ClipHub. Revisa tu conexión."}
}

func errNotFound() *Error {
	return &Error{Status: http.StatusNotFound, Code: "not_found", Message: "Ese trabajo de la nube no existe en este equipo."}
}

func errInvalidState(message string) *Error {
	return &Error{Status: http.StatusConflict, Code: "invalid_state", Message: message}
}

// errDemoGone means the demo to upload is no longer on this PC, which no
// retry can fix.
var errDemoGone = errors.New("cloudclient: the local demo is gone")

// JobSource loads a local job with its kill plan. The job repository
// satisfies it.
type JobSource interface {
	Get(ctx context.Context, id uuid.UUID) (job.Job, error)
}

// DemoFiles resolves a storage key to a file on disk. *storage.Local
// satisfies it.
type DemoFiles interface {
	ResolvePath(key string) (string, error)
}

// Timing holds the client's intervals. The zero value of a field means its
// default; tests shrink them.
type Timing struct {
	AccountTTL time.Duration
	ActiveSync time.Duration
	IdleSync   time.Duration
	LinkPoll   time.Duration
	// RetryWatch is how often failed and canceled jobs are checked for an
	// operator retry while nothing else is in flight.
	RetryWatch time.Duration
	// UploadRetry is the pause after a failed demo upload. It doubles up to
	// UploadRetryMax, until UploadWindow has passed since the submit.
	UploadRetry    time.Duration
	UploadRetryMax time.Duration
	UploadWindow   time.Duration
	// DownloadRetryMax caps the pause between attempts to fetch a result.
	DownloadRetryMax time.Duration
}

func (t Timing) withDefaults() Timing {
	setDuration := func(target *time.Duration, fallback time.Duration) {
		if *target <= 0 {
			*target = fallback
		}
	}
	setDuration(&t.AccountTTL, 20*time.Second)
	setDuration(&t.ActiveSync, 5*time.Second)
	setDuration(&t.IdleSync, 60*time.Second)
	setDuration(&t.RetryWatch, 5*time.Minute)
	setDuration(&t.UploadRetry, 5*time.Second)
	setDuration(&t.UploadRetryMax, 2*time.Minute)
	// The portal deletes a job that waited 24 h for its demo.
	setDuration(&t.UploadWindow, 24*time.Hour)
	setDuration(&t.DownloadRetryMax, 5*time.Minute)
	return t
}

// backoff doubles base once per failure, up to limit.
func backoff(base time.Duration, failures int, limit time.Duration) time.Duration {
	pause := base
	for range failures {
		if pause >= limit {
			break
		}
		pause *= 2
	}
	return min(pause, limit)
}

// Config wires a Service.
type Config struct {
	PortalURL string
	// DataDir is the orchestrator data directory. The store lives at
	// <DataDir>/cloud/client.json and results under <DataDir>/cloud/results.
	DataDir       string
	Jobs          JobSource
	Files         DemoFiles
	StudioVersion string
	// DeviceName is shown to the user when approving the link. Empty means
	// the host name.
	DeviceName string
	Timing     Timing
}

type downloadRetry struct {
	failures int
	nextAt   time.Time
}

type runningDownload struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type pendingLink struct {
	linkID    string
	pollToken string
	status    string
	userCode  string
	verifyURL string
	expiresAt time.Time
	interval  time.Duration
}

// Service is the cloud client of one Studio installation.
type Service struct {
	portal        *Portal
	store         *Store
	jobs          JobSource
	files         DemoFiles
	resultsDir    string
	studioVersion string
	deviceName    string
	timing        Timing
	now           func() time.Time

	mu        sync.Mutex
	root      context.Context
	me        *Me
	meAt      time.Time
	lastError string
	link      *pendingLink
	// percent is the progress of a local phase (demo upload or result
	// download) per cloud job id.
	percent map[string]int
	uploads map[string]context.CancelFunc
	// uploadRetrying holds the demo uploads waiting to try again.
	uploadRetrying map[string]bool
	// downloadRetry holds the result downloads that failed, per cloud job id.
	downloadRetry map[string]downloadRetry
	// downloading holds the download in flight per cloud job id.
	downloading map[string]*runningDownload
	// removing holds the jobs being removed: no download may start for them.
	removing map[string]bool
	// portalDown is true while the job list cannot be read from the portal.
	portalDown bool
	// watchedAt is when failed and canceled jobs were last checked for a retry.
	watchedAt time.Time
	// uploadSlots bounds the demo uploads running at once.
	uploadSlots chan struct{}
	wake        chan struct{}
	// refresh serializes account refreshes so two polls share one request.
	refresh sync.Mutex
	// work counts the background goroutines, so a stopped service can be
	// waited for.
	work sync.WaitGroup
}

// NewService opens the store and builds the service. Background work starts
// with Start.
func NewService(cfg Config) (*Service, error) {
	store, err := OpenStore(filepath.Join(cfg.DataDir, "cloud", "client.json"))
	if err != nil {
		return nil, err
	}
	deviceName := strings.TrimSpace(cfg.DeviceName)
	if deviceName == "" {
		deviceName, _ = os.Hostname()
	}
	if deviceName == "" {
		deviceName = "ClipHub Studio"
	}
	return &Service{
		portal:         NewPortal(cfg.PortalURL),
		store:          store,
		jobs:           cfg.Jobs,
		files:          cfg.Files,
		resultsDir:     filepath.Join(cfg.DataDir, "cloud", "results"),
		studioVersion:  cfg.StudioVersion,
		deviceName:     truncateRunes(deviceName, 60),
		timing:         cfg.Timing.withDefaults(),
		now:            time.Now,
		root:           context.Background(),
		percent:        map[string]int{},
		uploads:        map[string]context.CancelFunc{},
		uploadRetrying: map[string]bool{},
		downloadRetry:  map[string]downloadRetry{},
		downloading:    map[string]*runningDownload{},
		removing:       map[string]bool{},
		uploadSlots:    make(chan struct{}, maxDemoUploads),
		wake:           make(chan struct{}, 1),
	}, nil
}

// Start resumes demo uploads a previous run left unfinished and follows the
// submitted jobs until ctx ends.
func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	s.root = ctx
	s.mu.Unlock()
	for _, submission := range s.store.Submissions() {
		if !submission.DemoUploaded && submission.LocalFailure == "" {
			s.startDemoUpload(submission.CloudJobID)
		}
	}
	s.work.Go(func() { s.syncLoop(ctx) })
}

// wait blocks until the background work has ended. It only returns once the
// context given to Start is done.
func (s *Service) wait() {
	s.work.Wait()
}

func (s *Service) background() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.root
}

func (s *Service) nudge() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// ---- account and link ----

// LinkView is a device link in progress, or how the last one ended.
type LinkView struct {
	Status    string `json:"status"`
	UserCode  string `json:"user_code"`
	VerifyURL string `json:"verify_url"`
	ExpiresAt string `json:"expires_at"`
}

// AccountLimits are the user's limits on the portal.
type AccountLimits struct {
	MaxActive    int `json:"max_active"`
	DailySeconds int `json:"daily_seconds"`
}

// AccountUsage is what the user has consumed and committed.
type AccountUsage struct {
	Active           int `json:"active"`
	SecondsLast24h   int `json:"seconds_last_24h"`
	SecondsCommitted int `json:"seconds_committed"`
}

// AccountQueue is the state of the cloud queue.
type AccountQueue struct {
	State       string          `json:"state"`
	Queued      int             `json:"queued"`
	WaitSeconds map[string]*int `json:"wait_seconds"`
}

// Account is the document of GET /api/cloud/account.
type Account struct {
	PortalURL string         `json:"portal_url"`
	Linked    bool           `json:"linked"`
	Link      *LinkView      `json:"link"`
	User      *StoredUser    `json:"user"`
	Access    *string        `json:"access"`
	Limits    *AccountLimits `json:"limits"`
	Usage     *AccountUsage  `json:"usage"`
	Kinds     []string       `json:"kinds"`
	Queue     *AccountQueue  `json:"queue"`
	Error     *string        `json:"error"`
}

// Account returns the account document, refreshing the portal's part when
// it is older than the cache lifetime.
func (s *Service) Account(ctx context.Context) Account {
	if token := s.store.Token(); token != "" {
		// The UI polls this document; an unreachable portal must not hold
		// the answer for the length of a connect timeout.
		refreshCtx, cancel := context.WithTimeout(ctx, accountRefreshTimeout)
		defer cancel()
		s.refreshAccount(refreshCtx, token)
	}
	return s.accountView()
}

func (s *Service) refreshAccount(ctx context.Context, token string) {
	s.refresh.Lock()
	defer s.refresh.Unlock()
	s.mu.Lock()
	fresh := !s.meAt.IsZero() && s.now().Sub(s.meAt) < s.timing.AccountTTL
	s.mu.Unlock()
	if fresh {
		return
	}
	me, err := s.portal.Me(ctx, token)
	if errors.Is(err, ErrUnauthorized) {
		s.dropToken()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meAt = s.now()
	if err != nil {
		s.lastError = errorPortalUnreachable
		return
	}
	s.me, s.lastError = &me, ""
}

// dropToken forgets a token the portal no longer accepts.
func (s *Service) dropToken() {
	if err := s.store.Unlink(); err != nil {
		log.Printf("cloudclient: forget rejected device token: %v", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.me, s.meAt, s.lastError, s.portalDown = nil, time.Time{}, errorUnauthorized, false
}

func (s *Service) invalidateAccount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meAt = time.Time{}
}

func (s *Service) accountView() Account {
	token := s.store.Token()
	user := s.store.User()
	s.mu.Lock()
	defer s.mu.Unlock()
	view := Account{PortalURL: s.portal.BaseURL(), Linked: token != "", Kinds: []string{}}
	if s.lastError != "" {
		lastError := s.lastError
		view.Error = &lastError
	}
	if !view.Linked {
		if s.link != nil {
			view.Link = &LinkView{
				Status:    s.link.status,
				UserCode:  s.link.userCode,
				VerifyURL: s.link.verifyURL,
				ExpiresAt: s.link.expiresAt.UTC().Format(time.RFC3339),
			}
		}
		return view
	}
	view.User = user
	if s.me == nil {
		return view
	}
	access := s.me.Access
	view.User = &StoredUser{Name: s.me.User.Name, Email: s.me.User.Email}
	view.Access = &access
	view.Limits = &AccountLimits{MaxActive: s.me.Limits.MaxActive, DailySeconds: s.me.Limits.DailySeconds}
	view.Usage = &AccountUsage{
		Active:           s.me.Usage.Active,
		SecondsLast24h:   s.me.Usage.SecondsLast24h,
		SecondsCommitted: s.me.Usage.SecondsCommitted,
	}
	if s.me.Kinds != nil {
		view.Kinds = slices.Clone(s.me.Kinds)
	}
	view.Queue = &AccountQueue{State: s.me.Queue.State, Queued: s.me.Queue.Queued, WaitSeconds: s.me.Queue.WaitSeconds}
	if view.Queue.WaitSeconds == nil {
		view.Queue.WaitSeconds = map[string]*int{}
	}
	return view
}

// StartLink opens a device link and polls the portal in the background
// until the user approves or denies it in the browser, or the code expires.
func (s *Service) StartLink(ctx context.Context) (Account, error) {
	if s.store.Token() != "" {
		return s.Account(ctx), nil
	}
	s.mu.Lock()
	pending := s.link != nil && s.link.status == "pending" && s.now().Before(s.link.expiresAt)
	s.mu.Unlock()
	if pending {
		return s.accountView(), nil
	}
	started, err := s.portal.LinkStart(ctx, s.deviceName)
	if err != nil {
		return Account{}, s.portalError(err)
	}
	interval := s.timing.LinkPoll
	if interval <= 0 {
		interval = time.Duration(max(started.IntervalSeconds, 1)) * time.Second
	}
	link := &pendingLink{
		linkID:    started.LinkID,
		pollToken: started.PollToken,
		status:    "pending",
		userCode:  started.UserCode,
		verifyURL: started.VerifyURL,
		expiresAt: time.UnixMilli(started.ExpiresAt),
		interval:  interval,
	}
	s.mu.Lock()
	s.link, s.lastError = link, ""
	s.mu.Unlock()
	root := s.background()
	s.work.Go(func() { s.pollLink(root, link) })
	return s.accountView(), nil
}

func (s *Service) pollLink(ctx context.Context, link *pendingLink) {
	finish := func(status string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.link == link {
			link.status = status
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(link.interval):
		}
		s.mu.Lock()
		current := s.link == link
		s.mu.Unlock()
		if !current {
			return
		}
		if !s.now().Before(link.expiresAt) {
			finish("expired")
			return
		}
		poll, err := s.portal.LinkPoll(ctx, link.linkID, link.pollToken)
		if err != nil {
			// The portal may be briefly unreachable; the code's expiry bounds this.
			continue
		}
		switch poll.Status {
		case "approved":
			user := StoredUser{}
			if poll.User != nil {
				user = StoredUser{Name: poll.User.Name, Email: poll.User.Email}
			}
			if poll.DeviceToken == "" {
				finish("denied")
				return
			}
			if err := s.store.Link(poll.DeviceToken, user); err != nil {
				log.Printf("cloudclient: store device token: %v", err)
				finish("denied")
				return
			}
			s.mu.Lock()
			if s.link == link {
				s.link = nil
			}
			s.meAt, s.lastError = time.Time{}, ""
			s.mu.Unlock()
			s.nudge()
			return
		case "denied", "expired":
			finish(poll.Status)
			return
		}
	}
}

// Unlink revokes this device on the portal when it can be reached and
// always forgets the local token.
func (s *Service) Unlink(ctx context.Context) error {
	token := s.store.Token()
	s.mu.Lock()
	s.link, s.me, s.meAt, s.lastError, s.portalDown = nil, nil, time.Time{}, "", false
	cancels := make([]context.CancelFunc, 0, len(s.uploads))
	for _, cancel := range s.uploads {
		cancels = append(cancels, cancel)
	}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	// Without a token a demo upload can never finish. The portal job is
	// canceled if this PC is linked to the same account again in time.
	for _, submission := range s.store.Submissions() {
		if submission.DemoUploaded || submission.LocalFailure != "" {
			continue
		}
		s.failLocally(submission.CloudJobID, FailureDemoUpload, true)
	}
	if token != "" {
		if err := s.portal.DeleteDevice(ctx, token); err != nil && !errors.Is(err, ErrUnauthorized) {
			log.Printf("cloudclient: revoke device on the portal: %v", err)
		}
	}
	return s.store.Unlink()
}

// portalError turns a failed portal call into the loopback API's error:
// the portal's own status and code when it answered, 503 when it did not.
func (s *Service) portalError(err error) error {
	var apiErr *APIError
	switch {
	case errors.Is(err, ErrUnauthorized):
		s.dropToken()
		return errNotLinked()
	// A coded answer is the portal speaking ("the cloud is full"), also
	// with a 503; a bare 5xx is the portal or a proxy in trouble.
	case errors.As(err, &apiErr) && (apiErr.Status < 500 || apiErr.Code != ""):
		return &Error{Status: apiErr.Status, Code: apiErr.Code, Message: apiErr.Message}
	default:
		return errPortalUnreachable()
	}
}

// ---- submit ----

// SubmitInput is the body of POST /api/cloud/jobs: the local job plus what
// the UI would send to the local generate endpoint.
type SubmitInput struct {
	JobID      string          `json:"job_id"`
	Kind       string          `json:"kind"`
	Title      string          `json:"title"`
	Preset     string          `json:"preset"`
	Music      json.RawMessage `json:"music"`
	Edit       json.RawMessage `json:"edit"`
	SegmentIDs []string        `json:"segment_ids"`
}

type specWindow struct {
	ID        string `json:"id"`
	TickStart int    `json:"tickStart"`
	TickEnd   int    `json:"tickEnd"`
}

type specGenerate struct {
	Preset     string          `json:"preset"`
	Music      json.RawMessage `json:"music"`
	SegmentIDs []string        `json:"segment_ids"`
	Edit       json.RawMessage `json:"edit"`
}

// jobSpec is what the portal stores and the worker executes (contract D).
type jobSpec struct {
	Version       int         `json:"version"`
	TargetSteamID string      `json:"targetSteamId"`
	Rules         rules.Rules `json:"rules"`
	Capture       struct {
		Tickrate int          `json:"tickrate"`
		Windows  []specWindow `json:"windows"`
	} `json:"capture"`
	Generate specGenerate `json:"generate"`
	Client   struct {
		StudioVersion string `json:"studioVersion"`
		PlanSchema    string `json:"planSchema"`
	} `json:"client"`
}

// Submit sends a parsed local job to the cloud. It returns as soon as the
// portal has accepted the job; a demo the portal does not hold yet is
// uploaded in the background.
func (s *Service) Submit(ctx context.Context, in SubmitInput) (LocalJob, error) {
	token := s.store.Token()
	if token == "" {
		return LocalJob{}, errNotLinked()
	}
	localID, err := uuid.Parse(in.JobID)
	if err != nil {
		return LocalJob{}, &Error{Status: http.StatusBadRequest, Code: "invalid_request", Message: "job_id no es un identificador válido."}
	}
	local, err := s.jobs.Get(ctx, localID)
	switch {
	case errors.Is(err, job.ErrNotFound):
		return LocalJob{}, &Error{Status: http.StatusNotFound, Code: "not_found", Message: "Esa partida no existe en este equipo."}
	case err != nil:
		return LocalJob{}, fmt.Errorf("load local job: %w", err)
	case local.KillPlan == nil:
		return LocalJob{}, &Error{Status: http.StatusConflict, Code: "job_not_parsed", Message: "Esta partida todavía no está analizada."}
	}
	spec, err := buildSpec(local, in, s.studioVersion)
	if err != nil {
		return LocalJob{}, err
	}
	encodedSpec, err := json.Marshal(spec)
	if err != nil {
		return LocalJob{}, fmt.Errorf("encode job spec: %w", err)
	}
	demoPath, err := s.files.ResolvePath(local.DemoPath)
	if err != nil {
		return LocalJob{}, fmt.Errorf("resolve demo path: %w", err)
	}
	demoInfo, err := os.Stat(demoPath)
	if err != nil {
		return LocalJob{}, &Error{Status: http.StatusConflict, Code: "job_not_parsed", Message: "La demo de esta partida ya no está en este equipo."}
	}
	kind := in.Kind
	if kind == "" {
		kind = "short"
	}
	fileName := local.DemoFileName
	if fileName == "" {
		fileName = "demo.dem"
	}
	title := truncateRunes(strings.TrimSpace(in.Title), 120)
	created, err := s.portal.CreateJob(ctx, token, CreateJobRequest{
		Kind:  kind,
		Title: title,
		Demo:  PortalDemo{SHA256: local.DemoSHA256, SizeBytes: demoInfo.Size(), FileName: truncateRunes(fileName, 200)},
		Spec:  encodedSpec,
	})
	if err != nil {
		// A refusal means the cached account (access, limits, queue) is stale.
		s.invalidateAccount()
		return LocalJob{}, s.portalError(err)
	}
	submission := Submission{
		CloudJobID:     created.ID,
		LocalJobID:     localID.String(),
		Title:          title,
		Kind:           kind,
		CreatedAt:      s.now().UTC(),
		DemoUploaded:   created.DemoUpload != demoUploadRequired,
		LastPortalView: &PortalJob{ID: created.ID, Kind: kind, Title: title, Status: created.Status, CreatedAt: s.now().UnixMilli()},
		Videos:         []StoredVideo{},
	}
	if err := s.store.Add(submission); err != nil {
		return LocalJob{}, fmt.Errorf("record cloud job: %w", err)
	}
	if !submission.DemoUploaded {
		s.startDemoUpload(created.ID)
	}
	s.invalidateAccount()
	s.nudge()
	return s.localJob(submission), nil
}

func buildSpec(local job.Job, in SubmitInput, studioVersion string) (jobSpec, error) {
	plan := local.KillPlan
	segments := make(map[string]specWindow, len(plan.Segments))
	for _, segment := range plan.Segments {
		segments[segment.ID] = specWindow{ID: segment.ID, TickStart: segment.TickStart, TickEnd: segment.TickEnd}
	}
	// Locally an empty selection means every segment; the cloud wants the
	// windows spelled out, because the portal charges for exactly those.
	ids := in.SegmentIDs
	if len(ids) == 0 {
		ids = make([]string, len(plan.Segments))
		for i, segment := range plan.Segments {
			ids[i] = segment.ID
		}
	}
	var spec jobSpec
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		window, known := segments[id]
		if !known || seen[id] {
			return jobSpec{}, &Error{Status: http.StatusBadRequest, Code: "unknown_segment", Message: "La jugada " + id + " no existe en esta partida o está repetida."}
		}
		seen[id] = true
		spec.Capture.Windows = append(spec.Capture.Windows, window)
	}
	if len(spec.Capture.Windows) == 0 {
		return jobSpec{}, &Error{Status: http.StatusBadRequest, Code: "unknown_segment", Message: "Esta partida no tiene jugadas que grabar."}
	}
	music, err := objectOrNull(in.Music, true)
	if err != nil {
		return jobSpec{}, &Error{Status: http.StatusBadRequest, Code: "invalid_request", Message: "music no es válido."}
	}
	edit, err := objectOrNull(in.Edit, false)
	if err != nil {
		return jobSpec{}, &Error{Status: http.StatusBadRequest, Code: "invalid_request", Message: "edit no es válido."}
	}
	spec.Version = 1
	spec.TargetSteamID = local.TargetSteamID
	spec.Rules = local.Rules
	spec.Capture.Tickrate = plan.Demo.Tickrate
	spec.Generate = specGenerate{Preset: in.Preset, Music: music, SegmentIDs: ids, Edit: edit}
	spec.Client.StudioVersion = studioVersion
	spec.Client.PlanSchema = plan.SchemaVersion
	return spec, nil
}

// objectOrNull passes a JSON object through and maps an absent value to
// null. The local generate endpoint also accepts music as a bare track key;
// the portal only takes objects, so the key is wrapped.
func objectOrNull(raw json.RawMessage, allowKey bool) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	switch {
	case len(trimmed) == 0 || string(trimmed) == "null":
		return json.RawMessage("null"), nil
	case trimmed[0] == '{' && json.Valid(trimmed):
		return json.RawMessage(trimmed), nil
	case allowKey && trimmed[0] == '"':
		var key string
		if err := json.Unmarshal(trimmed, &key); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"key": key})
	default:
		return nil, errors.New("not a JSON object")
	}
}

// ---- demo upload ----

func (s *Service) startDemoUpload(cloudJobID string) {
	ctx, cancel := context.WithCancel(s.background())
	s.mu.Lock()
	if _, running := s.uploads[cloudJobID]; running {
		s.mu.Unlock()
		cancel()
		return
	}
	s.uploads[cloudJobID] = cancel
	s.mu.Unlock()
	s.work.Go(func() {
		defer func() {
			cancel()
			s.mu.Lock()
			delete(s.uploads, cloudJobID)
			delete(s.uploadRetrying, cloudJobID)
			s.mu.Unlock()
		}()
		s.uploadDemo(ctx, cloudJobID)
	})
}

// uploadDemo sends the demo and keeps trying while waiting can fix the
// failure (a laptop that slept, a portal that is restarting). A refusal, or
// UploadWindow without success, fails the submission on this PC.
func (s *Service) uploadDemo(ctx context.Context, cloudJobID string) {
	select {
	case s.uploadSlots <- struct{}{}:
		defer func() { <-s.uploadSlots }()
	case <-ctx.Done():
		return
	}
	for failures := 0; ; failures++ {
		err := s.tryDemoUpload(ctx, cloudJobID)
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			// Studio is closing (the next start resumes the upload), or the
			// user canceled the job or unlinked, which settles it elsewhere.
			return
		}
		submission, ok := s.store.Submission(cloudJobID)
		if !ok {
			return
		}
		if !retryableUpload(err) || s.now().Sub(submission.CreatedAt) >= s.timing.UploadWindow {
			s.abandonUpload(cloudJobID, err)
			return
		}
		log.Printf("cloudclient: demo upload for cloud job %s failed, trying again: %v", cloudJobID, err)
		s.setUploadRetrying(cloudJobID, true)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff(s.timing.UploadRetry, failures, s.timing.UploadRetryMax)):
		}
		s.setUploadRetrying(cloudJobID, false)
	}
}

func (s *Service) setUploadRetrying(cloudJobID string, waiting bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if waiting {
		s.uploadRetrying[cloudJobID] = true
		return
	}
	delete(s.uploadRetrying, cloudJobID)
}

// retryableUpload reports whether sending the demo again can succeed.
func retryableUpload(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.Code == codeUploadInProgress {
			return true
		}
		if uploadRefusals[apiErr.Code] {
			return false
		}
	}
	return retryable(err)
}

// abandonUpload fails the submission on this PC with the code the UI words,
// and leaves the portal job to be canceled so it stops holding one of the
// user's active slots.
func (s *Service) abandonUpload(cloudJobID string, cause error) {
	log.Printf("cloudclient: demo upload for cloud job %s given up: %v", cloudJobID, cause)
	if errors.Is(cause, ErrUnauthorized) {
		s.dropToken()
	}
	code := FailureDemoUpload
	var apiErr *APIError
	if errors.As(cause, &apiErr) && uploadRefusals[apiErr.Code] {
		code = apiErr.Code
	}
	// A 404 means the portal has no such job any more: nothing to cancel.
	gone := apiErr != nil && apiErr.Status == http.StatusNotFound
	s.failLocally(cloudJobID, code, !gone)
	s.nudge()
}

// failLocally ends a submission on this PC. abandon also queues the cancel
// of its portal job, which the sync loop repeats until the portal answers.
func (s *Service) failLocally(cloudJobID, code string, abandon bool) {
	if _, err := s.store.Update(cloudJobID, func(submission *Submission) {
		submission.LocalFailure = code
	}); err != nil {
		log.Printf("cloudclient: record local failure %s of cloud job %s: %v", code, cloudJobID, err)
	}
	s.clearPercent(cloudJobID)
	if !abandon {
		return
	}
	if err := s.store.Abandon(cloudJobID, s.now()); err != nil {
		log.Printf("cloudclient: record abandoned cloud job %s: %v", cloudJobID, err)
	}
}

// cancelAbandoned cancels a portal job this PC gave up on. A nil error means
// the job is settled there: canceled now, already finished, or gone.
func (s *Service) cancelAbandoned(ctx context.Context, token, cloudJobID string) error {
	_, err := s.portal.CancelJob(ctx, token, cloudJobID)
	var apiErr *APIError
	if err != nil && (!errors.As(err, &apiErr) || retryable(err)) {
		return err
	}
	if settleErr := s.store.Settle(cloudJobID); settleErr != nil {
		log.Printf("cloudclient: forget abandoned cloud job %s: %v", cloudJobID, settleErr)
	}
	s.invalidateAccount()
	return nil
}

// settleAbandoned retries the pending cancels. It stops at the first one the
// portal does not answer: the next round tries again.
func (s *Service) settleAbandoned(ctx context.Context, token string) {
	for _, abandoned := range s.store.Abandoned() {
		if s.now().Sub(abandoned.Since) > abandonedJobLifetime {
			if err := s.store.Settle(abandoned.CloudJobID); err != nil {
				log.Printf("cloudclient: forget abandoned cloud job %s: %v", abandoned.CloudJobID, err)
			}
			continue
		}
		err := s.cancelAbandoned(ctx, token, abandoned.CloudJobID)
		if errors.Is(err, ErrUnauthorized) {
			s.dropToken()
			return
		}
		if err != nil {
			return
		}
	}
}

func (s *Service) tryDemoUpload(ctx context.Context, cloudJobID string) error {
	submission, ok := s.store.Submission(cloudJobID)
	if !ok {
		return nil
	}
	token := s.store.Token()
	if token == "" {
		return ErrUnauthorized
	}
	localID, err := uuid.Parse(submission.LocalJobID)
	if err != nil {
		return fmt.Errorf("invalid local job id %q", submission.LocalJobID)
	}
	local, err := s.jobs.Get(ctx, localID)
	if errors.Is(err, job.ErrNotFound) {
		return fmt.Errorf("%w: the local job was deleted", errDemoGone)
	}
	if err != nil {
		return err
	}
	demoPath, err := s.files.ResolvePath(local.DemoPath)
	if err != nil {
		return fmt.Errorf("%w: %v", errDemoGone, err)
	}
	file, err := os.Open(demoPath) //nolint:gosec // the path was resolved inside the local storage root
	if err != nil {
		return fmt.Errorf("%w: %v", errDemoGone, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	size := info.Size()
	status, err := s.portal.PutDemo(ctx, token, DemoUpload{
		JobID: cloudJobID,
		Body:  file,
		Size:  size,
		Progress: func(sent int64) {
			if size > 0 {
				s.setPercent(cloudJobID, int(sent*100/size))
			}
		},
	})
	var apiErr *APIError
	var moved *PortalJob
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusConflict && apiErr.Code != codeUploadInProgress {
		// An earlier attempt got through and only its answer was lost.
		current, getErr := s.portal.GetJob(ctx, token, cloudJobID)
		if getErr != nil || current.Status == portalAwaitingDemo {
			return err
		}
		moved, err = &current, nil
	}
	if err != nil {
		return err
	}
	s.clearPercent(cloudJobID)
	if _, err := s.store.Update(cloudJobID, func(entry *Submission) {
		entry.DemoUploaded, entry.UploadPercent = true, 100
		switch {
		case moved != nil:
			// The job may have finished meanwhile: take its videos with its status.
			entry.LastPortalView = moved
			mergeVideos(entry, *moved)
		case entry.LastPortalView != nil && status != "":
			entry.LastPortalView.Status = status
		}
	}); err != nil {
		log.Printf("cloudclient: record demo upload: %v", err)
	}
	s.nudge()
	return nil
}

func (s *Service) setPercent(cloudJobID string, percent int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.percent[cloudJobID] = min(max(percent, 0), 100)
}

func (s *Service) clearPercent(cloudJobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.percent, cloudJobID)
}

// ---- jobs ----

// LocalQueue is where a queued job stands. Times are RFC 3339.
type LocalQueue struct {
	Position         int     `json:"position"`
	EstimatedStartAt *string `json:"estimated_start_at"`
	EstimatedDoneAt  *string `json:"estimated_done_at"`
	State            string  `json:"state"`
}

// LocalVideo is one result video and whether it is on this PC yet.
type LocalVideo struct {
	Name      string `json:"name"`
	Variant   string `json:"variant"`
	SizeBytes int64  `json:"size_bytes"`
	Ready     bool   `json:"ready"`
}

// LocalJob is one cloud job as the Studio UI sees it (LocalCloudJob).
type LocalJob struct {
	ID              string         `json:"id"`
	LocalJobID      string         `json:"local_job_id"`
	Kind            string         `json:"kind"`
	Title           string         `json:"title"`
	Status          string         `json:"status"`
	Stage           *string        `json:"stage"`
	Percent         *int           `json:"percent"`
	Queue           *LocalQueue    `json:"queue"`
	Failure         *PortalFailure `json:"failure"`
	CancelRequested bool           `json:"cancel_requested"`
	// Stalled says why an unfinished job is not advancing right now: the
	// portal cannot be reached (the status is the last one known), or the
	// demo upload or the result download is waiting to try again.
	Stalled    *string      `json:"stalled"`
	CreatedAt  string       `json:"created_at"`
	FinishedAt *string      `json:"finished_at"`
	Videos     []LocalVideo `json:"videos"`
}

// Jobs lists the cloud jobs this Studio submitted, newest first.
func (s *Service) Jobs() []LocalJob {
	submissions := s.store.Submissions()
	jobs := make([]LocalJob, 0, len(submissions))
	for _, submission := range submissions {
		jobs = append(jobs, s.localJob(submission))
	}
	return jobs
}

func rfc3339(ms *int64) *string {
	if ms == nil {
		return nil
	}
	formatted := time.UnixMilli(*ms).UTC().Format(time.RFC3339)
	return &formatted
}

// localJob merges what the portal last said with what only this PC knows:
// the demo upload, the downloads and local failures.
func (s *Service) localJob(submission Submission) LocalJob {
	view := LocalJob{
		ID:         submission.CloudJobID,
		LocalJobID: submission.LocalJobID,
		Kind:       submission.Kind,
		Title:      submission.Title,
		CreatedAt:  submission.CreatedAt.UTC().Format(time.RFC3339),
		Videos:     make([]LocalVideo, 0, len(submission.Videos)),
	}
	for _, video := range submission.Videos {
		view.Videos = append(view.Videos, LocalVideo{Name: video.Name, Variant: video.Variant, SizeBytes: video.Size, Ready: video.Ready})
	}
	s.mu.Lock()
	localPercent, hasLocalPercent := s.percent[submission.CloudJobID]
	uploadRetrying := s.uploadRetrying[submission.CloudJobID]
	_, downloadRetrying := s.downloadRetry[submission.CloudJobID]
	portalDown := s.portalDown
	s.mu.Unlock()
	portal := submission.LastPortalView
	if portal != nil {
		view.CancelRequested = portal.CancelRequested
		view.FinishedAt = rfc3339(portal.FinishedAt)
	}
	switch {
	case submission.LocalFailure != "":
		view.Status = portalFailed
		view.Failure = &PortalFailure{Code: submission.LocalFailure, Message: localFailureMessages[submission.LocalFailure]}
	case portal != nil && portal.Status == portalCanceled:
		// A cancel wins over a demo upload that was still running.
		view.Status = portalCanceled
	case !submission.DemoUploaded || portal == nil || portal.Status == portalAwaitingDemo:
		view.Status = StatusUploadingDemo
		percent := submission.UploadPercent
		if hasLocalPercent {
			percent = localPercent
		}
		view.Percent = &percent
	case portal.Status == portalDone && !allVideosReady(submission):
		view.Status = StatusDownloading
		percent := 0
		if hasLocalPercent {
			percent = localPercent
		}
		view.Percent = &percent
	default:
		view.Status = portal.Status
		view.Stage = portal.Stage
		view.Percent = portal.ProgressPercent
		view.Failure = portal.Failure
		if portal.Status == portalQueued && portal.Queue != nil {
			view.Queue = &LocalQueue{
				Position:         portal.Queue.Position,
				EstimatedStartAt: rfc3339(portal.Queue.EstimatedStartAt),
				EstimatedDoneAt:  rfc3339(portal.Queue.EstimatedDoneAt),
				State:            portal.Queue.State,
			}
		}
	}
	stalled := func(reason string) *string { return &reason }
	switch {
	case terminal(view.Status):
	case view.Status == StatusDownloading:
		if downloadRetrying {
			view.Stalled = stalled(StalledDownload)
		}
	case portalDown:
		view.Stalled = stalled(StalledPortal)
	case view.Status == StatusUploadingDemo && uploadRetrying:
		view.Stalled = stalled(StalledDemoUpload)
	}
	return view
}

func allVideosReady(submission Submission) bool {
	if len(submission.Videos) == 0 {
		return false
	}
	for _, video := range submission.Videos {
		if !video.Ready {
			return false
		}
	}
	return true
}

func terminal(status string) bool {
	return status == portalDone || status == portalFailed || status == portalCanceled
}

// Cancel cancels a cloud job. A running job stops within a heartbeat of the
// worker; the returned view says whether the cancel is still pending.
func (s *Service) Cancel(ctx context.Context, cloudJobID string) (LocalJob, error) {
	submission, ok := s.store.Submission(cloudJobID)
	if !ok {
		return LocalJob{}, errNotFound()
	}
	status := s.localJob(submission).Status
	abandoned := s.store.IsAbandoned(cloudJobID)
	switch {
	case status == StatusDownloading:
		return LocalJob{}, errInvalidState("Este trabajo ya terminó en la nube. Solo falta descargarlo.")
	case terminal(status) && !abandoned:
		return LocalJob{}, errInvalidState("Este trabajo ya terminó.")
	}
	token := s.store.Token()
	if token == "" {
		return LocalJob{}, errNotLinked()
	}
	if abandoned {
		// It failed on this PC, but the portal still counts it as active.
		if err := s.cancelAbandoned(ctx, token, cloudJobID); err != nil {
			return LocalJob{}, s.portalError(err)
		}
		return s.localJob(submission), nil
	}
	result, err := s.portal.CancelJob(ctx, token, cloudJobID)
	if err != nil {
		return LocalJob{}, s.portalError(err)
	}
	s.mu.Lock()
	stopUpload := s.uploads[cloudJobID]
	s.mu.Unlock()
	if stopUpload != nil && result.Status == portalCanceled {
		stopUpload()
	}
	if _, err := s.store.Update(cloudJobID, func(entry *Submission) {
		if entry.LastPortalView == nil {
			entry.LastPortalView = &PortalJob{ID: cloudJobID}
		}
		entry.LastPortalView.Status = result.Status
		entry.LastPortalView.CancelRequested = result.CancelRequested
		if result.Status == portalCanceled {
			// The portal's cancel is the outcome, not the upload it cut short.
			entry.LocalFailure = ""
		}
	}); err != nil {
		return LocalJob{}, fmt.Errorf("record cancel: %w", err)
	}
	s.invalidateAccount()
	s.nudge()
	updated, _ := s.store.Submission(cloudJobID)
	return s.localJob(updated), nil
}

// Remove deletes the local record of a job and its downloaded videos. The
// job must be finished on the portal: one that is still active there must be
// canceled first, one whose download does not complete can be removed.
func (s *Service) Remove(_ context.Context, cloudJobID string) error {
	submission, ok := s.store.Submission(cloudJobID)
	if !ok {
		return errNotFound()
	}
	if status := s.localJob(submission).Status; !terminal(status) && status != StatusDownloading {
		return errInvalidState("Este trabajo sigue activo. Cancélalo antes de quitarlo.")
	}
	defer s.holdDownloads(cloudJobID)()
	if dir, err := s.resultsPath(cloudJobID); err == nil {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("remove downloaded videos: %w", err)
		}
	}
	s.clearPercent(cloudJobID)
	return s.store.Remove(cloudJobID)
}

// holdDownloads stops the download of a job being removed and keeps a new
// one from starting until the returned function runs: a download in flight
// holds its file open and would write it back after the removal.
func (s *Service) holdDownloads(cloudJobID string) func() {
	s.mu.Lock()
	s.removing[cloudJobID] = true
	running := s.downloading[cloudJobID]
	s.mu.Unlock()
	if running != nil {
		running.cancel()
		<-running.done
	}
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.removing, cloudJobID)
		delete(s.downloadRetry, cloudJobID)
	}
}

// VideoPath returns the file of a downloaded video. The name must be one of
// the job's own ready videos; it is never joined into a path.
func (s *Service) VideoPath(cloudJobID, name string) (string, error) {
	submission, ok := s.store.Submission(cloudJobID)
	if !ok {
		return "", errNotFound()
	}
	for _, video := range submission.Videos {
		if video.Name == name && video.Ready && video.Path != "" {
			return video.Path, nil
		}
	}
	return "", &Error{Status: http.StatusNotFound, Code: "not_found", Message: "Ese vídeo todavía no está en este equipo."}
}

// resultsPath is <DataDir>/cloud/results/<cloudJobId>. The id comes from the
// portal, so it must be a UUID before it names a directory.
func (s *Service) resultsPath(cloudJobID string) (string, error) {
	parsed, err := uuid.Parse(cloudJobID)
	if err != nil {
		return "", fmt.Errorf("cloud job id %q is not a UUID", cloudJobID)
	}
	return filepath.Join(s.resultsDir, parsed.String()), nil
}

// ---- sync ----

var videoNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,119}\.mp4$`)

// syncLoop refreshes the submitted jobs and downloads finished videos:
// often while something is in progress, rarely otherwise, and not at all
// while this PC is not linked.
func (s *Service) syncLoop(ctx context.Context) {
	for {
		wait := s.timing.IdleSync
		if s.store.Token() != "" {
			if s.syncOnce(ctx) {
				wait = s.timing.ActiveSync
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-time.After(wait):
		}
	}
}

// follow is what the sync loop still owes a submission.
type follow int

const (
	// followNone: settled for good, or failed on this PC.
	followNone follow = iota
	// followMoving: the portal is still working on it.
	followMoving
	// followRetry: failed or canceled, so an operator may still retry it.
	followRetry
)

func (s *Service) followOf(submission Submission) follow {
	view := submission.LastPortalView
	switch {
	case submission.LocalFailure != "":
		return followNone
	case view == nil || !terminal(view.Status):
		return followMoving
	case view.Status == portalDone:
		if len(submission.Videos) == 0 {
			// Done, but its videos were never read: the list has them.
			return followMoving
		}
		return followNone
	}
	ended := submission.CreatedAt
	if view.FinishedAt != nil {
		ended = time.UnixMilli(*view.FinishedAt)
	}
	if s.now().Sub(ended) > retryWatchWindow {
		return followNone
	}
	return followRetry
}

// retryWatchDue reports whether failed and canceled jobs should be checked
// for an operator retry now: at the first round after a start, then rarely.
func (s *Service) retryWatchDue() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.watchedAt.IsZero() || s.now().Sub(s.watchedAt) >= s.timing.RetryWatch
}

// syncOnce reports whether any job still needs following.
func (s *Service) syncOnce(ctx context.Context) bool {
	token := s.store.Token()
	s.settleAbandoned(ctx, token)
	if s.store.Token() == "" {
		// The portal rejected the token while a cancel was settled.
		return false
	}
	submissions := s.store.Submissions()
	moving, retriable := false, false
	for _, submission := range submissions {
		switch s.followOf(submission) {
		case followMoving:
			moving = true
		case followRetry:
			retriable = true
		case followNone:
		}
	}
	if moving || (retriable && s.retryWatchDue()) {
		s.refreshViews(ctx, token, submissions)
	}
	active := false
	for _, submission := range s.store.Submissions() {
		if submission.LocalFailure != "" || submission.LastPortalView == nil {
			continue
		}
		if submission.LastPortalView.Status == portalDone && !allVideosReady(submission) {
			s.download(ctx, token, submission.CloudJobID)
		}
		current, ok := s.store.Submission(submission.CloudJobID)
		if ok && !terminal(s.localJob(current).Status) {
			active = true
		}
	}
	return active
}

// refreshViews reads the job list once and updates the submissions still
// owed something. The list holds every unfinished job, so a failed or
// canceled one it leaves out was not retried and costs no request of its own.
func (s *Service) refreshViews(ctx context.Context, token string, submissions []Submission) {
	listed, err := s.portal.ListJobs(ctx, token)
	if errors.Is(err, ErrUnauthorized) {
		s.dropToken()
		return
	}
	s.mu.Lock()
	s.portalDown = err != nil
	switch {
	case err != nil:
		s.lastError = errorPortalUnreachable
	case s.lastError == errorPortalUnreachable:
		s.lastError = ""
	}
	if err == nil {
		s.watchedAt = s.now()
	}
	s.mu.Unlock()
	if err != nil {
		return
	}
	byID := make(map[string]PortalJob, len(listed))
	for _, portalJob := range listed {
		byID[portalJob.ID] = portalJob
	}
	for _, submission := range submissions {
		owed := s.followOf(submission)
		if owed == followNone {
			continue
		}
		view, found := byID[submission.CloudJobID]
		if !found {
			if owed == followRetry {
				continue
			}
			// A moving job the list cut off: ask for this one by id.
			single, err := s.portal.GetJob(ctx, token, submission.CloudJobID)
			var apiErr *APIError
			switch {
			case errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound:
				s.dropUnknown(submission)
				continue
			case err != nil:
				continue
			}
			view = single
		}
		s.applyView(submission, view)
	}
}

// applyView stores what the portal says about a submission. Nothing is
// written when nothing changed, and a change of progress or queue estimate
// alone stays in memory: the portal resends those on every poll.
func (s *Service) applyView(previous Submission, view PortalJob) {
	mutate := func(entry *Submission) {
		entry.LastPortalView = &view
		mergeVideos(entry, view)
	}
	// mergeVideos only ever adds videos or sets the local failure.
	next := previous.clone()
	mutate(&next)
	merged := len(next.Videos) != len(previous.Videos) || next.LocalFailure != previous.LocalFailure
	switch {
	case merged || previous.LastPortalView == nil || !sameView(withoutProgress(*previous.LastPortalView), withoutProgress(view)):
		if _, err := s.store.Update(previous.CloudJobID, mutate); err != nil {
			log.Printf("cloudclient: save job view: %v", err)
		}
	case !sameView(*previous.LastPortalView, view):
		s.store.Refresh(previous.CloudJobID, mutate)
	}
}

// withoutProgress drops the fields that move while the status stays put.
func withoutProgress(view PortalJob) PortalJob {
	view.Stage, view.ProgressPercent, view.Queue = nil, nil, nil
	return view
}

func sameView(a, b PortalJob) bool {
	// An absent list and an empty one are the same answer.
	if len(a.Artifacts) == 0 {
		a.Artifacts = nil
	}
	if len(b.Artifacts) == 0 {
		b.Artifacts = nil
	}
	return reflect.DeepEqual(a, b)
}

// dropUnknown forgets a submission the portal does not know for this
// account, unless its videos are already here.
func (s *Service) dropUnknown(submission Submission) {
	if allVideosReady(submission) {
		return
	}
	if err := s.store.Remove(submission.CloudJobID); err != nil {
		log.Printf("cloudclient: forget unknown cloud job %s: %v", submission.CloudJobID, err)
	}
}

// mergeVideos adds the videos the portal offers to the submission. When the
// job is done and the portal offers none while none is here, the results
// expired before this Studio fetched them.
func mergeVideos(entry *Submission, view PortalJob) {
	for _, artifact := range view.Artifacts {
		if artifact.Kind != artifactVideo || !videoNamePattern.MatchString(artifact.Name) {
			continue
		}
		known := slices.ContainsFunc(entry.Videos, func(video StoredVideo) bool { return video.Name == artifact.Name })
		if known {
			continue
		}
		entry.Videos = append(entry.Videos, StoredVideo{
			ArtifactID: artifact.ID,
			Name:       artifact.Name,
			Variant:    artifact.Variant,
			Size:       artifact.SizeBytes,
			SHA256:     artifact.SHA256,
		})
	}
	if view.Status != portalDone || allVideosReady(*entry) {
		return
	}
	offered := func(video StoredVideo) bool {
		return slices.ContainsFunc(view.Artifacts, func(artifact PortalArtifact) bool { return artifact.ID == video.ArtifactID })
	}
	for _, video := range entry.Videos {
		if !video.Ready && !offered(video) {
			entry.LocalFailure = FailureResultsExpired
			return
		}
	}
	if len(entry.Videos) == 0 {
		entry.LocalFailure = FailureResultsExpired
	}
}

// download fetches every video of a finished job that is not on disk yet,
// verifies it, and tells the portal it arrived.
func (s *Service) download(ctx context.Context, token, cloudJobID string) {
	ctx, finish, ok := s.beginDownload(ctx, cloudJobID)
	if !ok {
		return
	}
	defer finish()
	submission, ok := s.store.Submission(cloudJobID)
	if !ok {
		return
	}
	dir, err := s.resultsPath(cloudJobID)
	if err != nil {
		log.Printf("cloudclient: %v", err)
		return
	}
	var total, done int64
	for _, video := range submission.Videos {
		total += video.Size
		if video.Ready {
			done += video.Size
		}
	}
	for index, video := range submission.Videos {
		if video.Ready {
			continue
		}
		dest := filepath.Join(dir, video.Name)
		err := s.portal.DownloadArtifact(ctx, token, ArtifactDownload{
			JobID:      cloudJobID,
			ArtifactID: video.ArtifactID,
			Dest:       dest,
			SizeBytes:  video.Size,
			SHA256:     video.SHA256,
			Progress: func(received int64) {
				if total > 0 {
					s.setPercent(cloudJobID, int((done+received)*100/total))
				}
			},
		})
		var apiErr *APIError
		switch {
		case errors.Is(err, ErrUnauthorized):
			s.dropToken()
			return
		case errors.As(err, &apiErr) && (apiErr.Status == http.StatusGone || apiErr.Status == http.StatusNotFound):
			if _, updateErr := s.store.Update(cloudJobID, func(entry *Submission) { entry.LocalFailure = FailureResultsExpired }); updateErr != nil {
				log.Printf("cloudclient: record expired results: %v", updateErr)
			}
			s.clearPercent(cloudJobID)
			return
		case err != nil && ctx.Err() != nil:
			// Studio is closing or the job is being removed: nothing failed.
			return
		case err != nil:
			s.downloadFailed(cloudJobID, video.Name, err)
			return
		}
		done += video.Size
		found, err := s.store.Update(cloudJobID, func(entry *Submission) {
			if index < len(entry.Videos) && entry.Videos[index].Name == video.Name {
				entry.Videos[index].Path, entry.Videos[index].Ready = dest, true
			}
		})
		if err != nil || !found {
			log.Printf("cloudclient: record downloaded video (found %v): %v", found, err)
			return
		}
		if err := s.portal.ArtifactReceived(ctx, token, cloudJobID, video.ArtifactID); err != nil {
			log.Printf("cloudclient: confirm receipt of %s: %v", video.Name, err)
			continue
		}
		if _, err := s.store.Update(cloudJobID, func(entry *Submission) {
			if index < len(entry.Videos) && entry.Videos[index].Name == video.Name {
				entry.Videos[index].Received = true
			}
		}); err != nil {
			log.Printf("cloudclient: record receipt: %v", err)
		}
	}
	s.clearPercent(cloudJobID)
	s.mu.Lock()
	delete(s.downloadRetry, cloudJobID)
	s.mu.Unlock()
}

// beginDownload registers a download unless the job is being removed or its
// last failure is too recent. The returned function must run when it ends.
func (s *Service) beginDownload(ctx context.Context, cloudJobID string) (context.Context, func(), bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.removing[cloudJobID] || s.now().Before(s.downloadRetry[cloudJobID].nextAt) {
		return ctx, nil, false
	}
	ctx, cancel := context.WithCancel(ctx)
	running := &runningDownload{cancel: cancel, done: make(chan struct{})}
	s.downloading[cloudJobID] = running
	return ctx, func() {
		cancel()
		s.mu.Lock()
		delete(s.downloading, cloudJobID)
		s.mu.Unlock()
		close(running.done)
	}, true
}

// downloadFailed spaces out the next attempt, so a file that keeps failing
// is not fetched again every sync round. A result that keeps arriving with
// the wrong sha256 fails the job on this PC instead.
func (s *Service) downloadFailed(cloudJobID, name string, cause error) {
	log.Printf("cloudclient: download %s of cloud job %s: %v", name, cloudJobID, cause)
	if errors.Is(cause, ErrChecksum) {
		mismatches := 0
		if _, err := s.store.Update(cloudJobID, func(entry *Submission) {
			entry.DownloadMismatches++
			mismatches = entry.DownloadMismatches
			if mismatches >= maxDownloadMismatches {
				entry.LocalFailure = FailureDownload
			}
		}); err != nil {
			log.Printf("cloudclient: record damaged download: %v", err)
		}
		if mismatches >= maxDownloadMismatches {
			s.clearPercent(cloudJobID)
			s.mu.Lock()
			delete(s.downloadRetry, cloudJobID)
			s.mu.Unlock()
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	retry := s.downloadRetry[cloudJobID]
	retry.nextAt = s.now().Add(backoff(s.timing.ActiveSync, retry.failures+1, s.timing.DownloadRetryMax))
	retry.failures++
	s.downloadRetry[cloudJobID] = retry
}

func truncateRunes(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit])
}
