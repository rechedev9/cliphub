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

	"github.com/rechedev9/cliphub/internal/rooted"

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
	// NeedsApproval: a new job waits for an operator before it is queued.
	NeedsApproval bool    `json:"needs_approval"`
	Error         *string `json:"error"`
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
	view.NeedsApproval = s.me.NeedsApproval
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

func truncateRunes(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit])
}
