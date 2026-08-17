package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tasks"
	"github.com/rechedev9/cliphub/internal/timelineplan"
)

const (
	maxEditorVideoBytes     = 8 << 30
	maxEditorMultipartBytes = maxEditorVideoBytes + 2<<20
	defaultEditorListLimit  = 50
	editorRenderUniqueTTL   = 24 * time.Hour
)

type createEditorAssetConfig struct {
	FileName string `json:"file_name,omitempty"`
}

type importEditorAssetRequest struct {
	Source  string `json:"source"`
	JobID   string `json:"job_id"`
	Variant string `json:"variant"`
	Name    string `json:"name"`
}

type createEditorProjectRequest struct {
	Title string `json:"title,omitempty"`
}

type editorPreviewRequest struct {
	Time float64 `json:"time"`
}

func (h *Handlers) editorReady(w http.ResponseWriter) bool {
	if h.editorAssets == nil || h.editorProjects == nil {
		writeError(w, http.StatusNotImplemented, "editor is not configured")
		return false
	}
	return true
}

func (h *Handlers) CreateEditorAsset(w http.ResponseWriter, r *http.Request) {
	if !h.editorReady(w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxEditorMultipartBytes)
	if err := r.ParseMultipartForm(multipartMemBudget); err != nil {
		writeError(w, http.StatusBadRequest, "parsing multipart form: "+err.Error())
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	file, header, err := r.FormFile("video")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing video file: "+err.Error())
		return
	}
	defer file.Close()

	var cfg createEditorAssetConfig
	if raw := r.FormValue("config"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid config JSON: "+err.Error())
			return
		}
	}
	fileName := strings.TrimSpace(cfg.FileName)
	if fileName == "" && header != nil {
		fileName = sanitizeDemoFileName(header.Filename)
	}
	fileName = mediaassets.SanitizeFileName(fileName)

	asset, err := h.ingestEditorAsset(r, file, fileName, mediaassets.OriginUpload, nil, "", "")
	if err != nil {
		if isBadRequest(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		internalError(w, "create editor asset", err)
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

func (h *Handlers) ImportEditorAsset(w http.ResponseWriter, r *http.Request) {
	if !h.editorReady(w) {
		return
	}
	var req importEditorAssetRequest
	if err := decodeSingleJSONBody(w, r, &req, false); err != nil {
		writeError(w, http.StatusBadRequest, "invalid import JSON")
		return
	}
	jobID, err := uuid.Parse(req.JobID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	key, fileName, err := h.resolveImportKey(req.Source, jobID, req.Variant, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rc, err := h.storage.Open(key)
	if err != nil {
		writeError(w, http.StatusNotFound, "source video not found")
		return
	}
	defer rc.Close()
	origin := mediaassets.OriginDemoRender
	if req.Source == "stream" {
		origin = mediaassets.OriginStreamRender
	}
	asset, err := h.ingestEditorAsset(r, rc, mediaassets.SanitizeFileName(fileName), origin, &jobID, req.Variant, req.Name)
	if err != nil {
		if isBadRequest(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		internalError(w, "import editor asset", err)
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

func (h *Handlers) resolveImportKey(source string, jobID uuid.UUID, variant, name string) (string, string, error) {
	switch source {
	case "demo":
		key, err := artifacts.RenderVariantVideoKey(jobID, variant, name)
		if err != nil {
			return "", "", err
		}
		return key, name + ".mp4", nil
	case "stream":
		key, err := streamclips.RenderVideoKey(jobID, variant, name)
		if err != nil {
			return "", "", err
		}
		return key, name + ".mp4", nil
	default:
		return "", "", errBadRequest("source must be demo or stream")
	}
}

func (h *Handlers) ingestEditorAsset(r *http.Request, src io.Reader, fileName string, origin mediaassets.Origin, jobID *uuid.UUID, variant, name string) (mediaassets.Asset, error) {
	tmp, err := os.CreateTemp("", "zv-editor-upload-*.mp4")
	if err != nil {
		return mediaassets.Asset{}, err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	h256 := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h256), src); err != nil {
		return mediaassets.Asset{}, err
	}
	digest := hex.EncodeToString(h256.Sum(nil))
	if existing, err := h.editorAssets.GetBySHA256(r.Context(), digest); err == nil {
		return existing, nil
	} else if err != nil && !errors.Is(err, mediaassets.ErrNotFound) {
		return mediaassets.Asset{}, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return mediaassets.Asset{}, err
	}
	probe := mediaassets.Probe{}
	if h.streamProber != nil {
		srcProbe, err := h.streamProber.Probe(r.Context(), tmp.Name())
		if err != nil {
			return mediaassets.Asset{}, errBadRequest("probe video: " + err.Error())
		}
		probe = mediaassets.Probe{
			Width:           srcProbe.Width,
			Height:          srcProbe.Height,
			DurationSeconds: srcProbe.DurationSeconds,
			VideoCodec:      srcProbe.VideoCodec,
			AudioCodec:      srcProbe.AudioCodec,
			FrameRate:       srcProbe.FrameRate,
			HasAudio:        srcProbe.AudioCodec != "",
		}
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return mediaassets.Asset{}, err
	}
	id := uuid.New()
	asset := mediaassets.Asset{
		ID:            id,
		SHA256:        digest,
		FileName:      fileName,
		Origin:        origin,
		OriginJobID:   jobID,
		OriginVariant: variant,
		OriginName:    name,
		Probe:         probe,
		MediaKey:      mediaassets.MediaKey(id),
	}
	if err := asset.Validate(); err != nil {
		return mediaassets.Asset{}, errBadRequest(err.Error())
	}
	if err := h.storage.Put(asset.MediaKey, tmp); err != nil {
		return mediaassets.Asset{}, err
	}
	if err := h.editorAssets.Create(r.Context(), &asset); err != nil {
		return mediaassets.Asset{}, err
	}
	return asset, nil
}

func (h *Handlers) ListEditorAssets(w http.ResponseWriter, r *http.Request) {
	if !h.editorReady(w) {
		return
	}
	assets, err := h.editorAssets.List(r.Context(), defaultEditorListLimit)
	if err != nil {
		internalError(w, "list editor assets", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": assets})
}

func (h *Handlers) GetEditorAsset(w http.ResponseWriter, r *http.Request) {
	asset, ok := h.loadEditorAsset(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (h *Handlers) GetEditorAssetMedia(w http.ResponseWriter, r *http.Request) {
	asset, ok := h.loadEditorAsset(w, r)
	if !ok {
		return
	}
	h.streamStorageKey(w, r, "video/mp4", asset.MediaKey)
}

func (h *Handlers) CreateEditorProject(w http.ResponseWriter, r *http.Request) {
	if !h.editorReady(w) {
		return
	}
	var req createEditorProjectRequest
	if r.ContentLength != 0 {
		if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
			writeError(w, http.StatusBadRequest, "invalid project JSON")
			return
		}
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Untitled"
	}
	plan := timelineplan.DefaultDocument()
	raw, err := json.Marshal(plan)
	if err != nil {
		internalError(w, "encode default timeline", err)
		return
	}
	p := &timelineplan.Project{Title: title, Status: timelineplan.StatusDraft, Plan: raw}
	if err := h.editorProjects.Create(r.Context(), p); err != nil {
		internalError(w, "create editor project", err)
		return
	}
	if err := h.writeEditorPlanArtifact(p.ID, plan); err != nil {
		internalError(w, "write default timeline", err)
		return
	}
	writeJSON(w, http.StatusCreated, editorProjectJSON(*p, plan))
}

func (h *Handlers) ListEditorProjects(w http.ResponseWriter, r *http.Request) {
	if !h.editorReady(w) {
		return
	}
	projects, err := h.editorProjects.List(r.Context(), defaultEditorListLimit)
	if err != nil {
		internalError(w, "list editor projects", err)
		return
	}
	out := make([]map[string]any, 0, len(projects))
	for _, p := range projects {
		out = append(out, editorProjectSummary(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": out})
}

func (h *Handlers) GetEditorProject(w http.ResponseWriter, r *http.Request) {
	p, plan, ok := h.loadEditorProject(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, editorProjectJSON(p, plan))
}

func (h *Handlers) GetEditorPlan(w http.ResponseWriter, r *http.Request) {
	_, plan, ok := h.loadEditorProject(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handlers) PutEditorPlan(w http.ResponseWriter, r *http.Request) {
	p, _, ok := h.loadEditorProject(w, r)
	if !ok {
		return
	}
	if p.Status == timelineplan.StatusRendering {
		writeError(w, http.StatusConflict, "project is rendering")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read timeline")
		return
	}
	plan, err := timelineplan.Decode(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plan.UpdatedAt = time.Now().UTC()
	if err := plan.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.editorPlanMu.Lock()
	defer h.editorPlanMu.Unlock()
	if err := h.editorProjects.SetPlan(r.Context(), p.ID, plan); err != nil {
		internalError(w, "save timeline", err)
		return
	}
	if err := h.writeEditorPlanArtifact(p.ID, plan); err != nil {
		internalError(w, "write timeline artifact", err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handlers) PreviewEditorPlan(w http.ResponseWriter, r *http.Request) {
	_, plan, ok := h.loadEditorProject(w, r)
	if !ok {
		return
	}
	var req editorPreviewRequest
	if err := decodeSingleJSONBody(w, r, &req, false); err != nil {
		writeError(w, http.StatusBadRequest, "invalid preview JSON")
		return
	}
	writeJSON(w, http.StatusOK, timelineplan.Evaluate(plan, req.Time))
}

func (h *Handlers) StartEditorRender(w http.ResponseWriter, r *http.Request) {
	h.editorPlanMu.Lock()
	defer h.editorPlanMu.Unlock()
	p, plan, ok := h.loadEditorProject(w, r)
	if !ok {
		return
	}
	if err := plan.ValidateForRender(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if p.Status == timelineplan.StatusRendering {
		writeError(w, http.StatusConflict, "project is already rendering")
		return
	}
	fp, err := timelineplan.Fingerprint(plan)
	if err != nil {
		internalError(w, "fingerprint timeline", err)
		return
	}
	task, err := tasks.NewRenderTimelineTask(p.ID, fp)
	if err != nil {
		internalError(w, "build timeline render task", err)
		return
	}
	previous, _, err := h.readEditorRenderState(p.ID)
	if err != nil {
		internalError(w, "load previous editor render state", err)
		return
	}
	state := timelineplan.RenderState{
		ProjectID:   p.ID,
		AttemptID:   uuid.New(),
		Status:      timelineplan.StatusRendering,
		Fingerprint: fp,
		VideoKey:    previous.VideoKey,
		CoverKey:    previous.CoverKey,
		ResultKey:   previous.ResultKey,
		UpdatedAt:   time.Now().UTC(),
	}
	_, err = h.queue.EnqueueWithTransition(task, func(decision error) error {
		if decision != nil {
			return nil
		}
		if err := h.editorProjects.UpdateStatus(r.Context(), p.ID, timelineplan.StatusRendering, ""); err != nil {
			return err
		}
		return h.writeEditorRenderState(state)
	}, asynq.MaxRetry(0), asynq.Unique(editorRenderUniqueTTL))
	if err != nil {
		if errors.Is(err, asynq.ErrDuplicateTask) {
			writeError(w, http.StatusConflict, "a render is already queued for this project")
			return
		}
		internalError(w, "enqueue timeline render", err)
		return
	}
	writeJSON(w, http.StatusAccepted, state)
}

func (h *Handlers) GetEditorRender(w http.ResponseWriter, r *http.Request) {
	p, ok := h.loadEditorProjectOnly(w, r)
	if !ok {
		return
	}
	state, exists, err := h.readEditorRenderState(p.ID)
	if err != nil {
		internalError(w, "read editor render state", err)
		return
	}
	if !exists {
		writeJSON(w, http.StatusOK, timelineplan.RenderState{ProjectID: p.ID, Status: timelineplan.StatusDraft})
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (h *Handlers) GetEditorRenderVideo(w http.ResponseWriter, r *http.Request) {
	h.streamEditorRenderArtifact(w, r, "video/mp4", func(state timelineplan.RenderState) string { return state.VideoKey })
}

func (h *Handlers) GetEditorRenderCover(w http.ResponseWriter, r *http.Request) {
	h.streamEditorRenderArtifact(w, r, "image/jpeg", func(state timelineplan.RenderState) string { return state.CoverKey })
}

func (h *Handlers) streamEditorRenderArtifact(w http.ResponseWriter, r *http.Request, contentType string, keyFn func(timelineplan.RenderState) string) {
	p, ok := h.loadEditorProjectOnly(w, r)
	if !ok {
		return
	}
	state, exists, err := h.readEditorRenderState(p.ID)
	if err != nil {
		internalError(w, "read editor render state", err)
		return
	}
	if !exists || keyFn(state) == "" {
		writeError(w, http.StatusNotFound, "editor render is not ready")
		return
	}
	key := keyFn(state)
	if key == "" {
		writeError(w, http.StatusNotFound, "editor render artifact not found")
		return
	}
	h.streamStorageKey(w, r, contentType, key)
}

func (h *Handlers) loadEditorAsset(w http.ResponseWriter, r *http.Request) (mediaassets.Asset, bool) {
	if !h.editorReady(w) {
		return mediaassets.Asset{}, false
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid asset id")
		return mediaassets.Asset{}, false
	}
	asset, err := h.editorAssets.Get(r.Context(), id)
	if errors.Is(err, mediaassets.ErrNotFound) {
		writeError(w, http.StatusNotFound, "editor asset not found")
		return mediaassets.Asset{}, false
	}
	if err != nil {
		internalError(w, "load editor asset", err)
		return mediaassets.Asset{}, false
	}
	return asset, true
}

func (h *Handlers) loadEditorProject(w http.ResponseWriter, r *http.Request) (timelineplan.Project, timelineplan.Document, bool) {
	p, ok := h.loadEditorProjectOnly(w, r)
	if !ok {
		return timelineplan.Project{}, timelineplan.Document{}, false
	}
	plan := timelineplan.DefaultDocument()
	if len(p.Plan) > 0 {
		decoded, err := timelineplan.Decode(p.Plan)
		if err != nil {
			internalError(w, "decode editor plan", err)
			return timelineplan.Project{}, timelineplan.Document{}, false
		}
		plan = decoded
	}
	return p, plan, true
}

func (h *Handlers) loadEditorProjectOnly(w http.ResponseWriter, r *http.Request) (timelineplan.Project, bool) {
	if !h.editorReady(w) {
		return timelineplan.Project{}, false
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return timelineplan.Project{}, false
	}
	p, err := h.editorProjects.Get(r.Context(), id)
	if errors.Is(err, timelineplan.ErrNotFound) {
		writeError(w, http.StatusNotFound, "editor project not found")
		return timelineplan.Project{}, false
	}
	if err != nil {
		internalError(w, "load editor project", err)
		return timelineplan.Project{}, false
	}
	return p, true
}

func (h *Handlers) writeEditorPlanArtifact(id uuid.UUID, plan timelineplan.Document) error {
	b, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return h.storage.Put(timelineplan.PlanKey(id), bytes.NewReader(append(b, '\n')))
}

func (h *Handlers) writeEditorRenderState(state timelineplan.RenderState) error {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return h.storage.Put(timelineplan.RenderStateKey(state.ProjectID), bytes.NewReader(append(b, '\n')))
}

func (h *Handlers) readEditorRenderState(id uuid.UUID) (timelineplan.RenderState, bool, error) {
	rc, err := h.storage.Open(timelineplan.RenderStateKey(id))
	if err != nil {
		if storageNotExist(err) {
			return timelineplan.RenderState{}, false, nil
		}
		return timelineplan.RenderState{}, false, err
	}
	defer rc.Close()
	var state timelineplan.RenderState
	if err := json.NewDecoder(rc).Decode(&state); err != nil {
		return timelineplan.RenderState{}, false, err
	}
	return state, true, nil
}

func editorProjectJSON(p timelineplan.Project, plan timelineplan.Document) map[string]any {
	return map[string]any{
		"id":             p.ID,
		"title":          p.Title,
		"status":         p.Status,
		"failure_reason": p.FailureReason,
		"plan":           plan,
		"created_at":     p.CreatedAt,
		"updated_at":     p.UpdatedAt,
	}
}

func editorProjectSummary(p timelineplan.Project) map[string]any {
	return map[string]any{
		"id":             p.ID,
		"title":          p.Title,
		"status":         p.Status,
		"failure_reason": p.FailureReason,
		"created_at":     p.CreatedAt,
		"updated_at":     p.UpdatedAt,
	}
}

type badRequestError struct{ msg string }

func (e badRequestError) Error() string { return e.msg }

func errBadRequest(msg string) error { return badRequestError{msg: msg} }

func isBadRequest(err error) bool {
	var br badRequestError
	return errors.As(err, &br)
}
