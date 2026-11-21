package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

type ImportedVideo struct {
	CloudJobID string    `json:"cloud_job_id"`
	Name       string    `json:"name"`
	Title      string    `json:"title"`
	LocalJobID string    `json:"local_job_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func ImportedVideoPath(cloudJobID, name string) string {
	return "/api/imported-videos/" + url.PathEscape(cloudJobID) + "/" + url.PathEscape(name)
}

func (c *Client) ListImportedVideos(ctx context.Context) ([]ImportedVideo, error) {
	var out struct {
		Videos []ImportedVideo `json:"videos"`
	}
	err := c.getJSON(ctx, "/api/imported-videos", &out)
	return out.Videos, err
}

func (c *Client) DeleteImportedVideo(ctx context.Context, cloudJobID, name string) error {
	return c.doJSON(ctx, http.MethodDelete, ImportedVideoPath(cloudJobID, name), nil, nil)
}
