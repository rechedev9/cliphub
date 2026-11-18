package tuiclient

import (
	"context"
	"encoding/json"
	"net/url"
	"time"
)

// HorizontalDraft is GET /api/jobs/{id}/renders/{variant}/horizontal: the 16:9 summary of a Short's plays
// (Crear horizontal), the 9:16 options it drops and whether generate_short would record any play again.
type HorizontalDraft struct {
	Revision   string          `json:"revision"`
	CreatedAt  time.Time       `json:"created_at"`
	Preset     string          `json:"preset"`
	SegmentIDs []string        `json:"segment_ids"`
	Edit       json.RawMessage `json:"edit"`
	Music      *ShortMusic     `json:"music,omitempty"`
	Dropped    []string        `json:"dropped"`
	Capture    struct {
		Needed            bool     `json:"needed"`
		MissingSegmentIDs []string `json:"missing_segment_ids"`
	} `json:"capture"`
}

// HorizontalDraft reads the horizontal version of a finished Short; it creates nothing.
func (c *Client) HorizontalDraft(ctx context.Context, jobID, variant string) (HorizontalDraft, error) {
	var out HorizontalDraft
	err := c.getJSON(ctx, jobPath(jobID)+"/renders/"+url.PathEscape(variant)+"/horizontal", &out)
	return out, err
}
