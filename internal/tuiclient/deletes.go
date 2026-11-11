package tuiclient

import (
	"context"
	"net/http"
	"net/url"
)

// DeleteRenderVideo removes one finished video with its cover and caption.
func (c *Client) DeleteRenderVideo(ctx context.Context, jobID, variant, name string) error {
	return c.doJSON(ctx, http.MethodDelete, jobPath(jobID)+"/renders/"+url.PathEscape(variant)+"/videos/"+url.PathEscape(name), nil, nil)
}

// DeleteStreamJob removes a stream with its source and every render.
func (c *Client) DeleteStreamJob(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/stream-jobs/"+url.PathEscape(id), nil, nil)
}

// DeleteStreamVideo removes one rendered stream clip.
func (c *Client) DeleteStreamVideo(ctx context.Context, id, variant, clipID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/stream-jobs/"+url.PathEscape(id)+"/renders/"+url.PathEscape(variant)+"/videos/"+url.PathEscape(clipID), nil, nil)
}
