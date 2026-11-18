package tuiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

// RenderVersion is one finished render of a variant, as Versiones lists it in Mis vídeos.
// PlanID and Rounds name the approved plan of a Full Demo version.
type RenderVersion struct {
	Revision   string          `json:"revision"`
	CreatedAt  time.Time       `json:"created_at"`
	Current    bool            `json:"current"`
	Videos     []string        `json:"videos"`
	SegmentIDs []string        `json:"segment_ids,omitempty"`
	Edit       json.RawMessage `json:"edit"`
	Music      *ShortMusic     `json:"music,omitempty"`
	PlanID     string          `json:"plan_id,omitempty"`
	Rounds     []int           `json:"rounds,omitempty"`
}

func renderVersionsPath(jobID, variant string) string {
	return jobPath(jobID) + "/renders/" + url.PathEscape(variant) + "/versions"
}

// RenderVersions lists every finished version of a job's variant, newest first.
func (c *Client) RenderVersions(ctx context.Context, jobID, variant string) ([]RenderVersion, error) {
	var out struct {
		Versions []RenderVersion `json:"versions"`
	}
	err := c.getJSON(ctx, renderVersionsPath(jobID, variant), &out)
	return out.Versions, err
}

// DeleteRenderVersion deletes one version that is not the current one.
func (c *Client) DeleteRenderVersion(ctx context.Context, jobID, variant, revision string) error {
	return c.doJSON(ctx, http.MethodDelete, renderVersionsPath(jobID, variant)+"/"+url.PathEscape(revision), nil, nil)
}

// SetCaptureRetention sets how many rendered Full Demos keep their captures and
// returns the bytes it freed from the older ones.
func (c *Client) SetCaptureRetention(ctx context.Context, keep int) (int64, error) {
	var out freedBytes
	err := c.doJSON(ctx, http.MethodPut, "/api/storage/capture-retention", map[string]int{"keep": keep}, &out)
	return out.FreedBytes, err
}
