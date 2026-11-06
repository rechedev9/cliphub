package tuiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// BatchStatusMaxItems is the server cap of one GET /api/jobs/batch-status call.
const BatchStatusMaxItems = 100

// BatchItem names one job and render variant to read.
type BatchItem struct {
	JobID   string
	Variant string
}

// BatchStatusRow is one row of GET /api/jobs/batch-status: a nil Job means the job
// is gone, a nil Render means that variant was never rendered.
type BatchStatusRow struct {
	JobID   string        `json:"job_id"`
	Variant string        `json:"variant"`
	Job     *JobStatus    `json:"job"`
	Render  *RenderVideos `json:"render"`
	Error   *struct {
		Code string `json:"code"`
	} `json:"error,omitempty"`
}

// RenderRequest is the body of POST /api/jobs/{id}/renders/{variant}: a nil Music
// renders without music, and Edit is the effective edit GetRenderVideos returned.
type RenderRequest struct {
	Music      *ShortMusic     `json:"music,omitempty"`
	SegmentIDs []string        `json:"segment_ids,omitempty"`
	Edit       json.RawMessage `json:"edit,omitempty"`
}

// BatchStatus reads up to BatchStatusMaxItems job and variant pairs in one call.
func (c *Client) BatchStatus(ctx context.Context, items []BatchItem) ([]BatchStatusRow, error) {
	if len(items) == 0 || len(items) > BatchStatusMaxItems {
		return nil, errors.New("batch status needs 1 to 100 items")
	}
	pairs := make([]string, 0, len(items))
	for _, item := range items {
		pairs = append(pairs, item.JobID+":"+item.Variant)
	}
	var out struct {
		Items []BatchStatusRow `json:"items"`
	}
	err := c.getJSON(ctx, "/api/jobs/batch-status?items="+url.QueryEscape(strings.Join(pairs, ",")), &out)
	return out.Items, err
}

// Rerender renders a recorded variant again without a new capture, as Reintentar and Cambiar música do.
func (c *Client) Rerender(ctx context.Context, id, variant string, req RenderRequest) (EnqueueResponse, error) {
	var out EnqueueResponse
	err := c.doJSON(ctx, http.MethodPost, renderPath(id, variant), req, &out)
	return out, err
}
