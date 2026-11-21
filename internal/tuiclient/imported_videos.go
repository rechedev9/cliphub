package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// ImportedVideo is one Short downloaded from the retired ClipHub cloud, listed in Mis vídeos.
type ImportedVideo struct {
	CloudJobID string    `json:"cloud_job_id"`
	Name       string    `json:"name"`
	Title      string    `json:"title"`
	LocalJobID string    `json:"local_job_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ImportedVideoPath is the orchestrator and web path that serves an imported video's MP4.
func ImportedVideoPath(cloudJobID, name string) string {
	return "/api/imported-videos/" + url.PathEscape(cloudJobID) + "/" + url.PathEscape(name)
}

// ListImportedVideos returns the videos imported from the retired cloud.
func (c *Client) ListImportedVideos(ctx context.Context) ([]ImportedVideo, error) {
	var out struct {
		Videos []ImportedVideo `json:"videos"`
	}
	err := c.getJSON(ctx, "/api/imported-videos", &out)
	return out.Videos, err
}

// DeleteImportedVideo removes one imported video from the PC.
func (c *Client) DeleteImportedVideo(ctx context.Context, cloudJobID, name string) error {
	return c.doJSON(ctx, http.MethodDelete, ImportedVideoPath(cloudJobID, name), nil, nil)
}
