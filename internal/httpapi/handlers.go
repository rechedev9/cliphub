// Package httpapi exposes the orchestrator's HTTP endpoints.
package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/anticheat"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/composition"
	"github.com/rechedev9/cliphub/internal/demozstd"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/generateintent"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/moments"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/steamresolve"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tasks"
	"github.com/rechedev9/cliphub/internal/timelineplan"
	"github.com/rechedev9/cliphub/internal/voiceprofile"
)

const (
	maxDemoBytes       = 700 << 20            // 700 MiB demo cap
	maxMultipartBytes  = maxDemoBytes + 1<<20 // allow multipart headers around the demo
	multipartMemBudget = 32 << 20             // 32 MiB in-memory; spill beyond
	maxJSONBodyBytes   = 1 << 20              // JSON control documents are small
	renderUniqueTTL    = 24 * time.Hour
	generateWorkActive = "generate_work_active"
)

var errGenerateRenderActive = errors.New("a render is already active for this job")

// JobRepository is the subset of *job.Repository used by handlers.
type JobRepository interface {
	Create(ctx context.Context, j *job.Job) error
	Get(ctx context.Context, id uuid.UUID) (job.Job, error)
	// GetMeta is Get without the kill plan blob.
	GetMeta(ctx context.Context, id uuid.UUID) (job.Job, error)
	// GetStatus returns segmentCount only while the job is recording.
	GetStatus(ctx context.Context, id uuid.UUID) (status job.Status, failureReason string, segmentCount int, err error)
	List(ctx context.Context, limit int) ([]job.Job, error)
	ListBySeries(ctx context.Context, seriesID string) ([]job.Job, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, s job.Status, failureReason string) error
	SetParseInputs(ctx context.Context, id uuid.UUID, steamID string, r rules.Rules) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type StreamJobRepository interface {
	Create(ctx context.Context, j *streamclips.Job) error
	Get(ctx context.Context, id uuid.UUID) (streamclips.Job, error)
	List(ctx context.Context, limit int) ([]streamclips.Job, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, s streamclips.Status, failureReason string) error
	SetEditPlan(ctx context.Context, id uuid.UUID, plan streamclips.EditPlan) error
	SetAcquired(ctx context.Context, id uuid.UUID, probe streamclips.SourceProbe, sha256, discoveredTitle string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type EditorAssetRepository interface {
	Create(ctx context.Context, a *mediaassets.Asset) error
	Get(ctx context.Context, id uuid.UUID) (mediaassets.Asset, error)
	GetBySHA256(ctx context.Context, sha256 string) (mediaassets.Asset, error)
	List(ctx context.Context, limit int) ([]mediaassets.Asset, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type EditorProjectRepository interface {
	Create(ctx context.Context, p *timelineplan.Project) error
	Get(ctx context.Context, id uuid.UUID) (timelineplan.Project, error)
	List(ctx context.Context, limit int) ([]timelineplan.Project, error)
	ListByStatus(ctx context.Context, s timelineplan.Status) ([]timelineplan.Project, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, s timelineplan.Status, failureReason string) error
	SetPlan(ctx context.Context, id uuid.UUID, plan timelineplan.Document) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// Enqueuer is the desktop queue contract used by handlers. A transition runs
// inside the queue's admission boundary before accepted work becomes visible;
// accepted pending work receives a later non-nil transition if shutdown
// discards it before a handler takes ownership.
type Enqueuer interface {
	Enqueue(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
	EnqueueWithTransition(*asynq.Task, func(error) error, ...asynq.Option) (*asynq.TaskInfo, error)
}

// Handlers bundles the dependencies needed by every endpoint.
type Handlers struct {
	repo                JobRepository
	streamRepo          StreamJobRepository
	editorAssets        EditorAssetRepository
	editorProjects      EditorProjectRepository
	streamPlanMu        sync.Mutex
	editorPlanMu        sync.Mutex
	renderStateMu       sync.Mutex
	rosterCache         rosterSummaryCache
	jobsListJSON        cachedJSON
	streamListJSON      cachedJSON
	anticheatJobLocks   *anticheat.JobLocks
	streamJobLocks      *streamclips.JobLocks
	storage             storage.Storage
	generateIntents     *generateintent.Store
	voiceProfiles       *voiceprofile.Store
	queue               Enqueuer
	mutationToken       string
	requireReadAuth     bool
	rateLimiter         *rateLimiter
	uploadLimiter       *uploadLimiter
	streamProber        streamclips.Prober
	musicDir            string
	capabilities        Capabilities
	youtubeTrends       YouTubeTrends
	publishAssistant    *publishAssistantCache
	faceit              *faceit.Client
	faceitFollows       *faceit.FollowStore
	faceitSeeds         *faceit.SeedStore
	faceitCache         faceitResponseCache
	faceitRoster        faceitRosterRefresh
	steamAvatarResolver steamAvatarResolver
	steamResolver       *steamresolve.Service
	steamTransport      steamresolve.Transport
	steamFactory        func(steamresolve.Session) steamresolve.Transport
	steamAccounts       *steamresolve.AccountStore
	steamHistory        *steamresolve.HistoryClient
	steamFetcher        *steamresolve.Fetcher
	steamSessionMu      sync.Mutex
	steamSessionCache   steamresolve.Session
	labBundles          LabBundlePreparer
	labBundleRoot       string
	cloud               CloudService
	// labBundleMu serializes bundle writes: two bundles of one job and variant
	// share a directory, and each copies gigabytes of clips.
	labBundleMu sync.Mutex
}

type Option func(*Handlers)

// WithMutationToken configures the per-session capability used for authenticated
// API reads and mutations.
func WithMutationToken(token string) Option {
	return func(h *Handlers) {
		h.mutationToken = token
	}
}

// WithRequireReadAuth gates API/workbench data reads behind the same session
// capability as mutations. Production enables it for loopback too; leaving it
// off is useful only for isolated handler tests.
func WithRequireReadAuth(require bool) Option {
	return func(h *Handlers) {
		h.requireReadAuth = require
	}
}

// WithUploadConcurrency bounds simultaneous multipart uploads. Values below
// one disable the bound, which is useful only for isolated handler tests.
func WithUploadConcurrency(limit int) Option {
	return func(h *Handlers) {
		h.uploadLimiter = newUploadLimiter(limit)
	}
}

// WithRateLimit throttles requests per client IP. When rps <= 0 the limiter is a
// no-op pass-through, which keeps loopback deployments unthrottled.
func WithRateLimit(rps float64, burst int) Option {
	return func(h *Handlers) {
		h.rateLimiter = newRateLimiter(rps, burst)
	}
}

func WithStreamRepository(repo StreamJobRepository) Option {
	return func(h *Handlers) {
		h.streamRepo = repo
	}
}

func WithEditorRepositories(assets EditorAssetRepository, projects EditorProjectRepository) Option {
	return func(h *Handlers) {
		h.editorAssets = assets
		h.editorProjects = projects
	}
}

// WithStreamJobLocks shares per-job render admission/finalization locks with
// the stream worker. The local orchestrator must pass the same instance to
// both owners; tests and handler-only deployments receive a private instance.
func WithStreamJobLocks(locks *streamclips.JobLocks) Option {
	return func(h *Handlers) {
		if locks != nil {
			h.streamJobLocks = locks
		}
	}
}

func WithStreamProber(prober streamclips.Prober) Option {
	return func(h *Handlers) {
		h.streamProber = prober
	}
}

// WithGenerateIntentStore shares guided-generate synchronization with the
// record worker. Desktop startup supplies one store to both owners so an old
// completion cannot race with accepting a newer run.
func WithGenerateIntentStore(store *generateintent.Store) Option {
	return func(h *Handlers) {
		h.generateIntents = store
	}
}

// WithCapabilities records which media workers are enabled and the tool paths
// they use, so GET /api/capabilities can report readiness and the record/
// generate handlers can reject a capture attempt with a clear 409 instead of
// enqueuing a task no worker will ever consume.
func WithCapabilities(c Capabilities) Option {
	return func(h *Handlers) {
		h.capabilities = c
	}
}

// WithPublishAssistantTrends enables optional Firecrawl discovery. Missing
// public trend data never makes the manual publishing assistant unavailable.
func WithPublishAssistantTrends(trends YouTubeTrends) Option {
	return func(h *Handlers) {
		h.youtubeTrends = trends
	}
}

func WithFaceit(client *faceit.Client, follows *faceit.FollowStore) Option {
	return func(h *Handlers) {
		h.faceit = client
		h.faceitFollows = follows
	}
}

// WithSteamAvatarResolver supplies the public-profile lookup used only for
// local Full Demo roster portraits. It is separate from the authenticated
// Steam history account and allows deterministic fake-service tests.
func WithSteamAvatarResolver(resolver steamAvatarResolver) Option {
	return func(h *Handlers) {
		h.steamAvatarResolver = resolver
	}
}

// WithFaceitSeeds supplies the refreshable default roster. A nil store still
// projects the embedded zone rosters: SeedStore.Document degrades to DefaultSeed.
func WithFaceitSeeds(seeds *faceit.SeedStore) Option {
	return func(h *Handlers) {
		h.faceitSeeds = seeds
	}
}

// WithSteamAccount wires the revocable history account (auth code + Web API key).
func WithSteamAccount(store *steamresolve.AccountStore, history *steamresolve.HistoryClient, fetcher *steamresolve.Fetcher) Option {
	return func(h *Handlers) {
		h.steamAccounts = store
		h.steamHistory = history
		h.steamFetcher = fetcher
	}
}

// WithSteamTransportFactory builds a short-lived GC transport from a session.
// The orchestrator supplies steamclient.New here so httpapi never imports go-steam.
func WithSteamTransportFactory(f func(steamresolve.Session) steamresolve.Transport) Option {
	return func(h *Handlers) {
		h.steamFactory = f
	}
}

// NewHandlers constructs an HTTP handler set.
func NewHandlers(repo JobRepository, store storage.Storage, queue Enqueuer, opts ...Option) *Handlers {
	h := &Handlers{
		repo:              repo,
		storage:           store,
		generateIntents:   generateintent.New(store),
		voiceProfiles:     voiceprofile.New(store),
		queue:             queue,
		publishAssistant:  newPublishAssistantCache(),
		streamJobLocks:    streamclips.NewJobLocks(),
		anticheatJobLocks: anticheat.NewJobLocks(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// createJobConfig is the JSON document sent in the "config" multipart field.
type createJobConfig struct {
	TargetSteamID string       `json:"target_steamid"`
	Rules         *rules.Rules `json:"rules,omitempty"`
}

// maxDemoFileNameRunes caps the stored original demo file name so a hostile or
// accidental upload cannot bloat the persisted job document.
const maxDemoFileNameRunes = 128

// sanitizeDemoFileName reduces an uploaded multipart file name to a safe display
// name: it strips any directory prefix using either separator (a client may send
// a Windows path or a URL-style one, so filepath.Base alone is not enough), drops
// control characters and invisible format characters (Cf: RTL overrides,
// zero-width runes, BOM) that could spoof the displayed name, and caps the
// result at maxDemoFileNameRunes runes. It returns "" when nothing usable
// remains, so the caller leaves the field empty.
func sanitizeDemoFileName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	var b strings.Builder
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		b.WriteRune(r)
	}
	cleaned := strings.TrimSpace(b.String())
	if runes := []rune(cleaned); len(runes) > maxDemoFileNameRunes {
		cleaned = strings.TrimSpace(string(runes[:maxDemoFileNameRunes]))
	}
	return cleaned
}

// Stable codes for a demo rejected at admission, so the web client can tell
// the user why instead of a generic "could not scan" message.
const (
	codeCSGODemo       = "csgo_demo"
	codeNotADemo       = "not_a_demo"
	codeUnreadableDemo = "unreadable_demo"
)

// isDemoHeader reports whether the leading bytes look like a CS2 (Source 2)
// demo, the only format the parser (demoinfocs v5) can read.
func isDemoHeader(header []byte) bool {
	return bytes.HasPrefix(header, []byte("PBDEMS2"))
}

// isCSGODemoHeader reports whether the leading bytes belong to a legacy CS:GO
// (Source 1) demo. demoinfocs v5 refuses those, so admitting one only queues a
// scan that is certain to fail.
func isCSGODemoHeader(header []byte) bool {
	return bytes.HasPrefix(header, []byte("HL2DEMO"))
}

// rejectDemoHeader writes a coded 400 and returns true when header is not a
// CS2 demo. subject names the file in the message ("uploaded", "downloaded").
func rejectDemoHeader(w http.ResponseWriter, header []byte, subject string) bool {
	switch {
	case isDemoHeader(header):
		return false
	case isCSGODemoHeader(header):
		writeCodedError(w, http.StatusBadRequest, codeCSGODemo, subject+" file is a CS:GO demo; only CS2 demos are supported")
	default:
		writeCodedError(w, http.StatusBadRequest, codeNotADemo, subject+" file is not a CS2 demo")
	}
	return true
}

// CreateJob handles POST /api/jobs.
func (h *Handlers) CreateJob(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBytes)

	// #nosec G120 -- r.Body is capped with MaxBytesReader immediately above.
	if err := r.ParseMultipartForm(multipartMemBudget); err != nil {
		writeError(w, http.StatusBadRequest, "parsing multipart form: "+err.Error())
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	file, fileHeader, err := r.FormFile("demo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing demo file: "+err.Error())
		return
	}
	defer file.Close()
	var demoFileName string
	if fileHeader != nil {
		demoFileName = sanitizeDemoFileName(fileHeader.Filename)
	}
	demoSrc, demoFileName, err := demozstd.Open(file, demoFileName, maxDemoBytes)
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, codeUnreadableDemo, "could not read uploaded demo")
		return
	}
	defer demoSrc.Close()

	// Peek the demo magic bytes before persisting so non-demo uploads are
	// rejected at the door. io.ReadFull tolerates a short read via ErrUnexpectedEOF.
	var header [8]byte
	n, err := io.ReadFull(demoSrc, header[:])
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		// demozstd.Open only sniffs the magic; a corrupt .dem.zst frame fails
		// here, on the first decompressed read of the user's own file.
		log.Printf("httpapi: read uploaded demo header: %v", err)
		writeCodedError(w, http.StatusBadRequest, codeUnreadableDemo, "could not read uploaded demo")
		return
	}
	if rejectDemoHeader(w, header[:n], "uploaded") {
		return
	}
	// Stitch the peeked bytes back ahead of the remaining stream so the upload is
	// neither truncated nor read twice.
	demo := io.MultiReader(bytes.NewReader(header[:n]), demoSrc)

	var cfg createJobConfig
	if raw := r.FormValue("config"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid config JSON: "+err.Error())
			return
		}
	}
	// target_steamid is optional: when present the job parses straight away;
	// when absent it runs a roster scan first so the user can pick a target.
	if cfg.TargetSteamID != "" {
		if _, err := strconv.ParseUint(cfg.TargetSteamID, 10, 64); err != nil {
			writeError(w, http.StatusBadRequest, "target_steamid must be a 64-bit unsigned integer")
			return
		}
	}

	effectiveRules := rules.Default()
	if cfg.Rules != nil {
		effectiveRules = *cfg.Rules
		if err := effectiveRules.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid rules: "+err.Error())
			return
		}
	}

	// series_id is an optional client-minted UUID that groups the demos of one
	// bo3/bo5 series. When present it must be a valid UUID; it is stored in the
	// canonical lowercase form so ListBySeries matches regardless of casing.
	var seriesID string
	if raw := strings.TrimSpace(r.FormValue("series_id")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "series_id must be a valid UUID")
			return
		}
		seriesID = parsed.String()
	}

	created, err := h.persistAndEnqueueDemo(r.Context(), demo, demoFileName, cfg.TargetSteamID, seriesID, "", effectiveRules)
	if err != nil {
		internalError(w, "admit uploaded demo", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     created.ID,
		"status": created.Status,
	})
}

// persistAndEnqueueDemo stores a validated demo stream and queues parse or roster scan.
func (h *Handlers) persistAndEnqueueDemo(ctx context.Context, demo io.Reader, fileName, targetSteamID, seriesID, cloudRequestID string, effectiveRules rules.Rules) (*job.Job, error) {
	j := &job.Job{
		ID:             uuid.New(),
		Status:         job.StatusQueued,
		SeriesID:       seriesID,
		DemoFileName:   fileName,
		TargetSteamID:  targetSteamID,
		CloudRequestID: cloudRequestID,
		Rules:          effectiveRules,
	}
	key := fmt.Sprintf("demos/%s.dem", j.ID)
	j.DemoPath = key

	h256 := sha256.New()
	tee := io.TeeReader(demo, h256)
	if err := h.storage.Put(key, tee); err != nil {
		return nil, fmt.Errorf("store demo: %w", err)
	}
	j.DemoSHA256 = hex.EncodeToString(h256.Sum(nil))

	if err := h.repo.Create(ctx, j); err != nil {
		// No job will ever point at the stored demo, so nothing else can
		// find it to delete it.
		h.discardStoredDemo(key)
		return nil, fmt.Errorf("create job: %w", err)
	}

	taskKind := "parse"
	build := tasks.NewParseDemoTask
	if j.TargetSteamID == "" {
		taskKind = "scan"
		build = tasks.NewScanRosterTask
	}
	task, err := build(j.ID)
	if err != nil {
		return nil, fmt.Errorf("build %s task: %w", taskKind, err)
	}
	if _, err := h.queue.EnqueueWithTransition(task, func(decision error) error {
		return h.persistJobQueueDecision(j.ID, taskKind, decision)
	}); err != nil {
		return nil, fmt.Errorf("enqueue %s task: %w", taskKind, err)
	}
	return j, nil
}

// discardStoredDemo removes a demo that was stored for a job that never came
// to exist, for example because the request was canceled in between.
func (h *Handlers) discardStoredDemo(key string) {
	deleter, ok := h.storage.(jobArtifactDeleter)
	if !ok {
		return
	}
	if err := deleter.Delete(key); err != nil {
		log.Printf("httpapi: remove the demo of a job that was not created (%s): %v", key, err)
	}
}

// ListJobs handles GET /api/jobs. With ?series_id=<uuid> it returns only that
// series' jobs ordered by creation time ascending (id as a deterministic
// tie-break); otherwise it returns the recent jobs (?limit).
func (h *Handlers) ListJobs(w http.ResponseWriter, r *http.Request) {
	if raw := r.URL.Query().Get("series_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "series_id must be a valid UUID")
			return
		}
		jobs, err := h.repo.ListBySeries(r.Context(), parsed.String())
		if err != nil {
			internalError(w, "list jobs by series", err)
			return
		}
		items := h.rosterCache.summarizeAll(h.storage, attachJobFailureCodes(jobs))
		h.jobsListJSON.write(w, r, jobListCacheKey(r, items), map[string]any{"jobs": items})
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "limit must be an integer from 1 to 100")
			return
		}
		limit = parsed
	}
	jobs, err := h.repo.List(r.Context(), limit)
	if err != nil {
		internalError(w, "list jobs", err)
		return
	}
	items := h.rosterCache.summarizeAll(h.storage, attachJobFailureCodes(jobs))
	h.jobsListJSON.write(w, r, jobListCacheKey(r, items), map[string]any{"jobs": items})
}

// ListLoadouts handles GET /api/loadouts.
func (h *Handlers) ListLoadouts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"loadouts": renderplan.LoadoutCatalog()})
}

// presetSummary is the UI-facing view of one editor render preset.
type presetSummary struct {
	Name              string `json:"name"`
	Label             string `json:"label,omitempty"`
	Description       string `json:"description"`
	DescriptionES     string `json:"description_es,omitempty"`
	Default           bool   `json:"default"`
	HUDMode           string `json:"hud_mode,omitempty"`
	FPS               int    `json:"fps"`
	Width             int    `json:"width"`
	Height            int    `json:"height"`
	EffectsPreset     string `json:"effects_preset,omitempty"`
	HQFilters         bool   `json:"hq_filters"`
	AudioNormalize    bool   `json:"audio_normalize"`
	QualityChecks     bool   `json:"quality_checks"`
	CoverSheets       bool   `json:"cover_sheets"`
	TemporalSmoothing bool   `json:"temporal_smoothing"`
}

// ListPresets handles GET /api/presets. It exposes the editor preset registry
// so UIs can derive their variant lists instead of hardcoding them.
func (h *Handlers) ListPresets(w http.ResponseWriter, r *http.Request) {
	defaultName := editor.DefaultPreset().Name
	names := editor.PresetNames()
	presets := make([]presetSummary, 0, len(names))
	for _, name := range names {
		preset, ok := editor.PresetByName(name)
		if !ok {
			continue
		}
		presets = append(presets, presetSummary{
			Name:              preset.Name,
			Label:             preset.Label,
			Description:       preset.Description,
			DescriptionES:     preset.DescriptionES,
			Default:           preset.Name == defaultName,
			HUDMode:           preset.HUDMode,
			FPS:               preset.FPS,
			Width:             preset.Width,
			Height:            preset.Height,
			EffectsPreset:     preset.EffectsPreset,
			HQFilters:         preset.HQFilters,
			AudioNormalize:    preset.AudioNormalize,
			QualityChecks:     preset.QualityChecks,
			CoverSheets:       preset.CoverSheets,
			TemporalSmoothing: preset.TemporalSmoothing,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"default": defaultName,
		"presets": presets,
	})
}

// GetJob handles GET /api/jobs/{id}.
func (h *Handlers) GetJob(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	if r.URL.Query().Get("view") == "status" {
		h.writeJobStatus(w, r, id)
		return
	}
	j, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, job.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		internalError(w, "get job", err)
		return
	}
	writeJSON(w, http.StatusOK, h.jobResponse(j))
}

// writeJobStatus serves the lightweight ?view=status representation. The
// default GetJob response remains the complete job for existing API/MCP users.
func (h *Handlers) writeJobStatus(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	resp, found, err := h.jobStatusView(r.Context(), id)
	if err != nil {
		internalError(w, "get job status", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// jobStatusView builds the ?view=status document; found is false for an
// unknown job so a batch caller can report it per item instead of failing.
func (h *Handlers) jobStatusView(ctx context.Context, id uuid.UUID) (jobStatusResponse, bool, error) {
	status, failureReason, segmentCount, err := h.repo.GetStatus(ctx, id)
	if errors.Is(err, job.ErrNotFound) {
		return jobStatusResponse{}, false, nil
	}
	if err != nil {
		return jobStatusResponse{}, false, err
	}
	resp := jobStatusResponse{
		Status:        status,
		FailureReason: failureReason,
		FailureCode:   jobFailureCode(failureReason, ""),
	}
	if status == job.StatusRecording {
		if progress, ok := captureProgressWithTotal(h.storage, id, status, segmentCount); ok {
			resp.Progress = &progress
		}
	} else if progress, ok := renderProgressDocument(h.storage, id); ok {
		resp.Progress = &progress
	}
	return resp, true, nil
}

type jobStatusResponse struct {
	Status        job.Status           `json:"status"`
	FailureReason string               `json:"failure_reason,omitempty"`
	FailureCode   string               `json:"failure_code,omitempty"`
	Progress      *captureProgressView `json:"progress,omitempty"`
}

// jobResponse is the GET /api/jobs/{id} body: the job plus optional capture
// progress. Progress is present only while the job is capturing and at least one
// segment clip exists (see captureProgress); otherwise the field is omitted and
// the response is byte-for-byte the raw job as before.
type jobResponse struct {
	job.Job
	Progress *captureProgressView `json:"progress,omitempty"`
}

func (h *Handlers) jobResponse(j job.Job) jobResponse {
	j.FailureCode = jobFailureCode(j.FailureReason, j.FailureCode)
	resp := jobResponse{Job: j}
	if progress, ok := captureProgress(h.storage, j); ok {
		resp.Progress = &progress
	}
	return resp
}

func jobFailureCode(reason, stored string) string {
	if stored != "" {
		return stored
	}
	if class := obs.ClassOf(reason); class != "" {
		return class
	}
	return streamclips.CodeFromReason(reason)
}

func attachJobFailureCodes(jobs []job.Job) []job.Job {
	for i := range jobs {
		jobs[i].FailureCode = jobFailureCode(jobs[i].FailureReason, jobs[i].FailureCode)
	}
	return jobs
}

// GetPlan handles GET /api/jobs/{id}/plan.
func (h *Handlers) GetPlan(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	j, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, job.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		internalError(w, "get plan", err)
		return
	}
	if j.KillPlan == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("job not ready (status=%s)", j.Status))
		return
	}
	writeJSON(w, http.StatusOK, j.KillPlan)
}

// GetRecapPlan handles GET /api/jobs/{id}/recap-plan.
func (h *Handlers) GetRecapPlan(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	if j.KillPlan == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("job not ready (status=%s)", j.Status))
		return
	}
	plan, found, err := recapplan.Load(h.storage, j.ID)
	if err != nil {
		internalError(w, "load recap plan", err)
		return
	}
	if !found {
		writeError(w, http.StatusConflict, "recap plan not ready")
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// GetMoments handles GET /api/jobs/{id}/moments.
func (h *Handlers) GetMoments(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	if rc, err := h.storage.Open(moments.ArtifactKey(j.ID)); err == nil {
		defer rc.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, rc)
		return
	} else if !storage.IsNotExist(err) {
		internalError(w, "open moments artifact", err)
		return
	}
	if j.KillPlan == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("job moments are not ready (status=%s)", j.Status))
		return
	}
	writeJSON(w, http.StatusOK, moments.Build(j.ID, *j.KillPlan))
}

// GetRoster handles GET /api/jobs/{id}/roster. It streams the roster scan
// result stored by the scan worker, already shaped as { "players": [...] }.
func (h *Handlers) GetRoster(w http.ResponseWriter, r *http.Request) {
	// Metadata only: the roster never needs the kill plan blob, and this is
	// the endpoint Studio polls per job while a parse is in flight.
	j, ok := h.loadJobMeta(w, r)
	if !ok {
		return
	}
	rc, err := h.storage.Open(artifacts.RosterKey(j.ID))
	if err != nil {
		if storage.IsNotExist(err) {
			// Either the scan is still running or this job was created with a
			// target and never scanned.
			writeError(w, http.StatusConflict, "roster not ready")
			return
		}
		internalError(w, "open roster artifact", err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

// startParseRequest is the JSON body for POST /api/jobs/{id}/parse.
type startParseRequest struct {
	TargetSteamID string       `json:"target_steamid"`
	Rules         *rules.Rules `json:"rules,omitempty"`
}

// StartParse handles POST /api/jobs/{id}/parse. After a roster scan it records
// the picked target (and optional rules) and enqueues the full parse.
func (h *Handlers) StartParse(w http.ResponseWriter, r *http.Request) {
	// Metadata only: parse reads the status and the stored rules, never the
	// kill plan a previous parse left behind.
	j, ok := h.loadJobMeta(w, r)
	if !ok {
		return
	}
	// Friendly early-out with the current status. The race-safe guard is the
	// status-gated SetParseInputs below, which atomically claims the job, so a
	// second concurrent request that slips past this check still conflicts there.
	if j.Status != job.StatusScanned && j.Status != job.StatusParsed {
		writeError(w, http.StatusConflict, fmt.Sprintf("job is not ready to parse (status=%s)", j.Status))
		return
	}

	var req startParseRequest
	if err := decodeSingleJSONBody(w, r, &req, false); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, "parse request JSON is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid parse request JSON")
		return
	}
	if _, err := strconv.ParseUint(req.TargetSteamID, 10, 64); err != nil {
		writeError(w, http.StatusBadRequest, "target_steamid must be a 64-bit unsigned integer")
		return
	}

	effectiveRules := j.Rules
	if req.Rules != nil {
		effectiveRules = *req.Rules
		if err := effectiveRules.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid rules: "+err.Error())
			return
		}
	}

	if err := h.repo.SetParseInputs(r.Context(), j.ID, req.TargetSteamID, effectiveRules); err != nil {
		switch {
		case errors.Is(err, job.ErrNotFound):
			writeError(w, http.StatusNotFound, "job not found")
		case errors.Is(err, job.ErrConflict):
			writeError(w, http.StatusConflict, "job is no longer ready to parse")
	default:
			internalError(w, "set parse inputs", err)
		}
		return
	}

	task, err := tasks.NewParseDemoTask(j.ID)
	if err != nil {
		internalError(w, "build parse task", err)
		return
	}
	if _, err := h.queue.EnqueueWithTransition(task, func(decision error) error {
		return h.persistJobQueueDecision(j.ID, "parse", decision)
	}); err != nil {
		internalError(w, "enqueue parse task", err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":     j.ID,
		"status": job.StatusParsing,
	})
}

// claimQueueStage builds the EnqueueWithTransition callback for a stage whose
// durable claim must be visible before its task is: claim runs for accepted
// work and compensate runs only when work that was actually claimed is later
// discarded. A rejection at admission (duplicate, full or closed queue) leaves
// the state untouched so the client can retry the POST once the queue recovers.
func claimQueueStage(claim func() error, compensate func(decision error) error) func(error) error {
	var admitted atomic.Bool
	return func(decision error) error {
		if decision == nil {
			if err := claim(); err != nil {
				return err
			}
			admitted.Store(true)
			return nil
		}
		if !admitted.Load() {
			return nil
		}
		return compensate(decision)
	}
}

// persistJobQueueDecision keeps a persisted active-looking job state aligned
// with ownership by the process-local queue. The queue calls it once during
// admission and again if accepted pending work is discarded during shutdown.
func (h *Handlers) persistJobQueueDecision(id uuid.UUID, taskKind string, decision error) error {
	if decision == nil {
		return nil
	}
	markCtx, markCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer markCancel()
	if err := h.repo.UpdateStatus(markCtx, id, job.StatusFailed, "enqueue "+taskKind+" task: "+decision.Error()); err != nil {
		return fmt.Errorf("mark job failed after %s queue decision: %w", taskKind, err)
	}
	return nil
}

// GetFinal handles GET /api/jobs/{id}/final.
func (h *Handlers) GetFinal(w http.ResponseWriter, r *http.Request) {
	// Metadata only: the final MP4 is gated on the status and streamed from
// storage, so decoding the kill plan blob would be pure overhead.
	j, ok := h.loadJobMeta(w, r)
	if !ok {
		return
	}
	if j.Status != job.StatusComposed && j.Status != job.StatusReviewRequired && j.Status != job.StatusDone {
		writeError(w, http.StatusConflict, fmt.Sprintf("job final is not ready (status=%s)", j.Status))
		return
	}
	rc, err := h.storage.Open(composition.FinalArtifactKey(j.ID))
	if err != nil {
		writeError(w, http.StatusNotFound, "final artifact not found")
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-final.mp4"`, j.ID))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}
