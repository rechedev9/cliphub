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
	repo              JobRepository
	streamRepo        StreamJobRepository
	editorAssets      EditorAssetRepository
	editorProjects    EditorProjectRepository
	streamPlanMu      sync.Mutex
	editorPlanMu      sync.Mutex
	renderStateMu     sync.Mutex
	anticheatJobLocks *anticheat.JobLocks
	streamJobLocks    *streamclips.JobLocks
	storage           storage.Storage
	generateIntents   *generateintent.Store
	voiceProfiles     *voiceprofile.Store
	queue             Enqueuer
	mutationToken     string
	requireReadAuth   bool
	rateLimiter       *rateLimiter
	uploadLimiter     *uploadLimiter
	streamProber      streamclips.Prober
	musicDir          string
	capabilities      Capabilities
	youtubeTrends     YouTubeTrends
	publishAssistant  *publishAssistantCache
	faceit            *faceit.Client
	faceitFollows     *faceit.FollowStore
	faceitCache       faceitResponseCache
	steamResolver     *steamresolve.Service
	steamTransport    steamresolve.Transport
	steamFactory      func(steamresolve.Session) steamresolve.Transport
	steamAccounts     *steamresolve.AccountStore
	steamHistory      *steamresolve.HistoryClient
	steamFetcher      *steamresolve.Fetcher
	steamSessionMu    sync.Mutex
	steamSessionCache steamresolve.Session
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

// isDemoHeader reports whether the leading bytes look like a CS2 (Source 2) or
// legacy GOTV (Source 1) demo.
func isDemoHeader(header []byte) bool {
	return bytes.HasPrefix(header, []byte("PBDEMS2")) || bytes.HasPrefix(header, []byte("HL2DEMO"))
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
		writeError(w, http.StatusBadRequest, "could not read uploaded demo")
		return
	}
	defer demoSrc.Close()

	// Peek the demo magic bytes before persisting so non-demo uploads are
	// rejected at the door. io.ReadFull tolerates a short read via ErrUnexpectedEOF.
	var header [8]byte
	n, err := io.ReadFull(demoSrc, header[:])
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		internalError(w, "read demo header", err)
		return
	}
	if !isDemoHeader(header[:n]) {
		writeError(w, http.StatusBadRequest, "uploaded file is not a CS2 demo")
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

	created, err := h.persistAndEnqueueDemo(r.Context(), demo, demoFileName, cfg.TargetSteamID, seriesID, effectiveRules)
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
func (h *Handlers) persistAndEnqueueDemo(ctx context.Context, demo io.Reader, fileName, targetSteamID, seriesID string, effectiveRules rules.Rules) (*job.Job, error) {
	j := &job.Job{
		ID:            uuid.New(),
		Status:        job.StatusQueued,
		SeriesID:      seriesID,
		DemoFileName:  fileName,
		TargetSteamID: targetSteamID,
		Rules:         effectiveRules,
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
		writeJSON(w, http.StatusOK, map[string]any{"jobs": attachJobFailureCodes(jobs)})
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
	writeJSON(w, http.StatusOK, map[string]any{"jobs": attachJobFailureCodes(jobs)})
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
	status, failureReason, segmentCount, err := h.repo.GetStatus(r.Context(), id)
	if errors.Is(err, job.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		internalError(w, "get job status", err)
		return
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
	writeJSON(w, http.StatusOK, resp)
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
	j, ok := h.loadJob(w, r)
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
	j, ok := h.loadJob(w, r)
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
	j, ok := h.loadJob(w, r)
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

// validateSegmentSelection rejects any requested segment id that is not in the
// job's kill plan, writing a 400 and returning false. An empty selection means
// "record every segment" and always passes. Callers guarantee a non-nil kill
// plan via their readiness check before calling this.
// validateSegmentSelection rejects a segment selection that names a segment
// outside the job's kill plan or names one segment twice; an empty selection
// means every segment. It writes the 400 itself and reports whether the
// selection is usable.
func validateSegmentSelection(w http.ResponseWriter, j job.Job, ids []string) bool {
	if len(ids) == 0 {
		return true
	}
	valid := make(map[string]bool, len(j.KillPlan.Segments))
	for _, s := range j.KillPlan.Segments {
		valid[s.ID] = true
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !valid[id] {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown segment id %q", id))
			return false
		}
		if seen[id] {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("duplicate segment id %q", id))
			return false
		}
		seen[id] = true
	}
	return true
}

func applyCaptureOverrides(hudMode string, edit renderplan.EditRequest) (string, bool) {
	if edit.NativeHUD {
		hudMode = "gameplay"
	}
	return hudMode, edit.MatchRecap
}

func (h *Handlers) requireRecapPlan(w http.ResponseWriter, j job.Job) bool {
	plan, found, err := recapplan.Load(h.storage, j.ID)
	if err != nil {
		internalError(w, "load recap plan", err)
		return false
	}
	if !found {
		writeError(w, http.StatusConflict, "recap plan not ready")
		return false
	}
	if len(plan.Segments) == 0 {
		writeError(w, http.StatusConflict, "recap plan has no rounds")
		return false
	}
	return true
}

// StartRecording handles POST /api/jobs/{id}/record.
func (h *Handlers) StartRecording(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	// Parsed/Recorded are the normal entry points. Failed is allowed too so a
	// failed capture can be retried in place (the .dem and kill plan are still
	// there); the KillPlan==nil guard still rejects a job that failed before it
	// was ever parsed.
	if j.KillPlan == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("job is not ready to record (status=%s)", j.Status))
		return
	}
	// The worker flips parsed→recording as soon as it dequeues, before HLAE
	// launches. Reconcile re-POSTs in that window; treat it like a unique
	// duplicate so the reel stays in progress instead of latching a 409.
	if j.Status == job.StatusRecording {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"id":        j.ID,
			"task":      tasks.TypeRecordDemo,
			"duplicate": true,
		})
		return
	}
	if j.Status != job.StatusParsed && j.Status != job.StatusRecorded && j.Status != job.StatusFailed {
		writeError(w, http.StatusConflict, fmt.Sprintf("job is not ready to record (status=%s)", j.Status))
		return
	}
	if !h.requireRecordEnabled(w) {
		return
	}
	// Optional JSON body { "preset": "<name>" } selects the recording HUD from
	// the shared preset registry (so a "Clean POV" reel records HUD-less). An
	// empty or absent body keeps the recorder's default HUD.
	var hudMode string
	var segmentIDs []string
	var portraitSafeKillfeed bool
	var useRecapPlan bool
	var demoSource string
	if r.Body != nil {
		var req struct {
			Preset     string                 `json:"preset"`
			SegmentIDs []string               `json:"segment_ids"`
			Edit       renderplan.EditRequest `json:"edit"`
		}
		switch err := decodeSingleJSONBody(w, r, &req, false); {
		case err == nil, errors.Is(err, io.EOF):
			edit := renderplan.NormalizeEditRequest(req.Edit)
			if req.Preset != "" {
				preset, ok := editor.PresetByName(req.Preset)
				if !ok {
					writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown preset %q", req.Preset))
					return
				}
				hudMode = preset.HUDMode
				if err := edit.Validate(); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				portraitSafeKillfeed = preset.KillfeedSource && edit.Format == renderplan.FormatShort9x16
			}
			hudMode, useRecapPlan = applyCaptureOverrides(hudMode, edit)
			if useRecapPlan {
				if err := edit.Validate(); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				if !h.requireRecapPlan(w, j) {
					return
				}
				segmentIDs = nil
				demoSource = edit.DemoSource
			} else {
				if !validateSegmentSelection(w, j, req.SegmentIDs) {
					return
				}
				segmentIDs = req.SegmentIDs
			}
		default:
			writeError(w, http.StatusBadRequest, "invalid record request JSON")
			return
		}
	}
	if useRecapPlan {
		if demoSource != renderplan.DemoSourceFACEIT {
			writeCodedError(w, http.StatusBadRequest, faceitRosterIncomplete, "Full Demo requires FACEIT as its data source")
			return
		}
		if err := h.storeFullDemoFaceit(r.Context(), j); err != nil {
			h.rejectFullDemoFaceit(w, j, err)
			return
		}
	}
	task, err := tasks.NewRecordDemoTaskWithRecap(j.ID, hudMode, segmentIDs, portraitSafeKillfeed, useRecapPlan)
	if err != nil {
		internalError(w, "build record task", err)
		return
	}
	// Admission claims the job as recording so the pending capture is visible
	// to the Studio poll and to the startup sweep: the task itself lives only in
	// the in-process queue, so without the claim a restart left the job idle at
	// parsed with nothing to retry. A rejection at admission leaves the state
	// untouched so the client can retry the POST once the queue recovers.
	if _, err := h.queue.EnqueueWithTransition(task, claimQueueStage(
		func() error { return h.repo.UpdateStatus(r.Context(), j.ID, job.StatusRecording, "") },
		func(decision error) error { return h.persistJobQueueDecision(j.ID, "record", decision) },
	), asynq.MaxRetry(0), asynq.Unique(renderUniqueTTL)); err != nil {
		// A duplicate is success: the reconcile loop re-POSTs record on every tick
		// until the worker dequeues the unique task, so a 202 here keeps the reel
		// advancing instead of being marked failed mid-capture.
		if errors.Is(err, asynq.ErrDuplicateTask) {
			writeJSON(w, http.StatusAccepted, map[string]any{
				"id":        j.ID,
				"task":      tasks.TypeRecordDemo,
				"duplicate": true,
			})
			return
		}
		internalError(w, "enqueue record task", err)
		return
	}
	if useRecapPlan {
		h.persistFullDemoSource(j.ID, demoSource)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":   j.ID,
		"task": tasks.TypeRecordDemo,
	})
}

// StartGenerate handles POST /api/jobs/{id}/generate. It captures the full
// one-click choice (preset, music, edit) as the job's generate intent and
// enqueues the recording. The record worker reads the intent on success and
// enqueues the matching render, so the user acts once and the chosen treatment
// flows automatically from capture to upload pack.
func (h *Handlers) StartGenerate(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	// Parsed and completed jobs keep their stored demo and plan, so either flow
	// can be generated again in place. Failed is also retryable when parsing had
	// already produced the kill plan.
	if !canGenerateFromStatus(j.Status) || j.KillPlan == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("job is not ready to generate (status=%s)", j.Status))
		return
	}
	if !h.requireRecordEnabled(w) {
		return
	}
	var req struct {
		Preset     string                 `json:"preset"`
		Music      renderMusicRequest     `json:"music"`
		Edit       renderplan.EditRequest `json:"edit"`
		SegmentIDs []string               `json:"segment_ids"`
	}
	if r.Body != nil {
		switch err := decodeSingleJSONBody(w, r, &req, false); {
		case err == nil, errors.Is(err, io.EOF):
		default:
			writeError(w, http.StatusBadRequest, "invalid generate request JSON")
			return
		}
	}
	preset, ok := editor.PresetByName(req.Preset)
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown preset %q", req.Preset))
		return
	}
	intent := renderplan.GenerateIntent{
		Variant:     preset.Name,
		MusicKey:    req.Music.Key,
		MusicVolume: req.Music.Volume,
		GameVolume:  req.Music.GameVolume,
		Edit:        renderplan.NormalizeEditRequest(req.Edit),
		ActiveRunID: uuid.New(),
		AcceptedAt:  time.Now().UTC(),
	}
	if err := intent.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hudMode, useRecapPlan := applyCaptureOverrides(preset.HUDMode, intent.Edit)
	segmentIDs := req.SegmentIDs
	if useRecapPlan {
		if !h.requireRecapPlan(w, j) {
			return
		}
		segmentIDs = nil
		if !intent.Edit.UsesFACEITOverlay() {
			writeCodedError(w, http.StatusBadRequest, faceitRosterIncomplete, "Full Demo requires FACEIT as its data source")
			return
		}
		if err := h.storeFullDemoFaceit(r.Context(), j); err != nil {
			h.rejectFullDemoFaceit(w, j, err)
			return
		}
	} else if !validateSegmentSelection(w, j, segmentIDs) {
		return
	}
	intent.SegmentIDs = segmentIDs
	// Build the render task now so an invalid music key fails fast here rather
	// than silently dropping the chained render later in the record worker.
	if _, err := tasks.NewRenderVariantTask(j.ID, intent.Variant, intent.MusicKey, intent.MusicVolume, intent.GameVolume, intent.Edit, intent.SegmentIDs); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	portraitSafeKillfeed := preset.KillfeedSource && intent.Edit.Format == renderplan.FormatShort9x16
	recordTask, err := tasks.NewGenerateRecordDemoTaskWithRecap(j.ID, hudMode, segmentIDs, portraitSafeKillfeed, useRecapPlan, intent)
	if err != nil {
		internalError(w, "build record task", err)
		return
	}
	// The task header is the worker's immutable source of truth. The job-scoped
	// artifact is the latest accepted choice shown by the workbench; duplicate
	// and rejected admissions must not replace the active choice.
	accepted := false
	_, err = h.queue.EnqueueWithTransition(recordTask, func(decision error) error {
		switch {
		case decision == nil:
			if err := h.generateIntents.Begin(j.ID, intent, func() error {
				return h.requireGenerateRenderIdle(j.ID)
			}); err != nil {
				return err
			}
			accepted = true
			return nil
		case errors.Is(decision, asynq.ErrDuplicateTask):
			existing, ok, readErr := h.readGenerateIntent(j.ID)
			if readErr != nil {
				return readErr
			}
			// Record uniqueness is per job, so any in-flight record:demo
			// answers duplicate. Only a stored generate intent that would
			// enqueue the same capture is this generate already in flight.
			// A plain queued record has no header and will not chain a
			// render; answering 202 here would claim generate admission
			// that never happened.
			if !ok {
				return fmt.Errorf("%w for job %s: queued capture is not a generate run", generateintent.ErrActiveRun, j.ID)
			}
			if !sameCapture(existing, intent) {
				return fmt.Errorf("%w for job %s: queued capture carries a different intent", generateintent.ErrActiveRun, j.ID)
			}
			intent = existing
			return nil
		default:
			if accepted {
				_, err := h.generateIntents.Finish(j.ID, intent.ActiveRunID, func() error {
					return h.persistJobQueueDecision(j.ID, "generate record", decision)
				})
				return err
			}
			return nil
		}
	}, asynq.MaxRetry(0), asynq.Unique(renderUniqueTTL))
	if err != nil {
		// A duplicate is success (see StartRecording): a re-drive must not flip a
		// reel that is already capturing to failed.
		if errors.Is(err, asynq.ErrDuplicateTask) {
			writeJSON(w, http.StatusAccepted, map[string]any{
				"id":        j.ID,
				"task":      tasks.TypeRecordDemo,
				"variant":   intent.Variant,
				"duplicate": true,
			})
			return
		}
		if errors.Is(err, generateintent.ErrActiveRun) || errors.Is(err, errGenerateRenderActive) {
			writeCodedError(w, http.StatusConflict, generateWorkActive, "job already has active generate or render work")
			return
		}
		internalError(w, "enqueue record task", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":      j.ID,
		"task":    tasks.TypeRecordDemo,
		"variant": intent.Variant,
	})
}

// sameCapture reports whether two intents would enqueue the same record task,
// i.e. the same HUD mode, segment selection, killfeed framing and recap flag.
// Music and post-capture edit choices never change the capture, so a re-drive
// that only differs there is still the duplicate the queue says it is.
func sameCapture(a, b renderplan.GenerateIntent) bool {
	ca, okA := captureRequestFor(a)
	cb, okB := captureRequestFor(b)
	return okA && okB && ca.hudMode == cb.hudMode && ca.useRecapPlan == cb.useRecapPlan &&
		ca.portraitSafeKillfeed == cb.portraitSafeKillfeed && slices.Equal(ca.segmentIDs, cb.segmentIDs)
}

type captureRequest struct {
	hudMode              string
	segmentIDs           []string
	portraitSafeKillfeed bool
	useRecapPlan         bool
}

// captureRequestFor derives the capture-time choices of an intent the same way
// StartGenerate does when it builds the record task.
func captureRequestFor(intent renderplan.GenerateIntent) (captureRequest, bool) {
	preset, ok := editor.PresetByName(intent.Variant)
	if !ok {
		return captureRequest{}, false
	}
	hudMode, useRecapPlan := applyCaptureOverrides(preset.HUDMode, intent.Edit)
	segmentIDs := intent.SegmentIDs
	if useRecapPlan || len(segmentIDs) == 0 {
		segmentIDs = nil
	}
	return captureRequest{
		hudMode:              hudMode,
		segmentIDs:           segmentIDs,
		portraitSafeKillfeed: preset.KillfeedSource && intent.Edit.Format == renderplan.FormatShort9x16,
		useRecapPlan:         useRecapPlan,
	}, true
}

func canGenerateFromStatus(status job.Status) bool {
	switch status {
	case job.StatusParsed, job.StatusRecorded, job.StatusComposed, job.StatusDone, job.StatusReviewRequired, job.StatusFailed:
		return true
	default:
		return false
	}
}

func (h *Handlers) requireGenerateRenderIdle(id uuid.UUID) error {
	for _, loadout := range renderplan.LoadoutCatalog() {
		state, ok, err := h.readRenderVariantState(id, loadout.Variant)
		if err != nil {
			return fmt.Errorf("read %s render state: %w", loadout.Variant, err)
		}
		if ok && (state.Status == renderplan.RenderVariantStatusQueued || state.Status == renderplan.RenderVariantStatusRendering) {
			return fmt.Errorf("%w: %s is %s", errGenerateRenderActive, loadout.Variant, state.Status)
		}
	}
	return nil
}

func (h *Handlers) readGenerateIntent(id uuid.UUID) (renderplan.GenerateIntent, bool, error) {
	return h.generateIntents.Read(id)
}

// StartComposition handles POST /api/jobs/{id}/compose. Admission claims the
// job as composing and is unique per job (same shape as record/render) so two
// concurrent POSTs that both see recorded cannot enqueue two compose tasks
// against the same final artifact. A rejection at admission leaves the
// recorded/composed state untouched so the client can retry once the queue
// recovers, while a shutdown discard of admitted work fails the job like
// every other queued stage.
func (h *Handlers) StartComposition(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	if j.Status != job.StatusRecorded && j.Status != job.StatusComposed && j.Status != job.StatusReviewRequired {
		writeError(w, http.StatusConflict, fmt.Sprintf("job is not ready to compose (status=%s)", j.Status))
		return
	}
	if !h.requireComposeEnabled(w) {
		return
	}
	task, err := tasks.NewComposeFinalTask(j.ID)
	if err != nil {
		internalError(w, "build compose task", err)
		return
	}
	if _, err := h.queue.EnqueueWithTransition(task, claimQueueStage(
		func() error { return h.repo.UpdateStatus(r.Context(), j.ID, job.StatusComposing, "") },
		func(decision error) error { return h.persistJobQueueDecision(j.ID, "compose", decision) },
	), asynq.MaxRetry(0), asynq.Unique(renderUniqueTTL)); err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			writeJSON(w, http.StatusAccepted, map[string]any{
				"id":        j.ID,
				"task":      tasks.TypeComposeFinal,
				"duplicate": true,
			})
			return
		}
		internalError(w, "enqueue compose task", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":     j.ID,
		"task":   tasks.TypeComposeFinal,
		"status": job.StatusComposing,
	})
}

// renderMusicRequest is the "music" field of a render request. It accepts
// either a bare track key ("phonk-01") or an object {"key","volume","game_volume"}
// so a client can set mix gains. Volume is in (0,1]; 0 means the render
// default. GameVolume is in [0,1]; nil keeps the 0.70 mix.
// Accepting a bare string keeps older clients working.
type renderMusicRequest struct {
	Key        string
	Volume     float64
	GameVolume *float64
	set        bool
}

func (m *renderMusicRequest) UnmarshalJSON(data []byte) error {
	*m = renderMusicRequest{set: true}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '"' {
		return json.Unmarshal(trimmed, &m.Key)
	}
	var obj struct {
		Key        string   `json:"key"`
		Volume     float64  `json:"volume"`
		GameVolume *float64 `json:"game_volume"`
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&obj); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errMultipleJSONValues
		}
		return fmt.Errorf("invalid trailing music JSON data: %w", err)
	}
	m.Key = obj.Key
	m.Volume = obj.Volume
	m.GameVolume = obj.GameVolume
	return nil
}

// renderEditRequest preserves JSON field presence so review corrections can
// update one choice without resetting the rest of the approved edit contract.
type renderEditRequest struct {
	Format              *string  `json:"format"`
	KillEffect          *string  `json:"killEffect"`
	Transition          *string  `json:"transition"`
	Intro               *bool    `json:"intro"`
	Outro               *bool    `json:"outro"`
	HookText            *bool    `json:"hook_text"`
	KillCounter         *bool    `json:"kill_counter"`
	MatchRecap          *bool    `json:"match_recap"`
	VoiceComms          *bool    `json:"voice_comms"`
	VoiceVolume         *float64 `json:"voice_volume"`
	NativeHUD           *bool    `json:"native_hud"`
	CoverStrategy       *string  `json:"cover_strategy"`
	CoverFirstFrame     *bool    `json:"cover_first_frame"`
	IntroText           *string  `json:"intro_text"`
	OutroText           *string  `json:"outro_text"`
	KeyDropFamily       *string  `json:"keydrop_family"`
	KeyDropStyle        *string  `json:"keydrop_style"`
	KeyDropCode         *string  `json:"keydrop_code"`
	KeyDropPositionY    *float64 `json:"keydrop_position_y"`
	KeyDropStartSeconds *float64 `json:"keydrop_start_seconds"`
	KeyDropEndSeconds   *float64 `json:"keydrop_end_seconds"`
	DemoSource          *string  `json:"demo_source"`
	OverlayTheme        *string  `json:"overlay_theme"`
}

func (r renderEditRequest) merge(base renderplan.EditRequest) renderplan.EditRequest {
	if r.Format != nil {
		base.Format = *r.Format
	}
	if r.KillEffect != nil {
		base.KillEffect = *r.KillEffect
	}
	if r.Transition != nil {
		base.Transition = *r.Transition
	}
	if r.Intro != nil {
		base.Intro = *r.Intro
	}
	if r.Outro != nil {
		base.Outro = *r.Outro
	}
	if r.HookText != nil {
		base.HookText = *r.HookText
	}
	if r.KillCounter != nil {
		base.KillCounter = *r.KillCounter
	}
	if r.MatchRecap != nil {
		base.MatchRecap = *r.MatchRecap
	}
	if r.VoiceComms != nil {
		base.VoiceComms = *r.VoiceComms
	}
	if r.VoiceVolume != nil {
		v := *r.VoiceVolume
		base.VoiceVolume = &v
	}
	if r.NativeHUD != nil {
		base.NativeHUD = *r.NativeHUD
	}
	if r.CoverStrategy != nil {
		base.CoverStrategy = *r.CoverStrategy
	}
	if r.CoverFirstFrame != nil {
		base.CoverFirstFrame = *r.CoverFirstFrame
	}
	if r.IntroText != nil {
		base.IntroText = *r.IntroText
	}
	if r.OutroText != nil {
		base.OutroText = *r.OutroText
	}
	if r.KeyDropFamily != nil {
		base.KeyDropFamily = *r.KeyDropFamily
	}
	if r.KeyDropStyle != nil {
		base.KeyDropStyle = *r.KeyDropStyle
	}
	if r.KeyDropCode != nil {
		base.KeyDropCode = *r.KeyDropCode
	}
	if r.KeyDropPositionY != nil {
		y := *r.KeyDropPositionY
		base.KeyDropPositionY = &y
	}
	if r.KeyDropStartSeconds != nil {
		s := *r.KeyDropStartSeconds
		base.KeyDropStartSeconds = &s
	}
	if r.KeyDropEndSeconds != nil {
		e := *r.KeyDropEndSeconds
		base.KeyDropEndSeconds = &e
	}
	if r.DemoSource != nil {
		base.DemoSource = *r.DemoSource
	}
	if r.OverlayTheme != nil {
		base.OverlayTheme = *r.OverlayTheme
	}
	return renderplan.NormalizeEditRequest(base)
}

func (r renderEditRequest) complete() bool {
	return r.Format != nil &&
		r.KillEffect != nil &&
		r.Transition != nil &&
		r.Intro != nil &&
		r.Outro != nil &&
		r.HookText != nil &&
		r.KillCounter != nil &&
		r.CoverStrategy != nil &&
		r.CoverFirstFrame != nil &&
		r.IntroText != nil &&
		r.OutroText != nil
}

// StartRenderVariant handles POST /api/jobs/{id}/renders/{variant}.
func (h *Handlers) StartRenderVariant(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	h.renderStateMu.Lock()
	defer h.renderStateMu.Unlock()
	if j.Status != job.StatusRecorded && j.Status != job.StatusComposed && j.Status != job.StatusReviewRequired && j.Status != job.StatusDone {
		writeError(w, http.StatusConflict, fmt.Sprintf("job is not ready to render (status=%s)", j.Status))
		return
	}
	loadout, err := renderplan.LoadoutForVariant(variant)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Optional JSON body { "music": "<track-key>", "edit": {...} } selects a
	// track to mix in. "music" also accepts an object
	// {"key","volume","game_volume"} so the client can set mix gains; volume
	// is in (0,1], 0 means the default. game_volume is in [0,1]; omitted keeps
	// the 0.70 mix. An optional "segment_ids" narrows the render to exactly
	// those recorded segments, compiled in the given order; omitted or empty
	// renders every recorded segment.
	var musicRequest renderMusicRequest
	var editPatch renderEditRequest
	var expectedArtifactPrefix string
	var expectedWarnings []string
	var segmentIDs []string
	if r.Body != nil {
		var req struct {
			Music                  renderMusicRequest `json:"music"`
			Edit                   renderEditRequest  `json:"edit"`
			ExpectedArtifactPrefix string             `json:"expected_artifact_prefix"`
			ExpectedWarnings       []string           `json:"expected_warnings"`
			SegmentIDs             []string           `json:"segment_ids"`
		}
		switch err := decodeSingleJSONBody(w, r, &req, true); {
		case err == nil, errors.Is(err, io.EOF):
			musicRequest = req.Music
			editPatch = req.Edit
			if musicRequest.Volume < 0 || musicRequest.Volume > 1 {
				writeError(w, http.StatusBadRequest, "music volume must be between 0 and 1")
				return
			}
			if musicRequest.GameVolume != nil && (*musicRequest.GameVolume < 0 || *musicRequest.GameVolume > 1) {
				writeError(w, http.StatusBadRequest, "game volume must be between 0 and 1")
				return
			}
			if !validateSegmentSelection(w, j, req.SegmentIDs) {
				return
			}
			segmentIDs = req.SegmentIDs
			expectedArtifactPrefix = req.ExpectedArtifactPrefix
			expectedWarnings = req.ExpectedWarnings
		default:
			writeError(w, http.StatusBadRequest, "invalid render request JSON")
			return
		}
	}
	previous, _, err := h.readOrMaterializeRenderVariantStateLocked(j.ID, variant)
	if err != nil {
		internalError(w, "read render state", err)
		return
	}
	reviewReplacement := previous != nil && previous.Status == renderplan.RenderVariantStatusReview
	if reviewReplacement &&
		(expectedArtifactPrefix == "" ||
			expectedWarnings == nil ||
			previous.ArtifactPrefix != expectedArtifactPrefix ||
			!slices.Equal(previous.Warnings, expectedWarnings)) {
		writeError(w, http.StatusConflict, "render changed while the correction was being prepared; inspect the current warnings")
		return
	}
	if !reviewReplacement && (expectedArtifactPrefix != "" || expectedWarnings != nil) {
		writeError(w, http.StatusConflict, "render is no longer awaiting this correction")
		return
	}
	editRequest := renderplan.DefaultEditRequest()
	var musicKey string
	var musicVolume float64
	var gameVolume *float64
	if reviewReplacement {
		document, err := h.readRenderVariantDocument(previous.EditDocumentKey)
		if err != nil {
			internalError(w, "read effective render document for correction", err)
			return
		}
		if document == nil {
			if !editPatch.complete() || !musicRequest.set {
				writeError(w, http.StatusConflict, "the reviewed render has no effective edit document; submit every edit and music choice")
				return
			}
		} else {
			editRequest = document.Edit
			if document.Music != nil {
				musicKey = document.Music.Key
				musicVolume = document.Music.Volume
				gameVolume = document.Music.GameVolume
			} else if !musicRequest.set {
				writeError(w, http.StatusConflict, "the reviewed render does not record its effective music choice; submit music explicitly")
				return
			}
		}
	}
	editRequest = editPatch.merge(editRequest)
	if err := editRequest.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if musicRequest.set {
		musicKey = musicRequest.Key
		musicVolume = musicRequest.Volume
		gameVolume = musicRequest.GameVolume
	}
	task, err := tasks.NewRenderVariantTask(j.ID, variant, musicKey, musicVolume, gameVolume, editRequest, segmentIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:    j.ID,
		Loadout:  loadout,
		Status:   renderplan.RenderVariantStatusQueued,
		Previous: previous,
	})
	if err != nil {
		internalError(w, "build render state", err)
		return
	}
	queuedState := state
	var admitted atomic.Bool
	_, err = h.queue.EnqueueWithTransition(task, func(decision error) error {
		postAdmission := admitted.Load()
		if postAdmission {
			h.renderStateMu.Lock()
			defer h.renderStateMu.Unlock()
		}
		switch {
		case decision == nil:
			err := h.generateIntents.WhileIdle(j.ID, func() error {
				return h.writeRenderVariantState(state)
			})
			if err == nil {
				admitted.Store(true)
			}
			return err
		case errors.Is(decision, asynq.ErrDuplicateTask):
			existing, ok, readErr := h.readRenderVariantState(j.ID, variant)
			if readErr != nil {
				return readErr
			}
			if ok {
				state = *existing
			}
			return nil
		default:
			if reviewReplacement {
				if !postAdmission {
					// Rejected work never published queuedState, so preserving
					// the prior review requires no compensating write.
					return nil
				}
				current, exists, readErr := h.readRenderVariantState(j.ID, variant)
				if readErr != nil {
					return readErr
				}
				matches, compareErr := sameRenderVariantState(current, &queuedState)
				if compareErr != nil {
					return compareErr
				}
				if !exists || !matches {
					// The worker or another request already advanced the
					// durable state. A stale discard must fail closed instead
					// of resurrecting the superseded review.
					return nil
				}
				state = *previous
				return h.writeRenderVariantState(*previous)
			}
			failedState, stateErr := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
				JobID:    j.ID,
				Loadout:  loadout,
				Status:   renderplan.RenderVariantStatusFailed,
				Error:    "enqueue render task: " + decision.Error(),
				Previous: &state,
			})
			if stateErr != nil {
				return stateErr
			}
			return h.writeRenderVariantState(failedState)
		}
	}, asynq.MaxRetry(0), asynq.Unique(renderUniqueTTL))
	if err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			if reviewReplacement {
				writeError(w, http.StatusConflict, "another render is already active; this correction was not accepted")
				return
			}
			writeJSON(w, http.StatusAccepted, map[string]any{
				"id":         j.ID,
				"task":       tasks.TypeRenderVariant,
				"variant":    variant,
				"status":     state.Status,
				"status_key": mustRenderVariantStatusKey(j.ID, variant),
				"duplicate":  true,
			})
			return
		}
		if errors.Is(err, generateintent.ErrActiveRun) {
			writeError(w, http.StatusConflict, "guided generation is active for this job")
			return
		}
		internalError(w, "enqueue render task", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":         j.ID,
		"task":       tasks.TypeRenderVariant,
		"variant":    variant,
		"status":     state.Status,
		"status_key": mustRenderVariantStatusKey(j.ID, variant),
		"accepted":   true,
	})
}

func sameRenderVariantState(a, b *renderplan.RenderVariantState) (bool, error) {
	if a == nil || b == nil {
		return a == nil && b == nil, nil
	}
	aJSON, err := json.Marshal(a)
	if err != nil {
		return false, fmt.Errorf("encode current render state for comparison: %w", err)
	}
	bJSON, err := json.Marshal(b)
	if err != nil {
		return false, fmt.Errorf("encode expected render state for comparison: %w", err)
	}
	return bytes.Equal(aJSON, bJSON), nil
}

const maxRenderReviewNoteLength = 1000

// ResolveRenderReview records that a human inspected the current QA warnings
// and documented why they are intentional. The request must echo the exact
// artifact revision and warnings it showed, so a racing or later render can
// never inherit a stale approval.
func (h *Handlers) ResolveRenderReview(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	if _, err := renderplan.LoadoutForVariant(variant); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req struct {
		Note                   string   `json:"note"`
		ExpectedArtifactPrefix string   `json:"expected_artifact_prefix"`
		ExpectedWarnings       []string `json:"expected_warnings"`
	}
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid review resolution JSON")
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	if req.Note == "" {
		writeError(w, http.StatusBadRequest, "review note is required")
		return
	}
	if len([]rune(req.Note)) > maxRenderReviewNoteLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("review note must be at most %d characters", maxRenderReviewNoteLength))
		return
	}

	h.renderStateMu.Lock()
	defer h.renderStateMu.Unlock()
	state, exists, err := h.readOrMaterializeRenderVariantStateLocked(j.ID, variant)
	if err != nil {
		internalError(w, "read render state for review", err)
		return
	}
	if !exists || state.Status != renderplan.RenderVariantStatusReview {
		writeError(w, http.StatusConflict, "render is not awaiting review")
		return
	}
	if req.ExpectedArtifactPrefix != state.ArtifactPrefix ||
		!slices.Equal(req.ExpectedWarnings, state.Warnings) {
		writeError(w, http.StatusConflict, "render changed while it was being reviewed; inspect the current warnings")
		return
	}
	now := time.Now().UTC()
	state.Status = renderplan.RenderVariantStatusReady
	state.ReviewResolution = &renderplan.RenderReviewResolution{
		ArtifactPrefix: state.ArtifactPrefix,
		Warnings:       append([]string(nil), state.Warnings...),
		Note:           req.Note,
		ReviewedAt:     now,
	}
	state.UpdatedAt = now
	if err := h.writeRenderVariantState(*state); err != nil {
		internalError(w, "write render review resolution", err)
		return
	}
	h.writeRenderVariant(w, state)
}

// GetRenderVariant handles GET /api/jobs/{id}/renders/{variant}.
func (h *Handlers) GetRenderVariant(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	if _, err := renderplan.LoadoutForVariant(variant); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	state, exists, err := h.readOrMaterializeRenderVariantState(j.ID, variant)
	if err != nil {
		internalError(w, "read render state", err)
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "render variant not found")
		return
	}
	h.writeRenderVariant(w, state)
}

// readOrMaterializeRenderVariantState reads the durable state, migrating a
// warning-bearing legacy render result before returning it. The shared render
// lock makes that migration atomic with correction and review-resolution POSTs:
// the API never exposes a review CAS token that those endpoints cannot consume.
func (h *Handlers) readOrMaterializeRenderVariantState(id uuid.UUID, variant string) (*renderplan.RenderVariantState, bool, error) {
	h.renderStateMu.Lock()
	defer h.renderStateMu.Unlock()
	return h.readOrMaterializeRenderVariantStateLocked(id, variant)
}

// MaterializeRenderVariantStates runs the legacy render-state migration for
// every variant of the given jobs once, at startup, so a Studio poll finds the
// durable state already settled and GetRenderVariant is a read in steady
// state. The per-request path keeps the same migration as a safety net for a
// result the worker writes after this pass. Returns how many states were
// rewritten; errors are joined so one corrupt document cannot hide the rest.
func (h *Handlers) MaterializeRenderVariantStates(ctx context.Context, jobs []job.Job) (int, error) {
	migrated := 0
	var errs []error
	for _, j := range jobs {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		for _, loadout := range renderplan.LoadoutCatalog() {
			before, hadState, err := h.readRenderVariantState(j.ID, loadout.Variant)
			if err != nil {
				errs = append(errs, fmt.Errorf("read render state for job %s variant %s: %w", j.ID, loadout.Variant, err))
				continue
			}
			after, ok, err := h.readOrMaterializeRenderVariantState(j.ID, loadout.Variant)
			if err != nil {
				errs = append(errs, fmt.Errorf("materialize render state for job %s variant %s: %w", j.ID, loadout.Variant, err))
				continue
			}
			if !ok {
				continue
			}
			// A missing state only materializes on disk when it lands in review;
			// a present one is rewritten when its status or warnings moved.
			rewritten := (!hadState && after.Status == renderplan.RenderVariantStatusReview) ||
				(hadState && (before.Status != after.Status || !slices.Equal(before.Warnings, after.Warnings)))
			if rewritten {
				migrated++
			}
		}
	}
	return migrated, errors.Join(errs...)
}

// readOrMaterializeRenderVariantStateLocked performs the durable migration.
// The caller must hold renderStateMu so the returned review token and the state
// consumed by correction or resolution requests are one coherent revision.
func (h *Handlers) readOrMaterializeRenderVariantStateLocked(id uuid.UUID, variant string) (*renderplan.RenderVariantState, bool, error) {
	if state, ok, err := h.readRenderVariantState(id, variant); err != nil {
		return nil, false, err
	} else if ok {
		if state.Status == renderplan.RenderVariantStatusReady {
			warnings, err := h.readCompleteRenderWarnings(*state)
			if err != nil {
				return nil, false, err
			}
			switch {
			case state.ReviewResolvedFor(warnings):
				if slices.Equal(state.Warnings, warnings) {
					return state, true, nil
				}
				state.Warnings = append([]string(nil), warnings...)
			case len(warnings) == 0:
				if len(state.Warnings) == 0 && state.ReviewResolution == nil {
					return state, true, nil
				}
				state.Warnings = nil
				state.ReviewResolution = nil
			default:
				state.Status = renderplan.RenderVariantStatusReview
				state.Warnings = append([]string(nil), warnings...)
				state.ReviewResolution = nil
			}
			state.UpdatedAt = time.Now().UTC()
			if err := h.writeRenderVariantState(*state); err != nil {
				return nil, false, err
			}
		}
		return state, true, nil
	}
	resultRef, err := renderplan.NewRenderVariantArtifactRef(id, variant, renderplan.RenderVariantArtifactResult, "")
	if err != nil {
		return nil, false, err
	}
	rc, err := h.storage.Open(resultRef.Key)
	if err != nil {
		if storage.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer rc.Close()

	var result editor.Result
	if err := json.NewDecoder(rc).Decode(&result); err != nil {
		return nil, false, err
	}
	warnings := renderplan.CompleteRenderWarnings(result)
	status := "ready"
	if result.Error != "" {
		status = "failed"
	} else if len(warnings) > 0 {
		status = renderplan.RenderVariantStatusReview
	}
	loadout, err := renderplan.LoadoutForVariant(variant)
	if err != nil {
		return nil, false, err
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:    id,
		Loadout:  loadout,
		Status:   status,
		Warnings: warnings,
		Error:    result.Error,
	})
	if err != nil {
		return nil, false, err
	}
	if state.Status == renderplan.RenderVariantStatusReview {
		if err := h.writeRenderVariantState(state); err != nil {
			return nil, false, err
		}
	}
	return &state, true, nil
}

func (h *Handlers) readCompleteRenderWarnings(state renderplan.RenderVariantState) ([]string, error) {
	resultRef, err := renderplan.NewRenderVariantArtifactRefForState(
		state,
		renderplan.RenderVariantArtifactResult,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("resolve render result for warning migration: %w", err)
	}
	rc, err := h.storage.Open(resultRef.Key)
	if err != nil {
		return nil, fmt.Errorf("open render result for warning migration: %w", err)
	}
	defer rc.Close()
	var result editor.Result
	if err := json.NewDecoder(rc).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode render result for warning migration: %w", err)
	}
	return renderplan.CompleteRenderWarnings(result), nil
}

// renderArtifactLister is the optional storage capability GetRenderVariant uses
// to report the reel's real artifact file names. Local filesystem storage
// implements it; a backend without listing reports empty arrays.
type renderArtifactLister interface {
	List(prefix string) ([]string, error)
}

// listArtifactDir lists the base file names in the storage directory that holds
// key (e.g. the segments dir for a segment-clip key, or the videos dir for a
// render-video key). ok is false when the backend cannot list directories or the
// listing failed; a directory a stage has not written yet lists as empty with
// ok true. Callers build their own key and filter the returned names.
func listArtifactDir(store storage.Storage, key string) ([]string, bool) {
	lister, ok := store.(renderArtifactLister)
	if !ok {
		return nil, false
	}
	files, err := lister.List(path.Dir(key))
	if err != nil {
		return nil, false
	}
	return files, true
}

// renderVariantResponse augments the durable render state with the reel's real
// on-disk artifact names, so the client addresses the reel's video and cover by
// the names the editor actually wrote instead of guessing them from segment ids.
type renderVariantResponse struct {
	*renderplan.RenderVariantState
	Videos     []string                  `json:"videos"`
	Covers     []string                  `json:"covers"`
	SegmentIDs []string                  `json:"segment_ids,omitempty"`
	Edit       *renderplan.EditRequest   `json:"edit,omitempty"`
	Music      *renderplan.MusicSnapshot `json:"music,omitempty"`
}

// artifactNamePlaceholder is a valid artifact token used only to resolve a
// variant's artifact directory key; its base name is discarded.
const artifactNamePlaceholder = "placeholder"

// writeRenderVariant writes the render-variant state plus the reel's real video
// and cover artifact names (empty arrays when the variant has none yet).
func (h *Handlers) writeRenderVariant(w http.ResponseWriter, state *renderplan.RenderVariantState) {
	videos, err := h.listRenderArtifactNames(*state, renderplan.RenderVariantArtifactVideo)
	if err != nil {
		internalError(w, "list render videos", err)
		return
	}
	covers, err := h.listRenderArtifactNames(*state, renderplan.RenderVariantArtifactCover)
	if err != nil {
		internalError(w, "list render covers", err)
		return
	}
	document, err := h.readRenderVariantDocument(state.EditDocumentKey)
	if err != nil {
		internalError(w, "read effective render document", err)
		return
	}
	var edit *renderplan.EditRequest
	var music *renderplan.MusicSnapshot
	var segmentIDs []string
	if document != nil {
		edit = &document.Edit
		music = document.Music
		segmentIDs = document.Selection.SegmentIDs
	}
	writeJSON(w, http.StatusOK, renderVariantResponse{
		RenderVariantState: state,
		Videos:             videos,
		Covers:             covers,
		SegmentIDs:         segmentIDs,
		Edit:               edit,
		Music:              music,
	})
}

func (h *Handlers) readRenderVariantDocument(key string) (*renderplan.EditDocument, error) {
	if key == "" {
		return nil, nil
	}
	rc, err := h.storage.Open(key)
	if err != nil {
		if storage.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rc.Close()
	var document renderplan.EditDocument
	if err := json.NewDecoder(rc).Decode(&document); err != nil {
		return nil, err
	}
	edit := renderplan.NormalizeEditRequest(document.Edit)
	if err := edit.Validate(); err != nil {
		return nil, err
	}
	document.Edit = edit
	return &document, nil
}

// listRenderArtifactNames returns the artifact names (file base names, extension
// stripped) present under the variant's directory for the given kind, reusing
// the same key resolution the videos/{name} and covers/{name} handlers use. The
// result is empty when the backend cannot list or the directory is absent.
func (h *Handlers) listRenderArtifactNames(state renderplan.RenderVariantState, kind renderplan.RenderVariantArtifactKind) ([]string, error) {
	ref, err := renderplan.NewRenderVariantArtifactRefForState(state, kind, artifactNamePlaceholder)
	if err != nil {
		return nil, err
	}
	files, ok := listArtifactDir(h.storage, ref.Key)
	if !ok {
		return []string{}, nil
	}
	ext := path.Ext(ref.Key)
	names := make([]string, 0, len(files))
	for _, f := range files {
		if ext != "" && !strings.HasSuffix(f, ext) {
			continue
		}
		names = append(names, strings.TrimSuffix(f, ext))
	}
	return names, nil
}

func (h *Handlers) readRenderVariantState(id uuid.UUID, variant string) (*renderplan.RenderVariantState, bool, error) {
	key, err := renderplan.RenderVariantStateKey(id, variant)
	if err != nil {
		return nil, false, err
	}
	rc, err := h.storage.Open(key)
	if err != nil {
		if !storage.IsNotExist(err) {
			return nil, false, err
		}
		return nil, false, nil
	}
	defer rc.Close()
	var state renderplan.RenderVariantState
	if err := json.NewDecoder(rc).Decode(&state); err != nil {
		return nil, false, err
	}
	if state.JobID != id || state.Variant != variant {
		return nil, false, fmt.Errorf("render state identity does not match request")
	}
	return &state, true, nil
}

func (h *Handlers) writeRenderVariantState(state renderplan.RenderVariantState) error {
	key, err := renderplan.RenderVariantStateKey(state.JobID, state.Variant)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return h.storage.Put(key, bytes.NewReader(b))
}

func mustRenderVariantStatusKey(id uuid.UUID, variant string) string {
	key, err := renderplan.RenderVariantStateKey(id, variant)
	if err != nil {
		return ""
	}
	return key
}

// GetRenderPublishBoard handles GET /api/jobs/{id}/renders/{variant}/publish.
func (h *Handlers) GetRenderPublishBoard(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	if _, err := renderplan.LoadoutForVariant(variant); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	state, exists, err := h.readOrMaterializeRenderVariantState(j.ID, variant)
	if err != nil {
		internalError(w, "read render state for publish board", err)
		return
	}
	snapshot := renderVariantSnapshot{jobID: j.ID, variant: variant}
	if exists {
		snapshot.state = state
	}
	result, _, ok := h.loadRenderResultFromSnapshot(w, snapshot)
	if !ok {
		return
	}
	segmentIDs := make([]string, 0, len(result.Shorts))
	for _, short := range result.Shorts {
		segmentIDs = append(segmentIDs, short.SegmentID)
	}
	artifactPrefix := ""
	if snapshot.state != nil {
		artifactPrefix = snapshot.state.ArtifactPrefix
	}
	board, err := renderplan.NewPublishBoardForVariant(renderplan.NewPublishBoardForVariantOptions{
		JobID:          j.ID,
		Variant:        variant,
		SegmentIDs:     segmentIDs,
		Warnings:       unresolvedRenderWarnings(snapshot.state, renderplan.CompleteRenderWarnings(result)),
		Error:          result.Error,
		CoversRequired: result.CoversEnabled,
		ArtifactPrefix: artifactPrefix,
		ArtifactExists: h.storage.Exists,
	})
	if err != nil {
		internalError(w, "build publish board", err)
		return
	}
	if snapshot.state != nil {
		switch snapshot.state.Status {
		case renderplan.RenderVariantStatusQueued, renderplan.RenderVariantStatusRendering:
			board.Status = snapshot.state.Status
			board.RenderReady = false
		case renderplan.RenderVariantStatusFailed:
			board.Status = renderplan.RenderVariantStatusFailed
			board.RenderReady = false
			board.Error = snapshot.state.Error
		}
	}
	response := renderPublishBoardResponse{PublishBoard: board}
	if snapshot.state != nil && snapshot.state.Status == renderplan.RenderVariantStatusReview {
		response.ExpectedArtifactPrefix = snapshot.state.ArtifactPrefix
		response.ExpectedWarnings = append([]string(nil), snapshot.state.Warnings...)
	}
	writeJSON(w, http.StatusOK, response)
}

type renderPublishBoardResponse struct {
	renderplan.PublishBoard
	ExpectedArtifactPrefix string   `json:"expected_artifact_prefix,omitempty"`
	ExpectedWarnings       []string `json:"expected_warnings,omitempty"`
}

func unresolvedRenderWarnings(state *renderplan.RenderVariantState, warnings []string) []string {
	if state != nil && state.ReviewResolvedFor(warnings) {
		return nil
	}
	return warnings
}

// GetRenderQuality handles GET /api/jobs/{id}/renders/{variant}/quality.
func (h *Handlers) GetRenderQuality(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	result, _, ok := h.loadRenderResult(w, j.ID, variant)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, renderplan.NewQualityReport(j.ID, variant, result))
}

// GetRenderPack streams the render variant publish-pack manifest.
func (h *Handlers) GetRenderPack(w http.ResponseWriter, r *http.Request) {
	h.streamRenderVariantArtifact(w, r, "application/json", renderplan.RenderVariantArtifactPackManifest, "")
}

// GetRenderEditDocument streams the stable edit intent document.
func (h *Handlers) GetRenderEditDocument(w http.ResponseWriter, r *http.Request) {
	h.streamRenderVariantArtifact(w, r, "application/json", renderplan.RenderVariantArtifactEditDocument, "")
}

// GetRenderGallery streams the render variant publish gallery.
func (h *Handlers) GetRenderGallery(w http.ResponseWriter, r *http.Request) {
	h.streamRenderVariantArtifact(w, r, "text/html; charset=utf-8", renderplan.RenderVariantArtifactGallery, "")
}

// GetRenderVideo streams one render variant MP4 artifact.
func (h *Handlers) GetRenderVideo(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	h.streamRenderVariantArtifact(w, r, "video/mp4", renderplan.RenderVariantArtifactVideo, name)
}

// renderArtifactDeleter is the optional storage capability DeleteRenderVideo
// needs. Local filesystem storage implements it; a backend without delete
// support makes the endpoint report 501 rather than pretending to delete.
type renderArtifactDeleter interface {
	Delete(key string) error
}

// DeleteRenderVideo handles DELETE /api/jobs/{id}/renders/{variant}/videos/{name}:
// it removes one reel's video plus its cover and caption artifacts so the user
// can clear finished reels from the library and free disk space. Idempotent —
// deleting an already-deleted reel succeeds.
func (h *Handlers) DeleteRenderVideo(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	if _, err := renderplan.LoadoutForVariant(variant); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	name := chi.URLParam(r, "name")
	if _, err := renderplan.NewRenderVariantArtifactRef(
		j.ID,
		variant,
		renderplan.RenderVariantArtifactVideo,
		name,
	); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	deleter, ok := h.storage.(renderArtifactDeleter)
	if !ok {
		writeError(w, http.StatusNotImplemented, "storage backend does not support delete")
		return
	}
	snapshot, err := h.currentRenderVariantSnapshot(j.ID, variant)
	if err != nil {
		internalError(w, "read render state for video deletion", err)
		return
	}
	kinds := []renderplan.RenderVariantArtifactKind{
		renderplan.RenderVariantArtifactVideo,
		renderplan.RenderVariantArtifactCover,
		renderplan.RenderVariantArtifactCaption,
	}
	for _, kind := range kinds {
		ref, err := snapshot.artifactRef(kind, name)
		if err != nil {
			internalError(w, "resolve render artifact for video deletion", err)
			return
		}
		if err := deleter.Delete(ref.Key); err != nil {
			internalError(w, "delete render artifact", err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// jobArtifactDeleter is the optional storage capability DeleteJob needs: a
// single-file Delete for the stored demo copy and a recursive DeleteTree for
// the job's artifact directory. Local filesystem storage implements it; a
// backend without delete support makes the endpoint report 501 rather than
// pretending to delete.
type jobArtifactDeleter interface {
	Delete(key string) error
	DeleteTree(key string) error
}

// jobIsInFlight reports whether a stage is actively working on the job's files
// or processes, so deleting now would race that work. queued is included: a
// parse/scan task may be about to run against the stored demo.
func jobIsInFlight(s job.Status) bool {
	switch s {
	case job.StatusQueued, job.StatusScanning, job.StatusParsing, job.StatusRecording, job.StatusComposing:
		return true
	default:
		return false
	}
}

// DeleteJob handles DELETE /api/jobs/{id}: it removes a job together with its
// artifact tree (jobs/<id>) and its stored demo copy (demos/<id>.dem) so the
// user can clear a demo from the library and reclaim disk space. Settled jobs
// (scanned, parsed, recorded, composed, done, failed) delete; a job with work
// in flight is refused with 409 until it settles. Render variants and guided
// generate runs have their own lifecycle that never moves job.Status, so the
// delete also holds the render-state lock and the generate intent lock and
// refuses while either reports active work: otherwise a running render worker
// keeps writing into a tree that no longer exists. The job row is removed last
// so a failed artifact delete leaves the row in place to retry. Idempotent —
// a repeat delete after success returns 404.
func (h *Handlers) DeleteJob(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	if jobIsInFlight(j.Status) {
		writeError(w, http.StatusConflict, fmt.Sprintf("job is %s; wait for it to settle before deleting", j.Status))
		return
	}
	deleter, ok := h.storage.(jobArtifactDeleter)
	if !ok {
		writeError(w, http.StatusNotImplemented, "storage backend does not support delete")
		return
	}
	// The per-job intent lock is what keeps a render from being admitted for
	// this job while its tree goes away (StartRenderVariant admits inside the
	// same WhileIdle). The process-wide renderStateMu is deliberately not
	// held: the tree can be gigabytes of captures and every other job's
	// render poll would stall behind the RemoveAll. The idle read is the same
	// lock-free read StartGenerate does under this lock.
	err := h.generateIntents.WhileIdle(j.ID, func() error {
		if err := h.requireGenerateRenderIdle(j.ID); err != nil {
			return err
		}
		if err := deleter.DeleteTree(artifacts.JobPrefix(j.ID)); err != nil {
			return fmt.Errorf("delete job artifacts: %w", err)
		}
		if err := deleter.Delete(fmt.Sprintf("demos/%s.dem", j.ID)); err != nil {
			return fmt.Errorf("delete job demo: %w", err)
		}
		if err := h.repo.Delete(r.Context(), j.ID); err != nil {
			return fmt.Errorf("delete job: %w", err)
		}
		return nil
	})
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, generateintent.ErrActiveRun), errors.Is(err, errGenerateRenderActive):
		writeCodedError(w, http.StatusConflict, generateWorkActive, "job has an active render or generate run; wait for it to settle before deleting")
	default:
		internalError(w, "delete job", err)
	}
}

// GetRenderCover streams one render variant cover artifact.
func (h *Handlers) GetRenderCover(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	h.streamRenderVariantArtifact(w, r, "image/jpeg", renderplan.RenderVariantArtifactCover, name)
}

// GetRenderCaption streams one render variant caption artifact.
func (h *Handlers) GetRenderCaption(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	h.streamRenderVariantArtifact(w, r, "text/plain; charset=utf-8", renderplan.RenderVariantArtifactCaption, name)
}

// GetRenderRevisionGallery streams the immutable gallery for one render revision.
func (h *Handlers) GetRenderRevisionGallery(w http.ResponseWriter, r *http.Request) {
	h.streamRenderVariantRevisionArtifact(w, r, "text/html; charset=utf-8", renderplan.RenderVariantArtifactGallery, "")
}

// GetRenderRevisionVideo streams one immutable render revision MP4.
func (h *Handlers) GetRenderRevisionVideo(w http.ResponseWriter, r *http.Request) {
	h.streamRenderVariantRevisionArtifact(w, r, "video/mp4", renderplan.RenderVariantArtifactVideo, chi.URLParam(r, "name"))
}

// GetRenderRevisionCover streams one immutable render revision cover.
func (h *Handlers) GetRenderRevisionCover(w http.ResponseWriter, r *http.Request) {
	h.streamRenderVariantRevisionArtifact(w, r, "image/jpeg", renderplan.RenderVariantArtifactCover, chi.URLParam(r, "name"))
}

// GetRenderRevisionCaption streams one immutable render revision caption.
func (h *Handlers) GetRenderRevisionCaption(w http.ResponseWriter, r *http.Request) {
	h.streamRenderVariantRevisionArtifact(w, r, "text/plain; charset=utf-8", renderplan.RenderVariantArtifactCaption, chi.URLParam(r, "name"))
}

func (h *Handlers) streamRenderVariantRevisionArtifact(w http.ResponseWriter, r *http.Request, contentType string, kind renderplan.RenderVariantArtifactKind, segmentID string) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	if _, err := renderplan.LoadoutForVariant(variant); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	revisionID, err := uuid.Parse(chi.URLParam(r, "revision"))
	if err != nil || revisionID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid render revision id")
		return
	}
	ref, err := renderplan.NewRenderVariantRevisionArtifactRef(j.ID, variant, revisionID, kind, segmentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rc, err := h.storage.Open(ref.Key)
	if err != nil {
		writeError(w, http.StatusNotFound, "render revision artifact not found")
		return
	}
	serveArtifact(w, r, contentType, rc)
}

func (h *Handlers) streamRenderVariantArtifact(w http.ResponseWriter, r *http.Request, contentType string, kind renderplan.RenderVariantArtifactKind, segmentID string) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	if _, err := renderplan.LoadoutForVariant(variant); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := renderplan.NewRenderVariantArtifactRef(j.ID, variant, kind, segmentID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	snapshot, err := h.currentRenderVariantSnapshot(j.ID, variant)
	if err != nil {
		internalError(w, "read render state for artifact", err)
		return
	}
	ref, err := snapshot.artifactRef(kind, segmentID)
	if err != nil {
		internalError(w, "resolve current render artifact", err)
		return
	}
	rc, err := h.storage.Open(ref.Key)
	if err != nil {
		writeError(w, http.StatusNotFound, "render artifact not found")
		return
	}
	serveArtifact(w, r, contentType, rc)
}

// serveArtifact writes an artifact body with the given content type. An empty
// type asks Go to sniff the stored bytes, which is useful when the durable key
// intentionally omits the uploaded source container. When the storage reader
// is seekable (the local filesystem backend hands out *os.File), it serves
// through http.ServeContent so Range requests are honoured. Non-seekable
// backends sniff a bounded prefix before streaming the complete body.
func serveArtifact(w http.ResponseWriter, r *http.Request, contentType string, rc io.ReadCloser) {
	defer rc.Close()
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if rs, ok := rc.(io.ReadSeeker); ok {
		http.ServeContent(w, r, "", time.Time{}, rs)
		return
	}
	var body io.Reader = rc
	if contentType == "" {
		buffered := bufio.NewReader(rc)
		prefix, _ := buffered.Peek(512)
		w.Header().Set("Content-Type", http.DetectContentType(prefix))
		body = buffered
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

func (h *Handlers) loadRenderResult(w http.ResponseWriter, id uuid.UUID, variant string) (editor.Result, string, bool) {
	if _, err := renderplan.LoadoutForVariant(variant); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return editor.Result{}, "", false
	}
	snapshot, err := h.currentRenderVariantSnapshot(id, variant)
	if err != nil {
		internalError(w, "read render state for result", err)
		return editor.Result{}, "", false
	}
	return h.loadRenderResultFromSnapshot(w, snapshot)
}

func (h *Handlers) loadRenderResultFromSnapshot(w http.ResponseWriter, snapshot renderVariantSnapshot) (editor.Result, string, bool) {
	resultRef, err := snapshot.artifactRef(renderplan.RenderVariantArtifactResult, "")
	if err != nil {
		internalError(w, "resolve current render result", err)
		return editor.Result{}, "", false
	}
	rc, err := h.storage.Open(resultRef.Key)
	if err != nil {
		writeError(w, http.StatusNotFound, "render variant not found")
		return editor.Result{}, "", false
	}
	defer rc.Close()
	var result editor.Result
	if err := json.NewDecoder(rc).Decode(&result); err != nil {
		internalError(w, "decode render result", err)
		return editor.Result{}, "", false
	}
	return result, resultRef.Key, true
}

type renderVariantSnapshot struct {
	jobID   uuid.UUID
	variant string
	state   *renderplan.RenderVariantState
}

func (h *Handlers) currentRenderVariantSnapshot(id uuid.UUID, variant string) (renderVariantSnapshot, error) {
	state, exists, err := h.readRenderVariantState(id, variant)
	if err != nil {
		return renderVariantSnapshot{}, err
	}
	snapshot := renderVariantSnapshot{jobID: id, variant: variant}
	if exists {
		snapshot.state = state
	}
	return snapshot, nil
}

func (s renderVariantSnapshot) artifactRef(kind renderplan.RenderVariantArtifactKind, name string) (renderplan.RenderVariantArtifactRef, error) {
	if s.state != nil {
		return renderplan.NewRenderVariantArtifactRefForState(*s.state, kind, name)
	}
	return renderplan.NewRenderVariantArtifactRef(s.jobID, s.variant, kind, name)
}

func (h *Handlers) loadJob(w http.ResponseWriter, r *http.Request) (job.Job, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return job.Job{}, false
	}
	j, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, job.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return job.Job{}, false
	}
	if err != nil {
		internalError(w, "load job", err)
		return job.Job{}, false
	}
	return j, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Default error codes by HTTP status. Every error body carries a `code` so a
// client can branch on it instead of grepping Spanish or English text; a
// handler with a more specific class uses writeCodedError. 503 deliberately
// has no default: the Studio proxy reserves `service_unavailable` for "the
// orchestrator is unreachable", so a Go 503 must name what is missing.
var defaultErrorCodes = map[int]string{
	http.StatusBadRequest:            "invalid_request",
	http.StatusUnauthorized:          "unauthorized",
	http.StatusForbidden:             "forbidden",
	http.StatusNotFound:              "not_found",
	http.StatusConflict:              "conflict",
	http.StatusRequestEntityTooLarge: "payload_too_large",
	http.StatusMisdirectedRequest:    "misdirected_request",
	http.StatusTooManyRequests:       "rate_limited",
	http.StatusInternalServerError:   "internal_error",
	http.StatusNotImplemented:        "not_implemented",
	http.StatusServiceUnavailable:    "not_configured",
	http.StatusGatewayTimeout:        "timeout",
}

func writeError(w http.ResponseWriter, status int, msg string) {
	code, ok := defaultErrorCodes[status]
	if !ok {
		code = "error"
	}
	writeCodedError(w, status, code, msg)
}

func writeCodedError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"code": code, "error": msg})
}

// internalError logs the underlying error at the boundary and returns a generic
// 500 to the client so driver/SQL/storage internals are not exposed.
func internalError(w http.ResponseWriter, op string, err error) {
	log.Printf("httpapi: %s: %v", op, err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
