package tuiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// StreamCreateResponse is the body of POST /api/stream-jobs.
type StreamCreateResponse struct {
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Probe  SourceProbe `json:"probe"`
}

// ListStreamJobs returns the most recent stream-clip jobs, newest first.
func (c *Client) ListStreamJobs(ctx context.Context, limit int) ([]StreamJob, error) {
	if limit <= 0 {
		limit = 50
	}
	var out struct {
		Jobs []StreamJob `json:"jobs"`
	}
	if err := c.getJSON(ctx, "/api/stream-jobs"+query(map[string]string{"limit": strconv.Itoa(limit)}), &out); err != nil {
		return nil, err
	}
	return out.Jobs, nil
}

// GetStreamJob returns one stream-clip job.
func (c *Client) GetStreamJob(ctx context.Context, id string) (StreamJob, error) {
	var job StreamJob
	err := c.getJSON(ctx, "/api/stream-jobs/"+id, &job)
	return job, err
}

// CreateStreamJobUpload uploads a streamer MP4 file. The job starts in
// "uploaded" and becomes "ready" once probed.
func (c *Client) CreateStreamJobUpload(ctx context.Context, videoPath, title string) (StreamCreateResponse, error) {
	config := configJSON(map[string]string{"title": title})
	var out StreamCreateResponse
	err := c.uploadMultipart(ctx, upload{Path: "/api/stream-jobs", FileField: "video", FilePath: videoPath, Fields: configField(config)}, &out)
	return out, err
}

// GetStreamEditPlan returns the job's edit plan (or the default plan when none
// has been saved yet).
func (c *Client) GetStreamEditPlan(ctx context.Context, id string) (StreamEditPlan, error) {
	var plan StreamEditPlan
	err := c.getJSON(ctx, "/api/stream-jobs/"+id+"/edit-plan", &plan)
	return plan, err
}

// PutStreamEditPlan saves an edit plan and returns the normalized result.
func (c *Client) PutStreamEditPlan(ctx context.Context, id string, plan StreamEditPlan) (StreamEditPlan, error) {
	var out StreamEditPlan
	err := c.doJSON(ctx, http.MethodPut, "/api/stream-jobs/"+id+"/edit-plan", plan, &out)
	return out, err
}

// StartStreamRender enqueues a stream render. Valid only when the job is "ready"
// or "rendered"; the default variant is StreamDefaultVariant.
func (c *Client) StartStreamRender(ctx context.Context, id, variant string) (EnqueueResponse, error) {
	if variant == "" {
		variant = StreamDefaultVariant
	}
	var out EnqueueResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/stream-jobs/"+id+"/renders/"+variant, nil, &out)
	return out, err
}

// GetStreamRender returns the render state of a stream job variant.
func (c *Client) GetStreamRender(ctx context.Context, id, variant string) (StreamRenderState, error) {
	if variant == "" {
		variant = StreamDefaultVariant
	}
	var state StreamRenderState
	err := c.getJSON(ctx, "/api/stream-jobs/"+id+"/renders/"+variant, &state)
	return state, err
}

// CreateStreamJobFromURL creates a stream-clip job that downloads its source
// from a URL (requires yt-dlp on the orchestrator host). The job starts in
// "acquiring". kind is "vod" or "clip"; empty lets Studio infer it from the URL.
func (c *Client) CreateStreamJobFromURL(ctx context.Context, sourceURL, title, kind string) (StreamCreateResponse, error) {
	body := map[string]string{"source_url": sourceURL}
	if title != "" {
		body["title"] = title
	}
	if kind != "" {
		body["source_kind"] = kind
	}
	var out StreamCreateResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/stream-jobs", body, &out)
	return out, err
}

// StreamPlay is one run of the streamer's kills found in the kill feed (internal/vodplays.Play).
type StreamPlay struct {
	ID           string    `json:"id"`
	Label        string    `json:"label"`
	StartSeconds float64   `json:"start_seconds"`
	EndSeconds   float64   `json:"end_seconds"`
	KillSeconds  []float64 `json:"kill_seconds"`
	Score        int       `json:"score"`
	// Kind is "kill" or "voice_peak"; empty on plays saved before voice peaks.
	Kind         string    `json:"kind,omitempty"`
	VoiceSeconds []float64 `json:"voice_seconds,omitempty"`
	VoiceDB      float64   `json:"voice_db,omitempty"`
	Marked       bool      `json:"marked,omitempty"`
}

// StreamVoiceScan is what the voice peak scan did (internal/httpapi voiceScan).
type StreamVoiceScan struct {
	Enabled     bool   `json:"enabled"`
	Sensitivity int    `json:"sensitivity"`
	Status      string `json:"status"`
	Peaks       int    `json:"peaks"`
}

// StreamPlayScanOptions is the optional body of the scan start; nil fields keep the Studio defaults.
type StreamPlayScanOptions struct {
	Voice            *bool `json:"voice,omitempty"`
	VoiceSensitivity int   `json:"voice_sensitivity,omitempty"`
}

// StreamPlayScan mirrors GET /api/stream-jobs/{id}/plays.
type StreamPlayScan struct {
	Status   string       `json:"status"`
	Progress float64      `json:"progress"`
	Plays    []StreamPlay `json:"plays"`
	Error    string       `json:"error,omitempty"`
	// Voice is absent on a scan saved before voice peaks.
	Voice *StreamVoiceScan `json:"voice,omitempty"`
}

// StreamSummary mirrors GET /api/stream-jobs/{id}/summary: the 16:9 video of every play.
type StreamSummary struct {
	Status          string  `json:"status"`
	Progress        float64 `json:"progress"`
	PlayCount       int     `json:"play_count,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	Error           string  `json:"error,omitempty"`
	RenderedAt      string  `json:"rendered_at,omitempty"`
}

// StreamVariant is one stream layout from GET /api/stream-variants.
type StreamVariant struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
	FullFrame   bool   `json:"full_frame"`
	OutputWidth int    `json:"output_width"`
	// GameOutputHeight plus FaceOutputHeight is the output height.
	GameOutputHeight int `json:"game_output_height"`
	FaceOutputHeight int `json:"face_output_height,omitempty"`
}

// GetStreamPlays reads the kill-feed scan of a stream job.
func (c *Client) GetStreamPlays(ctx context.Context, id string) (StreamPlayScan, error) {
	var out StreamPlayScan
	err := c.getJSON(ctx, "/api/stream-jobs/"+id+"/plays", &out)
	return out, err
}

// StartStreamPlayScan starts the kill-feed scan; a running scan is left alone.
func (c *Client) StartStreamPlayScan(ctx context.Context, id string, opts StreamPlayScanOptions) (StreamPlayScan, error) {
	var out StreamPlayScan
	err := c.doJSON(ctx, http.MethodPost, "/api/stream-jobs/"+id+"/plays", opts, &out)
	return out, err
}

// GetStreamSummary reads the state of the 16:9 summary of a stream job.
func (c *Client) GetStreamSummary(ctx context.Context, id string) (StreamSummary, error) {
	var out StreamSummary
	err := c.getJSON(ctx, "/api/stream-jobs/"+id+"/summary", &out)
	return out, err
}

// StreamSummaryStart picks the plays of a 16:9 summary; empty PlayIDs means every play.
type StreamSummaryStart struct {
	ID      string
	PlayIDs []string
}

// StartStreamSummary renders the chosen plays into one 16:9 video.
func (c *Client) StartStreamSummary(ctx context.Context, in StreamSummaryStart) (StreamSummary, error) {
	var body any
	if len(in.PlayIDs) > 0 {
		body = map[string][]string{"play_ids": in.PlayIDs}
	}
	var out StreamSummary
	err := c.doJSON(ctx, http.MethodPost, "/api/stream-jobs/"+in.ID+"/summary", body, &out)
	return out, err
}

// ListStreamVariants returns the stream layouts the render accepts.
func (c *Client) ListStreamVariants(ctx context.Context) ([]StreamVariant, error) {
	var out struct {
		Variants []StreamVariant `json:"variants"`
	}
	err := c.getJSON(ctx, "/api/stream-variants", &out)
	return out.Variants, err
}

// GetStreamEditPlanDocument returns the edit plan as raw fields, so a caller
// that changes some of them saves the rest (banners, music, effects) unchanged.
func (c *Client) GetStreamEditPlanDocument(ctx context.Context, id string) (map[string]json.RawMessage, error) {
	var out map[string]json.RawMessage
	err := c.getJSON(ctx, "/api/stream-jobs/"+id+"/edit-plan", &out)
	return out, err
}

// StreamPlanPut is a raw edit plan to save. A non-empty ExpectedUpdatedAt makes
// Studio refuse the save (409) when another save landed after that revision.
type StreamPlanPut struct {
	ID                string
	Plan              map[string]json.RawMessage
	ExpectedUpdatedAt string
}

// PutStreamEditPlanDocument saves a raw edit plan and returns the saved one.
func (c *Client) PutStreamEditPlanDocument(ctx context.Context, in StreamPlanPut) (map[string]json.RawMessage, error) {
	path := "/api/stream-jobs/" + in.ID + "/edit-plan"
	if in.ExpectedUpdatedAt != "" {
		path += "?expected_updated_at=" + url.QueryEscape(in.ExpectedUpdatedAt)
	}
	var out map[string]json.RawMessage
	err := c.doJSON(ctx, http.MethodPut, path, in.Plan, &out)
	return out, err
}

// StartStreamRenderOfPlan renders variant only if the edit plan is still the
// one saved at planUpdatedAt (RFC 3339), so a later edit cannot slip in.
func (c *Client) StartStreamRenderOfPlan(ctx context.Context, id, variant, planUpdatedAt string) (EnqueueResponse, error) {
	var out EnqueueResponse
	body := map[string]string{"expected_edit_plan_updated_at": planUpdatedAt}
	err := c.doJSON(ctx, http.MethodPost, "/api/stream-jobs/"+id+"/renders/"+variant, body, &out)
	return out, err
}
