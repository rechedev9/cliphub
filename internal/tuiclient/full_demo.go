package tuiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// Full Demo DTOs remain raw at this dependency-light transport boundary. The
// caller validates the versioned recapplan contract; the server owns planning.
func fullDemoJobPath(id string) (string, error) {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed == uuid.Nil || parsed.String() != id {
		return "", fmt.Errorf("invalid job UUID")
	}
	return "/api/jobs/" + id, nil
}

func (c *Client) PlanFullDemo(ctx context.Context, id string, options json.RawMessage) (json.RawMessage, error) {
	root, err := fullDemoJobPath(id)
	if err != nil {
		return nil, err
	}
	var result json.RawMessage
	err = c.doJSON(ctx, http.MethodPost, root+"/full-demo/plan", struct {
		Options json.RawMessage `json:"options"`
	}{options}, &result)
	return result, err
}

func (c *Client) GetFullDemoPlan(ctx context.Context, id string) (json.RawMessage, error) {
	root, err := fullDemoJobPath(id)
	if err != nil {
		return nil, err
	}
	var result json.RawMessage
	err = c.getJSON(ctx, root+"/full-demo/plan", &result)
	return result, err
}

func (c *Client) GetFullDemoEvidence(ctx context.Context, id, document string) (json.RawMessage, error) {
	root, err := fullDemoJobPath(id)
	if err != nil {
		return nil, err
	}
	switch document {
	case "approved", "effective", "audio", "loudness", "delivery":
	default:
		return nil, fmt.Errorf("unknown Full Demo evidence document")
	}
	var result json.RawMessage
	err = c.getJSON(ctx, root+"/renders/gameplay-pov-60/full-demo/"+document, &result)
	return result, err
}

func (c *Client) GenerateFullDemo(ctx context.Context, id string, edit json.RawMessage) (EnqueueResponse, error) {
	root, err := fullDemoJobPath(id)
	if err != nil {
		return EnqueueResponse{}, err
	}
	var result EnqueueResponse
	err = c.doJSON(ctx, http.MethodPost, root+"/generate", struct {
		Preset     string          `json:"preset"`
		SegmentIDs []string        `json:"segment_ids"`
		Edit       json.RawMessage `json:"edit"`
	}{"gameplay-pov-60", []string{}, edit}, &result)
	return result, err
}

func (c *Client) UploadFullDemoAsset(ctx context.Context, filePath string, provenance json.RawMessage) (json.RawMessage, error) {
	config, err := json.Marshal(struct {
		Provenance json.RawMessage `json:"provenance"`
	}{provenance})
	if err != nil {
		return nil, err
	}
	var result json.RawMessage
	err = c.uploadMultipart(ctx, "/api/editor/assets", "video", filePath, string(config), &result)
	return result, err
}
